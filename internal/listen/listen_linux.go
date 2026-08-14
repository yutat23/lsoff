//go:build linux

package listen

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	procNetTCP  = "/proc/net/tcp"
	procNetTCP6 = "/proc/net/tcp6"
	procNetUDP  = "/proc/net/udp"
	procNetUDP6 = "/proc/net/udp6"

	tcpListen = 0x0a
)

type sock struct {
	proto Proto
	port  uint16
	addr  string
	ino   string
}

// List returns LISTEN TCP sockets and bound (unconnected) UDP sockets.
func List() ([]Entry, error) {
	socks, err := loadSockets()
	if err != nil {
		return nil, err
	}
	if len(socks) == 0 {
		return nil, nil
	}

	byInode := make(map[string][]sock, len(socks))
	for _, s := range socks {
		byInode[s.ino] = append(byInode[s.ino], s)
	}

	matched := make(map[string]struct{}, len(socks))
	var entries []Entry

	procDir, err := os.Open("/proc")
	if err != nil {
		return nil, err
	}
	defer procDir.Close()

	names, err := procDir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	for _, name := range names {
		pid, err := strconv.Atoi(name)
		if err != nil || pid <= 0 {
			continue
		}
		procEntries := matchProcSockets(pid, byInode)
		for _, e := range procEntries {
			matched[e.ino] = struct{}{}
			entries = append(entries, e.Entry)
		}
	}

	for ino, ss := range byInode {
		if _, ok := matched[ino]; ok {
			continue
		}
		for _, s := range ss {
			entries = append(entries, Entry{
				Proto: s.proto,
				Port:  s.port,
				Addr:  s.addr,
			})
		}
	}

	Sort(entries)
	return entries, nil
}

type matchedEntry struct {
	Entry
	ino string
}

func matchProcSockets(pid int, byInode map[string][]sock) []matchedEntry {
	fdDir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	d, err := os.Open(fdDir)
	if err != nil {
		return nil
	}
	defer d.Close()

	fds, err := d.Readdirnames(-1)
	if err != nil {
		return nil
	}

	var name, path, cmdline, cwd string
	var start uint64
	loaded := false
	var out []matchedEntry
	seen := make(map[string]struct{})

	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join(fdDir, fd))
		if err != nil || !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
			continue
		}
		ino := target[len("socket:[") : len(target)-1]
		ss, ok := byInode[ino]
		if !ok {
			continue
		}
		if !loaded {
			name, path, cmdline, cwd, start = procMeta(pid)
			loaded = true
		}
		for _, s := range ss {
			key := s.proto.String() + "/" + s.addr + "/" + strconv.Itoa(int(s.port)) + "/" + ino
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, matchedEntry{
				Entry: Entry{
					Proto:   s.proto,
					Port:    s.port,
					Addr:    s.addr,
					PID:     pid,
					Name:    name,
					Path:    path,
					Cmdline: cmdline,
					Cwd:     cwd,
					Project: ProjectName(cwd),
					Start:   start,
				},
				ino: ino,
			})
		}
	}
	return out
}

func procMeta(pid int) (string, string, string, string, uint64) {
	base := filepath.Join("/proc", strconv.Itoa(pid))
	name := ""
	if b, err := os.ReadFile(filepath.Join(base, "comm")); err == nil {
		name = strings.TrimSpace(string(b))
	}
	path, err := os.Readlink(filepath.Join(base, "exe"))
	if err != nil {
		path = ""
	}
	cmdline := ""
	if b, err := os.ReadFile(filepath.Join(base, "cmdline")); err == nil {
		cmdline = strings.TrimSpace(strings.ReplaceAll(string(b), "\x00", " "))
	}
	cwd, err := os.Readlink(filepath.Join(base, "cwd"))
	if err != nil {
		cwd = ""
	}
	if name == "" && path != "" {
		name = filepath.Base(strings.TrimSuffix(path, " (deleted)"))
	}
	start, _ := procStartToken(pid)
	return name, path, cmdline, cwd, start
}

func loadSockets() ([]sock, error) {
	var out []sock
	for _, spec := range []struct {
		path  string
		proto Proto
		tcp   bool
		v6    bool
	}{
		{procNetTCP, TCP, true, false},
		{procNetTCP6, TCP, true, true},
		{procNetUDP, UDP, false, false},
		{procNetUDP6, UDP, false, true},
	} {
		ss, err := parseProcNet(spec.path, spec.proto, spec.tcp, spec.v6)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		out = append(out, ss...)
	}
	return out, nil
}

func parseProcNet(path string, proto Proto, tcp, v6 bool) ([]sock, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return nil, sc.Err()
	}

	var out []sock
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 {
			continue
		}
		if tcp {
			st, err := strconv.ParseUint(fields[3], 16, 8)
			if err != nil || st != tcpListen {
				continue
			}
		} else {
			// Unconnected UDP (remote port 0) is the listening equivalent.
			_, rport, err := parseHexAddr(fields[2], v6)
			if err != nil || rport != 0 {
				continue
			}
		}
		addr, port, err := parseHexAddr(fields[1], v6)
		if err != nil || port == 0 {
			continue
		}
		out = append(out, sock{
			proto: proto,
			port:  port,
			addr:  addr,
			ino:   fields[9],
		})
	}
	return out, sc.Err()
}

func parseHexAddr(s string, v6 bool) (string, uint16, error) {
	ipStr, portStr, ok := strings.Cut(s, ":")
	if !ok {
		return "", 0, fmt.Errorf("bad address %q", s)
	}
	port, err := strconv.ParseUint(portStr, 16, 16)
	if err != nil {
		return "", 0, err
	}
	raw, err := hex.DecodeString(ipStr)
	if err != nil {
		return "", 0, err
	}
	var ip net.IP
	if !v6 {
		if len(raw) != 4 {
			return "", 0, fmt.Errorf("bad ipv4 %q", ipStr)
		}
		ip = net.IP{raw[3], raw[2], raw[1], raw[0]}
	} else {
		if len(raw) != 16 {
			return "", 0, fmt.Errorf("bad ipv6 %q", ipStr)
		}
		ip = make(net.IP, 16)
		for i := 0; i < 16; i += 4 {
			binary.LittleEndian.PutUint32(ip[i:i+4], binary.BigEndian.Uint32(raw[i:i+4]))
		}
	}
	return normalizeAddr(ip.String()), uint16(port), nil
}
