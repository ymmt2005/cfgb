package build

import (
	"errors"
	"fmt"
	"io"
)

// copyAndClose owns dst. A successful copy does not imply a successful close:
// filesystems can report a delayed write failure when the destination closes.
// Always close it, preserving both failures if copying also failed.
func copyAndClose(dst io.WriteCloser, src io.Reader) error {
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		copyErr = fmt.Errorf("copy file: %w", copyErr)
	}
	if closeErr != nil {
		closeErr = fmt.Errorf("close copied file: %w", closeErr)
	}
	return errors.Join(copyErr, closeErr)
}
