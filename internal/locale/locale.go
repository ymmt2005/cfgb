// Package locale checks locale identifiers against the shared release catalog.
package locale

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	cfgb "github.com/ymmt2005/cfgb"
)

// Entry is one configured locale label.
type Entry struct {
	Label string
}

type catalog struct {
	Identifier string                     `json:"identifier"`
	Locales    map[string]json.RawMessage `json:"locales"`
}

var (
	loadOnce sync.Once
	loaded   catalog
	loadErr  error
)

// Validate checks configured languages before site rendering.
// Support is exact, case-sensitive membership in the shared catalog.
// This release defines ja and en.
func Validate(configured map[string]Entry, defaultLocale string) error {
	cat, err := releaseCatalog()
	if err != nil {
		return err
	}
	if len(configured) == 0 {
		return fmt.Errorf("cfgb.yaml locales must be a nonempty map")
	}
	keys := make([]string, 0, len(configured))
	for key := range configured {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := cat.Locales[key]; !ok {
			return fmt.Errorf("cfgb.yaml locale %q is not supported by this release (%s)", key, supportedList(cat))
		}
	}
	for _, key := range keys {
		if strings.TrimSpace(configured[key].Label) == "" {
			return fmt.Errorf("cfgb.yaml locale %q requires a nonempty label", key)
		}
	}
	if _, ok := configured[defaultLocale]; !ok {
		return fmt.Errorf("cfgb.yaml defaultLocale %q is not one of the configured locales", defaultLocale)
	}
	return nil
}

func releaseCatalog() (catalog, error) {
	loadOnce.Do(func() {
		raw, err := cfgb.FS.ReadFile("renderer/src/lib/locales.json")
		if err != nil {
			loadErr = fmt.Errorf("locale catalog: %w", err)
			return
		}
		if err := json.Unmarshal(raw, &loaded); err != nil {
			loadErr = fmt.Errorf("locale catalog: %w", err)
			return
		}
		if len(loaded.Locales) == 0 {
			loadErr = fmt.Errorf("locale catalog is empty")
			return
		}
	})
	return loaded, loadErr
}

func supportedList(cat catalog) string {
	keys := make([]string, 0, len(cat.Locales))
	for key := range cat.Locales {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
