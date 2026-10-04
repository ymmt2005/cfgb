package frontmatter

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitPreservesScalarsArraysAndBody(t *testing.T) {
	data, body := article(t, ""+
		"topics: [protobuf, oss]\n"+
		"aliases: [\"/ja/posts/old/\"]\n"+
		"---\n"+
		"body\n")
	assertStrings(t, data["topics"], "protobuf", "oss")
	assertStrings(t, data["aliases"], "/ja/posts/old/")
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
	assertStrings(t, data["topics"], "protobuf", "oss")
	assertStrings(t, data["aliases"], "/ja/posts/old-protobuf-guide/")
	if len(body) != 0 {
		t.Fatalf("body = %q", body)
	}

	data, body = article(t, ""+
		"topics:\n"+
		"  - protobuf\n"+
		"  - oss\n"+
		"---\n"+
		"kept\n")
	assertStrings(t, data["topics"], "protobuf", "oss")
	if string(body) != "kept\n" {
		t.Fatalf("body = %q", body)
	}

	data, _ = article(t, ""+
		"summary: >\n"+
		"  hello\n"+
		"  world\n"+
		"---\n")
	if data["summary"] != "hello world\n" {
		t.Fatalf("folded = %q", data["summary"])
	}

	data, _ = article(t, ""+
		"summary: |\n"+
		"  hello\n"+
		"  world\n"+
		"---\n")
	if data["summary"] != "hello\nworld\n" {
		t.Fatalf("literal = %q", data["summary"])
	}

	data, _ = article(t, ""+
		"title: \"say \\\"hi\\\"\"\n"+
		"---\n")
	if data["title"] != "say \"hi\"" {
		t.Fatalf("title = %q", data["title"])
	}
	if published, ok := data["publishedAt"].(string); !ok || published != "2026-01-02T03:04:05Z" {
		t.Fatalf("publishedAt = %#v", data["publishedAt"])
	}
}

func TestSplitLineEndingsAndDelimiters(t *testing.T) {
	crlf := "---\r\ntitle: T\r\nslug: t\r\npublishedAt: '2026-01-02T03:04:05Z'\r\ntopics:\r\n- protobuf\r\n---\r\nbody\r\n"
	data, body := mustArticle(t, crlf)
	assertStrings(t, data["topics"], "protobuf")
	if !bytes.Equal(body, []byte("body\r\n")) {
		t.Fatalf("body = %q", body)
	}

	data, body = article(t, ""+
		"summary: |\n"+
		"  before\n"+
		"  ---\n"+
		"  after\n"+
		"---\n"+
		"BODY\n")
	if data["summary"] != "before\n---\nafter\n" {
		t.Fatalf("summary = %q", data["summary"])
	}
	if string(body) != "BODY\n" {
		t.Fatalf("body = %q", body)
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
	if _, _, _, err := Split(strings.NewReader("\ufeff---\n")); err == nil || !strings.Contains(err.Error(), "BOM") {
		t.Fatalf("bom err = %v", err)
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

func TestArticleDataDoesNotCoerce(t *testing.T) {
	doc, err := Document([]byte(identity + "topics: protobuf\n"))
	if err != nil {
		t.Fatal(err)
	}
	mapping := doc.(map[string]any)
	if _, ok := mapping["topics"].(string); !ok {
		t.Fatalf("topics = %#v", mapping["topics"])
	}
	if _, err := ArticleData(doc); err == nil || !strings.Contains(err.Error(), "topics must be an array") {
		t.Fatalf("topics err = %v", err)
	}
	cases := []string{
		identity + "topics: []\n",
		identity + "topics: [protobuf]\naliases: /ja/posts/old/\n",
		"title: 1\nslug: t\npublishedAt: '2026-01-02T03:04:05Z'\ntopics: [protobuf]\n",
		identity + "topics: [protobuf]\nsummary:\n",
	}
	for _, raw := range cases {
		doc, err := Document([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ArticleData(doc); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if _, err := Document([]byte("a: 1\na: 2\n")); err == nil {
		t.Fatal("duplicate key was accepted")
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
	if post.Data["slug"] != "protobuf-schema-guide" || post.ID != "posts/2026/2026-09-19-protobuf-guide/ja" {
		t.Fatalf("post = %#v", post)
	}
	aliases, _ := post.Data["aliases"].([]string)
	if len(aliases) != 1 || aliases[0] != "/ja/posts/old-protobuf-guide/" {
		t.Fatalf("aliases = %#v", post.Data["aliases"])
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
			if err == nil || !strings.Contains(err.Error(), "unknown article field") || !strings.Contains(err.Error(), key) {
				t.Fatalf("unknown field %s: %v", key, err)
			}
		})
	}
	data, body, err := ReadArticle(strings.NewReader("---\n" + required +
		"updatedAt: '2026-01-03T03:04:05Z'\nsummary: Summary\nogImage: ./assets/picture.png\naliases: [/ja/posts/old/]\n---\nBody\n---\n"))
	if err != nil || len(data) != 8 || string(body) != "Body\n---\n" {
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

func TestCollectKeepsSharedAssetsAndExcludesNonPosts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"posts/2026/example/ja.md", "posts/2026/example/assets/fr.md", "tests/fr.md", "docs/fr.md", "examples/fr.md"} {
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

func article(t *testing.T, fields string) (map[string]any, []byte) {
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

func mustArticle(t *testing.T, raw string) (map[string]any, []byte) {
	t.Helper()
	data, body, err := ReadArticle(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("%v\n%s", err, raw)
	}
	return data, body
}

func assertStrings(t *testing.T, value any, want ...string) {
	t.Helper()
	got, ok := value.([]string)
	if !ok {
		t.Fatalf("got %#v", value)
	}
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
