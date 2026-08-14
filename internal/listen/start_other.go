//go:build !linux && !darwin && !windows

package listen

import "fmt"

func procStartToken(pid int) (uint64, error) {
	return 0, fmt.Errorf("process identity not supported")
}
