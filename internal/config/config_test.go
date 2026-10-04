package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSelectedFile(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, repo, strings.Replace(minimalConfig, "CFGB Example", "Default", 1))
	dir := filepath.Join(repo, "settings")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "blog.yaml")
	if err := os.WriteFile(file, []byte(strings.Replace(minimalConfig, "CFGB Example", "Selected", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Site.Title != "Selected" || cfg.Path() != file || cfg.Root() != repo {
		t.Fatalf("selected config = %#v", cfg)
	}
	for _, invalid := range []string{"", filepath.Join(dir, "missing.yaml"), dir} {
		if _, err := LoadFile(invalid); err == nil {
			t.Fatalf("selected %q silently fell back to ancestor configuration", invalid)
		}
	}
}

func TestSelectedFileSymlinkBoundary(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(repo, "inside.yaml")
	if err := os.WriteFile(inside, []byte(minimalConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(repo, "selected.yaml")
	if err := os.Symlink("inside.yaml", link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if cfg, err := LoadFile(link); err != nil || cfg.Path() != link {
		t.Fatalf("in-repository symlink = %v, %v", cfg, err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	if err := os.WriteFile(outside, []byte(minimalConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(link); err == nil {
		t.Fatal("selected configuration symlink escaped its repository")
	}
}

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
	if len(cfg.Hatena.Blogs) != 1 || cfg.Hatena.Blogs[0].URL != "https://example.invalid" || cfg.Hatena.Blogs[0].Locale != "ja" {
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
		{"null-entry", "  summary:\n    ja: null\n", false},
		{"missing-model", "  summary:\n    ja:\n      provider: example\n", false},
		{"empty-provider", "  summary:\n    ja:\n      provider: ''\n      model: summary-v1\n", false},
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

func TestDiscoveryDoesNotSkipBrokenConfigurationEntries(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"directory", "symlink loop", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			repo := t.TempDir()
			writeConfig(t, repo, minimalConfig)
			if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(repo, "nested")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			selected := filepath.Join(dir, "cfgb.yaml")
			if kind == "directory" {
				if err := os.Mkdir(selected, 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				target := "cfgb.yaml"
				if kind == "dangling symlink" {
					target = "missing.yaml"
				}
				if err := os.Symlink(target, selected); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), selected) {
				t.Fatalf("broken entry silently selected ancestor config: %v", err)
			}
		})
	}
}

func TestDiscoveryReportsRepositoryProbeError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, minimalConfig)
	file := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(file, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(file); err == nil || !strings.Contains(err.Error(), "find repository root") {
		t.Fatalf("repository probe failure was not reported: %v", err)
	}
}

// YAML syntax is the decoder's responsibility, without a separate AST policy.
func TestLoadUsesYAMLDecoderBehavior(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		strings.Replace(minimalConfig, "title: CFGB Example", "title: !custom CFGB Example", 1),
		minimalConfig + "\n---\nignored: second document\n",
		minimalConfig + "\n---\n",
	} {
		dir := t.TempDir()
		writeConfig(t, dir, body)
		cfg, err := Load(dir)
		if err != nil || cfg.Site.Title != "CFGB Example" {
			t.Fatalf("decoder-compatible config = %v, %v", cfg, err)
		}
	}
}

func TestLoadDoesNotRequireSchemaFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "schemaVersion: 2\n")
	cfg, err := Load(dir)
	if err != nil || cfg.SchemaVersion != 2 || cfg.Site.Title != "" || cfg.Home.LatestPosts != 5 {
		t.Fatalf("decodable configuration was schema-gated: %v, %v", cfg, err)
	}
}

func TestValidateSiteRejectsLocaleProblems(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unsafe identifier",
			body: localeConfig("ja", "  \"../../escape\":\n    label: Bad\n"),
			want: "locale",
		},
		{
			name: "percent escape",
			body: localeConfig("ja", "  \"%2e%2e\":\n    label: Bad\n"),
			want: "locale",
		},
		{
			name: "dot segment",
			body: localeConfig("ja", "  \"..\":\n    label: Bad\n"),
			want: "locale",
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
			cfg, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			err = cfg.ValidateSite()
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

func TestLoadRejectsDecodeErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body, field string }{
		{"malformed YAML", minimalConfig + "\nhome: [\n", "cfgb.yaml"},
		{"incompatible type", minimalConfig + "\nhome: [one, two]\n", "home"},
		{"hatena unknown field", minimalConfig + "\nhatena:\n  extra: true\n", "extra"},
		{"hatena blog unknown field", minimalConfig + "\nhatena:\n  blogs:\n    - url: https://example.invalid\n      locale: ja\n      extra: true\n", "extra"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeConfig(t, dir, tc.body)
			if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("Load = %v, want %s error", err, tc.field)
			}
		})
	}
}

func TestLoadPreservesExplicitValuesWithoutSchemaValidation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, strings.Replace(minimalConfig, "https://example.invalid", "http://example.invalid/blog", 1)+`
content:
  root: ''
home:
  latestPosts: 0
search:
  provider: other
deploy:
  productionBranch: master
security:
  previewAccess: false
ai:
  enabled: false
  summary:
    ja: {provider: example}
`)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Site.BaseURL != "http://example.invalid/blog" || cfg.Content.Root != "" || cfg.Home.LatestPosts != 0 || cfg.Search.Provider != "other" || cfg.Deploy.ProductionBranch != "master" || cfg.Security.PreviewAccess || cfg.AI.Summary["ja"].Model != "" {
		t.Fatalf("explicit values were rejected or replaced: %+v", cfg)
	}
}

func TestTimezoneChecksAreSeparateFromDecoding(t *testing.T) {
	t.Parallel()
	for _, timezone := range []string{"Invalid/Timezone", "Local"} {
		dir := t.TempDir()
		writeConfig(t, dir, strings.Replace(minimalConfig, "Asia/Tokyo", timezone, 1))
		cfg, err := Load(dir)
		if err != nil || cfg.Site.Timezone != timezone {
			t.Fatalf("decodable timezone was rejected: %v, %v", cfg, err)
		}
		if err := cfg.ValidateSite(); err == nil || !strings.Contains(err.Error(), "timezone") {
			t.Fatalf("invalid render timezone accepted: %v", err)
		}
	}
}

func TestLoadingDistinguishesIOFromConfigurationErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, tc := range []struct {
		name, body string
		read       bool
	}{
		{"missing.yaml", "", true},
		{"invalid.yaml", "site: [invalid]\n", false},
	} {
		file := filepath.Join(dir, tc.name)
		if tc.body != "" {
			if err := os.WriteFile(file, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, err := LoadFile(file)
		var ioErr *IOError
		if err == nil || errors.As(err, &ioErr) != tc.read {
			t.Fatalf("%s: %v, want IO=%v", tc.name, err, tc.read)
		}
		if tc.read && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("lost I/O cause: %v", err)
		}
	}
	_, err := Load(dir)
	var ioErr *IOError
	if err == nil || errors.As(err, &ioErr) {
		t.Fatalf("missing discovery should remain config error: %v", err)
	}
}

func TestConfigurationEncoding(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "\ufeff"+minimalConfig)
	if _, err := Load(dir); err != nil {
		t.Fatalf("BOM: %v", err)
	}
	writeConfig(t, dir, minimalConfig+"# malformed \xff\n")
	_, err := Load(dir)
	var ioErr *IOError
	if err == nil || !strings.Contains(err.Error(), "invalid UTF-8") || errors.As(err, &ioErr) {
		t.Fatalf("encoding error = %v", err)
	}
}
