//go:build darwin

package listen

import (
	"net"
	"testing"
)

// laddr builds the 16-byte local-address union the kernel stores for ip: an
// IPv6 address as is, an IPv4 one after 12 bytes of padding.
func laddr(t *testing.T, ip string) [16]byte {
	t.Helper()
	p := net.ParseIP(ip)
	if p == nil {
		t.Fatalf("bad ip %q", ip)
	}
	var b [16]byte
	if v4 := p.To4(); v4 != nil {
		copy(b[12:], v4)
		return b
	}
	copy(b[:], p.To16())
	return b
}

// A dual-stack IPv6 wildcard listener (AF_INET6 without IPV6_V6ONLY) has both
// version flags set. It must be reported as ::, like lsof and Linux do, and
// the libproc and sysctl paths must agree or mergeUnowned lists it twice.
func TestPCBAddrVflag(t *testing.T) {
	cases := []struct {
		name   string
		v4, v6 bool
		ip     string
		want   string
	}{
		{"ipv4 wildcard", true, false, "0.0.0.0", "0.0.0.0"},
		{"ipv4 loopback", true, false, "127.0.0.1", "127.0.0.1"},
		{"ipv6 only wildcard", false, true, "::", "::"},
		{"ipv6 only loopback", false, true, "::1", "::1"},
		{"dual-stack wildcard", true, true, "::", "::"},
		{"no flags", false, false, "::", "::"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var ini, inp uint8
			if c.v4 {
				ini |= iniIPv4
				inp |= inpIPv4
			}
			if c.v6 {
				ini |= iniIPv6
				inp |= inpIPv6
			}
			a := laddr(t, c.ip)

			lp := normalizeAddr(libprocListenAddr(ini, a))
			xp := normalizeAddr(xinpcbListenAddr(inp, a))
			if lp != c.want {
				t.Errorf("libproc path = %q, want %q", lp, c.want)
			}
			if xp != c.want {
				t.Errorf("sysctl path = %q, want %q", xp, c.want)
			}
		})
	}
}
