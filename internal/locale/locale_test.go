package locale

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	cfgb "github.com/ymmt2005/cfgb"
)

func TestIdentifierContract(t *testing.T) {
	t.Parallel()
	if _, err := releaseCatalog(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ja", "en", "pt-BR", "zh-Hant", "zh-Hant-TW", "es-419"} {
		if !pattern.MatchString(id) {
			t.Fatalf("%s was rejected", id)
		}
	}
	for _, id := range []string{"", ".", "..", "../../escape", "%2e%2e", "en/us", "ja.", "EN", "pt-br", "en_US", "zh-hant"} {
		if pattern.MatchString(id) {
			t.Fatalf("%s was accepted", id)
		}
	}
}

func TestValidateConfiguredLocales(t *testing.T) {
	t.Parallel()
	ja := map[string]Entry{"ja": {Label: "日本語"}, "en": {Label: "English"}}
	if err := Validate(ja, "ja"); err != nil {
		t.Fatal(err)
	}
	if err := Validate(map[string]Entry{}, "ja"); err == nil || !strings.Contains(err.Error(), "nonempty") {
		t.Fatalf("empty map: %v", err)
	}
	unsafe := map[string]Entry{"../../escape": {Label: "Bad"}, "ja": {Label: "日本語"}}
	if err := Validate(unsafe, "ja"); err == nil || !strings.Contains(err.Error(), "path-safe") {
		t.Fatalf("unsafe: %v", err)
	}
	unsupported := map[string]Entry{"pt-BR": {Label: "Português"}, "ja": {Label: "日本語"}}
	err := Validate(unsupported, "ja")
	if err == nil || !strings.Contains(err.Error(), "not supported") || strings.Contains(err.Error(), "path-safe") {
		t.Fatalf("unsupported: %v", err)
	}
	blank := map[string]Entry{"ja": {Label: "  "}}
	if err := Validate(blank, "ja"); err == nil || !strings.Contains(err.Error(), "label") {
		t.Fatalf("blank label: %v", err)
	}
	if err := Validate(map[string]Entry{"ja": {Label: "日本語"}}, "en"); err == nil || !strings.Contains(err.Error(), "defaultLocale") {
		t.Fatalf("default: %v", err)
	}
}

func TestLanguageCatalogRequiresExactSupportedIdentifier(t *testing.T) {
	t.Parallel()
	for _, language := range []string{"EN", "JA", "en-US", "ja-JP", "pt-BR", "fr"} {
		if err := Validate(map[string]Entry{language: {Label: "Label"}}, language); err == nil {
			t.Fatalf("unsupported identifier %q was accepted as a catalog language", language)
		}
	}
}

func TestLocalePatternMatchesSchema(t *testing.T) {
	t.Parallel()
	cat, err := releaseCatalog()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := cfgb.FS.ReadFile("schemas/locale-id.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if schema.Pattern != cat.Identifier {
		t.Fatalf("schema pattern = %s, catalog = %s", schema.Pattern, cat.Identifier)
	}
	err = fs.WalkDir(cfgb.FS, "schemas", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(name, ".json") {
			return err
		}
		body, err := cfgb.FS.ReadFile(name)
		if err != nil {
			return err
		}
		if bytes.Contains(body, []byte(`"ja"`)) || bytes.Contains(body, []byte(`"en"`)) {
			t.Errorf("%s still names a release locale", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
