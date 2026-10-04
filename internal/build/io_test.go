package build

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

type failingWriteCloser struct {
	data               bytes.Buffer
	writeErr, closeErr error
	closes             int
}

func (w *failingWriteCloser) Write(p []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.data.Write(p)
}

func (w *failingWriteCloser) Close() error {
	w.closes++
	return w.closeErr
}

func TestCopyReportsDelayedCloseAndCopyFailures(t *testing.T) {
	t.Parallel()
	readErr := errors.New("source read failed")
	writeErr := errors.New("destination write failed")
	closeErr := errors.New("delayed destination close failure")
	for _, tc := range []struct {
		name                        string
		readErr, writeErr, closeErr error
	}{
		{name: "success"},
		{name: "close only", closeErr: closeErr},
		{name: "read and close", readErr: readErr, closeErr: closeErr},
		{name: "write and close", writeErr: writeErr, closeErr: closeErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dest := &failingWriteCloser{writeErr: tc.writeErr, closeErr: tc.closeErr}
			var source io.Reader = strings.NewReader("copied bytes")
			if tc.readErr != nil {
				source = io.MultiReader(source, iotest.ErrReader(tc.readErr))
			}
			err := copyAndClose(dest, source)
			for _, want := range []error{tc.readErr, tc.writeErr, tc.closeErr} {
				if want != nil && !errors.Is(err, want) {
					t.Fatalf("copy error = %v, missing %v", err, want)
				}
			}
			if tc.readErr == nil && tc.writeErr == nil && tc.closeErr == nil && err != nil {
				t.Fatal(err)
			}
			if dest.closes != 1 {
				t.Fatalf("destination closed %d times", dest.closes)
			}
			if tc.writeErr == nil && dest.data.String() != "copied bytes" {
				t.Fatalf("copied bytes = %q", &dest.data)
			}
		})
	}
}
