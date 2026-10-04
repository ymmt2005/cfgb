package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExampleStaticHost(t *testing.T) {
	manager := os.Getenv("CFGB_PACKAGE_MANAGER")
	if manager == "" {
		manager = "npm"
	}
	if _, err := exec.LookPath("node"); err != nil || !packageManagerPresent(manager) {
		if os.Getenv("CFGB_REQUIRE_EXAMPLE") == "1" {
			t.Fatal("renderer toolchain is required")
		}
		t.Skip("renderer toolchain is unavailable")
	}
	root := exampleRoot(t)
	out := filepath.Join(t.TempDir(), "artifact")
	if err := Run(Options{Dir: root, Out: out, Static: true, BaseURL: "https://example.invalid/blog/"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "ja/index.html", "ja/posts/markdown-showcase/index.html", "ja/posts/old-protobuf-guide/index.html", "ja/feed.xml", "sitemap-index.xml", "sitemap-0.xml", "robots.txt"} {
		raw, err := os.ReadFile(filepath.Join(out, "site", name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.Contains(text, "/blog/") || strings.Contains(text, "/blog/blog/") {
			t.Errorf("hosting prefix in %s", name)
		}
		if strings.Contains(text, "href=\"/__locale") || strings.Contains(text, "href=\"/assets/") || strings.Contains(text, "href=\"/ja/") {
			t.Errorf("origin-root link in %s", name)
		}
	}
}
