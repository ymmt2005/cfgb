package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml/ast"
)

func TestLoad(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, `
schemaVersion: 1
site:
  title: CFGB Example
  baseUrl: https://example.invalid
  defaultLocale: ja
  timezone: Asia/Tokyo
locales:
  ja:
    label: 日本語
hatena:
  blogs:
    - url: https://example.invalid
      locale: ja
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Content.Root != "src/content" || cfg.Content.Topics != "src/data/topics.yaml" || cfg.Home.LatestPosts != 5 {
		t.Fatalf("defaults = %+v home=%d", cfg.Content, cfg.Home.LatestPosts)
	}
	if cfg.Hatena == nil || cfg.Hatena.Type() != ast.MappingType {
		t.Fatalf("hatena = %#v", cfg.Hatena)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, minimalConfig+"\npalette: dusk\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("unknown field was accepted")
	}
}

func TestLoadFollowsInRootConfigSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(minimalConfig, "CFGB Example", "Linked", 1)
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("site.yaml", filepath.Join(dir, "cfgb.yaml")); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Site.Title != "Linked" {
		t.Fatalf("title = %s", cfg.Site.Title)
	}
}

func TestLoadRejectsConfigSymlinkEscape(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	body := strings.Replace(minimalConfig, "CFGB Example", "External", 1)
	if err := os.WriteFile(filepath.Join(outside, "cfgb.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "cfgb.yaml"), filepath.Join(dir, "cfgb.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("external cfgb.yaml was read")
	}
}

func TestLoadRejectsDuplicateKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, minimalConfig+"\nschemaVersion: 1\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("duplicate key was accepted")
	}
}

const minimalConfig = `
schemaVersion: 1
site:
  title: CFGB Example
  baseUrl: https://example.invalid
  defaultLocale: ja
  timezone: Asia/Tokyo
`

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "cfgb.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
