//go:build linux

package listen

import "testing"

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
