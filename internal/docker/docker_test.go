package docker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestSocketCandidatesUnixDockerHostIsOnlyCandidate(t *testing.T) {
	root := t.TempDir()
	got, err := socketCandidates("unix:///custom/docker.sock", root, root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/custom/docker.sock"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q without fallbacks", got, want)
	}
}

func TestSocketCandidatesNonUnixDockerHostSkips(t *testing.T) {
	root := t.TempDir()
	for _, host := range []string{
		"tcp://192.0.2.1:2375",
		"ssh://user@example.com",
		"npipe:////./pipe/docker_engine",
		"unix://",
		"/var/run/docker.sock",
		"garbage",
	} {
		got, err := socketCandidates(host, root, root)
		if !errors.Is(err, errSkipped) || got != nil {
			t.Fatalf("DOCKER_HOST=%q: got %q, %v; want skip", host, got, err)
		}
	}
}

func TestSocketCandidatesDefaults(t *testing.T) {
	xdg := filepath.Join(t.TempDir(), "run")
	home := filepath.Join(t.TempDir(), "home")
	system := filepath.Clean(DefaultSocket)
	xdgSock := filepath.Join(xdg, "docker.sock")
	desktop := filepath.Join(home, ".docker", "run", "docker.sock")
	tests := []struct {
		name, host, xdg, home string
		want                  []string
	}{
		{"all", "", xdg, home, []string{system, xdgSock, desktop}},
		{"blank DOCKER_HOST", "  ", xdg, home, []string{system, xdgSock, desktop}},
		{"no XDG_RUNTIME_DIR", "", "", home, []string{system, desktop}},
		{"relative XDG_RUNTIME_DIR", "", "run", home, []string{system, desktop}},
		{"no home", "", xdg, "", []string{system, xdgSock}},
		{"neither", "", "", "", []string{system}},
		{"XDG same as desktop", "", filepath.Join(home, ".docker", "run"), home, []string{system, desktop}},
		{"XDG same as system", "", filepath.Dir(system), home, []string{system, desktop}},
	}
	for _, tt := range tests {
		got, err := socketCandidates(tt.host, tt.xdg, tt.home)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestDiscoverFromReturnsFirstAnsweringSocket(t *testing.T) {
	var queried []string
	exists := func(path string) bool { return path != "missing" }
	query := func(_ context.Context, path string) ([]Published, error) {
		queried = append(queried, path)
		switch path {
		case "broken":
			return nil, errors.New("connection refused")
		case "good":
			return []Published{{ContainerName: "web", HostPort: 7000}}, nil
		}
		return []Published{{ContainerName: "later"}}, nil
	}
	got, err := discoverFrom(context.Background(), []string{"missing", "broken", "good", "later"}, exists, query)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ContainerName != "web" {
		t.Fatalf("got %#v, want the first answering socket's result", got)
	}
	if want := []string{"broken", "good"}; !reflect.DeepEqual(queried, want) {
		t.Fatalf("queried %q, want %q", queried, want)
	}
}

func TestDiscoverFromFailsWhenNothingAnswers(t *testing.T) {
	query := func(context.Context, string) ([]Published, error) { return nil, errors.New("refused") }
	if _, err := discoverFrom(context.Background(), []string{"a", "b"}, func(string) bool { return true }, query); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("got %v, want the query errors", err)
	}
	called := false
	query = func(context.Context, string) ([]Published, error) { called = true; return nil, nil }
	if _, err := discoverFrom(context.Background(), []string{"a", "b"}, func(string) bool { return false }, query); err == nil || called {
		t.Fatalf("got err %v, queried %v; want an error without queries", err, called)
	}
}

func TestDiscoverFromSharesOneDeadline(t *testing.T) {
	deadline := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	var seen []time.Time
	query := func(ctx context.Context, _ string) ([]Published, error) {
		d, ok := ctx.Deadline()
		if !ok {
			t.Fatal("query context has no deadline")
		}
		seen = append(seen, d)
		return nil, errors.New("refused")
	}
	_, _ = discoverFrom(ctx, []string{"a", "b", "c"}, func(string) bool { return true }, query)
	if want := []time.Time{deadline, deadline, deadline}; !reflect.DeepEqual(seen, want) {
		t.Fatalf("deadlines %v, want the shared %v for every candidate", seen, deadline)
	}
}

func TestDiscoverFromStopsWhenBudgetIsSpent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var queried []string
	query := func(_ context.Context, path string) ([]Published, error) {
		queried = append(queried, path)
		// Stands in for a hung socket that used up the whole budget.
		cancel()
		return nil, context.DeadlineExceeded
	}
	_, err := discoverFrom(ctx, []string{"hung", "next"}, func(string) bool { return true }, query)
	if err == nil {
		t.Fatal("got nil error, want budget exhaustion")
	}
	if want := []string{"hung"}; !reflect.DeepEqual(queried, want) {
		t.Fatalf("queried %q, want later candidates skipped once the budget is spent", queried)
	}
}

func stubSockets(t *testing.T, exists func(string) bool, query func(context.Context, string) ([]Published, error)) {
	t.Helper()
	oldExists, oldQuery := socketExists, querySocket
	socketExists, querySocket = exists, query
	t.Cleanup(func() { socketExists, querySocket = oldExists, oldQuery })
}

func TestDiscoverHonoursUnixDockerHost(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///custom/docker.sock")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	var checked []string
	stubSockets(t, func(path string) bool { checked = append(checked, path); return true },
		func(context.Context, string) ([]Published, error) { return nil, errors.New("refused") })
	if _, err := Discover(); err == nil {
		t.Fatal("got nil error, want failure without fallback")
	}
	if want := []string{"/custom/docker.sock"}; !reflect.DeepEqual(checked, want) {
		t.Fatalf("checked %q, want only %q", checked, want)
	}
}

func TestDiscoverSkipsRemoteDockerHost(t *testing.T) {
	for _, host := range []string{"tcp://192.0.2.1:2375", "ssh://user@example.com", "garbage"} {
		t.Setenv("DOCKER_HOST", host)
		stubSockets(t, func(path string) bool { t.Fatalf("DOCKER_HOST=%q: checked %q", host, path); return false },
			func(_ context.Context, path string) ([]Published, error) {
				t.Fatalf("DOCKER_HOST=%q: queried %q", host, path)
				return nil, nil
			})
		if _, err := Discover(); !errors.Is(err, errSkipped) {
			t.Fatalf("DOCKER_HOST=%q: got %v, want skip", host, err)
		}
	}
}

func TestDiscoverBoundsAllCandidatesByOneBudget(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	var deadlines []time.Time
	stubSockets(t, func(string) bool { return true }, func(ctx context.Context, _ string) ([]Published, error) {
		d, ok := ctx.Deadline()
		if !ok {
			t.Fatal("query context has no deadline")
		}
		deadlines = append(deadlines, d)
		return nil, errors.New("refused")
	})
	start := time.Now()
	if _, err := Discover(); err == nil {
		t.Fatal("got nil error, want failure")
	}
	if len(deadlines) < 2 {
		t.Fatalf("queried %d candidates, want at least the system and XDG sockets", len(deadlines))
	}
	for _, d := range deadlines {
		if !d.Equal(deadlines[0]) {
			t.Fatalf("deadlines %v differ; want one shared budget", deadlines)
		}
	}
	if d := deadlines[0]; d.Before(start) || d.After(time.Now().Add(discoveryBudget)) {
		t.Fatalf("deadline %v is not within %v of the call", d, discoveryBudget)
	}
}
