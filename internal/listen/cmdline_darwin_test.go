//go:build darwin

package listen

import "testing"

func TestParseProcArgs2(t *testing.T) {
	buf := make([]byte, 0, 64)
	buf = append(buf, 3, 0, 0, 0) // argc = 3
	buf = append(buf, "/usr/bin/node"...)
	buf = append(buf, 0, 0, 0)
	buf = append(buf, "/usr/bin/node"...)
	buf = append(buf, 0)
	buf = append(buf, "server.js"...)
	buf = append(buf, 0)
	buf = append(buf, "--port"...)
	buf = append(buf, 0)

	got := parseProcArgs2(buf)
	if got != "/usr/bin/node server.js --port" {
		t.Fatalf("got %q", got)
	}
}
