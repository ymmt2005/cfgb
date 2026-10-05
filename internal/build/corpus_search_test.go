package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
)

type corpusSearchQueries struct {
	SchemaVersion int `yaml:"schemaVersion" json:"schemaVersion"`
	Cases         []struct {
		Locale   string            `yaml:"locale" json:"locale"`
		Query    string            `yaml:"query" json:"query"`
		TopK     int               `yaml:"topK" json:"topK"`
		Expected []string          `yaml:"expected" json:"expected"`
		Filters  map[string]string `yaml:"filters" json:"filters,omitempty"`
	} `yaml:"cases" json:"cases"`
}

// Reuse the completed normal/static artifacts. Only the CI Chromium job opts
// in; when requested, missing fixtures, browser dependencies or WASM fail.
func checkExampleSearch(t *testing.T, want corpusExpectations) {
	t.Helper()
	if os.Getenv("CFGB_REQUIRE_SEARCH") != "1" {
		return
	}
	t.Run("Pagefind query corpus", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join(want.Root, "tests", "search", "queries.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var queries corpusSearchQueries
		if err := yaml.Unmarshal(raw, &queries); err != nil {
			t.Fatal(err)
		}
		input, err := json.Marshal(struct {
			corpusSearchQueries
			Corpus corpusExpectations `json:"corpus"`
		}{queries, want})
		if err != nil {
			t.Fatal(err)
		}
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("cannot locate search acceptance helper")
		}
		script := filepath.Join(filepath.Dir(file), "..", "..", "renderer", "tests", "browser", "corpus-search.mjs")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "node", script)
		cmd.Stdin = bytes.NewReader(input)
		output, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() != nil {
				err = errors.Join(err, ctx.Err())
			}
			t.Fatalf("Pagefind browser acceptance: %v\n%s", err, output)
		}
		t.Logf("%s", output)
	})
}
