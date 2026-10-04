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

func TestLoadSummaryEntries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		summary string
		wantErr bool
	}{
		{"valid", "  summary:\n    ja:\n      provider: example\n      model: summary-v1\n", false},
		{"offline-empty", "  summary: {}\n", false},
		{"misspelled-provider", "  summary:\n    ja:\n      provder: example\n      model: summary-v1\n", true},
		{"extra-field", "  summary:\n    ja:\n      provider: example\n      model: summary-v1\n      temperature: 0\n", true},
		{"scalar-entry", "  summary:\n    ja: example\n", true},
		{"list-entry", "  summary:\n    ja: [example]\n", true},
		{"null-entry", "  summary:\n    ja: null\n", true},
		{"missing-model", "  summary:\n    ja:\n      provider: example\n", true},
		{"empty-provider", "  summary:\n    ja:\n      provider: ''\n      model: summary-v1\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeConfig(t, dir, minimalConfig+"\nai:\n  enabled: false\n"+tc.summary)
			cfg, err := Load(dir)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Load error = %v, want error %v", err, tc.wantErr)
			}
			if tc.name == "valid" && (cfg.AI.Summary["ja"].Provider != "example" || cfg.AI.Summary["ja"].Model != "summary-v1") {
				t.Fatalf("summary = %#v", cfg.AI.Summary)
			}
		})
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
locales:
  ja:
    label: 日本語
`

func TestLoadRejectsLocaleProblems(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unsafe identifier",
			body: localeConfig("ja", "  \"../../escape\":\n    label: Bad\n"),
			want: "path-safe",
		},
		{
			name: "percent escape",
			body: localeConfig("ja", "  \"%2e%2e\":\n    label: Bad\n"),
			want: "path-safe",
		},
		{
			name: "dot segment",
			body: localeConfig("ja", "  \"..\":\n    label: Bad\n"),
			want: "path-safe",
		},
		{
			name: "unsupported",
			body: localeConfig("ja", "  pt-BR:\n    label: Português\n"),
			want: "not supported",
		},
		{
			name: "script tag",
			body: localeConfig("ja", "  zh-Hant:\n    label: 中文\n"),
			want: "not supported",
		},
		{
			name: "empty map",
			body: strings.Replace(minimalConfig, "locales:\n  ja:\n    label: 日本語\n", "locales: {}\n", 1),
			want: "nonempty",
		},
		{
			name: "missing map",
			body: strings.Replace(minimalConfig, "locales:\n  ja:\n    label: 日本語\n", "", 1),
			want: "nonempty",
		},
		{
			name: "blank label",
			body: strings.Replace(minimalConfig, "label: 日本語", "label: \" \"", 1),
			want: "label",
		},
		{
			name: "default absent",
			body: strings.Replace(minimalConfig, "defaultLocale: ja", "defaultLocale: en", 1),
			want: "defaultLocale",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeConfig(t, dir, tc.body)
			_, err := Load(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func localeConfig(defaultLocale, extra string) string {
	return `
schemaVersion: 1
site:
  title: CFGB Example
  baseUrl: https://example.invalid
  defaultLocale: ` + defaultLocale + `
  timezone: Asia/Tokyo
locales:
  ja:
    label: 日本語
` + extra
}

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "cfgb.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsInvalidEncodingAndExtraDocuments(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"\ufeff" + minimalConfig,
		strings.Replace(minimalConfig, "CFGB Example", "Bad\xff", 1),
		minimalConfig + "\n---\nschemaVersion: 1\n",
		minimalConfig + "\n---\n",
	} {
		dir := t.TempDir()
		writeConfig(t, dir, body)
		if _, err := Load(dir); err == nil {
			t.Fatalf("invalid config accepted: %q", body)
		}
	}
}
