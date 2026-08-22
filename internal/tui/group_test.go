package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yutat23/lsoff/internal/listen"
)

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

	rows = flattenGroups(in, listen.SortPort, false, map[int]bool{1: true})
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
	rows := flattenGroups(in, listen.SortPort, false, map[int]bool{7: true})
	if len(rows) != 3 {
		t.Fatalf("len=%d, want 3", len(rows))
	}
	wantMarks := []string{"▾", "├─", "└─"}
	for i, want := range wantMarks {
		if got := rows[i].mark(); got != want {
			t.Fatalf("row %d mark=%q, want %q", i, got, want)
		}
	}
	if rows[2].fold != foldChild || !rows[2].last {
		t.Fatalf("last row should be last child: %+v", rows[2])
	}
	m := model{width: 100}
	line := m.formatRow(rows[2], false)
	if !strings.HasPrefix(line, " └─ ") {
		t.Fatalf("rendered last child line=%q", line)
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
