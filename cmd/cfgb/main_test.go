package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cfgb "github.com/ymmt2005/cfgb"
	"github.com/ymmt2005/cfgb/internal/build"
	"github.com/ymmt2005/cfgb/internal/version"
)

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestCLIOutputErrorsFailIncludingFrameworkHelp(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	broken := errors.New("output unavailable")
	for _, args := range [][]string{{"version"}, {"--version"}, {"build", "--version"}, {"--help"}, {"build", "--help"}, {"help", "build"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stderr bytes.Buffer
			if code := run(args, failingWriter{broken}, &stderr); code != 3 || !strings.Contains(stderr.String(), broken.Error()) {
				t.Fatalf("exit=%d stderr=%q", code, &stderr)
			}
		})
	}
	if code := run([]string{"unknown"}, io.Discard, failingWriter{broken}); code != 3 {
		t.Fatalf("failed diagnostic write: exit=%d", code)
	}
}

func TestVersion(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"version"}, &out, &err); code != 0 {
		t.Fatalf("exit %d: %s", code, err.String())
	}
	want := "cfgb " + version.Version + "\n"
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestGlobalConfigAndBuildFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--config", "settings/site.yaml", "build", "--out", "output"},
		{"build", "--config", "settings/site.yaml", "--out", "output"},
		{"build", "--out=output", "--config=settings/site.yaml"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			calls := 0
			cmd := newRootCommand(func(opts build.Options) error {
				calls++
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				if opts.Dir != cwd || opts.Config != "settings/site.yaml" || opts.Out != "output" {
					t.Fatalf("options = %+v", opts)
				}
				if opts.Stdout != &stdout || opts.Stderr != &stderr {
					t.Fatal("command did not forward its output streams")
				}
				return nil
			})
			cmd.SetArgs(args)
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			if err := cmd.Execute(); err != nil || calls != 1 {
				t.Fatalf("Execute = %v, calls = %d", err, calls)
			}
		})
	}
}

func TestCLIHelpVersionAndUsage(t *testing.T) {
	// Neither help nor version may load configuration or probe Node.
	t.Setenv("PATH", t.TempDir())
	for _, tc := range []struct {
		args []string
		exit int
		text string
	}{
		{[]string{"--version"}, 0, "cfgb " + version.Version},
		{[]string{"-v"}, 0, "cfgb " + version.Version},
		{[]string{"build", "--version"}, 0, "cfgb " + version.Version},
		{[]string{"--config", "missing.yaml", "build", "--version"}, 0, "cfgb " + version.Version},
		{[]string{"--help"}, 0, "--config"},
		{[]string{"build", "--help"}, 0, "--out"},
		{[]string{"help", "build"}, 0, "--config"},
		{nil, 2, "command is required"},
		{[]string{"--config"}, 2, "flag needs an argument"},
		{[]string{"build", "--config="}, 2, "nonempty file path"},
		{[]string{"build", "--out"}, 2, "flag needs an argument"},
		{[]string{"build", "extra"}, 2, "unknown command"},
		{[]string{"version", "extra"}, 2, "unknown command"},
		{[]string{"build", "--unknown"}, 2, "unknown flag"},
		{[]string{"deploy"}, 2, "unknown command"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var out, err bytes.Buffer
			if got := run(tc.args, &out, &err); got != tc.exit {
				t.Fatalf("exit = %d, want %d: stdout=%q stderr=%q", got, tc.exit, &out, &err)
			}
			output := out.String()
			if tc.exit != 0 {
				output = err.String()
				if out.Len() != 0 || strings.Contains(output, "Usage:") {
					t.Fatalf("error was mixed with help: stdout=%q stderr=%q", &out, &err)
				}
			} else if err.Len() != 0 {
				t.Fatalf("success wrote stderr: %q", &err)
			}
			if !strings.Contains(output, tc.text) {
				t.Fatalf("output = %q, want %q", output, tc.text)
			}
		})
	}
}

func TestCLIUsesSelectedConfiguration(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	file := filepath.Join(dir, "alternate.yaml")
	if err := os.WriteFile(file, []byte("schemaVersion: wrong\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"build", "--config", file}, &out, &stderr); code != 2 || !strings.Contains(stderr.String(), "E_SCHEMA") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, &out, &stderr)
	}
}

func TestCLIInvocationDoesNotRetainFlags(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := run([]string{"--version"}, &out, &stderr); code != 0 {
		t.Fatal(code)
	}
	out.Reset()
	stderr.Reset()
	if code := run(nil, &out, &stderr); code != 2 || out.Len() != 0 {
		t.Fatalf("second invocation reused version flag: exit=%d stdout=%q stderr=%q", code, &out, &stderr)
	}
}

func TestUnknownCommand(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"deploy"}, &out, &err); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestEmbeddedAssets(t *testing.T) {
	for _, name := range []string{
		"schemas/cfgb.schema.json",
		"schemas/article.schema.json",
		"prompts/summary-ja-v1.txt",
		"prompts/summary-en-v1.txt",
		"renderer/package.json",
		"renderer/package-lock.json",
		"renderer/pnpm-lock.yaml",
		"renderer/pnpm-workspace.yaml",
		"renderer/astro.config.mjs",
		"renderer/src/content.config.ts",
	} {
		if _, err := cfgb.FS.ReadFile(name); err != nil {
			t.Errorf("missing embedded %s: %v", name, err)
		}
	}
}
