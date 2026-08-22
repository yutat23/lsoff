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
	cfg, err = parseArgs([]string{"--process"})
	if err != nil || !cfg.pid {
		t.Fatalf("--process: %+v err=%v", cfg, err)
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
