// Package textinput handles source-file encoding before decoding or rendering.
package textinput

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

// Normalize removes a file-start UTF-8 BOM and rejects malformed UTF-8.
// Other characters, including an interior U+FEFF, and line endings are retained.
func Normalize(raw []byte) ([]byte, error) {
	if err := Validate(raw); err != nil {
		return nil, err
	}
	return bytes.TrimPrefix(raw, []byte("\ufeff")), nil
}

// Validate checks a text segment without changing its contents.
func Validate(raw []byte) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("invalid UTF-8")
	}
	return nil
}
