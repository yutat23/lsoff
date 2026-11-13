// Package docker reads published-port metadata from the local Docker Engine.
package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultSocket is the system-wide Engine socket, tried first when
// DOCKER_HOST is unset.
const DefaultSocket = "/var/run/docker.sock"

// discoveryBudget bounds the whole discovery, across every candidate
// socket. List runs on each refresh, so a hung socket must not add more
// than this, however many candidates there are.
const discoveryBudget = 750 * time.Millisecond

// errSkipped reports that DOCKER_HOST names an engine lsoff must not
// contact, so discovery is not attempted at all.
var errSkipped = errors.New("docker: DOCKER_HOST is not a local unix:// socket")

// Seams for tests; the real implementations touch the filesystem and dial.
var (
	socketExists = func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}
	querySocket = queryUnixSocket
)

// Published is a host port published by a running container.
type Published struct {
	ContainerID   string
	ContainerName string
	HostIP        string
	HostPort      uint16
	ContainerPort uint16
	Protocol      string
}

type container struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	Ports []port   `json:"Ports"`
}

type port struct {
	PrivatePort uint16 `json:"PrivatePort"`
	PublicPort  uint16 `json:"PublicPort"`
	Type        string `json:"Type"`
	IP          string `json:"IP"`
}

// Discover queries running containers through a local Engine Unix socket,
// resolved as described in socketCandidates. Results come from the first
// socket that answers; engines are never merged.
func Discover() ([]Published, error) {
	// An unknown home dir only drops the Docker Desktop candidate.
	home, _ := os.UserHomeDir()
	candidates, err := socketCandidates(os.Getenv("DOCKER_HOST"), os.Getenv("XDG_RUNTIME_DIR"), home)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), discoveryBudget)
	defer cancel()
	return discoverFrom(ctx, candidates, socketExists, querySocket)
}

// socketCandidates returns the Engine sockets to try, in order. A
// unix:// DOCKER_HOST is the only candidate: the user said where the
// daemon is. Any other DOCKER_HOST returns errSkipped, because ports
// published by a remote engine are not listeners on this host and lsoff
// must not open network connections. Otherwise the system socket, the
// rootless socket and the Docker Desktop per-user socket are tried.
func socketCandidates(dockerHost, xdgRuntimeDir, home string) ([]string, error) {
	// The Docker CLI trims DOCKER_HOST and treats a blank value as unset.
	if host := strings.TrimSpace(dockerHost); host != "" {
		path, ok := strings.CutPrefix(host, "unix://")
		if !ok || path == "" {
			return nil, errSkipped
		}
		return []string{path}, nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(path string) {
		path = filepath.Clean(path)
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	add(DefaultSocket)
	// Relative values are invalid per the XDG spec and would depend on cwd.
	if filepath.IsAbs(xdgRuntimeDir) {
		add(filepath.Join(xdgRuntimeDir, "docker.sock"))
	}
	if filepath.IsAbs(home) {
		add(filepath.Join(home, ".docker", "run", "docker.sock"))
	}
	return out, nil
}

// discoverFrom returns the result of the first candidate that exists and
// answers. All queries share ctx, so its deadline is the total budget
// rather than a per-socket timeout.
func discoverFrom(ctx context.Context, candidates []string, exists func(string) bool, query func(context.Context, string) ([]Published, error)) ([]Published, error) {
	var errs []error
	for _, socket := range candidates {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if !exists(socket) {
			continue
		}
		published, err := query(ctx, socket)
		if err == nil {
			return published, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", socket, err))
	}
	if len(errs) == 0 {
		return nil, errors.New("docker: no Engine socket found")
	}
	return nil, errors.Join(errs...)
}

func queryUnixSocket(ctx context.Context, socket string) ([]Published, error) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		},
	}
	defer transport.CloseIdleConnections()
	return discoverWithContext(ctx, &http.Client{Transport: transport}, "http://docker")
}

// DiscoverWithClient is the transport-independent part of Docker discovery.
// It is useful for tests and keeps the Engine JSON decoding independent from
// the Unix socket.
func DiscoverWithClient(client *http.Client, endpoint string) ([]Published, error) {
	return discoverWithContext(context.Background(), client, endpoint)
}

// discoverWithContext lets the request, including reading the body, be
// cut off by the shared discovery deadline.
func discoverWithContext(ctx context.Context, client *http.Client, endpoint string) ([]Published, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+"/containers/json?all=false", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker API: %s", resp.Status)
	}
	var containers []container
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, err
	}
	return publishedPorts(containers), nil
}

func publishedPorts(containers []container) []Published {
	var out []Published
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		id := c.ID
		if len(id) > 12 {
			id = id[:12]
		}
		for _, p := range c.Ports {
			protocol := strings.ToLower(strings.TrimSpace(p.Type))
			if p.PublicPort == 0 || p.PrivatePort == 0 || (protocol != "tcp" && protocol != "udp") {
				continue
			}
			hostIP := strings.TrimSpace(p.IP)
			if hostIP == "" {
				hostIP = "0.0.0.0"
			}
			out = append(out, Published{
				ContainerID: id, ContainerName: name, HostIP: hostIP,
				HostPort: p.PublicPort, ContainerPort: p.PrivatePort, Protocol: protocol,
			})
		}
	}
	return out
}
