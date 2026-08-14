//go:build windows

package listen

import (
	"encoding/binary"
	"testing"
)

func TestDecodeUNICODEInBuffer64(t *testing.T) {
	buf := make([]byte, 16+6)
	binary.LittleEndian.PutUint16(buf[0:2], 6)
	copy(buf[16:], []byte{'c', 0, 'm', 0, 'd', 0})
	if got := decodeUNICODEInBuffer(buf, 8); got != "cmd" {
		t.Fatalf("got %q", got)
	}
}

func TestDecodeUNICODEInBuffer32(t *testing.T) {
	buf := make([]byte, 8+6)
	binary.LittleEndian.PutUint16(buf[0:2], 6)
	copy(buf[8:], []byte{'c', 0, 'm', 0, 'd', 0})
	if got := decodeUNICODEInBuffer(buf, 4); got != "cmd" {
		t.Fatalf("got %q", got)
	}
}

func TestPebOffsets(t *testing.T) {
	p, cwd, cmd := pebOffsets(8)
	if p != 0x20 || cwd != 0x38 || cmd != 0x70 {
		t.Fatalf("64-bit offsets: %#x %#x %#x", p, cwd, cmd)
	}
	p, cwd, cmd = pebOffsets(4)
	if p != 0x10 || cwd != 0x24 || cmd != 0x40 {
		t.Fatalf("32-bit offsets: %#x %#x %#x", p, cwd, cmd)
	}
}
