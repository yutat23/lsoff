package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yutat23/lsoff/internal/listen"
)

func TestParseArgsEmpty(t *testing.T) {
	cfg, err := parseArgs(nil)
	if err != nil || cfg.port != nil || cfg.json || cfg.kill {
		t.Fatalf("%+v err=%v", cfg, err)
	}
}

func TestParseArgsPortAndFlags(t *testing.T) {
	cfg, err := parseArgs([]string{"-t", "--json", "8080"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.port == nil || *cfg.port != 8080 {
		t.Fatalf("port=%v", cfg.port)
	}
	if !cfg.tcp || cfg.udp || !cfg.json {
		t.Fatalf("%+v", cfg)
	}
}

func TestParseArgsPID(t *testing.T) {
	cfg, err := parseArgs([]string{"-p"})
	if err != nil || !cfg.pid {
		t.Fatalf("-p: %+v err=%v", cfg, err)
	}
	cfg, err = parseArgs([]string{"--pid"})
	if err != nil || !cfg.pid {
		t.Fatalf("--pid: %+v err=%v", cfg, err)
	}
	if _, err := parseArgs([]string{"--process"}); err == nil {
		t.Fatal("expected error: --process is not a flag")
	}
}

// stubEntries replaces the socket source so CLI behaviour is testable without
// depending on what the machine happens to be listening on.
func stubEntries(t *testing.T, entries []listen.Entry) {
	t.Helper()
	prev := listAll
	listAll = func() ([]listen.Entry, error) { return entries, nil }
	t.Cleanup(func() { listAll = prev })
}

func pidFixture() []listen.Entry {
	return []listen.Entry{
		{Proto: listen.TCP, Port: 2222, Addr: "0.0.0.0", PID: 0, Name: ""},
		{Proto: listen.TCP, Port: 8080, Addr: "127.0.0.1", PID: 41233, Name: "node", Project: "lsoff"},
	}
}

func TestRunPIDFiltersTable(t *testing.T) {
	stubEntries(t, pidFixture())
	var out, errw bytes.Buffer
	if err := run([]string{"-p"}, strings.NewReader(""), &out, &errw); err != nil {
		t.Fatalf("-p: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want header + 1 row, got %d lines:\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[1], "41233") || !strings.Contains(lines[1], "node") {
		t.Fatalf("pid row missing: %q", lines[1])
	}
	if strings.Contains(out.String(), "2222") {
		t.Fatalf("unknown-PID row not filtered:\n%s", out.String())
	}
}

func TestRunPIDFiltersJSON(t *testing.T) {
	stubEntries(t, pidFixture())
	var out, errw bytes.Buffer
	if err := run([]string{"-p", "-j"}, strings.NewReader(""), &out, &errw); err != nil {
		t.Fatalf("-p -j: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d: %s", len(got), out.String())
	}
	if got[0]["pid"].(float64) != 41233 || got[0]["port"].(float64) != 8080 {
		t.Fatalf("wrong entry kept: %s", out.String())
	}
}

// -p is a view filter, so an empty result is an empty view, not an error.
func TestRunPIDEmptyExitsZero(t *testing.T) {
	stubEntries(t, []listen.Entry{{Proto: listen.TCP, Port: 2222, Addr: "0.0.0.0", PID: 0}})

	var out, errw bytes.Buffer
	if err := run([]string{"-p"}, strings.NewReader(""), &out, &errw); err != nil {
		t.Fatalf("-p with no PID rows should exit 0: %v", err)
	}
	if !strings.Contains(out.String(), "PROTO") || strings.Contains(out.String(), "2222") {
		t.Fatalf("want an empty table:\n%s", out.String())
	}

	out.Reset()
	if err := run([]string{"-p", "-j"}, strings.NewReader(""), &out, &errw); err != nil {
		t.Fatalf("-p -j with no PID rows should exit 0: %v", err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("want []: %q", out.String())
	}
}

// A port or query is a lookup, so no match still exits 1 even with -p.
func TestRunPIDWithLookupStillExitsOne(t *testing.T) {
	stubEntries(t, pidFixture())
	for _, args := range [][]string{{"-p", "8081"}, {"-p", "-j", "8081"}, {"-p", "redis"}} {
		var out, errw bytes.Buffer
		err := run(args, strings.NewReader(""), &out, &errw)
		var ee *exitError
		if !errors.As(err, &ee) || ee.code != 1 {
			t.Fatalf("%v: err=%v, want exit 1", args, err)
		}
	}
}

func TestParseArgsKill(t *testing.T) {
	cfg, err := parseArgs([]string{"-k", "-y", "80"})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.kill || !cfg.yes || cfg.port == nil || *cfg.port != 80 {
		t.Fatalf("%+v", cfg)
	}
	if _, err := parseArgs([]string{"-k"}); err == nil {
		t.Fatal("expected error: -k needs a port")
	}
	if _, err := parseArgs([]string{"-k", "--json", "80"}); err == nil {
		t.Fatal("expected error: -k with --json")
	}
	if _, err := parseArgs([]string{"-y"}); err == nil {
		t.Fatal("expected error: -y without -k")
	}
}

func TestParseArgsInvalid(t *testing.T) {
	if _, err := parseArgs([]string{"8080", "80"}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := parseArgs([]string{"--nope"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseArgsQuery(t *testing.T) {
	cfg, err := parseArgs([]string{"nginx"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.query != "nginx" || cfg.port != nil {
		t.Fatalf("%+v", cfg)
	}
	cfg, err = parseArgs([]string{"-q", "node 8080", "-t"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.query != "node 8080" || !cfg.tcp {
		t.Fatalf("%+v", cfg)
	}
	cfg, err = parseArgs([]string{"--query=Chrome"})
	if err != nil || cfg.query != "Chrome" {
		t.Fatalf("%+v %v", cfg, err)
	}
}

func TestRunJSONNoMatch(t *testing.T) {
	var out, errw bytes.Buffer
	err := run([]string{"--json", "1"}, strings.NewReader(""), &out, &errw)
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 1 {
		t.Fatalf("err=%v", err)
	}
}

func TestRunJSONShape(t *testing.T) {
	var buf bytes.Buffer
	entries := []listen.Entry{
		{Proto: listen.TCP, Port: 8080, Addr: "127.0.0.1", PID: 9, Name: "node", Path: "/bin/node"},
	}
	if err := listen.FormatJSON(&buf, entries); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["proto"] != "tcp" || got[0]["port"].(float64) != 8080 {
		t.Fatalf("%s", buf.String())
	}
}

func TestConfirmKillYesNo(t *testing.T) {
	var errw bytes.Buffer
	ok, err := confirmKill(strings.NewReader("y\n"), &errw, []int{12})
	if err != nil || !ok {
		t.Fatalf("yes: ok=%v err=%v", ok, err)
	}
	ok, err = confirmKill(strings.NewReader("n\n"), &errw, []int{12})
	if err != nil || ok {
		t.Fatalf("no: ok=%v err=%v", ok, err)
	}
}

func TestRunHelpVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-h"}, strings.NewReader(""), &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("help: %s", out.String())
	}
	out.Reset()
	if err := run([]string{"-v"}, strings.NewReader(""), &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "lsoff ") {
		t.Fatalf("version: %s", out.String())
	}
}

// A bare number that cannot be a port is a typo, not a search term.
func TestParseArgsOutOfRangePort(t *testing.T) {
	for _, arg := range []string{"65536", "99999999999"} {
		cfg, err := parseArgs([]string{arg})
		if err == nil {
			t.Fatalf("%s: want error, got %+v", arg, cfg)
		}
		want := "invalid port \"" + arg + "\" (must be 0-65535)"
		if err.Error() != want {
			t.Fatalf("%s: got %q, want %q", arg, err.Error(), want)
		}
	}
	// Not all digits, so still a search term.
	cfg, err := parseArgs([]string{"8080a"})
	if err != nil || cfg.query != "8080a" || cfg.port != nil {
		t.Fatalf("8080a: %+v err=%v", cfg, err)
	}
	// The boundary values are still ports.
	for _, arg := range []string{"0", "65535"} {
		cfg, err := parseArgs([]string{arg})
		if err != nil || cfg.port == nil {
			t.Fatalf("%s: %+v err=%v", arg, cfg, err)
		}
	}
}

// An out-of-range port exits 2 (the generic error path), not 1.
func TestRunOutOfRangePortExitsTwo(t *testing.T) {
	stubEntries(t, pidFixture())
	var out, errw bytes.Buffer
	err := run([]string{"65536"}, strings.NewReader(""), &out, &errw)
	if err == nil {
		t.Fatal("want error")
	}
	var ee *exitError
	if errors.As(err, &ee) {
		t.Fatalf("want a plain error (exit 2), got exitError code %d", ee.code)
	}
	if !strings.Contains(err.Error(), "invalid port") {
		t.Fatalf("got %v", err)
	}
}

func stubKill(t *testing.T, fn func(listen.Ident) error) {
	t.Helper()
	prev := killOne
	killOne = fn
	t.Cleanup(func() { killOne = prev })
}

func killFixture() []listen.Entry {
	return []listen.Entry{
		{Proto: listen.TCP, Port: 8080, Addr: "127.0.0.1", PID: 11, Start: 1, Name: "node"},
		{Proto: listen.TCP, Port: 8080, Addr: "::1", PID: 22, Start: 2, Name: "node"},
	}
}

// A failure on one PID must not hide the ones that were killed.
func TestRunKillReportsEachPID(t *testing.T) {
	stubEntries(t, killFixture())
	stubKill(t, func(id listen.Ident) error {
		if id.PID == 11 {
			return errors.New("operation not permitted")
		}
		return nil
	})
	var out, errw bytes.Buffer
	err := run([]string{"-k", "-y", "8080"}, strings.NewReader(""), &out, &errw)
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 1 {
		t.Fatalf("err=%v, want exit 1", err)
	}
	if ee.msg != "" {
		t.Fatalf("msg should be empty (already printed), got %q", ee.msg)
	}
	got := errw.String()
	if !strings.Contains(got, "pid 11: operation not permitted\n") {
		t.Fatalf("missing failure line:\n%s", got)
	}
	if !strings.Contains(got, "killed pid 22\n") {
		t.Fatalf("missing success line:\n%s", got)
	}
	if i, j := strings.Index(got, "pid 11:"), strings.Index(got, "killed pid 22"); i > j {
		t.Fatalf("results out of order:\n%s", got)
	}
}

func TestRunKillAllSucceed(t *testing.T) {
	stubEntries(t, killFixture())
	var killed []int
	stubKill(t, func(id listen.Ident) error {
		killed = append(killed, id.PID)
		return nil
	})
	var out, errw bytes.Buffer
	if err := run([]string{"-k", "-y", "8080"}, strings.NewReader(""), &out, &errw); err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(killed) != 2 || killed[0] != 11 || killed[1] != 22 {
		t.Fatalf("killed=%v", killed)
	}
	if !strings.Contains(errw.String(), "killed pid 11\n") || !strings.Contains(errw.String(), "killed pid 22\n") {
		t.Fatalf("%s", errw.String())
	}
}

// Without -y the prompt still gates the kill.
func TestRunKillConfirmation(t *testing.T) {
	stubEntries(t, killFixture())
	var killed []int
	stubKill(t, func(id listen.Ident) error {
		killed = append(killed, id.PID)
		return nil
	})
	var out, errw bytes.Buffer
	if err := run([]string{"-k", "8080"}, strings.NewReader("y\n"), &out, &errw); err != nil {
		t.Fatalf("confirmed kill: %v", err)
	}
	if len(killed) != 2 {
		t.Fatalf("killed=%v", killed)
	}
	if !strings.Contains(errw.String(), "Kill 2 processes") {
		t.Fatalf("no prompt:\n%s", errw.String())
	}

	killed = nil
	out.Reset()
	errw.Reset()
	err := run([]string{"-k", "8080"}, strings.NewReader("n\n"), &out, &errw)
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 1 {
		t.Fatalf("declined: err=%v", err)
	}
	if len(killed) != 0 {
		t.Fatalf("declined but killed %v", killed)
	}
	if !strings.Contains(errw.String(), "cancelled") {
		t.Fatalf("%s", errw.String())
	}
}
