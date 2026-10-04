package frontmatter

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLocalizeDates(t *testing.T) {
	for _, tc := range []struct{ zone, source, want string }{
		{"Asia/Tokyo", "2026-09-30T16:30:00Z", "2026-10-01T01:30:00+09:00"},
		{"America/New_York", "2026-01-01T01:00:00Z", "2025-12-31T20:00:00-05:00"},
		{"America/New_York", "2026-03-08T06:59:00Z", "2026-03-08T01:59:00-05:00"},
		{"America/New_York", "2026-03-08T07:00:00Z", "2026-03-08T03:00:00-04:00"},
		{"Asia/Kathmandu", "2026-09-30T20:00:00Z", "2026-10-01T01:45:00+05:45"},
		{"", "2026-10-01T01:30:00+09:00", "2026-09-30T16:30:00Z"},
		// Factory is in Go's bundled tzdata but not in Node's Intl zone list.
		{"Factory", "2026-10-01T01:30:00+09:00", "2026-09-30T16:30:00Z"},
	} {
		t.Run(tc.zone+tc.source, func(t *testing.T) {
			location, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			source, err := time.Parse(time.RFC3339, tc.source)
			if err != nil {
				t.Fatal(err)
			}
			index := Index{Posts: []Post{{Data: Metadata{PublishedAt: source, UpdatedAt: &source}}, {Data: Metadata{PublishedAt: source}}}}
			index.LocalizeDates(location)
			if !source.Equal(index.Posts[0].Data.PublishedAt) || source.Format(time.RFC3339) != tc.source {
				t.Fatal("instant or original timestamp changed")
			}
			raw, err := json.Marshal(index)
			if err != nil {
				t.Fatal(err)
			}
			var decoded Index
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			data := decoded.Posts[0].Data
			if data.PublishedAt.Format(time.RFC3339) != tc.want || data.UpdatedAt.Format(time.RFC3339) != tc.want || decoded.Posts[1].Data.UpdatedAt != nil {
				t.Fatalf("localized metadata = %s", raw)
			}
		})
	}
}
