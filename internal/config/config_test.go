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
	if cfg.Search.Provider != "pagefind" || cfg.AI.Gateway != "cloudflare" || cfg.Deploy.ProductionBranch != "main" || !cfg.Security.PreviewAccess {
		t.Fatalf("defaults = %+v", cfg)
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
			want: "locales",
		},
		{
			name: "percent escape",
			body: localeConfig("ja", "  \"%2e%2e\":\n    label: Bad\n"),
			want: "locales",
		},
		{
			name: "dot segment",
			body: localeConfig("ja", "  \"..\":\n    label: Bad\n"),
			want: "locales",
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
			want: "locales",
		},
		{
			name: "missing map",
			body: strings.Replace(minimalConfig, "locales:\n  ja:\n    label: 日本語\n", "", 1),
			want: "locales",
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

func TestLoadSchemaAndTimezone(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, from, to, tail, field string
	}{
		{"negative count", "", "", "home:\n  latestPosts: -1\n", "latestPosts"},
		{"zero count", "", "", "home:\n  latestPosts: 0\n", "latestPosts"},
		{"large count", "", "", "home:\n  latestPosts: 51\n", "latestPosts"},
		{"string count", "", "", "home:\n  latestPosts: '5'\n", "latestPosts"},
		{"HTTP origin", "https://example.invalid", "http://example.invalid", "", "baseUrl"},
		{"origin path", "https://example.invalid", "https://example.invalid/blog", "", "baseUrl"},
		{"origin slash", "https://example.invalid", "https://example.invalid/", "", "baseUrl"},
		{"origin query", "https://example.invalid", "https://example.invalid?q=1", "", "baseUrl"},
		{"origin credentials", "https://example.invalid", "https://user@example.invalid", "", "baseUrl"},
		{"URI format", "https://example.invalid", "https://bad host", "", "baseUrl"},
		{"unknown timezone", "Asia/Tokyo", "Invalid/Timezone", "", "timezone"},
		{"machine timezone", "Asia/Tokyo", "Local", "", "timezone"},
		{"search provider", "", "", "search:\n  provider: other\n", "provider"},
		{"preview access", "", "", "security:\n  previewAccess: false\n", "previewAccess"},
		{"null section", "", "", "home: null\n", "home"},
		{"empty path", "", "", "content:\n  root: ''\n", "root"},
		{"hatena field", "", "", "hatena:\n  extra: true\n", "extra"},
		{"hatena locale ref", "", "", "hatena:\n  blogs:\n    - url: https://example.invalid\n      locale: '../ja'\n", "locale"},
		{"default locale ref", "defaultLocale: ja", "defaultLocale: '../ja'", "", "defaultLocale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := minimalConfig
			if tc.from != "" {
				body = strings.Replace(body, tc.from, tc.to, 1)
			}
			dir := t.TempDir()
			writeConfig(t, dir, body+"\n"+tc.tail)
			if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("Load = %v, want %s error", err, tc.field)
			}
		})
	}
	for _, tail := range []string{"home:\n  latestPosts: 1\n", "home:\n  latestPosts: 50\n", "security: {}\n", "security:\n  previewAccess: true\n", "content: {}\n", "deploy:\n  productionBranch: master\n"} {
		dir := t.TempDir()
		writeConfig(t, dir, minimalConfig+"\n"+tail)
		if cfg, err := Load(dir); err != nil || !cfg.Security.PreviewAccess {
			t.Fatalf("valid config %s: %v", tail, err)
		}
	}
}
