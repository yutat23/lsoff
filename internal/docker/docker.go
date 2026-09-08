// Package docker reads published-port metadata from the local Docker Engine.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const DefaultSocket = "/var/run/docker.sock"

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

// Discover queries running containers through the local Engine Unix socket.
func Discover() ([]Published, error) {
	if _, err := os.Stat(DefaultSocket); err != nil {
		return nil, err
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", DefaultSocket)
		},
	}
	client := &http.Client{Transport: transport, Timeout: 750 * time.Millisecond}
	defer transport.CloseIdleConnections()
	return DiscoverWithClient(client, "http://docker")
}

// DiscoverWithClient is the transport-independent part of Docker discovery.
// It is useful for tests and keeps the Engine JSON decoding independent from
// the Unix socket.
func DiscoverWithClient(client *http.Client, endpoint string) ([]Published, error) {
	resp, err := client.Get(strings.TrimRight(endpoint, "/") + "/containers/json?all=false")
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
