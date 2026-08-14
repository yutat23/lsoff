package listen

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListFindsTCPListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	deadline := time.Now().Add(2 * time.Second)
	for {
		entries, err := List()
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Proto == TCP && int(e.Port) == port && e.PID == os.Getpid() {
				if e.Name == "" && e.Path == "" {
					t.Fatal("expected process name or path")
				}
				if e.Cmdline == "" {
					t.Fatal("expected cmdline")
				}
				wantCwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				if e.Cwd == "" {
					t.Fatal("expected cwd")
				}
				if e.Project == "" {
					t.Fatal("expected project")
				}
				wantBase := filepath.Base(wantCwd)
				gotBase := filepath.Base(e.Cwd)
				if e.Project != wantBase && e.Project != gotBase {
					t.Fatalf("project=%q cwd=%q want base of %q", e.Project, e.Cwd, wantCwd)
				}
				if e.Start == 0 {
					t.Fatal("expected process start identity")
				}
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("did not find listener on port %d (pid %d)", port, os.Getpid())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestListFindsUDPSocket(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()

	port := pc.LocalAddr().(*net.UDPAddr).Port
	deadline := time.Now().Add(2 * time.Second)
	for {
		entries, err := List()
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Proto == UDP && int(e.Port) == port && e.PID == os.Getpid() {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("did not find UDP socket on port %d (pid %d)", port, os.Getpid())
		}
		time.Sleep(50 * time.Millisecond)
	}
}
