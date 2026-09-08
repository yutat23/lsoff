package listen

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yutat23/lsoff/internal/docker"
)

var discoverDocker = docker.Discover

// listWithDocker supplements platform socket discovery with Docker's
// published-port metadata. Docker errors are intentionally non-fatal.
func listWithDocker(entries []Entry) []Entry {
	published, err := discoverDocker()
	if err != nil {
		return entries
	}
	var dockerEntries []Entry
	for _, p := range published {
		entry := Entry{
			Proto: TCP, Port: p.HostPort, Addr: normalizeAddr(p.HostIP),
			Name: "docker:" + p.ContainerName, Source: SourceDocker,
			ContainerID: p.ContainerID, ContainerPort: p.ContainerPort,
			ContainerProtocol: p.Protocol,
		}
		if p.Protocol == "udp" {
			entry.Proto = UDP
		}
		if entry.Name == "docker:" {
			entry.Name = "docker:" + p.ContainerID
		}
		dockerEntries = append(dockerEntries, entry)
	}
	return mergeDocker(entries, dockerEntries)
}

// mergeDocker gives a Docker publication precedence over an anonymous host
// row or a clearly identified docker-proxy row with the same listener
// identity. A real, identified host process is retained because the mapping
// may be unrelated to that process.
func mergeDocker(entries, dockerEntries []Entry) []Entry {
	dockerKeys := make(map[string]struct{}, len(dockerEntries))
	uniqueDocker := make([]Entry, 0, len(dockerEntries))
	for _, e := range dockerEntries {
		key := listenerIdentity(e)
		if _, seen := dockerKeys[key]; seen {
			continue
		}
		dockerKeys[key] = struct{}{}
		uniqueDocker = append(uniqueDocker, e)
	}
	if len(dockerKeys) == 0 {
		return entries
	}
	out := entries[:0]
	for _, e := range entries {
		if e.Source != SourceDocker {
			if _, ok := dockerKeys[listenerIdentity(e)]; ok && (e.PID <= 0 || isDockerProxy(e)) {
				continue
			}
		}
		out = append(out, e)
	}
	return append(out, uniqueDocker...)
}

func listenerIdentity(e Entry) string {
	return fmt.Sprintf("%s/%s/%d", e.Proto, normalizeAddr(e.Addr), e.Port)
}

func isDockerProxy(e Entry) bool {
	cmdline := strings.Fields(e.Cmdline)
	cmd := ""
	if len(cmdline) > 0 {
		cmd = filepath.Base(cmdline[0])
	}
	for _, value := range []string{e.Name, filepath.Base(e.Path), cmd} {
		if strings.EqualFold(strings.TrimSuffix(value, " (deleted)"), "docker-proxy") {
			return true
		}
	}
	return false
}
