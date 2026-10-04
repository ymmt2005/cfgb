package textinput

import (
	"bytes"
	"testing"
)

func TestNormalize(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"", ""}, {"\ufeff日本語\r\n", "日本語\r\n"},
		{"text\ufeffinside �", "text\ufeffinside �"},
	} {
		got, err := Normalize([]byte(tc.raw))
		if err != nil || string(got) != tc.want {
			t.Fatalf("Normalize(%q) = %q, %v", tc.raw, got, err)
		}
	}
	for _, raw := range [][]byte{{0xff}, {0xe3, 0x81}, {0xc0, 0xaf}, {0xed, 0xa0, 0x80}, {0xff, 0xfe, 'a', 0}} {
		if _, err := Normalize(raw); err == nil {
			t.Fatalf("malformed UTF-8 accepted: %x", raw)
		}
	}
	raw := []byte("\ufeffbody")
	if err := Validate(raw); err != nil || !bytes.Equal(raw, []byte("\ufeffbody")) {
		t.Fatalf("segment validation changed BOM: %q, %v", raw, err)
	}
}
