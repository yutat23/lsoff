package listen

import "testing"

func TestServiceName(t *testing.T) {
	if got := ServiceName(TCP, 80); got != "http" {
		t.Fatalf("80: %q", got)
	}
	if got := ServiceName(TCP, 5432); got != "postgres" {
		t.Fatalf("5432: %q", got)
	}
	if got := ServiceName(UDP, 53); got != "dns" {
		t.Fatalf("udp/53: %q", got)
	}
	if got := ServiceName(TCP, 8080); got != "http" {
		t.Fatalf("8080: %q", got)
	}
	if got := ServiceName(TCP, 3000); got != "" {
		t.Fatalf("3000 should be ambiguous, got %q", got)
	}
	if got := ServiceName(TCP, 7); got != "" {
		t.Fatalf("historic echo should be absent, got %q", got)
	}
}

func TestFilterQueryByService(t *testing.T) {
	in := []Entry{
		{Proto: TCP, Port: 5432, Name: "postgres"},
		{Proto: TCP, Port: 6379, Name: "redis-server"},
		{Proto: TCP, Port: 3000, Name: "node"},
		{Proto: TCP, Port: 7, Name: "echo"},
	}
	got := FilterQuery(in, "postgres")
	if len(got) != 1 || got[0].Port != 5432 {
		t.Fatalf("postgres: %+v", got)
	}
	got = FilterQuery(in, "redis")
	if len(got) != 1 || got[0].Port != 6379 {
		t.Fatalf("redis: %+v", got)
	}
	got = FilterQuery(in, "http")
	if len(got) != 0 {
		t.Fatalf("http should miss these: %+v", got)
	}
	got = FilterQuery(in, "grafana")
	if len(got) != 1 || got[0].Port != 3000 {
		t.Fatalf("grafana alias: %+v", got)
	}
	got = FilterQuery(in, "chargen")
	if len(got) != 0 {
		t.Fatalf("historic chargen: %+v", got)
	}
}
