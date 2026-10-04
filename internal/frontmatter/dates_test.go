package frontmatter

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPrepareDates(t *testing.T) {
	for _, tc := range []struct{ zone, source, archive, utc string }{
		{"Asia/Tokyo", "2026-09-30T16:30:00Z", "2026/10", "2026-09-30T16:30:00Z"},
		{"America/New_York", "2026-01-01T01:00:00Z", "2025/12", "2026-01-01T01:00:00Z"},
		{"America/New_York", "2026-03-01T04:30:00Z", "2026/02", "2026-03-01T04:30:00Z"},
		{"America/New_York", "2026-04-01T04:30:00Z", "2026/04", "2026-04-01T04:30:00Z"},
		{"Asia/Kathmandu", "2026-09-30T20:00:00Z", "2026/10", "2026-09-30T20:00:00Z"},
		{"", "2026-10-01T01:30:00+09:00", "2026/09", "2026-09-30T16:30:00Z"},
		// Factory exists in Go's tzdata, not Node's Intl zone list.
		{"Factory", "2026-10-01T01:30:00+09:00", "2026/09", "2026-09-30T16:30:00Z"},
		// Historical offsets can contain seconds. Do not serialize a lossy offset.
		{"Asia/Tokyo", "1880-01-01T00:00:00Z", "1880/01", "1880-01-01T00:00:00Z"},
		{"Asia/Tokyo", "2026-09-30T23:59:59.123456789-04:00", "2026/10", "2026-10-01T03:59:59.123456789Z"},
	} {
		t.Run(tc.zone+tc.source, func(t *testing.T) {
			location, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			source, err := time.Parse(time.RFC3339Nano, tc.source)
			if err != nil {
				t.Fatal(err)
			}
			updated := source.Add(26 * time.Hour)
			index := Index{Posts: []Post{{Data: Metadata{PublishedAt: source, UpdatedAt: &updated}}, {Data: Metadata{PublishedAt: source}}}}
			index.PrepareDates(location)
			raw, err := json.Marshal(index)
			if err != nil {
				t.Fatal(err)
			}
			var decoded Index
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			post := decoded.Posts[0]
			if post.Archive.Year+"/"+post.Archive.Month != tc.archive ||
				post.Data.PublishedAt.Format(time.RFC3339Nano) != tc.utc ||
				!post.Data.PublishedAt.Equal(source) || !post.Data.UpdatedAt.Equal(updated) ||
				post.Data.PublishedAt.Location() != time.UTC || post.Data.UpdatedAt.Location() != time.UTC ||
				decoded.Posts[1].Data.UpdatedAt != nil {
				t.Fatalf("prepared metadata = %s", raw)
			}
			if source.Format(time.RFC3339Nano) != tc.source {
				t.Fatal("original timestamp changed")
			}
		})
	}
}
