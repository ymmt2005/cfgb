package frontmatter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadArticlePreservesBodyBytes(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"日本語 😀\r\n", "interior: \ufeff\n", "replacement character: �\n", ""} {
		_, got, err := ReadArticle(strings.NewReader("---\n" + required + "---\n" + body))
		if err != nil || !bytes.Equal(got, []byte(body)) {
			t.Fatalf("body = %q, error = %v", got, err)
		}
	}
}

func TestSourceValidationErrors(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"no front matter\n", "---\ntitle: T\n",
		"---\n" + required + "title: duplicate\n---\n",
		"---\ntitle: [unterminated\n---\n", "---\n" + required + "extra: unknown\n---\n",
	} {
		_, _, err := ReadArticle(strings.NewReader(raw))
		assertValidationCode(t, err, "E_SCHEMA")
	}
	for _, raw := range []string{"notes: invalid\n", "notes: [invalid]\n"} {
		_, err := Topics([]byte(raw))
		assertValidationCode(t, err, "E_SCHEMA")
	}
}

func TestReadArticlePreservesReaderErrors(t *testing.T) {
	t.Parallel()
	broken := errors.New("read failure")
	for _, prefix := range []string{"", "---\ntitle: T\n", "---\n" + required + "---\nBody\n"} {
		_, _, err := ReadArticle(io.MultiReader(strings.NewReader(prefix), failingReader{broken}))
		if !errors.Is(err, broken) {
			t.Fatalf("reader error = %v", err)
		}
		var validation *ValidationError
		if errors.As(err, &validation) {
			t.Fatalf("I/O error was classified as validation: %v", err)
		}
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func assertValidationCode(t *testing.T, err error, want string) {
	t.Helper()
	var validation *ValidationError
	if !errors.As(fmt.Errorf("file context: %w", err), &validation) || validation.Code != want {
		t.Fatalf("error = %v, want validation code %s", err, want)
	}
}

func TestTopicsUsesDecoderDocumentBehavior(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"notes: {ja: Label}\n---\nother: {ja: Ignored}\n", "notes: {ja: Label}\n---\n"} {
		topics, err := Topics([]byte(raw))
		if err != nil || topics["notes"]["ja"] != "Label" || len(topics) != 1 {
			t.Fatalf("topics decoder = %v, %v", topics, err)
		}
	}
}

func TestArticlesAndTopicsDecodeTags(t *testing.T) {
	t.Parallel()
	front := strings.Replace(required, "title: T", "title: &title !custom T", 1) + "summary: *title\n"
	data, body, err := ReadArticle(strings.NewReader("\ufeff---\n" + front + "---\n!custom Example\n"))
	if err != nil || data.Title != "T" || data.Summary != "T" || string(body) != "!custom Example\n" {
		t.Fatalf("decoded tag/body = %v, %q, %v", data, body, err)
	}
	if topics, err := Topics([]byte("protobuf: {ja: !custom PB}\n")); err != nil || topics["protobuf"]["ja"] != "PB" {
		t.Fatalf("decoded topic tag = %v, %v", topics, err)
	}
	if topics, err := Topics([]byte("protobuf: {ja: 3}\n")); err != nil || topics["protobuf"]["ja"] != "3" {
		t.Fatalf("native label conversion = %v, %v", topics, err)
	}
}

func TestCollectDuplicateSlugDiagnostic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, key := range []string{"first", "second"} {
		group := filepath.Join(dir, "posts", "2026", key)
		if err := os.MkdirAll(group, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(group, "ja.md"), []byte("---\n"+required+"---\nBody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	topics := filepath.Join(dir, "topics.yaml")
	if err := os.WriteFile(topics, []byte("protobuf:\n  ja: Protocol\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Collect(dir, topics, []string{"ja"})
	assertValidationCode(t, err, "E_SLUG_DUPLICATE")
}

func TestArticleAndTopicsEncoding(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"---\n" + required + "summary: Bad\xff\n---\nBody\n",
		"---\n" + required + "---\nBody\xff\n",
	} {
		_, _, err := ReadArticle(strings.NewReader(raw))
		assertValidationCode(t, err, "E_SCHEMA")
		if !strings.Contains(err.Error(), "invalid UTF-8") {
			t.Fatal(err)
		}
	}
	if _, err := Topics([]byte("notes: {ja: Bad\xff}\n")); err == nil {
		t.Fatal("invalid topic UTF-8 accepted")
	}
	topics, err := Topics([]byte("\ufeffnotes: {ja: 日本語}\r\n"))
	if err != nil || topics["notes"]["ja"] != "日本語" {
		t.Fatalf("BOM topics = %v, %v", topics, err)
	}
}

func TestCollectRejectsMalformedProseUTF8(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"home/ja.md", "pages/about/ja.md", "aside/ja.md"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte("Body\xff\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			topics := filepath.Join(dir, "topics.yaml")
			if err := os.WriteFile(topics, []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Collect(dir, topics, []string{"ja"})
			assertValidationCode(t, err, "E_SCHEMA")
			if !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "invalid UTF-8") {
				t.Fatal(err)
			}
		})
	}
}
