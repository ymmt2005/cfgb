package config

import (
	"os"
	"path/filepath"
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
