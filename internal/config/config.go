// Package config locates and reads cfgb.yaml.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"

	"github.com/ymmt2005/cfgb/internal/textinput"
)

// IOError marks a filesystem failure while locating or reading configuration.
// Decoding and missing-discovery errors remain ordinary configuration errors.
type IOError struct{ Err error }

func (e *IOError) Error() string { return e.Err.Error() }
func (e *IOError) Unwrap() error { return e.Err }

// File is the typed site configuration decoded from cfgb.yaml.
type File struct {
	SchemaVersion int `yaml:"schemaVersion"`
	Site          struct {
		Title         string `yaml:"title"`
		Image         string `yaml:"image"`
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
	Hatena struct {
		Blogs []HatenaBlog `yaml:"blogs"`
	} `yaml:"hatena"`
	path string
	root string
}

// HatenaBlog is one configured source blog.
type HatenaBlog struct {
	URL    string `yaml:"url"`
	Locale string `yaml:"locale"`
}

// SummaryConfig selects one locale's summary model.
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
		return nil, &IOError{Err: err}
	}
	repo, err := repositoryRoot(start)
	if err != nil {
		return nil, &IOError{Err: err}
	}
	dir := start
	var file string
	for {
		candidate := filepath.Join(dir, "cfgb.yaml")
		// Select an existing entry, including a dangling symlink. Reading it
		// through the repository root must fail rather than select an ancestor.
		st, err := os.Lstat(candidate)
		if err == nil {
			if st.IsDir() {
				return nil, fmt.Errorf("configuration is a directory: %s", candidate)
			}
			file = candidate
			break
		}
		if !os.IsNotExist(err) {
			return nil, &IOError{Err: fmt.Errorf("find configuration: %w", err)}
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
	return load(repo, file)
}

// LoadFile loads exactly the selected configuration file, without ancestor
// discovery. Relative paths in it resolve from its directory. The repository
// containing that file remains the rooted input boundary.
func LoadFile(file string) (*File, error) {
	if file == "" {
		return nil, fmt.Errorf("configuration path must not be empty")
	}
	file, err := filepath.Abs(file)
	if err != nil {
		return nil, &IOError{Err: err}
	}
	repo, err := repositoryRoot(filepath.Dir(file))
	if err != nil {
		return nil, &IOError{Err: err}
	}
	return load(repo, file)
}

func load(repo, file string) (*File, error) {
	raw, err := readConfig(repo, file)
	if err != nil {
		return nil, err
	}
	raw, err = textinput.Normalize(raw)
	if err != nil {
		return nil, fmt.Errorf("cfgb.yaml: %w", err)
	}
	var cfg File
	// Seed defaults before decoding so explicitly supplied values remain intact.
	cfg.Content.Root = "src/content"
	cfg.Content.Topics = "src/data/topics.yaml"
	cfg.Content.Linkcards = "src/data/linkcards"
	cfg.Home.LatestPosts = 5
	cfg.Search.Provider = "pagefind"
	cfg.AI.Gateway = "cloudflare"
	cfg.Deploy.ProductionBranch = "main"
	cfg.Security.PreviewAccess = true
	if err := yaml.UnmarshalWithOptions(raw, &cfg, yaml.DisallowUnknownField()); err != nil {
		return nil, fmt.Errorf("cfgb.yaml: %w", err)
	}
	cfg.path = file
	cfg.root = repo
	return &cfg, nil
}

// readConfig reads cfgb.yaml through a root at the repository. Symlinks that
// stay inside that root are followed. A symlink that leaves the repository fails.
func readConfig(repo, file string) (raw []byte, err error) {
	defer func() {
		if err != nil {
			err = &IOError{Err: err}
		}
	}()
	root, err := os.OpenRoot(repo)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	rel, err := filepath.Rel(repo, file)
	if err != nil {
		return nil, err
	}
	if !filepath.IsLocal(rel) {
		return nil, fmt.Errorf("cfgb.yaml escapes the repository")
	}
	raw, err = root.ReadFile(filepath.ToSlash(rel))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return raw, nil
}

func repositoryRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("find repository root: %w", err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start, nil
		}
		dir = parent
	}
}
