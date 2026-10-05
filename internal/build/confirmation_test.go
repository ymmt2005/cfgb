package build

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func TestRunConfirmsExistingOutputBeforeRemoval(t *testing.T) {
	stubBuildProbes(t)
	for _, kind := range []string{"file", "directory", "symlink", "dangling symlink"} {
		for _, answer := range []struct {
			name, text     string
			force, replace bool
		}{
			{name: "no", text: "no\n"},
			{name: "blank", text: "\n"},
			{name: "EOF"},
			{name: "unrecognized", text: "sure\n"},
			{name: "yes", text: "yes\n", replace: true},
			{name: "y", text: " Y \r\n", replace: true},
			{name: "yes at EOF", text: "yes", replace: true},
			{name: "force", force: true, replace: true},
		} {
			t.Run(kind+"/"+answer.name, func(t *testing.T) {
				_, repo := testRepo(t)
				out := filepath.Join(repo, "dist")
				marker := out
				if kind == "directory" {
					if err := os.Mkdir(out, 0o755); err != nil {
						t.Fatal(err)
					}
					marker = filepath.Join(out, "existing.html")
				} else if strings.Contains(kind, "symlink") {
					marker = filepath.Join(repo, "target")
					if err := os.Symlink("target", out); err != nil {
						t.Skipf("symlinks unavailable: %v", err)
					}
				}
				if kind != "dangling symlink" {
					if err := os.WriteFile(marker, []byte("existing output"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				// Fail immediately after output replacement, before installation/rendering.
				if err := os.Remove(filepath.Join(repo, "src", "data", "topics.yaml")); err != nil {
					t.Fatal(err)
				}
				workspaces := t.TempDir()
				t.Setenv("TMPDIR", workspaces)
				var prompt bytes.Buffer
				var input io.Reader = strings.NewReader(answer.text)
				var stderr io.Writer = &prompt
				if answer.force {
					// Force must neither prompt nor consume stdin.
					input = iotest.ErrReader(errors.New("unexpected confirmation read"))
					stderr = progressWriter(func([]byte) (int, error) {
						t.Fatal("force prompted for confirmation")
						return 0, io.ErrClosedPipe
					})
				}
				err := Run(Options{Dir: repo, Stdin: input, Stderr: stderr, Force: answer.force})
				var exit *ExitError
				wantCode := 1
				if answer.replace {
					wantCode = 3
				}
				if !errors.As(err, &exit) || exit.Code != wantCode {
					t.Fatalf("Run = %v, want exit %d", err, wantCode)
				}
				if !answer.force && (!strings.Contains(prompt.String(), out) || !strings.Contains(prompt.String(), "[y/N]") || !strings.Contains(prompt.String(), "--force")) {
					t.Fatalf("missing target/default/automation instructions: %q", &prompt)
				}
				if answer.replace {
					if !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("build did not proceed to staging: %v", err)
					}
					if _, err := os.Lstat(out); !os.IsNotExist(err) {
						t.Fatalf("failed build retained output: %v", err)
					}
				} else {
					if !strings.Contains(err.Error(), "cancelled") {
						t.Fatalf("missing cancellation diagnostic: %v", err)
					}
					if _, err := os.Lstat(out); err != nil {
						t.Fatalf("cancelled build removed output: %v", err)
					}
					if strings.Contains(kind, "symlink") {
						if target, err := os.Readlink(out); err != nil || target != "target" {
							t.Fatalf("cancelled build changed symlink: %q, %v", target, err)
						}
					}
				}
				if (!answer.replace || kind == "symlink") && kind != "dangling symlink" {
					if data, err := os.ReadFile(marker); err != nil || string(data) != "existing output" {
						t.Fatalf("existing bytes changed: %q, %v", data, err)
					}
				}
				entries, err := os.ReadDir(workspaces)
				if err != nil || len(entries) != 0 {
					t.Fatalf("build retained workspaces: %v, %v", entries, err)
				}
			})
		}
	}
}

func TestRunConfirmationIOFailuresPreserveOutput(t *testing.T) {
	stubBuildProbes(t)
	broken := errors.New("confirmation stream failed")
	for _, stream := range []string{"input", "prompt"} {
		t.Run(stream, func(t *testing.T) {
			_, repo := testRepo(t)
			out := filepath.Join(repo, "dist")
			if err := os.WriteFile(out, []byte("keep"), 0o644); err != nil {
				t.Fatal(err)
			}
			var input io.Reader = strings.NewReader("yes\n")
			var stderr io.Writer = io.Discard
			if stream == "input" {
				input = io.MultiReader(strings.NewReader("yes"), iotest.ErrReader(broken))
			} else {
				stderr = progressWriter(func([]byte) (int, error) { return 0, broken })
			}
			err := Run(Options{Dir: repo, Stdin: input, Stderr: stderr})
			var exit *ExitError
			if !errors.As(err, &exit) || exit.Code != 3 || !errors.Is(err, broken) {
				t.Fatalf("confirmation failure lost code/cause: %v", err)
			}
			if data, err := os.ReadFile(out); err != nil || string(data) != "keep" {
				t.Fatalf("confirmation failure removed output: %q, %v", data, err)
			}
		})
	}
}

func TestRunMissingOutputDoesNotPrompt(t *testing.T) {
	stubBuildProbes(t)
	_, repo := testRepo(t)
	if err := os.Remove(filepath.Join(repo, "src", "data", "topics.yaml")); err != nil {
		t.Fatal(err)
	}
	err := Run(Options{
		Dir:   repo,
		Stdin: iotest.ErrReader(errors.New("unexpected confirmation read")),
		Stderr: progressWriter(func([]byte) (int, error) {
			t.Fatal("missing output prompted for confirmation")
			return 0, io.ErrClosedPipe
		}),
	})
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code != 3 || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("build did not proceed to staging: %v", err)
	}
}

func TestRunOutputInspectionFailurePreservesParent(t *testing.T) {
	stubBuildProbes(t)
	_, repo := testRepo(t)
	parent := filepath.Join(repo, "not-a-directory")
	if err := os.WriteFile(parent, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	var prompt bytes.Buffer
	err := Run(Options{Dir: repo, Out: filepath.Join(parent, "dist"), Stderr: &prompt})
	var exit *ExitError
	var pathErr *os.PathError
	if !errors.As(err, &exit) || exit.Code != 3 || !errors.As(err, &pathErr) || !strings.Contains(err.Error(), "inspect output") {
		t.Fatalf("inspection failure lost code/cause: %v", err)
	}
	if prompt.Len() != 0 {
		t.Fatalf("inspection failure prompted: %q", &prompt)
	}
	if data, err := os.ReadFile(parent); err != nil || string(data) != "keep" {
		t.Fatalf("inspection failure changed parent: %q, %v", data, err)
	}
}
