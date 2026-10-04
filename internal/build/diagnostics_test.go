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
	for _, raw := range []string{"missing front matter", "---\ntitle: [bad\n---\n", "---\ntitle: T\nslug: BAD\npublishedAt: '2026-01-02T00:00:00Z'\ntopics: [notes]\n---\n"} {
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

func TestRunSourceDiagnosticExitCodes(t *testing.T) {
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
	const article = "---\ntitle: T\nslug: test\npublishedAt: '2026-01-02T00:00:00Z'\ntopics: [protobuf]\n---\nBody\n"
	for _, tc := range []struct {
		name, file, body, code string
		exit                   int
	}{
		{"bad YAML", "posts/2026/example/ja.md", "---\ntitle: [bad\n---\n", "E_SCHEMA", 1},
		{"unknown field", "posts/2026/example/ja.md", strings.Replace(article, "---\nBody", "summray: unknown\n---\nBody", 1), "E_SCHEMA", 1},
		{"invalid UTF-8", "posts/2026/example/ja.md", article + "\xff", "E_SCHEMA", 1},
		{"invalid prose", "home/ja.md", "Body\xff\n", "E_SCHEMA", 1},
		{"disabled locale", "posts/2026/example/en.md", article, "E_TRANSLATION_GROUP", 1},
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
