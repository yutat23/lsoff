package listen

import (
	"errors"
	"testing"

	"github.com/yutat23/lsoff/internal/docker"
)

func TestListWithDockerMergesOnlyDockerProxy(t *testing.T) {
	old := discoverDocker
	discoverDocker = func() ([]docker.Published, error) {
		return []docker.Published{{ContainerID: "abcdef012345", ContainerName: "web", HostIP: "0.0.0.0", HostPort: 7000, ContainerPort: 4000, Protocol: "tcp"}}, nil
	}
	t.Cleanup(func() { discoverDocker = old })

	entries := []Entry{
		{Proto: TCP, Addr: "0.0.0.0", Port: 7000, PID: 10, Name: "docker-proxy"},
		{Proto: TCP, Addr: "127.0.0.1", Port: 7000, PID: 11, Name: "other-process"},
		{Proto: UDP, Addr: "0.0.0.0", Port: 7000, PID: 12, Name: "docker-proxy"},
	}
	got := listWithDocker(entries)
	if len(got) != 3 {
		t.Fatalf("got %d entries, want matching proxy suppressed plus unrelated rows and Docker row: %#v", len(got), got)
	}
	for _, e := range got {
		if e.PID == 10 {
			t.Fatalf("equivalent low-level row was not suppressed: %#v", e)
		}
	}
	var dockerEntry Entry
	for _, e := range got {
		if e.Source == SourceDocker {
			dockerEntry = e
		}
	}
	if dockerEntry.PID != 0 || dockerEntry.Name != "docker:web" || dockerEntry.ContainerID != "abcdef012345" {
		t.Fatalf("bad Docker entry: %#v", dockerEntry)
	}
}

func TestListWithDockerReplacesAnonymousWildcardMappings(t *testing.T) {
	old := discoverDocker
	discoverDocker = func() ([]docker.Published, error) {
		return []docker.Published{
			{ContainerID: "v4id", ContainerName: "v4", HostIP: "0.0.0.0", HostPort: 7000, ContainerPort: 4000, Protocol: "tcp"},
			{ContainerID: "v6id", ContainerName: "v6", HostIP: "::", HostPort: 7000, ContainerPort: 4000, Protocol: "tcp"},
		}, nil
	}
	t.Cleanup(func() { discoverDocker = old })

	got := listWithDocker([]Entry{
		{Proto: TCP, Addr: "0.0.0.0", Port: 7000},
		{Proto: TCP, Addr: "::ffff:0:0", Port: 7000},
		{Proto: TCP, Addr: "::", Port: 7000},
		{Proto: UDP, Addr: "0.0.0.0", Port: 7000},
		{Proto: TCP, Addr: "127.0.0.1", Port: 7000},
	})
	if len(got) != 4 {
		t.Fatalf("got %d entries, want two Docker rows plus unrelated anonymous rows: %#v", len(got), got)
	}
	for _, e := range got {
		if e.Source == SourceDocker && e.ContainerID == "" {
			t.Fatal("Docker metadata was lost")
		}
		if e.Source == SourceProcess && e.Proto == TCP && (e.Addr == "0.0.0.0" || e.Addr == "::ffff:0:0" || e.Addr == "::") {
			t.Fatalf("matching anonymous wildcard row survived: %#v", e)
		}
	}
}

func TestListWithDockerKeepsRealHostProcess(t *testing.T) {
	old := discoverDocker
	discoverDocker = func() ([]docker.Published, error) {
		return []docker.Published{{ContainerID: "id", ContainerName: "web", HostIP: "127.0.0.1", HostPort: 7000, ContainerPort: 4000, Protocol: "tcp"}}, nil
	}
	t.Cleanup(func() { discoverDocker = old })
	got := listWithDocker([]Entry{{Proto: TCP, Addr: "127.0.0.1", Port: 7000, PID: 42, Name: "web-server"}})
	if len(got) != 2 {
		t.Fatalf("real host process was merged away: %#v", got)
	}
}

func TestListWithDockerFailureIsNonFatal(t *testing.T) {
	old := discoverDocker
	discoverDocker = func() ([]docker.Published, error) { return nil, errors.New("Docker unavailable") }
	t.Cleanup(func() { discoverDocker = old })
	in := []Entry{{Proto: TCP, Addr: "127.0.0.1", Port: 8080, PID: 1}}
	got := listWithDocker(in)
	if len(got) != 1 || got[0] != in[0] {
		t.Fatalf("Docker failure changed host entries: %#v", got)
	}
}
