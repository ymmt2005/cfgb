package frontmatter

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSplitPreservesScalarsArraysAndBody(t *testing.T) {
	data, body := article(t, ""+
		"topics: [protobuf, oss]\n"+
		"aliases: [\"/ja/posts/old/\"]\n"+
		"---\n"+
		"body\n")
	assertStrings(t, data.Topics, "protobuf", "oss")
	assertStrings(t, data.Aliases, "/ja/posts/old/")
	if string(body) != "body\n" {
		t.Fatalf("body = %q", body)
	}

	data, body = article(t, ""+
		"topics:\n"+
		"- protobuf\n"+
		"- oss\n"+
		"aliases:\n"+
		"- /ja/posts/old-protobuf-guide/\n"+
		"---\n")
	assertStrings(t, data.Topics, "protobuf", "oss")
	assertStrings(t, data.Aliases, "/ja/posts/old-protobuf-guide/")
	if len(body) != 0 {
		t.Fatalf("body = %q", body)
	}

	data, body = article(t, ""+
		"topics:\n"+
		"  - protobuf\n"+
		"  - oss\n"+
		"---\n"+
		"kept\n")
	assertStrings(t, data.Topics, "protobuf", "oss")
	if string(body) != "kept\n" {
		t.Fatalf("body = %q", body)
	}

	data = decodedFront(t, ""+
		"summary: >\n"+
		"  hello\n"+
		"  world\n"+
		"---\n")
	if data.Summary != "hello world\n" {
		t.Fatalf("folded = %q", data.Summary)
	}

	data = decodedFront(t, ""+
		"summary: |\n"+
		"  hello\n"+
		"  world\n"+
		"---\n")
	if data.Summary != "hello\nworld\n" {
		t.Fatalf("literal = %q", data.Summary)
	}

	data, _ = article(t, ""+
		"title: \"say \\\"hi\\\"\"\n"+
		"---\n")
	if data.Title != "say \"hi\"" {
		t.Fatalf("title = %q", data.Title)
	}
	if data.PublishedAt.Format(time.RFC3339) != "2026-01-02T03:04:05Z" {
		t.Fatalf("publishedAt = %#v", data.PublishedAt)
	}
}

func TestSplitLineEndingsAndDelimiters(t *testing.T) {
	crlf := "---\r\ntitle: T\r\nslug: t\r\npublishedAt: '2026-01-02T03:04:05Z'\r\ntopics:\r\n- protobuf\r\n---\r\nbody\r\n"
	data, body := mustArticle(t, crlf)
	assertStrings(t, data.Topics, "protobuf")
	if !bytes.Equal(body, []byte("body\r\n")) {
		t.Fatalf("body = %q", body)
	}

	data = decodedFront(t, ""+
		"summary: |\n"+
		"  before\n"+
		"  ---\n"+
		"  after\n"+
		"---\n"+
		"BODY\n")
	if data.Summary != "before\n---\nafter\n" {
		t.Fatalf("summary = %q", data.Summary)
	}
	_, body, _, err := Split(strings.NewReader("---\n" + required + "summary: |\n  before\n  ---\n  after\n---\nBODY\n"))
	if err != nil || string(body) != "BODY\n" {
		t.Fatalf("body = %q, %v", body, err)
	}

	const markdown = "---\n" + required + "---\npara\n\n---\n\n```\n---\n```\n"
	_, body = mustArticle(t, markdown)
	const want = "para\n\n---\n\n```\n---\n```\n"
	if string(body) != want {
		t.Fatalf("body = %q", body)
	}
	_, chunkBody, err := ReadArticle(chunkReader{r: strings.NewReader(markdown)})
	if err != nil {
		t.Fatal(err)
	}
	if string(chunkBody) != want {
		t.Fatalf("chunked body = %q", chunkBody)
	}

	if _, _, err := ReadArticle(strings.NewReader("---\ntitle: T\n")); err == nil || !strings.Contains(err.Error(), "unterminated") {
		t.Fatalf("unterminated err = %v", err)
	}
	if _, _, err := ReadArticle(strings.NewReader("no front matter\n")); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("missing err = %v", err)
	}
	prose := "whole\n\n---\n\nfile\n"
	_, proseBody, found, err := Split(strings.NewReader(prose))
	if err != nil || found || string(proseBody) != prose {
		t.Fatalf("prose found=%v err=%v body=%q", found, err, proseBody)
	}
	if _, _, found, err := Split(strings.NewReader("\ufeff---\n" + required + "---\nBody\n")); err != nil || !found {
		t.Fatalf("BOM-prefixed delimiter = %v, %v", found, err)
	}

	spaced := "---\n" + required + "--- \nstill\n----\nalso\n---\nbody\n"
	front, body, found, err := Split(strings.NewReader(spaced))
	if err != nil || !found {
		t.Fatal(err)
	}
	if !bytes.Contains(front, []byte("--- \n")) || !bytes.Contains(front, []byte("----\n")) {
		t.Fatalf("front = %q", front)
	}
	if string(body) != "body\n" {
		t.Fatalf("delimiter body = %q", body)
	}
}

func TestArticleUsesTypedDecoder(t *testing.T) {
	for _, fields := range []string{
		"topics: protobuf\n", "aliases: /ja/posts/old/\n", "title: [not, a, string]\n",
	} {
		_, _, err := ReadArticle(strings.NewReader("---\n" + fields + "---\nBody\n"))
		if err == nil {
			t.Fatalf("incompatible Go field type accepted: %q", fields)
		}
	}
	for _, fields := range []string{"", "title: 1\n", "topics: []\n", "summary: null\n"} {
		if _, _, err := ReadArticle(strings.NewReader("---\n" + fields + "---\nBody\n")); err != nil {
			t.Fatalf("decodable Go fields rejected: %q: %v", fields, err)
		}
	}
	if _, _, err := ReadArticle(strings.NewReader("---\ntitle: first\ntitle: second\n---\n")); err == nil {
		t.Fatal("YAML decoder's duplicate-key error was lost")
	}
	if _, _, err := ReadArticle(strings.NewReader("---\ntitle: [unterminated\n---\n")); err == nil {
		t.Fatal("malformed YAML was accepted")
	}
}

func TestCollectAgreesOnMetadata(t *testing.T) {
	dir := t.TempDir()
	articleDir := filepath.Join(dir, "posts", "2026", "2026-09-19-protobuf-guide")
	if err := os.MkdirAll(articleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	article := "---\ntitle: Protocol\nslug: protobuf-schema-guide\npublishedAt: '2026-09-19T13:12:40+09:00'\ntopics:\n- protobuf\n- oss\naliases:\n- /ja/posts/old-protobuf-guide/\n---\n## Schema\n\n---\n\n```\n---\n```\n"
	if err := os.WriteFile(filepath.Join(articleDir, "ja.md"), []byte(article), 0o644); err != nil {
		t.Fatal(err)
	}
	home := "welcome\n\n---\n\nstay\n"
	if err := os.WriteFile(filepath.Join(dir, "home", "ja.md"), []byte(home), 0o644); err != nil {
		t.Fatal(err)
	}
	topics := "protobuf:\n  ja: Protocol Buffers\n  en: Protocol Buffers\noss:\n  ja: オープンソース\n"
	topicsFile := filepath.Join(dir, "topics.yaml")
	if err := os.WriteFile(topicsFile, []byte(topics), 0o644); err != nil {
		t.Fatal(err)
	}
	index, err := Collect(dir, topicsFile, []string{"ja", "en"})
	if err != nil {
		t.Fatal(err)
	}
	if index.Topics["protobuf"]["ja"] != "Protocol Buffers" || index.Topics["oss"]["ja"] != "オープンソース" {
		t.Fatalf("topics = %#v", index.Topics)
	}
	if len(index.Posts) != 1 {
		t.Fatalf("posts = %#v", index.Posts)
	}
	post := index.Posts[0]
	if post.Data.Slug != "protobuf-schema-guide" || post.ID != "posts/2026/2026-09-19-protobuf-guide/ja" {
		t.Fatalf("post = %#v", post)
	}
	aliases := post.Data.Aliases
	if len(aliases) != 1 || aliases[0] != "/ja/posts/old-protobuf-guide/" {
		t.Fatalf("aliases = %#v", post.Data.Aliases)
	}
	if post.Body != "## Schema\n\n---\n\n```\n---\n```\n" {
		t.Fatalf("body = %q", post.Body)
	}
	if post.File != filepath.Join(articleDir, "ja.md") {
		t.Fatalf("file = %s", post.File)
	}
	if len(index.Prose) != 1 || index.Prose[0].ID != "home/ja" || index.Prose[0].Body != home {
		t.Fatalf("prose = %#v", index.Prose)
	}
	if _, err := Topics([]byte("protobuf: Protocol Buffers\n")); err == nil {
		t.Fatal("string topic was accepted")
	}
}

const identity = "title: T\nslug: t\npublishedAt: '2026-01-02T03:04:05Z'\n"
const required = identity + "topics: [protobuf]\n"

func TestArticleRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"summray", "extra", "draft"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			_, _, err := ReadArticle(strings.NewReader("---\n" + required + key + ": value\n---\nBody\n"))
			if err == nil || !strings.Contains(err.Error(), "unknown") || !strings.Contains(err.Error(), key) {
				t.Fatalf("unknown field %s: %v", key, err)
			}
		})
	}
	data, body, err := ReadArticle(strings.NewReader("---\n" + required +
		"updatedAt: '2026-01-03T03:04:05Z'\nsummary: Summary\nogImage: ./assets/picture.png\naliases: [/ja/posts/old/]\n---\nBody\n---\n"))
	if err != nil || data.UpdatedAt == nil || data.Summary != "Summary" || data.OGImage != "./assets/picture.png" || len(data.Aliases) != 1 || string(body) != "Body\n---\n" {
		t.Fatalf("supported fields/body = %#v, %q, %v", data, body, err)
	}
}

func TestCollectRejectsUnconfiguredVariants(t *testing.T) {
	t.Parallel()
	for _, locale := range []string{"fr", "en"} {
		t.Run(locale, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			group := filepath.Join(dir, "posts", "2026", "example")
			if err := os.MkdirAll(group, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, variant := range []string{"ja", locale} {
				if err := os.WriteFile(filepath.Join(group, variant+".md"), []byte("---\n"+required+"---\nBody\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			topics := filepath.Join(dir, "topics.yaml")
			if err := os.WriteFile(topics, []byte("protobuf:\n  ja: Protocol Buffers\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Collect(dir, topics, []string{"ja"})
			if err == nil || !strings.Contains(err.Error(), "posts/2026/example/"+locale+".md") || !strings.Contains(err.Error(), "not enabled") {
				t.Fatalf("unconfigured variant = %v", err)
			}
		})
	}
}

func TestCollectRejectsInvalidGroupLayout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, key, nested string }{
		{"space", "Bad Key", ""},
		{"uppercase", "Example", ""},
		{"percent encoded", "%2e%2e", ""},
		{"dot", "example.key", ""},
		{"underscore", "example_key", ""},
		{"double hyphen", "example--key", ""},
		{"nested variant", "example", "nested"},
		{"nested locale", "example", "ja"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			group := filepath.Join(dir, "posts", "2026", tc.key, tc.nested)
			if err := os.MkdirAll(group, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(group, "ja.md"), []byte("---\n"+required+"---\nBody\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			topics := filepath.Join(dir, "topics.yaml")
			if err := os.WriteFile(topics, []byte("protobuf:\n  ja: Protocol Buffers\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Collect(dir, topics, []string{"ja"})
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Code != "E_TRANSLATION_GROUP" || !strings.Contains(err.Error(), "posts/2026/"+tc.key) {
				t.Fatalf("invalid layout accepted or misclassified: %v", err)
			}
		})
	}
}

func TestCollectKeepsSharedAssetsAndExcludesNonPosts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"posts/2026/2026-01-02-example/ja.md", "posts/2026/2026-01-02-example/assets/fr.md", "posts/2026/2026-01-02-example/assets/nested/en.md", "posts/2026/2026-01-02-example/.cfgb.json", "tests/fr.md", "docs/fr.md", "examples/fr.md"} {
		filename := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte("---\n"+required+"---\nBody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	topics := filepath.Join(dir, "topics.yaml")
	if err := os.WriteFile(topics, []byte("protobuf:\n  ja: Protocol Buffers\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	index, err := Collect(dir, topics, []string{"ja"})
	if err != nil || len(index.Posts) != 1 {
		t.Fatalf("posts = %#v, error = %v", index.Posts, err)
	}
}

func article(t *testing.T, fields string) (Metadata, []byte) {
	t.Helper()
	raw := "---\n"
	if !strings.Contains(fields, "title:") {
		raw += "title: T\n"
	}
	if !strings.Contains(fields, "slug:") {
		raw += "slug: t\n"
	}
	if !strings.Contains(fields, "publishedAt:") {
		raw += "publishedAt: '2026-01-02T03:04:05Z'\n"
	}
	if !strings.Contains(fields, "topics:") {
		raw += "topics: [protobuf]\n"
	}
	return mustArticle(t, raw+fields)
}

func mustArticle(t *testing.T, raw string) (Metadata, []byte) {
	t.Helper()
	data, body, err := ReadArticle(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("%v\n%s", err, raw)
	}
	return data, body
}

func assertStrings(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %q want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

type chunkReader struct {
	r io.Reader
}

func (c chunkReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return c.r.Read(p[:1])
}

// Folding/literal values are decoded into the same struct used by consumers.
func decodedFront(t *testing.T, tail string) Metadata {
	t.Helper()
	data, _, err := ReadArticle(strings.NewReader("---\n" + identity + "topics: [protobuf]\n" + tail))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestArticleDoesNotApplySchemaConstraints(t *testing.T) {
	t.Parallel()
	for _, fields := range []string{
		"slug: Bad\n", "slug: ../outside\n", "topics: [bad_topic, bad_topic]\n",
		"summary: |\n  first\n  second\n", "ogImage: ../outside.png\n", "aliases: [relative/]\n",
	} {
		if _, _, err := ReadArticle(strings.NewReader("---\n" + fields + "---\nBody\n")); err != nil {
			t.Fatalf("decodable metadata was schema-gated: %q: %v", fields, err)
		}
	}
}
