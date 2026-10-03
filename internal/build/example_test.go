package build

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExampleCorpus(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil || exec.Command("npm", "-v").Run() != nil {
		if os.Getenv("CFGB_REQUIRE_EXAMPLE") == "1" {
			t.Fatal("node and npm are required")
		}
		t.Skip("node or npm is not installed")
	}
	t.Setenv("CFGB_PACKAGE_MANAGER", "")
	root := exampleRoot(t)
	out := filepath.Join(root, ".cfgb-build-test")
	os.RemoveAll(out)
	t.Cleanup(func() { os.RemoveAll(out) })
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
	if !bytes.Contains(raw, []byte("/ja/posts/old-protobuf-guide/ /ja/posts/protobuf-schema-guide/ 301\n")) {
		t.Fatalf("redirects = %q", raw)
	}
	article, err := os.ReadFile(filepath.Join(out, "site", "ja", "posts", "markdown-showcase", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(article, []byte("data-footnote-ref")) || !bytes.Contains(article, []byte("id=\"テスト-1\"")) {
		t.Fatal("japanese showcase is missing the footnote or duplicate heading")
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
	if manifest.Toolchain.PackageManager != "npm" || !strings.HasPrefix(manifest.Toolchain.ObservedNpmVersion, "10.") || manifest.Toolchain.ObservedPnpmVersion != "" {
		t.Fatalf("toolchain = %+v", manifest.Toolchain)
	}
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
