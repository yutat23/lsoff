//go:build !linux && !darwin && !windows

package listen

import (
	"fmt"
	"runtime"
)

// List is not implemented on this OS.
func List() ([]Entry, error) {
	return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
}
