package build

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ymmt2005/cfgb/internal/config"
)

func TestSiteImageSnapshot(t *testing.T) {
	_, repo := testRepo(t)
	settings := filepath.Join(repo, "settings")
	if err := os.Mkdir(settings, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(settings, "blog.yaml")
	if err := os.WriteFile(file, []byte("site:\n  title: Example\n  image: ../brand.svg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	image := filepath.Join(repo, "brand.svg")
	if err := os.WriteFile(image, []byte("original image"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := t.TempDir()
	staged, err := stageSiteImage(cfg, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(image, []byte("later edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(staged); err != nil || string(raw) != "original image" {
		t.Fatalf("snapshot = %q, %v", raw, err)
	}
	siteFile := filepath.Join(snapshot, "site.json")
	if err := writeSiteJSON(siteFile, cfg, "content", "topics", "cards", "metadata", staged, false); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(siteFile)
	if err != nil {
		t.Fatal(err)
	}
	var site siteJSON
	if err := json.Unmarshal(raw, &site); err != nil {
		t.Fatal(err)
	}
	if site.Image != staged {
		t.Fatalf("renderer image = %q, want captured %q", site.Image, staged)
	}
	for _, out := range []string{image, repo} {
		if _, err := outputDir(cfg, out); err == nil {
			t.Fatalf("output may remove the branding image: %s", out)
		}
	}
}

func TestSiteImageRootedReads(t *testing.T) {
	cfg, repo := testRepo(t)
	if dest, err := stageSiteImage(cfg, t.TempDir()); err != nil || dest != "" {
		t.Fatalf("optional image = %q, %v", dest, err)
	}
	cfg.Site.Image = "missing.svg"
	if _, err := stageSiteImage(cfg, t.TempDir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing image = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "brand.svg"), []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("brand.svg", filepath.Join(repo, "contained.svg")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	cfg.Site.Image = "contained.svg"
	if _, err := stageSiteImage(cfg, t.TempDir()); err != nil {
		t.Fatalf("contained symlink: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.svg")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "escaping.svg")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"escaping.svg", outside} {
		cfg.Site.Image = name
		if _, err := stageSiteImage(cfg, t.TempDir()); err == nil {
			t.Fatalf("read escaped repository: %s", name)
		}
	}
}
