package tui

import (
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
	var exp, child, foundLeaf bool
	for _, r := range rows {
		if r.e.PID == 1 && r.fold == foldExpanded {
			exp = true
		}
		if r.e.PID == 1 && r.fold == foldChild {
			child = true
		}
		if r.e.PID == 2 && r.fold == foldNone {
			foundLeaf = true
		}
	}
	if !exp || !child || !foundLeaf {
		t.Fatalf("expanded rows: %+v", rows)
	}
}

func TestEnterTogglesFold(t *testing.T) {
	m := newModel(false, false, "")
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
