package docker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoverWithClientPublishedPorts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/json" || r.URL.Query().Get("all") != "false" {
			t.Fatalf("unexpected request %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
{"Id":"abcdef0123456789","Names":["/web"],"Ports":[
 {"PrivatePort":4000,"PublicPort":7000,"Type":"tcp","IP":"0.0.0.0"},
 {"PrivatePort":5353,"PublicPort":5353,"Type":"udp","IP":"127.0.0.1"},
 {"PrivatePort":8080,"Type":"tcp"},
 {"PrivatePort":9000,"PublicPort":9001,"Type":"tcp","IP":"::"},
 {"PrivatePort":9002,"PublicPort":9002,"Type":"sctp","IP":"0.0.0.0"}
]},
{"Id":"123456789012","Names":["/other"],"Ports":[
 {"PrivatePort":4000,"PublicPort":7001,"Type":"tcp","IP":"127.0.0.1"},
 {"PrivatePort":4000,"PublicPort":7002,"Type":"tcp","IP":"192.0.2.1"}
]}
]`))
	}))
	defer server.Close()

	got, err := DiscoverWithClient(server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d published ports, want 5: %#v", len(got), got)
	}
	if got[0].ContainerID != "abcdef012345" || got[0].ContainerName != "web" || got[0].HostPort != 7000 || got[0].ContainerPort != 4000 || got[0].Protocol != "tcp" {
		t.Fatalf("wrong TCP mapping: %#v", got[0])
	}
	if got[1].HostIP != "127.0.0.1" || got[1].Protocol != "udp" {
		t.Fatalf("wrong UDP mapping: %#v", got[1])
	}
	if got[2].HostIP != "::" || got[2].HostPort != 9001 {
		t.Fatalf("wrong IPv6 mapping: %#v", got[2])
	}
	for _, p := range got {
		if p.HostPort == 8080 || p.HostPort == 9002 {
			t.Fatalf("unpublished/unsupported port was included: %#v", p)
		}
	}
}

func TestDiscoverWithClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	_, err := DiscoverWithClient(server.Client(), server.URL)
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("got error %v, want API status error", err)
	}
}
