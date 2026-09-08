package listen

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
)

// Proto is the transport protocol of a listening socket.
type Proto uint8

const (
	TCP Proto = iota
	UDP
)

func (p Proto) String() string {
	if p == UDP {
		return "udp"
	}
	return "tcp"
}

func (p Proto) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

// Source identifies the owner metadata source for an entry.
type Source uint8

const (
	SourceProcess Source = iota
	SourceDocker
)

func (s Source) String() string {
	if s == SourceDocker {
		return "docker"
	}
	return "process"
}

func (s Source) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// Entry is a listening TCP/UDP endpoint and its ownership metadata. Docker
// entries intentionally have no process identity.
type Entry struct {
	Proto             Proto  `json:"proto"`
	Port              uint16 `json:"port"`
	Addr              string `json:"addr"`
	PID               int    `json:"pid"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	Cmdline           string `json:"cmdline"`
	Cwd               string `json:"cwd"`
	Project           string `json:"project"`
	Start             uint64 `json:"-"`
	Source            Source `json:"source,omitempty"`
	ContainerID       string `json:"container_id,omitempty"`
	ContainerPort     uint16 `json:"container_port,omitempty"`
	ContainerProtocol string `json:"container_protocol,omitempty"`
}

// Key uniquely identifies a listener row.
func (e Entry) Key() string {
	return fmt.Sprintf("%s/%s/%d/%d", e.Proto, e.Addr, e.Port, e.PID)
}

// FilterValue is used by the TUI search box.
func (e Entry) FilterValue() string {
	pid := ""
	if e.PID > 0 {
		pid = strconv.Itoa(e.PID)
	}
	parts := []string{
		e.Proto.String(),
		strconv.Itoa(int(e.Port)),
		e.Addr,
		pid,
		e.Name,
		e.Path,
		e.Cmdline,
		e.Cwd,
		e.Project,
	}
	parts = append(parts, SearchTerms(e.Proto, e.Port)...)
	return strings.ToLower(strings.Join(parts, " "))
}

// FilterPort returns entries whose local port matches.
func FilterPort(entries []Entry, port uint16) []Entry {
	out := make([]Entry, 0, 4)
	for _, e := range entries {
		if e.Port == port {
			out = append(out, e)
		}
	}
	return out
}

// FilterProto keeps TCP and/or UDP rows. If both flags are false, all rows are kept.
func FilterProto(entries []Entry, tcp, udp bool) []Entry {
	if tcp == udp {
		return entries
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if tcp && e.Proto == TCP {
			out = append(out, e)
		}
		if udp && e.Proto == UDP {
			out = append(out, e)
		}
	}
	return out
}

// FilterHasPID returns only entries with a known PID (PID > 0). A PID can be
// missing because of permissions, so this hides unidentifiable rows; it does
// not check whether the process is still alive.
func FilterHasPID(entries []Entry) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.PID > 0 {
			out = append(out, e)
		}
	}
	return out
}

// UniquePIDs returns distinct positive PIDs in first-seen order.
func UniquePIDs(entries []Entry) []int {
	seen := make(map[int]struct{}, len(entries))
	out := make([]int, 0, len(entries))
	for _, e := range entries {
		if e.PID <= 0 {
			continue
		}
		if _, ok := seen[e.PID]; ok {
			continue
		}
		seen[e.PID] = struct{}{}
		out = append(out, e.PID)
	}
	return out
}

// FilterQuery returns entries matching all whitespace-separated terms
// (case-insensitive) against proto, port, service aliases, address, pid, name, path, cmdline, cwd, and project.
func FilterQuery(entries []Entry, q string) []Entry {
	words := strings.Fields(strings.ToLower(q))
	if len(words) == 0 {
		return entries
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		hay := e.FilterValue()
		match := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				match = false
				break
			}
		}
		if match {
			out = append(out, e)
		}
	}
	return out
}

// SortKey selects the primary column for SortBy.
type SortKey int

const (
	SortPort SortKey = iota
	SortProto
	SortAddr
	SortPID
	SortName
	SortProject
)

func (k SortKey) String() string {
	switch k {
	case SortProto:
		return "proto"
	case SortAddr:
		return "addr"
	case SortPID:
		return "pid"
	case SortName:
		return "name"
	case SortProject:
		return "project"
	default:
		return "port"
	}
}

// Sort orders listeners by port, protocol, address, then PID.
func Sort(entries []Entry) {
	SortBy(entries, SortPort, false)
}

// SortBy orders listeners by key. Ties break on port, proto, addr, PID.
func SortBy(entries []Entry, key SortKey, desc bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		c := compareEntries(a, b, key)
		if c == 0 {
			c = cmp.Compare(a.Port, b.Port)
		}
		if c == 0 {
			c = cmp.Compare(a.Proto, b.Proto)
		}
		if c == 0 {
			c = strings.Compare(a.Addr, b.Addr)
		}
		if c == 0 {
			c = cmp.Compare(a.PID, b.PID)
		}
		if desc {
			return c > 0
		}
		return c < 0
	})
}

func compareEntries(a, b Entry, key SortKey) int {
	switch key {
	case SortProto:
		return cmp.Compare(a.Proto, b.Proto)
	case SortAddr:
		return strings.Compare(a.Addr, b.Addr)
	case SortPID:
		return cmp.Compare(a.PID, b.PID)
	case SortName:
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a.Cmdline), strings.ToLower(b.Cmdline))
	case SortProject:
		return strings.Compare(strings.ToLower(a.Project), strings.ToLower(b.Project))
	default:
		return cmp.Compare(a.Port, b.Port)
	}
}

// Endpoint returns addr:port, with IPv6 addresses in brackets.
func Endpoint(e Entry) string {
	ip := net.ParseIP(e.Addr)
	if ip != nil && ip.To4() == nil {
		return fmt.Sprintf("[%s]:%d", e.Addr, e.Port)
	}
	return fmt.Sprintf("%s:%d", e.Addr, e.Port)
}

// ProjectName is the last path component of cwd (the working directory's folder).
func ProjectName(cwd string) string {
	cwd = strings.TrimRight(cwd, `/\`)
	if cwd == "" {
		return ""
	}
	base := filepath.Base(cwd)
	if base == "." || base == "/" || base == `\` {
		return ""
	}
	return base
}

// ShortCwd replaces the home directory prefix with ~.
func ShortCwd(cwd string) string {
	if cwd == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return cwd
	}
	if cwd == home {
		return "~"
	}
	sep := string(os.PathSeparator)
	if strings.HasPrefix(cwd, home+sep) {
		return "~" + cwd[len(home):]
	}
	return cwd
}

// tableHeader is the column order of FormatTable.
var tableHeader = []string{"PROTO", "PORT", "ADDRESS", "PID", "PROJECT", "PROCESS", "PATH", "CMD", "CWD"}

// tableColumnGap is the number of spaces between two columns.
const tableColumnGap = 2

// FormatTable writes a stable, script-friendly table.
//
// Columns are padded by terminal display width (not rune count), so rows
// containing full-width CJK text stay aligned. For ASCII-only input the
// output is identical to text/tabwriter with padding 2.
func FormatTable(w io.Writer, entries []Entry) error {
	rows := make([][]string, 0, len(entries)+1)
	rows = append(rows, tableHeader)
	for _, e := range entries {
		rows = append(rows, []string{
			e.Proto.String(),
			strconv.FormatUint(uint64(e.Port), 10),
			displayCell(e.Addr),
			pidString(e.PID),
			displayCell(e.Project),
			displayCell(e.Name),
			displayCell(e.Path),
			displayCell(e.Cmdline),
			displayCell(ShortCwd(e.Cwd)),
		})
	}
	_, err := io.WriteString(w, renderTable(rows))
	return err
}

// renderTable pads every cell but the last on each line to the widest cell in
// its column plus tableColumnGap. The last column is never padded, so no line
// carries trailing spaces.
func renderTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	cols := len(rows[0])
	widths := make([]int, cols)
	cellW := make([][]int, len(rows))
	for i, row := range rows {
		cellW[i] = make([]int, len(row))
		for j, cell := range row {
			n := runewidth.StringWidth(cell)
			cellW[i][j] = n
			if j < cols-1 && n > widths[j] {
				widths[j] = n
			}
		}
	}
	var b strings.Builder
	for i, row := range rows {
		for j, cell := range row {
			b.WriteString(cell)
			if j == len(row)-1 {
				break
			}
			b.WriteString(strings.Repeat(" ", widths[j]+tableColumnGap-cellW[i][j]))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// FormatJSON writes entries as a JSON array. A nil slice becomes [].
func FormatJSON(w io.Writer, entries []Entry) error {
	if entries == nil {
		entries = []Entry{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	type row struct {
		Entry
		Service string `json:"service,omitempty"`
	}
	out := make([]row, len(entries))
	for i, e := range entries {
		out[i] = row{Entry: e, Service: ServiceName(e.Proto, e.Port)}
	}
	return enc.Encode(out)
}

func pidString(pid int) string {
	if pid <= 0 {
		return "-"
	}
	return strconv.Itoa(pid)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func normalizeAddr(s string) string {
	if s == "" {
		return s
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return s
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.String()
}
