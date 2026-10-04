package config

import (
	"fmt"
	"time"
	_ "time/tzdata"

	"github.com/ymmt2005/cfgb/internal/locale"
)

// ValidateSite checks language and timezone settings before rendering uses them.
// It is separate from decoding: Load and LoadFile only read/decode configuration.
func (f *File) ValidateSite() error {
	if f.Site.Timezone == "Local" {
		return fmt.Errorf("cfgb.yaml site.timezone must be an IANA timezone, not Local")
	}
	if _, err := time.LoadLocation(f.Site.Timezone); err != nil {
		return fmt.Errorf("cfgb.yaml site.timezone: %w", err)
	}
	configured := make(map[string]locale.Entry, len(f.Locales))
	for key, item := range f.Locales {
		configured[key] = locale.Entry{Label: item.Label}
	}
	if err := locale.Validate(configured, f.Site.DefaultLocale); err != nil {
		return err
	}
	return nil
}
