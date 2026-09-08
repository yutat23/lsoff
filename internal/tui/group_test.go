package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yutat23/lsoff/internal/listen"
)

func expandedFor(e listen.Entry) map[groupKey]bool {
	return map[groupKey]bool{groupKeyForEntry(e, 0): true}
}

func TestFlattenGroupsCollapsesSamePID(t *testing.T) {
	in := []listen.Entry{
		{PID: 1, Proto: listen.TCP, Port: 80, Addr: "127.0.0.1", Name: "nginx"},
		{PID: 1, Proto: listen.TCP, Port: 80, Addr: "::1", Name: "nginx"},
		{PID: 2, Proto: listen.TCP, Port: 22, Addr: "0.0.0.0", Name: "sshd"},
	}
	rows := flattenGroups(in, listen.SortPort, false, nil)
	if len(rows) != 2 {
		t.Fatalf("collapsed len=%d, want 2", len(rows))
	}
	var parent, leaf viewRow
	for _, r := range rows {
		switch r.e.PID {
		case 1:
			parent = r
		case 2:
			leaf = r
		}
	}
	if parent.fold != foldCollapsed || parent.hidden != 1 {
		t.Fatalf("parent: %+v", parent)
	}
	if leaf.fold != foldNone {
		t.Fatalf("leaf: %+v", leaf)
	}

	rows = flattenGroups(in, listen.SortPort, false, expandedFor(in[0]))
	if len(rows) != 3 {
		t.Fatalf("expanded len=%d, want 3", len(rows))
	}
	var exp, child *viewRow
	for i := range rows {
		switch {
		case rows[i].fold == foldExpanded:
			exp = &rows[i]
		case rows[i].fold == foldChild:
			child = &rows[i]
		}
	}
	if exp == nil || child == nil {
		t.Fatalf("missing expanded head or child: %+v", rows)
	}
	if got := exp.mark(); got != "▾" {
		t.Fatalf("expanded head mark=%q, want ▾", got)
	}
	if got := child.mark(); got != "└─" {
		t.Fatalf("last child mark=%q, want └─", got)
	}
	if !child.last || child.id() != child.e.Key() {
		t.Fatalf("child should be last keyed by socket: %+v", child)
	}
	for _, r := range rows {
		if r.fold == foldNone && r.mark() != " " {
			t.Fatalf("leaf mark=%q, want space", r.mark())
		}
	}
}

func TestFlattenGroupsTreeConnectorsOrder(t *testing.T) {
	in := []listen.Entry{
		{PID: 7, Proto: listen.TCP, Port: 3000, Addr: "127.0.0.1", Name: "vite"},
		{PID: 7, Proto: listen.TCP, Port: 3000, Addr: "::1", Name: "vite"},
		{PID: 7, Proto: listen.TCP, Port: 3001, Addr: "0.0.0.0", Name: "vite"},
	}
	rows := flattenGroups(in, listen.SortPort, false, expandedFor(in[0]))
	if len(rows) != 3 {
		t.Fatalf("len=%d, want 3", len(rows))
	}
	wantMarks := []string{"▾", "└─", " "}
	for i, want := range wantMarks {
		if got := rows[i].mark(); got != want {
			t.Fatalf("row %d mark=%q, want %q", i, got, want)
		}
	}
	if rows[1].fold != foldChild || !rows[1].last || rows[2].fold != foldNone {
		t.Fatalf("pair child and different-port leaf are wrong: %+v", rows)
	}
	m := model{width: 100}
	line := m.formatRow(rows[1], false)
	if !strings.HasPrefix(line, " └─ ") {
		t.Fatalf("rendered last child line=%q", line)
	}
}

func TestFlattenGroupsUsesOwnerProtocolAndPort(t *testing.T) {
	in := []listen.Entry{
		{PID: 7, Proto: listen.TCP, Port: 3000, Addr: "0.0.0.0"},
		{PID: 7, Proto: listen.TCP, Port: 3000, Addr: "::"},
		{PID: 7, Proto: listen.TCP, Port: 9090, Addr: "0.0.0.0"},
		{PID: 7, Proto: listen.UDP, Port: 3000, Addr: "0.0.0.0"},
		{PID: 8, Proto: listen.TCP, Port: 3000, Addr: "0.0.0.0"},
	}
	rows := flattenGroups(in, listen.SortPort, false, nil)
	if len(rows) != 4 {
		t.Fatalf("groups=%d, want 4: %+v", len(rows), rows)
	}
	if rows[0].fold != foldCollapsed || rows[0].hidden != 1 {
		t.Fatalf("same PID/protocol/port should group: %+v", rows[0])
	}
	for _, r := range rows[1:] {
		if r.fold != foldNone {
			t.Fatalf("different port/protocol/owner grouped unexpectedly: %+v", r)
		}
	}
}

func TestFlattenGroupsDockerOwnerUsesContainerID(t *testing.T) {
	in := []listen.Entry{
		{Source: listen.SourceDocker, ContainerID: "abc", Proto: listen.TCP, Port: 3000, Addr: "0.0.0.0", Name: "docker:grafana"},
		{Source: listen.SourceDocker, ContainerID: "abc", Proto: listen.TCP, Port: 3000, Addr: "::", Name: "docker:grafana"},
		{Source: listen.SourceDocker, ContainerID: "abc", Proto: listen.TCP, Port: 9090, Addr: "0.0.0.0", Name: "docker:grafana"},
		{Source: listen.SourceDocker, ContainerID: "abc", Proto: listen.UDP, Port: 3000, Addr: "0.0.0.0", Name: "docker:grafana"},
		{Source: listen.SourceDocker, ContainerID: "def", Proto: listen.TCP, Port: 3000, Addr: "0.0.0.0", Name: "docker:grafana"},
	}
	rows := flattenGroups(in, listen.SortPort, false, nil)
	if len(rows) != 4 {
		t.Fatalf("groups=%d, want 4: %+v", len(rows), rows)
	}
	var grouped bool
	for _, r := range rows {
		if r.fold == foldCollapsed && r.hidden == 1 {
			grouped = true
		}
	}
	if !grouped {
		t.Fatalf("same Docker container/protocol/port did not group: %+v", rows)
	}
}

func TestDockerGroupFiltersAndExpansion(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 100
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{
		{Source: listen.SourceDocker, ContainerID: "abc", Name: "docker:grafana", Proto: listen.TCP, Port: 3000, Addr: "0.0.0.0"},
		{Source: listen.SourceDocker, ContainerID: "abc", Name: "docker:grafana", Proto: listen.TCP, Port: 3000, Addr: "::"},
	}
	m.applyFilter()
	if len(m.rows) != 1 || m.rows[0].fold != foldCollapsed || m.rows[0].hidden != 1 {
		t.Fatalf("initial Docker group: %+v", m.rows)
	}

	press := func(key tea.KeyMsg) {
		next, _ := m.Update(key)
		m = next.(model)
	}
	press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	if len(m.rows) != 1 || m.rows[0].fold != foldNone || m.rows[0].e.Addr != "0.0.0.0" {
		t.Fatalf("IPv4 view should contain one non-expandable member: %+v", m.rows)
	}
	press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("6")})
	if len(m.rows) != 1 || m.rows[0].fold != foldNone || m.rows[0].e.Addr != "::" {
		t.Fatalf("IPv6 view should contain one non-expandable member: %+v", m.rows)
	}
	press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("6")})
	if len(m.rows) != 1 || m.rows[0].fold != foldCollapsed || m.rows[0].hidden != 1 {
		t.Fatalf("cleared family view should restore group count: %+v", m.rows)
	}
	press(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.rows) != 2 || m.rows[0].fold != foldExpanded || m.rows[1].fold != foldChild {
		t.Fatalf("Docker group did not expand: %+v", m.rows)
	}

	m.loading = true
	next, _ := m.Update(loadedMsg{gen: m.loadGen, entries: m.all})
	m = next.(model)
	if len(m.rows) != 2 || m.rows[0].fold != foldExpanded {
		t.Fatalf("refresh lost Docker expansion state: %+v", m.rows)
	}

	m.filter.SetValue("grafana")
	m.applyFilter()
	if len(m.rows) != 2 || m.rows[0].e.Name != "docker:grafana" {
		t.Fatalf("Docker group search failed: %+v", m.rows)
	}
}

func TestEnterTogglesFold(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 80
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{
		{PID: 1, Proto: listen.TCP, Port: 80, Addr: "127.0.0.1", Name: "nginx"},
		{PID: 1, Proto: listen.TCP, Port: 80, Addr: "::1", Name: "nginx"},
	}
	m.applyFilter()
	if len(m.rows) != 1 || m.rows[0].fold != foldCollapsed {
		t.Fatalf("start: %+v", m.rows)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(model)
	if len(got.rows) != 2 || got.rows[0].fold != foldExpanded {
		t.Fatalf("after enter: %+v", got.rows)
	}
}

// TestFlattenGroupsKeepsGroupsWithSameKey covers two PID-0 rows on the same
// proto/addr/port: their representative Entry.Key() is identical, which used to
// make one group render twice while the other disappeared.
func TestFlattenGroupsKeepsGroupsWithSameKey(t *testing.T) {
	in := []listen.Entry{
		{PID: 0, Proto: listen.TCP, Port: 80, Addr: "0.0.0.0", Name: "a"},
		{PID: 0, Proto: listen.TCP, Port: 80, Addr: "0.0.0.0", Name: "b"},
		{PID: 5, Proto: listen.TCP, Port: 22, Addr: "0.0.0.0", Name: "sshd"},
	}
	rows := flattenGroups(in, listen.SortPort, false, nil)
	if len(rows) != 3 {
		t.Fatalf("len=%d, want 3: %+v", len(rows), rows)
	}
	got := make([]string, len(rows))
	seen := make(map[string]int)
	for i, r := range rows {
		got[i] = r.e.Name
		seen[r.e.Name]++
	}
	if got[0] != "sshd" {
		t.Fatalf("port 22 should sort first: %v", got)
	}
	for _, name := range []string{"a", "b", "sshd"} {
		if seen[name] != 1 {
			t.Fatalf("%q appears %d times, want once: %v", name, seen[name], got)
		}
	}
}

func TestSortGroupsOrderMatchesSortBy(t *testing.T) {
	entries := []listen.Entry{
		{PID: 3, Proto: listen.TCP, Port: 443, Addr: "0.0.0.0", Name: "nginx", Project: "web"},
		{PID: 1, Proto: listen.UDP, Port: 53, Addr: "127.0.0.1", Name: "named", Project: "dns"},
		{PID: 2, Proto: listen.TCP, Port: 22, Addr: "::", Name: "sshd", Project: "ops"},
		{PID: 4, Proto: listen.TCP, Port: 8080, Addr: "127.0.0.1", Name: "node", Project: "app"},
	}
	keys := []listen.SortKey{listen.SortPort, listen.SortProto, listen.SortAddr, listen.SortPID, listen.SortName, listen.SortProject}
	for _, key := range keys {
		for _, desc := range []bool{false, true} {
			want := make([]listen.Entry, len(entries))
			copy(want, entries)
			listen.SortBy(want, key, desc)

			rows := flattenGroups(entries, key, desc, nil)
			if len(rows) != len(want) {
				t.Fatalf("key=%v desc=%v: len=%d", key, desc, len(rows))
			}
			for i := range want {
				if rows[i].e != want[i] {
					t.Fatalf("key=%v desc=%v: row %d = %q, want %q", key, desc, i, rows[i].e.Name, want[i].Name)
				}
			}
		}
	}
}
