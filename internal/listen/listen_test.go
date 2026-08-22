package listen

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestFilterPort(t *testing.T) {
	in := []Entry{
		{Proto: TCP, Port: 80, Addr: "0.0.0.0", PID: 1, Name: "nginx"},
		{Proto: TCP, Port: 443, Addr: "0.0.0.0", PID: 1, Name: "nginx"},
		{Proto: UDP, Port: 80, Addr: "127.0.0.1", PID: 2, Name: "app"},
	}
	got := FilterPort(in, 80)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
}

func TestFilterProto(t *testing.T) {
	in := []Entry{
		{Proto: TCP, Port: 80},
		{Proto: UDP, Port: 53},
		{Proto: TCP, Port: 443},
	}
	tcp := FilterProto(in, true, false)
	if len(tcp) != 2 || tcp[0].Proto != TCP || tcp[1].Proto != TCP {
		t.Fatalf("%+v", tcp)
	}
	udp := FilterProto(in, false, true)
	if len(udp) != 1 || udp[0].Proto != UDP {
		t.Fatalf("%+v", udp)
	}
	all := FilterProto(in, false, false)
	if len(all) != 3 {
		t.Fatalf("%+v", all)
	}
}

func TestFilterHasPID(t *testing.T) {
	in := []Entry{
		{PID: 100, Port: 80},
		{PID: 0, Port: 22},
		{PID: -1, Port: 53},
		{PID: 200, Port: 443},
	}
	got := FilterHasPID(in)
	if len(got) != 2 || got[0].PID != 100 || got[1].PID != 200 {
		t.Fatalf("FilterHasPID: %+v", got)
	}
}

func TestUniquePIDs(t *testing.T) {
	in := []Entry{
		{PID: 10},
		{PID: 0},
		{PID: 10},
		{PID: 11},
	}
	got := UniquePIDs(in)
	if len(got) != 2 || got[0] != 10 || got[1] != 11 {
		t.Fatalf("%v", got)
	}
}

func TestFilterQuery(t *testing.T) {
	in := []Entry{
		{Proto: TCP, Port: 8080, Addr: "127.0.0.1", PID: 99, Name: "node", Path: "/usr/bin/node"},
		{Proto: UDP, Port: 53, Addr: "::", PID: 12, Name: "named", Path: "/usr/sbin/named"},
	}
	got := FilterQuery(in, "8080")
	if len(got) != 1 || got[0].Name != "node" {
		t.Fatalf("filter by port: %+v", got)
	}
	got = FilterQuery(in, "NAMED")
	if len(got) != 1 || got[0].Port != 53 {
		t.Fatalf("filter by name: %+v", got)
	}
	got = FilterQuery(in, "node 8080")
	if len(got) != 1 || got[0].Name != "node" {
		t.Fatalf("AND query: %+v", got)
	}
	got = FilterQuery(in, "node named")
	if len(got) != 0 {
		t.Fatalf("AND should miss: %+v", got)
	}
	in[0].Cmdline = "/usr/bin/node server.js"
	got = FilterQuery(in, "server.js")
	if len(got) != 1 || got[0].Name != "node" {
		t.Fatalf("filter by cmdline: %+v", got)
	}
	in[0].Project = "lsoff"
	in[0].Cwd = "/Users/me/lsoff"
	got = FilterQuery(in, "lsoff")
	if len(got) != 1 || got[0].Name != "node" {
		t.Fatalf("filter by project: %+v", got)
	}
}

func TestSortAndFormatTable(t *testing.T) {
	in := []Entry{
		{Proto: UDP, Port: 53, Addr: "127.0.0.1", PID: 2, Name: "named", Path: "/usr/sbin/named"},
		{Proto: TCP, Port: 22, Addr: "0.0.0.0", PID: 1, Name: "sshd", Path: "/usr/sbin/sshd"},
	}
	Sort(in)
	if in[0].Port != 22 {
		t.Fatalf("expected port 22 first, got %d", in[0].Port)
	}
	var buf bytes.Buffer
	if err := FormatTable(&buf, in); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "PROTO") || !strings.Contains(out, "sshd") || !strings.Contains(out, "/usr/sbin/sshd") {
		t.Fatalf("unexpected table:\n%s", out)
	}
	if !strings.Contains(out, "CMD") {
		t.Fatalf("missing CMD column:\n%s", out)
	}
	if !strings.Contains(out, "PROJECT") || !strings.Contains(out, "CWD") {
		t.Fatalf("missing PROJECT/CWD columns:\n%s", out)
	}
	buf.Reset()
	if err := FormatJSON(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("nil json: %s", buf.String())
	}
}

func TestNormalizeAddr(t *testing.T) {
	if got := normalizeAddr("::ffff:127.0.0.1"); got != "127.0.0.1" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeAddr("::1"); got != "::1" {
		t.Fatalf("got %q", got)
	}
}

func TestKillGuards(t *testing.T) {
	if err := Kill(Ident{PID: 0, Start: 1}); err == nil {
		t.Fatal("expected error for pid 0")
	}
	if err := Kill(Ident{PID: 1, Start: 1}); err == nil {
		t.Fatal("expected error for pid 1")
	}
	if err := Kill(Ident{PID: 99, Start: 0}); err == nil {
		t.Fatal("expected error for missing identity")
	}
}

func TestKillIdentityMismatch(t *testing.T) {
	ppid := os.Getppid()
	if ppid <= 1 {
		t.Skip("no parent pid")
	}
	err := Kill(Ident{PID: ppid, Start: 1})
	if err == nil {
		t.Fatal("expected identity mismatch or open error, not success")
	}
}

func TestUniqueIdents(t *testing.T) {
	in := []Entry{
		{PID: 10, Start: 100},
		{PID: 0, Start: 1},
		{PID: 10, Start: 100},
		{PID: 11, Start: 200},
	}
	got := UniqueIdents(in)
	if len(got) != 2 || got[0] != (Ident{PID: 10, Start: 100}) || got[1] != (Ident{PID: 11, Start: 200}) {
		t.Fatalf("%v", got)
	}
}

func TestSanitizeDisplay(t *testing.T) {
	if got := SanitizeDisplay("a\x1b[31mb"); got != "ab" {
		t.Fatalf("ansi: %q", got)
	}
	if got := SanitizeDisplay("a\tb\nc"); got != "a b c" {
		t.Fatalf("c0: %q", got)
	}
	if got := SanitizeDisplay("x\x1b]0;title\x07y"); got != "xy" {
		t.Fatalf("osc: %q", got)
	}
	raw := "secret\x1b[0m"
	var buf bytes.Buffer
	if err := FormatJSON(&buf, []Entry{{Cmdline: raw}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `\u001b`) && !strings.Contains(buf.String(), "\x1b") {
		t.Fatalf("json should keep raw cmdline: %s", buf.String())
	}
	buf.Reset()
	if err := FormatTable(&buf, []Entry{{Name: "a\x1b[31mb", Cmdline: "x\ty"}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "\x1b") {
		t.Fatalf("table leaked ansi:\n%s", buf.String())
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestFormatTableWriteError(t *testing.T) {
	err := FormatTable(errWriter{}, []Entry{{Proto: TCP, Port: 80, Name: "x"}})
	if err == nil {
		t.Fatal("expected write error")
	}
}

func TestSortByName(t *testing.T) {
	in := []Entry{
		{Proto: TCP, Port: 80, Name: "nginx", PID: 2},
		{Proto: TCP, Port: 22, Name: "sshd", PID: 1},
	}
	SortBy(in, SortName, false)
	if in[0].Name != "nginx" {
		t.Fatalf("asc: %+v", in)
	}
	SortBy(in, SortName, true)
	if in[0].Name != "sshd" {
		t.Fatalf("desc: %+v", in)
	}
}

func TestEndpoint(t *testing.T) {
	if got := Endpoint(Entry{Addr: "127.0.0.1", Port: 8080}); got != "127.0.0.1:8080" {
		t.Fatalf("got %q", got)
	}
	if got := Endpoint(Entry{Addr: "::1", Port: 80}); got != "[::1]:80" {
		t.Fatalf("got %q", got)
	}
}

func TestProjectName(t *testing.T) {
	if got := ProjectName("/Users/me/lsoff"); got != "lsoff" {
		t.Fatalf("got %q", got)
	}
	if got := ProjectName(`/Users/me/lsoff/`); got != "lsoff" {
		t.Fatalf("trailing slash: %q", got)
	}
	if got := ProjectName(""); got != "" {
		t.Fatalf("empty: %q", got)
	}
}

func TestSortByProject(t *testing.T) {
	in := []Entry{
		{Proto: TCP, Port: 80, Project: "zeta", PID: 2},
		{Proto: TCP, Port: 22, Project: "alpha", PID: 1},
	}
	SortBy(in, SortProject, false)
	if in[0].Project != "alpha" {
		t.Fatalf("asc: %+v", in)
	}
}
