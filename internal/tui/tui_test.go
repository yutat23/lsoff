package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yutat23/lsoff/internal/listen"
)

func TestRowIndexAt(t *testing.T) {
	m := model{
		rows:   make([]viewRow, 10),
		offset: 2,
		height: 20,
	}
	if _, ok := m.rowIndexAt(viewHeaderY); ok {
		t.Fatal("header should not be a row")
	}
	i, ok := m.rowIndexAt(viewRowsY)
	if !ok || i != 2 {
		t.Fatalf("first visible row: i=%d ok=%v", i, ok)
	}
	i, ok = m.rowIndexAt(viewRowsY + 1)
	if !ok || i != 3 {
		t.Fatalf("second visible row: i=%d ok=%v", i, ok)
	}
	if _, ok := m.rowIndexAt(viewRowsY + 100); ok {
		t.Fatal("click below table should miss")
	}
}

func TestMouseSelectsRow(t *testing.T) {
	m := newModel(false, false, "")
	m.width = 80
	m.height = 24
	m.loading = false
	m.rows = []viewRow{
		{e: listen.Entry{Proto: listen.TCP, Port: 80, Name: "nginx"}},
		{e: listen.Entry{Proto: listen.TCP, Port: 443, Name: "nginx"}},
		{e: listen.Entry{Proto: listen.UDP, Port: 53, Name: "named"}},
	}
	m.all = []listen.Entry{
		{Proto: listen.TCP, Port: 80, Name: "nginx"},
		{Proto: listen.TCP, Port: 443, Name: "nginx"},
		{Proto: listen.UDP, Port: 53, Name: "named"},
	}

	next, _ := m.Update(tea.MouseMsg{
		X: 1, Y: viewRowsY + 2,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	got := next.(model)
	if got.cursor != 2 {
		t.Fatalf("cursor=%d, want 2", got.cursor)
	}
}

func TestMouseWheel(t *testing.T) {
	m := newModel(false, false, "")
	m.width = 80
	m.height = 24
	m.rows = make([]viewRow, 5)
	m.cursor = 2

	next, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
	})
	got := next.(model)
	if got.cursor != 1 {
		t.Fatalf("wheel up cursor=%d", got.cursor)
	}
}

func TestSlashStartsSearch(t *testing.T) {
	m := newModel(false, false, "")
	m.width = 80
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{
		{Proto: listen.TCP, Port: 80, Name: "nginx"},
		{Proto: listen.TCP, Port: 443, Name: "sshd"},
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	got := next.(model)
	if !got.filtering {
		t.Fatal("expected filter mode after /")
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	got = next.(model)
	if got.filter.Value() != "n" {
		t.Fatalf("value=%q", got.filter.Value())
	}
	if len(got.rows) != 1 || got.rows[0].e.Name != "nginx" {
		t.Fatalf("rows=%+v", got.rows)
	}
}

func TestLetterDoesNotStartSearch(t *testing.T) {
	m := newModel(false, false, "")
	m.width = 80
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{
		{Proto: listen.TCP, Port: 80, Name: "nginx"},
		{Proto: listen.TCP, Port: 443, Name: "sshd"},
	}
	m.rows = []viewRow{
		{e: m.all[0]},
		{e: m.all[1]},
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	got := next.(model)
	if got.filtering || got.filter.Value() != "" {
		t.Fatalf("filtering=%v value=%q", got.filtering, got.filter.Value())
	}
	if len(got.rows) != 2 {
		t.Fatalf("rows=%d", len(got.rows))
	}
}

func TestFilterCtrlCClearsLikeEsc(t *testing.T) {
	m := newModel(false, false, "")
	m.width = 80
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{
		{Proto: listen.TCP, Port: 80, Name: "nginx"},
		{Proto: listen.TCP, Port: 443, Name: "sshd"},
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	got := next.(model)
	next, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	got = next.(model)
	if !got.filtering {
		t.Fatal("expected filter mode")
	}

	next, cmd = got.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got = next.(model)
	if cmd != nil {
		t.Fatal("ctrl+c in filter mode should not quit")
	}
	if got.filtering || got.filter.Value() != "" {
		t.Fatalf("filtering=%v value=%q", got.filtering, got.filter.Value())
	}
	if len(got.rows) != 2 {
		t.Fatalf("rows=%d, want 2", len(got.rows))
	}
}

func TestInitialQuery(t *testing.T) {
	m := newModel(false, false, "sshd")
	if m.filter.Value() != "sshd" || !m.filtering {
		t.Fatalf("query not applied: %q filtering=%v", m.filter.Value(), m.filtering)
	}
}

func TestSortKeyAtX(t *testing.T) {
	if sortKeyAtX(2) != listen.SortProto {
		t.Fatal("proto")
	}
	if sortKeyAtX(10) != listen.SortPort {
		t.Fatal("port")
	}
	if sortKeyAtX(50) != listen.SortProject {
		t.Fatal("project")
	}
	if sortKeyAtX(70) != listen.SortName {
		t.Fatal("name")
	}
}

func TestCycleSort(t *testing.T) {
	m := newModel(false, false, "")
	if m.sortKey != listen.SortPort {
		t.Fatal(m.sortKey)
	}
	m.cycleSort()
	if m.sortKey != listen.SortName {
		t.Fatal(m.sortKey)
	}
}

func TestHeaderClickTogglesSort(t *testing.T) {
	m := newModel(false, false, "")
	m.width = 80
	m.height = 24
	m.all = []listen.Entry{
		{Proto: listen.TCP, Port: 80, Name: "nginx"},
		{Proto: listen.TCP, Port: 22, Name: "sshd"},
	}
	next, _ := m.Update(tea.MouseMsg{
		X: 70, Y: viewHeaderY,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	got := next.(model)
	if got.sortKey != listen.SortName {
		t.Fatalf("sortKey=%v", got.sortKey)
	}
	if got.rows[0].e.Name != "nginx" {
		t.Fatalf("rows=%+v", got.rows)
	}
}

func TestProtoCellColors(t *testing.T) {
	tcp := protoCell(listen.TCP)
	udp := protoCell(listen.UDP)
	if tcp == udp {
		t.Fatal("tcp and udp cells should differ")
	}
	if !strings.Contains(tcp, "tcp") || !strings.Contains(udp, "udp") {
		t.Fatalf("tcp=%q udp=%q", tcp, udp)
	}
}

func TestSelectedRowStylesWholeLine(t *testing.T) {
	m := model{width: 80}
	e := listen.Entry{Proto: listen.TCP, Port: 8080, Addr: "127.0.0.1", PID: 1, Name: "node", Project: "lsoff"}
	rest := fmt.Sprintf("  %5d  %-21s  %7s  %-14s  %s", 8080, "127.0.0.1", "1", "lsoff", "node")
	got := m.formatRow(viewRow{e: e}, true)
	want := selStyle.Render(padRight("   "+fmt.Sprintf("%-4s", "tcp")+rest, 80))
	if got != want {
		t.Fatalf("selected row should style the whole uncolored line\ngot:  %q\nwant: %q", got, want)
	}
	if unsel := m.formatRow(viewRow{e: e}, false); unsel != "   "+protoCell(listen.TCP)+rest {
		t.Fatalf("unselected: %q", unsel)
	}
}

func TestStaleLoadIgnored(t *testing.T) {
	m := newModel(false, false, "")
	m.loadGen = 2
	m.all = []listen.Entry{{Port: 1}}
	next, _ := m.Update(loadedMsg{gen: 1, entries: []listen.Entry{{Port: 99}}})
	got := next.(model)
	if len(got.all) != 1 || got.all[0].Port != 1 {
		t.Fatalf("stale load applied: %+v", got.all)
	}
}

func TestFormatRowSanitizes(t *testing.T) {
	m := model{width: 80}
	e := listen.Entry{Proto: listen.TCP, Port: 80, Addr: "127.0.0.1", Name: "a\x1b[31mb", Project: "x"}
	got := m.formatRow(viewRow{e: e}, false)
	if strings.Contains(got, "\x1b") {
		t.Fatalf("ansi leaked: %q", got)
	}
	if !strings.Contains(got, "ab") {
		t.Fatalf("missing sanitized name: %q", got)
	}
}

func TestJKMovesCursor(t *testing.T) {
	m := newModel(false, false, "")
	m.width = 80
	m.height = 24
	m.rows = make([]viewRow, 5)
	m.cursor = 2

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	got := next.(model)
	if got.cursor != 3 {
		t.Fatalf("j cursor=%d", got.cursor)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	got = next.(model)
	if got.cursor != 2 {
		t.Fatalf("k cursor=%d", got.cursor)
	}
	if got.confirm {
		t.Fatal("k should move, not kill")
	}
}

func TestXStartsKillConfirm(t *testing.T) {
	m := newModel(false, false, "")
	m.width = 80
	m.height = 24
	m.loading = false
	m.rows = []viewRow{
		{e: listen.Entry{Proto: listen.TCP, Port: 80, PID: 12, Start: 1, Name: "nginx"}},
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	got := next.(model)
	if !got.confirm {
		t.Fatal("expected confirm after x")
	}
}
