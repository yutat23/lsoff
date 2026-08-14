//go:build darwin

package listen

import (
	"bytes"
	"encoding/binary"
	"strings"

	"golang.org/x/sys/unix"
)

func procCmdline(pid int) string {
	raw, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return ""
	}
	return parseProcArgs2(raw)
}

func parseProcArgs2(buf []byte) string {
	if len(buf) < 4 {
		return ""
	}
	argc := int(int32(binary.LittleEndian.Uint32(buf[:4])))
	if argc <= 0 {
		return ""
	}
	rest := buf[4:]
	if i := bytes.IndexByte(rest, 0); i >= 0 {
		rest = rest[i+1:]
	}
	for len(rest) > 0 && rest[0] == 0 {
		rest = rest[1:]
	}
	args := make([]string, 0, argc)
	for i := 0; i < argc && len(rest) > 0; i++ {
		n := bytes.IndexByte(rest, 0)
		if n < 0 {
			if s := string(rest); s != "" {
				args = append(args, s)
			}
			break
		}
		if n > 0 {
			args = append(args, string(rest[:n]))
		}
		rest = rest[n+1:]
	}
	return strings.TrimSpace(strings.Join(args, " "))
}
