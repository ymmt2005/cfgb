package build

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
	switch manager {
	case "npm":
		if manifest.Toolchain.PackageManager != "npm" || !npmAtLeast12(manifest.Toolchain.ObservedNpmVersion) || manifest.Toolchain.ObservedPnpmVersion != "" {
			t.Fatalf("toolchain = %+v", manifest.Toolchain)
		}
	case "pnpm":
		req, err := loadRequirements()
		if err != nil {
			t.Fatal(err)
		}
		if manifest.Toolchain.PackageManager != "pnpm" || manifest.Toolchain.ObservedPnpmVersion != req.PnpmVersion || manifest.Toolchain.ObservedNpmVersion != "" {
			t.Fatalf("toolchain = %+v", manifest.Toolchain)
		}
	default:
		t.Fatalf("CFGB_PACKAGE_MANAGER = %s", manager)
	}
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
	major, _, _ := strings.Cut(version, ".")
	n, err := strconv.Atoi(major)
	return err == nil && n >= 12
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
