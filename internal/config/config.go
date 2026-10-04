// Package config locates and reads cfgb.yaml.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/ymmt2005/cfgb/internal/locale"
)

// File is the subset of cfgb.yaml the renderer needs.
type File struct {
	SchemaVersion int `yaml:"schemaVersion"`
	Site          struct {
		Title         string `yaml:"title"`
		BaseURL       string `yaml:"baseUrl"`
		DefaultLocale string `yaml:"defaultLocale"`
		Timezone      string `yaml:"timezone"`
	} `yaml:"site"`
	Locales map[string]struct {
		Label string `yaml:"label"`
	} `yaml:"locales"`
	Content struct {
		Root      string `yaml:"root"`
		Topics    string `yaml:"topics"`
		Linkcards string `yaml:"linkcards"`
	} `yaml:"content"`
	Home struct {
		LatestPosts int `yaml:"latestPosts"`
	} `yaml:"home"`
	Search struct {
		Provider string `yaml:"provider"`
	} `yaml:"search"`
	AI struct {
		Enabled bool                     `yaml:"enabled"`
		Gateway string                   `yaml:"gateway"`
		Summary map[string]SummaryConfig `yaml:"summary"`
	} `yaml:"ai"`
	Deploy struct {
		ProductionBranch string `yaml:"productionBranch"`
	} `yaml:"deploy"`
	Security struct {
		PreviewAccess bool `yaml:"previewAccess"`
	} `yaml:"security"`
	Hatena ast.Node `yaml:"hatena"`
	path   string
	root   string
}

// SummaryConfig is the structural contract for one locale's summary model.
type SummaryConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// Path is the cfgb.yaml file.
func (f *File) Path() string { return f.path }

// Root is the content repository root.
func (f *File) Root() string { return f.root }

// Load finds cfgb.yaml from start and parses it.
func Load(start string) (*File, error) {
	start, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	repo := repositoryRoot(start)
	dir := start
	var file string
	for {
		candidate := filepath.Join(dir, "cfgb.yaml")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			file = candidate
			break
		}
		if dir == repo {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if file == "" {
		return nil, fmt.Errorf("cfgb.yaml not found")
	}
	raw, err := readConfig(repo, file)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(raw) || bytes.HasPrefix(raw, []byte("\ufeff")) {
		return nil, fmt.Errorf("cfgb.yaml must be UTF-8 without a BOM")
	}
	parsed, err := parser.ParseBytes(raw, 0)
	if err != nil {
		return nil, fmt.Errorf("cfgb.yaml: %w", err)
	}
	if len(parsed.Docs) != 1 || parsed.Docs[0].Body == nil {
		return nil, fmt.Errorf("cfgb.yaml must contain a single nonempty YAML document")
	}
	var cfg File
	if err := yaml.NodeToValue(parsed.Docs[0].Body, &cfg, yaml.DisallowUnknownField()); err != nil {
		return nil, fmt.Errorf("cfgb.yaml: %w", err)
	}
	if cfg.SchemaVersion != 1 {
		return nil, fmt.Errorf("cfgb.yaml schemaVersion must be 1")
	}
	for locale, summary := range cfg.AI.Summary {
		if summary.Provider == "" || summary.Model == "" {
			return nil, fmt.Errorf("cfgb.yaml ai.summary.%s requires provider and model", locale)
		}
	}
	if cfg.Site.Title == "" || cfg.Site.BaseURL == "" || cfg.Site.DefaultLocale == "" || cfg.Site.Timezone == "" {
		return nil, fmt.Errorf("cfgb.yaml site title, baseUrl, defaultLocale and timezone are required")
	}
	configured := make(map[string]locale.Entry, len(cfg.Locales))
	for key, item := range cfg.Locales {
		configured[key] = locale.Entry{Label: item.Label}
	}
	if err := locale.Validate(configured, cfg.Site.DefaultLocale); err != nil {
		return nil, err
	}
	if cfg.Content.Root == "" {
		cfg.Content.Root = "src/content"
	}
	if cfg.Content.Topics == "" {
		cfg.Content.Topics = "src/data/topics.yaml"
	}
	if cfg.Content.Linkcards == "" {
		cfg.Content.Linkcards = "src/data/linkcards"
	}
	if cfg.Home.LatestPosts == 0 {
		cfg.Home.LatestPosts = 5
	}
	cfg.path = file
	cfg.root = repo
	return &cfg, nil
}

// readConfig reads cfgb.yaml through a root at the repository. Symlinks that
// stay inside that root are followed. A symlink that leaves the repository fails.
func readConfig(repo, file string) ([]byte, error) {
	root, err := os.OpenRoot(repo)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	rel, err := filepath.Rel(repo, file)
	if err != nil {
		return nil, err
	}
	if !filepath.IsLocal(rel) {
		return nil, fmt.Errorf("cfgb.yaml escapes the repository")
	}
	raw, err := root.ReadFile(filepath.ToSlash(rel))
	if err != nil {
		return nil, fmt.Errorf("cfgb.yaml: %w", err)
	}
	return raw, nil
}

func repositoryRoot(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}
