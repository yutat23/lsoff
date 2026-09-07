//go:build linux

package listen

import (
	"encoding/binary"
	"testing"
)

func TestParseHexAddrIPv4(t *testing.T) {
	addr, port, err := parseHexAddr("0100007F:1F90", false)
	if err != nil {
		t.Fatal(err)
	}
	if addr != "127.0.0.1" || port != 8080 {
		t.Fatalf("got %s:%d", addr, port)
	}
	addr, port, err = parseHexAddr("00000000:0016", false)
	if err != nil {
		t.Fatal(err)
	}
	if addr != "0.0.0.0" || port != 22 {
		t.Fatalf("got %s:%d", addr, port)
	}
}

func TestParseHexAddrIPv6(t *testing.T) {
	addr, port, err := parseHexAddr("00000000000000000000000000000000:0050", true)
	if err != nil {
		t.Fatal(err)
	}
	if addr != "::" || port != 80 {
		t.Fatalf("got %s:%d", addr, port)
	}
	addr, port, err = parseHexAddr("00000000000000000000000001000000:1F90", true)
	if err != nil {
		t.Fatal(err)
	}
	if addr != "::1" || port != 8080 {
		t.Fatalf("got %s:%d", addr, port)
	}
}

// /proc prints each 32-bit word of the address in host byte order, so the
// decode must swap only on little-endian hosts.
func TestParseHexAddrByteOrder(t *testing.T) {
	cases := []struct {
		name string
		in   string
		v6   bool
		le   string // expected on a little-endian host
		be   string // expected on a big-endian host
	}{
		{"ipv4 loopback", "0100007F:1F90", false, "127.0.0.1", "1.0.0.127"},
		{"ipv4 any", "00000000:0016", false, "0.0.0.0", "0.0.0.0"},
		{"ipv4 lan", "0A00A8C0:0050", false, "192.168.0.10", "10.0.168.192"},
		{"ipv6 any", "00000000000000000000000000000000:0050", true, "::", "::"},
		{"ipv6 loopback", "00000000000000000000000001000000:1F90", true, "::1", "::100:0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, port, err := parseHexAddrOrder(tc.in, tc.v6, binary.LittleEndian)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.le {
				t.Fatalf("little-endian host: got %q, want %q", got, tc.le)
			}
			got, _, err = parseHexAddrOrder(tc.in, tc.v6, binary.BigEndian)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.be {
				t.Fatalf("big-endian host: got %q, want %q", got, tc.be)
			}
			if port == 0 {
				t.Fatalf("port not parsed")
			}
		})
	}
}

// parseHexAddr must use the host's own byte order.
func TestParseHexAddrUsesNativeOrder(t *testing.T) {
	want, _, err := parseHexAddrOrder("0100007F:1F90", false, binary.NativeEndian)
	if err != nil {
		t.Fatal(err)
	}
	got, port, err := parseHexAddr("0100007F:1F90", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || port != 8080 {
		t.Fatalf("got %s:%d, want %s:8080", got, port, want)
	}
}

func TestParseHexAddrBadInput(t *testing.T) {
	if _, _, err := parseHexAddr("0100007F", false); err == nil {
		t.Fatal("want error: no port separator")
	}
	if _, _, err := parseHexAddr("0100:1F90", false); err == nil {
		t.Fatal("want error: short ipv4")
	}
	if _, _, err := parseHexAddr("0100007F:1F90", true); err == nil {
		t.Fatal("want error: short ipv6")
	}
	if _, _, err := parseHexAddr("ZZZZZZZZ:1F90", false); err == nil {
		t.Fatal("want error: not hex")
	}
}
