package build

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ymmt2005/cfgb/internal/frontmatter"
)

func TestContentErrorClassification(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"missing front matter", "---\ntitle: [bad\n---\n"} {
		_, _, err := frontmatter.ReadArticle(strings.NewReader(raw))
		got := contentError(fmt.Errorf("article.md: %w", err))
		if got.Code != 1 || !strings.Contains(got.Error(), "E_SCHEMA") || !strings.Contains(got.Error(), "article.md") {
			t.Fatalf("validation error = %+v", got)
		}
	}
	ioErr := &os.PathError{Op: "open", Path: filepath.Join(t.TempDir(), "missing.md"), Err: os.ErrNotExist}
	got := contentError(fmt.Errorf("source: %w", ioErr))
	if got.Code != 3 || !errors.Is(got, os.ErrNotExist) {
		t.Fatalf("I/O error = %+v", got)
	}
}

// Fail unexpected tool operations so error-path tests cannot install or render.
func stubBuildProbes(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("toolchain probe stubs use shell scripts")
	}
	tools := t.TempDir()
	for name, script := range map[string]string{
		"node": "#!/bin/sh\n[ \"$1\" = -p ] || exit 99\nprintf '24.21.0\\n'\n",
		"npm":  "#!/bin/sh\n[ \"$1\" = -v ] || exit 99\nprintf '12.2.0\\n'\n",
	} {
		if err := os.WriteFile(filepath.Join(tools, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CFGB_PACKAGE_MANAGER", "npm")
}

func TestRunSourceDiagnosticExitCodes(t *testing.T) {
	stubBuildProbes(t)
	const article = "---\ntitle: T\nslug: test\npublishedAt: '2026-01-02T00:00:00Z'\ntopics: [protobuf]\n---\nBody\n"
	for _, tc := range []struct {
		name, file, body, code string
		exit                   int
	}{
		{"invalid article encoding", "posts/2026/example/ja.md", article + "\xff", "E_SCHEMA", 1},
		{"invalid prose encoding", "home/ja.md", "Body\xff\n", "E_SCHEMA", 1},
		{"bad YAML", "posts/2026/example/ja.md", "---\ntitle: [bad\n---\n", "E_SCHEMA", 1},
		{"unknown field", "posts/2026/example/ja.md", strings.Replace(article, "---\nBody", "summray: unknown\n---\nBody", 1), "E_SCHEMA", 1},
		{"disabled locale", "posts/2026/example/en.md", article, "E_TRANSLATION_GROUP", 1},
		{"nested variant", "posts/2026/example/nested/ja.md", article, "E_TRANSLATION_GROUP", 1},
		{"invalid topics", "", "protobuf: invalid\n", "E_SCHEMA", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, repo := testRepo(t)
			target := filepath.Join(repo, "src", "content", filepath.FromSlash(tc.file))
			if tc.file == "" {
				target = filepath.Join(repo, "src", "data", "topics.yaml")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(repo, "dist")
			err := Run(Options{Dir: repo, Out: out})
			var exit *ExitError
			if !errors.As(err, &exit) || exit.Code != tc.exit || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("Run = %v, want exit %d and %s", err, tc.exit, tc.code)
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("failed build left output: %v", err)
			}
		})
	}
	t.Run("missing topics is I/O", func(t *testing.T) {
		_, repo := testRepo(t)
		if err := os.Remove(filepath.Join(repo, "src", "data", "topics.yaml")); err != nil {
			t.Fatal(err)
		}
		err := Run(Options{Dir: repo, Out: filepath.Join(repo, "dist")})
		var exit *ExitError
		if !errors.As(err, &exit) || exit.Code != 3 || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Run I/O = %v", err)
		}
	})
}

func TestRunDirectoryCycleRemovesIncompleteBuild(t *testing.T) {
	stubBuildProbes(t)
	_, repo := testRepo(t)
	if err := os.Symlink(".", filepath.Join(repo, "src", "content", "loop")); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "artifact")
	workspaces := t.TempDir()
	t.Setenv("TMPDIR", workspaces)
	err := Run(Options{Dir: repo, Out: out})
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code != 3 || !strings.Contains(err.Error(), "directory cycle:") {
		t.Fatalf("Run cycle = %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("failed cycle left output: %v", err)
	}
	entries, err := os.ReadDir(workspaces)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed cycle left workspace: %v, %v", entries, err)
	}
}

func TestRunInvalidConfigurationPreservesOutput(t *testing.T) {
	// No toolchain is installed: configuration must fail before probing Node.
	t.Setenv("PATH", t.TempDir())
	for _, body := range []string{
		testConfig + "\nhome: [invalid]\n",
		strings.Replace(testConfig, "Asia/Tokyo", "Invalid/Timezone", 1),
		strings.Replace(testConfig, "defaultLocale: ja", "defaultLocale: en", 1),
		testConfig + "\nsecurity:\n  previewAcess: false\n",
		testConfig + "\nhatena:\n  blogs:\n    - unknown: true\n",
	} {
		repo := t.TempDir()
		if err := os.WriteFile(filepath.Join(repo, "cfgb.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(repo, "dist")
		if err := os.Mkdir(out, 0o755); err != nil {
			t.Fatal(err)
		}
		sentinel := filepath.Join(out, "existing.html")
		if err := os.WriteFile(sentinel, []byte("existing site"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := Run(Options{Dir: repo, Out: out})
		var exit *ExitError
		if !errors.As(err, &exit) || exit.Code != 2 || !strings.Contains(err.Error(), "cfgb.yaml") {
			t.Fatalf("Run = %v, want config exit 2", err)
		}
		if got, err := os.ReadFile(sentinel); err != nil || string(got) != "existing site" {
			t.Fatalf("invalid config removed existing output: %q, %v", got, err)
		}
	}
}

func TestRunSelectedConfigurationDoesNotFallBack(t *testing.T) {
	_, repo := testRepo(t)
	selected := filepath.Join(repo, "settings", "blog.yaml")
	if err := os.MkdirAll(filepath.Dir(selected), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(selected, []byte("schemaVersion: wrong\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(repo, "dist")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(out, "keep.txt")
	if err := os.WriteFile(marker, []byte("existing output"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	for _, tc := range []struct {
		name string
		code int
	}{{"settings/blog.yaml", 2}, {"settings/missing.yaml", 3}} {
		name := tc.name
		err := Run(Options{Dir: repo, Config: name, Out: out})
		var exit *ExitError
		if !errors.As(err, &exit) || exit.Code != tc.code || strings.Contains(err.Error(), "E_TOOLCHAIN") {
			t.Fatalf("selected config %s: %v", name, err)
		}
		if got, err := os.ReadFile(marker); err != nil || string(got) != "existing output" {
			t.Fatalf("invalid selected config removed output: %q, %v", got, err)
		}
	}
}

type progressWriter func([]byte) (int, error)

func (w progressWriter) Write(p []byte) (int, error) { return w(p) }

func TestRunProgressFailureAndCleanupErrors(t *testing.T) {
	stubBuildProbes(t)
	for _, blockedCleanup := range []bool{false, true} {
		t.Run(fmt.Sprintf("blocked cleanup=%t", blockedCleanup), func(t *testing.T) {
			if blockedCleanup && os.Geteuid() == 0 {
				t.Skip("root bypasses directory permissions; this case runs in CI as a normal user")
			}
			_, repo := testRepo(t)
			out := filepath.Join(repo, "dist")
			workspaces := t.TempDir()
			t.Setenv("TMPDIR", workspaces)
			broken := errors.New("progress stream failed")
			t.Cleanup(func() {
				if err := os.Chmod(repo, 0o755); err != nil {
					t.Error(err)
				}
			})
			err := Run(Options{Dir: repo, Out: out, Stdout: progressWriter(func([]byte) (int, error) {
				if blockedCleanup {
					if err := os.Chmod(repo, 0o555); err != nil {
						t.Fatal(err)
					}
				}
				return 0, broken
			})})
			var exit *ExitError
			if !errors.As(err, &exit) || exit.Code != 3 || !errors.Is(err, broken) {
				t.Fatalf("Run lost progress failure: %v", err)
			}
			if blockedCleanup {
				if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), out) {
					t.Fatalf("Run lost cleanup failure/path: %v", err)
				}
			} else if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("incomplete output retained: %v", err)
			}
			entries, err := os.ReadDir(workspaces)
			if err != nil || len(entries) != 0 {
				t.Fatalf("workspace cleanup stopped: %v, %v", entries, err)
			}
		})
	}
}

func TestRunWorkspaceCreationFailureCleansOutput(t *testing.T) {
	stubBuildProbes(t)
	_, repo := testRepo(t)
	out := filepath.Join(repo, "dist")
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	err := Run(Options{Dir: repo, Out: out})
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code != 3 || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Run lost workspace error: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("workspace failure retained incomplete output: %v", err)
	}
}

func TestRunConfigurationReadFailurePreservesOutput(t *testing.T) {
	for _, kind := range []string{"dangling symlink", "symlink loop", "permission"} {
		t.Run(kind, func(t *testing.T) {
			_, repo := testRepo(t)
			t.Setenv("PATH", t.TempDir())
			file := filepath.Join(repo, "cfgb.yaml")
			if kind == "permission" {
				if os.Geteuid() == 0 {
					t.Skip("root bypasses permissions; CI exercises this as a normal user")
				}
				if err := os.Chmod(file, 0); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
				target := "missing.yaml"
				if kind == "symlink loop" {
					target = "cfgb.yaml"
				}
				if err := os.Symlink(target, file); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			out := filepath.Join(repo, "dist")
			if err := os.Mkdir(out, 0o755); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(out, "keep.txt")
			if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
				t.Fatal(err)
			}
			err := Run(Options{Dir: repo, Out: out})
			var exit *ExitError
			if !errors.As(err, &exit) || exit.Code != 3 || strings.Contains(err.Error(), "E_TOOLCHAIN") {
				t.Fatalf("read failure: %v", err)
			}
			if raw, err := os.ReadFile(marker); err != nil || string(raw) != "keep" {
				t.Fatalf("output changed: %q, %v", raw, err)
			}
		})
	}
}
