//go:build linux

package listen

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func procStartToken(pid int) (uint64, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	return parseProcStatStart(b)
}

func parseProcStatStart(stat []byte) (uint64, error) {
	i := bytes.LastIndexByte(stat, ')')
	if i < 0 || i+2 >= len(stat) {
		return 0, fmt.Errorf("malformed /proc/stat")
	}
	fields := strings.Fields(string(stat[i+2:]))
	// After comm, field 3 is state; starttime is field 22 → index 19.
	if len(fields) < 20 {
		return 0, fmt.Errorf("malformed /proc/stat")
	}
	return strconv.ParseUint(fields[19], 10, 64)
}
