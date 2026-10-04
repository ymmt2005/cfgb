package yamlutil

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml/parser"
)

func TestTagsBeforeConversion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, raw string
		invalid   bool
	}{
		{"scalar", "value: !custom Example\n", true},
		{"nested mapping", "value: {inner: !custom Example}\n", true},
		{"sequence", "value: [normal, !custom Example]\n", true},
		{"anchor", "value: &label !custom Example\nother: *label\n", true},
		{"mapping tag", "value: !custom {inner: Example}\n", true},
		{"verbatim", "value: !<tag:example.invalid,2026:label> Example\n", true},
		{"unknown reserved spelling", "value: !!custom Example\n", true},
		{"remapped handle", "%TAG !! tag:example.invalid,2026:\n---\nvalue: !!str Example\n", true},
		{"built-in tags", "value: !!map {text: !!str 42, number: !!int 42, fraction: !!float 1.5, bool: !!bool false, empty: !!null null, array: !!seq [Example]}\n", false},
		{"literal text", "value: '!custom Example' # !custom is a comment\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := parser.ParseBytes([]byte(tc.raw), 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(parsed.Docs) == 0 {
				t.Fatalf("unexpected document shape: %d", len(parsed.Docs))
			}
			// The parser represents a %TAG directive as a separate document.
			// Callers reject multiple documents; inspect each body here to test
			// the tag guard independently of that document-count check.
			for _, doc := range parsed.Docs {
				if doc.Body == nil {
					t.Fatal("unexpected empty document")
				}
				if err = ValidateTags(doc.Body); err != nil {
					break
				}
			}
			if (err != nil) != tc.invalid {
				t.Fatalf("ValidateTags = %v", err)
			}
			if err != nil && (!strings.Contains(err.Error(), "YAML tag") || !strings.Contains(err.Error(), "line")) {
				t.Fatalf("tag diagnostic lacks source context: %v", err)
			}
		})
	}
}
