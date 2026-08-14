//go:build linux

package listen

import (
	"os"
	"testing"
)

func TestParseProcStatStart(t *testing.T) {
	stat := []byte("10 (a b) S 1 1 1 0 -1 4194304 0 0 0 0 0 0 0 0 20 0 1 0 99 0")
	got, err := parseProcStatStart(stat)
	if err != nil {
		t.Fatal(err)
	}
	if got != 99 {
		t.Fatalf("got %d, want 99", got)
	}
}

func TestProcStartTokenSelf(t *testing.T) {
	got, err := procStartToken(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if got == 0 {
		t.Fatal("expected non-zero starttime")
	}
}
