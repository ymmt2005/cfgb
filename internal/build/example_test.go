package build

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestExampleCorpus(t *testing.T) {
	manager := strings.TrimSpace(os.Getenv("CFGB_PACKAGE_MANAGER"))
	if manager == "" {
		manager = "npm"
	}
	if _, err := exec.LookPath("node"); err != nil || !packageManagerPresent(manager) {
		if os.Getenv("CFGB_REQUIRE_EXAMPLE") == "1" {
			t.Fatalf("node and %s are required", manager)
		}
		t.Skipf("node or %s is not installed", manager)
	}
	root := exampleRoot(t)
	out := filepath.Join(t.TempDir(), "artifact")
	t.Cleanup(func() {
		if err := os.RemoveAll(out); err != nil {
			t.Error(err)
		}
	})
	if err := Run(Options{Dir: root, Out: out}); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"site/ja/index.html",
		"site/en/index.html",
		"site/ja/posts/markdown-showcase/index.html",
		"site/en/posts/markdown-rendering-showcase/index.html",
		"site/ja/404.html",
		"site/404.html",
		"site/sitemap-index.xml",
		"site/sitemap-0.xml",
		"site/robots.txt",
		"site/_redirects",
		"worker/index.js",
		"build-manifest.json",
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Errorf("missing %s", rel)
		}
	}
	raw, err := os.ReadFile(filepath.Join(out, "site", "_redirects"))
	if err != nil {
		t.Fatal(err)
	}
	redirects := strings.TrimSuffix(string(raw), "\n")
	if redirects == "" || !strings.HasSuffix(string(raw), "\n") {
		t.Fatalf("redirects = %q", raw)
	}
	const alias = "/ja/posts/old-protobuf-guide/ /ja/posts/protobuf-schema-guide/ 301"
	foundAlias := false
	for _, line := range strings.Split(redirects, "\n") {
		if strings.HasPrefix(line, "-") || strings.Contains(line, " -") {
			t.Fatalf("redirect line keeps a YAML marker: %q", line)
		}
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[2] != "301" || !strings.HasPrefix(fields[0], "/") || !strings.HasPrefix(fields[1], "/") {
			t.Fatalf("redirect line = %q", line)
		}
		if line == alias {
			foundAlias = true
		}
	}
	if !foundAlias {
		t.Fatalf("redirects = %q", raw)
	}
	for _, rel := range []string{
		"site/ja/topics/protobuf/index.html",
		"site/en/topics/protobuf/index.html",
		"site/ja/topics/oss/index.html",
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Errorf("missing %s", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(out, ".tmp")); !os.IsNotExist(err) {
		t.Fatal("staging directory was left in the artifact")
	}
	sitemap, err := os.ReadFile(filepath.Join(out, "site", "sitemap-0.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, loc := range []string{
		"/ja/posts/protobuf-schema-guide/</loc>",
		"/en/posts/reading-protobuf-schemas/</loc>",
		"/ja/topics/protobuf/</loc>",
		"/en/</loc>",
		"/ja/about/</loc>",
		`hreflang="en"`,
		`hreflang="ja"`,
	} {
		if !bytes.Contains(sitemap, []byte(loc)) {
			t.Errorf("sitemap missing %s", loc)
		}
	}
	for _, blocked := range []string{"/search/", "feed.xml", "robots.txt", "404.html"} {
		if bytes.Contains(sitemap, []byte(blocked)) {
			t.Errorf("sitemap contains %s", blocked)
		}
	}
	headers, err := os.ReadFile(filepath.Join(out, "site", "_headers"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(headers, []byte("script-src 'self' 'wasm-unsafe-eval'")) {
		t.Fatalf("headers = %s", headers)
	}
	searchPage, err := os.ReadFile(filepath.Join(out, "site", "en", "search", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(searchPage, []byte(`src="/assets/search.js"`)) || bytes.Contains(searchPage, []byte("new window.PagefindUI")) {
		t.Fatal("search initializer is not an external script")
	}
	home, err := os.ReadFile(filepath.Join(out, "site", "ja", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(home, []byte("/__locale?lang=en&amp;next=")) || !bytes.Contains(home, []byte("/__locale?lang=ja&amp;next=")) {
		t.Fatal("language links do not set the locale cookie")
	}
	sourceArticle, err := os.ReadFile(filepath.Join(root, "src", "content", "posts", "2026", "2026-09-20-markdown-showcase", "en.md"))
	if err != nil {
		t.Fatal(err)
	}
	const rawLink = `href="../2026-09-19-protobuf-guide/en.md#field-numbers"`
	if bytes.Contains(sourceArticle, []byte(rawLink)) {
		rendered, err := os.ReadFile(filepath.Join(out, "site", "en", "posts", "markdown-rendering-showcase", "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		const rewritten = `<a href="/en/posts/reading-protobuf-schemas/#field-numbers">Field numbers</a>`
		if bytes.Contains(rendered, []byte(rawLink)) || !bytes.Contains(rendered, []byte(rewritten)) {
			t.Fatal("raw HTML article link was not rewritten inside its anchor")
		}
	}
	article, err := os.ReadFile(filepath.Join(out, "site", "ja", "posts", "markdown-showcase", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(article, []byte("data-footnote-ref")) || (!bytes.Contains(article, []byte("id=\"補足-1\"")) && !bytes.Contains(article, []byte("id=\"テスト-1\""))) {
		t.Fatal("japanese showcase is missing the footnote or duplicate heading")
	}
	if !bytes.Contains(article, []byte(`data-pagefind-filter="year:2026"`)) || !bytes.Contains(article, []byte(`data-pagefind-meta="published[datetime]"`)) {
		t.Fatal("article is missing the site-local year or publication metadata")
	}
	published := pagefindPublished(t, filepath.Join(out, "site", "ja", "posts", "markdown-showcase", "index.html"))
	stamp := publicationStamp(article)
	if published == "" || published != stamp {
		t.Fatalf("pagefind published = %q, datetime = %q", published, stamp)
	}
	root404, err := os.ReadFile(filepath.Join(out, "site", "404.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(root404, []byte(`href="/ja/"`)) || !bytes.Contains(root404, []byte(`href="/en/"`)) {
		t.Fatal("bilingual root fallback is missing a configured locale")
	}
	raw, err = os.ReadFile(filepath.Join(out, "build-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Toolchain struct {
			PackageManager      string `json:"packageManager"`
			ObservedNpmVersion  string `json:"observedNpmVersion"`
			ObservedPnpmVersion string `json:"observedPnpmVersion"`
		} `json:"toolchain"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	cleanupExampleWorkspace(t, out)
	switch manager {
	case "npm":
		if manifest.Toolchain.PackageManager != "npm" || !npmAtLeast12(manifest.Toolchain.ObservedNpmVersion) || manifest.Toolchain.ObservedPnpmVersion != "" {
			t.Fatalf("toolchain = %+v", manifest.Toolchain)
		}
	case "pnpm":
		if manifest.Toolchain.PackageManager != "pnpm" || !pnpmAtLeast11(manifest.Toolchain.ObservedPnpmVersion) || manifest.Toolchain.ObservedNpmVersion != "" {
			t.Fatalf("toolchain = %+v", manifest.Toolchain)
		}
	default:
		t.Fatalf("CFGB_PACKAGE_MANAGER = %s", manager)
	}
	t.Run("final output failure retains artifact and toolchain", func(t *testing.T) {
		failedOut := filepath.Join(t.TempDir(), "artifact")
		t.Cleanup(func() {
			if err := os.RemoveAll(failedOut); err != nil {
				t.Error(err)
			}
		})
		broken := errors.New("final output stream failed")
		err := Run(Options{Dir: root, Out: failedOut, Stdout: progressWriter(func(p []byte) (int, error) {
			if bytes.HasPrefix(p, []byte("built ")) {
				return 0, broken
			}
			return len(p), nil
		})})
		var exit *ExitError
		if !errors.As(err, &exit) || exit.Code != 3 || !errors.Is(err, broken) {
			t.Fatalf("final output failure = %v", err)
		}
		for _, rel := range []string{"site/en/index.html", "worker/index.js", "build-manifest.json"} {
			if _, err := os.Stat(filepath.Join(failedOut, rel)); err != nil {
				t.Fatalf("completed artifact removed: %s: %v", rel, err)
			}
		}
		cleanupExampleWorkspace(t, failedOut)
	})
	t.Run("external output and filesystem article key", func(t *testing.T) {
		buildExtendedExample(t, root)
	})
}

func buildExtendedExample(t *testing.T, source string) {
	t.Helper()
	repo := t.TempDir()
	root, err := os.OpenRoot(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, name := range []string{"cfgb.yaml", "src"} {
		if err := copyFromRoot(root, name, filepath.Join(repo, name)); err != nil {
			t.Fatal(err)
		}
	}
	// Prose uses the same native text path as article bodies: the loader adds
	// no encoding ban. JSON serialization supplies its own replacement behavior.
	for _, name := range []string{"home/ja.md", "pages/about/ja.md", "aside/ja.md"} {
		filename := filepath.Join(repo, "src", "content", filepath.FromSlash(name))
		raw, err := os.ReadFile(filename)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		raw = append([]byte("\ufeff"), raw...)
		raw = append(raw, []byte("\n\nProse regression "+name+" \xff\n")...)
		if err := os.WriteFile(filename, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const original = "2026-09-19-protobuf-guide"
	const key = "Protocol Buffers_図.#%"
	posts := filepath.Join(repo, "src", "content", "posts", "2026")
	group := filepath.Join(posts, key)
	if err := os.Rename(filepath.Join(posts, original), group); err != nil {
		t.Fatal(err)
	}
	// Source links still identify the renamed group, with the key encoded as
	// a literal URL component rather than becoming whitespace or a fragment.
	err = filepath.WalkDir(filepath.Join(repo, "src", "content"), func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			return nil
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		raw = bytes.ReplaceAll(raw, []byte(original), []byte(url.PathEscape(key)))
		if filepath.Dir(name) == group {
			raw = append(raw, []byte("\n<img alt=\"filesystem key\" src=\"./assets/schema.svg\">\n")...)
		}
		return os.WriteFile(name, raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join("assets")
	for range 80 {
		deep = filepath.Join(deep, "d")
	}
	if err := os.MkdirAll(filepath.Join(group, deep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, deep, "leaf.txt"), []byte("deep asset"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "artifact")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	if err := Run(Options{Dir: repo, Out: out, Stdout: &log, Stderr: &log}); err != nil {
		t.Fatalf("extended build: %v\n%s", err, log.String())
	}
	cleanupExampleWorkspace(t, out)
	if _, err := os.Stat(filepath.Join(out, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("external output was not replaced: %v", err)
	}
	for _, route := range []string{"ja/posts/protobuf-schema-guide", "en/posts/reading-protobuf-schemas"} {
		html, err := os.ReadFile(filepath.Join(out, "site", route, "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(html, []byte(`src="/media/2026/`+url.PathEscape(key)+`/schema.svg"`)) {
			t.Fatalf("literal article key was not encoded in %s", route)
		}
	}
	for _, tc := range []struct{ route, marker string }{
		{"ja/index.html", "home/ja.md"},
		{"ja/about/index.html", "pages/about/ja.md"},
		{"ja/index.html", "aside/ja.md"},
	} {
		html, err := os.ReadFile(filepath.Join(out, "site", filepath.FromSlash(tc.route)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(html, []byte("Prose regression "+tc.marker+" \ufffd")) {
			t.Fatalf("prose did not render through native JSON text handling: %s", tc.marker)
		}
	}
	leaf := filepath.Join(out, "site", "media", "2026", key, strings.TrimPrefix(deep, "assets"+string(filepath.Separator)), "leaf.txt")
	if raw, err := os.ReadFile(leaf); err != nil || string(raw) != "deep asset" {
		t.Fatalf("deep media was not delivered: %q, %v", raw, err)
	}
}

func cleanupExampleWorkspace(t *testing.T, out string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(out, "build-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Session string `json:"toolchainSessionId"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(manifest.Session, "cfgb-build-") || filepath.Base(manifest.Session) != manifest.Session {
		t.Fatalf("unexpected workspace basename: %q", manifest.Session)
	}
	workspace := filepath.Join(os.TempDir(), manifest.Session)
	if _, err := os.Stat(filepath.Join(workspace, "renderer", "node_modules")); err != nil {
		t.Fatalf("completed toolchain was not retained: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(workspace); err != nil {
			t.Error(err)
		}
	})
}

var publicationStampPattern = regexp.MustCompile(`<time datetime="([^"]+)" data-pagefind-meta="published\[datetime\]">`)

func publicationStamp(article []byte) string {
	match := publicationStampPattern.FindSubmatch(article)
	if match == nil {
		return ""
	}
	return string(match[1])
}

func pagefindPublished(t *testing.T, htmlPath string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	script := `
import { createIndex } from "pagefind";
import { readFileSync } from "node:fs";
const html = readFileSync(process.argv[1], "utf8");
const created = await createIndex();
if (created.errors?.length) throw new Error(created.errors.join("\n"));
const added = await created.index.addHTMLFile({ url: "/article/", content: html });
if (added.errors?.length) throw new Error(added.errors.join("\n"));
process.stdout.write(JSON.stringify(added.file.meta));
`
	cmd := exec.Command("node", "--input-type=module", "-e", script, htmlPath)
	cmd.Dir = filepath.Join(filepath.Dir(file), "..", "..", "renderer")
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if exit, ok := err.(*exec.ExitError); ok {
			stderr = string(exit.Stderr)
		}
		t.Fatalf("pagefind: %v\n%s%s", err, stderr, out)
	}
	var meta map[string]string
	if err := json.Unmarshal(out, &meta); err != nil {
		t.Fatalf("pagefind meta: %v\n%s", err, out)
	}
	return meta["published"]
}

func packageManagerPresent(manager string) bool {
	switch manager {
	case "npm":
		return exec.Command("npm", "-v").Run() == nil
	case "pnpm":
		return exec.Command("pnpm", "-v").Run() == nil
	default:
		return false
	}
}

func npmAtLeast12(version string) bool {
	return versionAtLeast(version, 12)
}

func pnpmAtLeast11(version string) bool {
	return versionAtLeast(version, 11)
}

func versionAtLeast(version string, major int) bool {
	got, _, _ := strings.Cut(version, ".")
	n, err := strconv.Atoi(got)
	return err == nil && n >= major
}

func exampleRoot(t *testing.T) string {
	t.Helper()
	if value := os.Getenv("CFGB_EXAMPLE"); value != "" {
		return value
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	module := filepath.Join(filepath.Dir(file), "..", "..")
	for _, rel := range []string{filepath.Join("..", "cfgb-example"), "cfgb-example"} {
		candidate := filepath.Clean(filepath.Join(module, rel))
		if _, err := os.Stat(filepath.Join(candidate, "cfgb.yaml")); err == nil {
			return candidate
		}
	}
	if os.Getenv("CFGB_REQUIRE_EXAMPLE") == "1" {
		t.Fatal("cfgb-example checkout was not found")
	}
	t.Skip("cfgb-example checkout was not found")
	return ""
}
