package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	m := newModel(false, false, false, "")
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
	m := newModel(false, false, false, "")
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
	m := newModel(false, false, false, "")
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
	m := newModel(false, false, false, "")
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
	m := newModel(false, false, false, "")
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
	m := newModel(false, false, false, "sshd")
	if m.filter.Value() != "sshd" || !m.filtering {
		t.Fatalf("query not applied: %q filtering=%v", m.filter.Value(), m.filtering)
	}
}

// cellX returns the display column where sub starts in line, ignoring styling.
func cellX(t *testing.T, line, sub string) int {
	t.Helper()
	plain := listen.SanitizeDisplay(line)
	i := strings.Index(plain, sub)
	if i < 0 {
		t.Fatalf("%q not found in %q", sub, plain)
	}
	return lipgloss.Width(plain[:i])
}

// headerCells pairs a header label with a row value that lands in that column.
// PORT and PID are right-aligned, so the 4-digit port and 3-digit pid below sit
// under the 4- and 3-character labels.
var headerCells = []struct {
	label string
	value string
	sort  listen.SortKey
}{
	{"PROTO", "tcp", listen.SortProto},
	{"PORT", "8080", listen.SortPort},
	{"ADDRESS", "127.0.0.1", listen.SortAddr},
	{"PID", "900", listen.SortPID},
	{"PROJECT", "lsoff", listen.SortProject},
	{"PROCESS", "node", listen.SortName},
}

// TestRowsAlignWithHeader checks the grid itself: the tree connectors (├─ / └─)
// are two cells wide, so a child row must not shift PROTO and the columns after
// it by a cell.
func TestRowsAlignWithHeader(t *testing.T) {
	m := model{width: 120}
	header := m.formatHeader()
	e := listen.Entry{Proto: listen.TCP, Port: 8080, Addr: "127.0.0.1", PID: 900, Name: "node", Project: "lsoff"}
	rows := []struct {
		name string
		row  viewRow
	}{
		{"leaf", viewRow{e: e}},
		{"collapsed head", viewRow{e: e, fold: foldCollapsed, hidden: 2}},
		{"expanded head", viewRow{e: e, fold: foldExpanded}},
		{"middle child", viewRow{e: e, fold: foldChild}},
		{"last child", viewRow{e: e, fold: foldChild, last: true}},
	}
	for _, r := range rows {
		for _, selected := range []bool{false, true} {
			line := m.formatRow(r.row, selected)
			for _, c := range headerCells {
				want := cellX(t, header, c.label)
				if got := cellX(t, line, c.value); got != want {
					t.Fatalf("%s (selected=%v): %s starts at column %d, header %s at %d\nheader: %q\nrow:    %q",
						r.name, selected, c.value, got, c.label, want, header, listen.SanitizeDisplay(line))
				}
			}
		}
	}
}

func TestSortKeyAtX(t *testing.T) {
	m := model{width: 120}
	header := m.formatHeader()
	for _, c := range headerCells {
		x := cellX(t, header, c.label)
		if got := sortKeyAtX(x); got != c.sort {
			t.Fatalf("click on %s (x=%d): got %v, want %v", c.label, x, got, c.sort)
		}
	}
	if sortKeyAtX(0) != listen.SortProto {
		t.Fatal("click on the mark column should sort by proto")
	}
	if sortKeyAtX(1000) != listen.SortName {
		t.Fatal("click past the last column should sort by name")
	}
}

func TestCycleSort(t *testing.T) {
	m := newModel(false, false, false, "")
	if m.sortKey != listen.SortPort {
		t.Fatal(m.sortKey)
	}
	m.cycleSort()
	if m.sortKey != listen.SortName {
		t.Fatal(m.sortKey)
	}
}

func TestHeaderClickTogglesSort(t *testing.T) {
	m := newModel(false, false, false, "")
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
	prefix := strings.Repeat(" ", colProtoX)
	rest := fmt.Sprintf("  %5d  %-21s  %7s  %-14s  %s", 8080, "127.0.0.1", "1", "lsoff", "node")
	got := m.formatRow(viewRow{e: e}, true)
	want := selStyle.Render(padRight(prefix+fmt.Sprintf("%-5s", "tcp")+rest, 80))
	if got != want {
		t.Fatalf("selected row should style the whole uncolored line\ngot:  %q\nwant: %q", got, want)
	}
	if unsel := m.formatRow(viewRow{e: e}, false); unsel != prefix+protoCell(listen.TCP)+rest {
		t.Fatalf("unselected: %q", unsel)
	}
}

func TestStaleLoadIgnored(t *testing.T) {
	m := newModel(false, false, false, "")
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
	m := newModel(false, false, false, "")
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

func TestShortcutBarAtBottom(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 100
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) < 3 {
		t.Fatalf("too few lines: %q", view)
	}
	if !strings.Contains(lines[0], "lsoff") {
		t.Fatalf("title should be first: %q", lines[0])
	}
	bar := lines[len(lines)-1]
	for _, want := range []string{"search", "move", "expand", "pid", "copy", "auto", "sort", "kill", "quit"} {
		if !strings.Contains(bar, want) {
			t.Fatalf("shortcuts missing %q: %q", want, bar)
		}
	}
	if bar == helpStyle.Render("/ search  j/k move  enter expand  y copy  a auto  s sort  x kill  q quit") {
		t.Fatal("shortcut bar should be styled, not the old dim help line")
	}
	if strings.Contains(lines[1], "search") && strings.Contains(lines[1], "quit") {
		t.Fatalf("shortcuts should stay at the bottom: %q", lines[1])
	}
	rule := lines[len(lines)-2]
	if !strings.Contains(rule, "─") {
		t.Fatalf("footer should have a rule: %q", rule)
	}
}

func TestRenderShortcutsStylesKeys(t *testing.T) {
	got := renderShortcuts(80)
	if !strings.Contains(got, shortcutKey.Render("/")) {
		t.Fatalf("missing styled /: %q", got)
	}
	if !strings.Contains(got, shortcutDanger.Render("x")) {
		t.Fatalf("missing styled kill key: %q", got)
	}
	if !strings.Contains(got, shortcutQuit.Render("q")) {
		t.Fatalf("missing styled quit key: %q", got)
	}
	if !strings.Contains(got, shortcutSep.Render(" · ")) {
		t.Fatalf("missing separator: %q", got)
	}
}

func TestRenderShortcutsFitsWidth(t *testing.T) {
	for _, w := range []int{40, 60, 80, 120} {
		got := renderShortcuts(w)
		if n := lipgloss.Width(got); n > w {
			t.Fatalf("width %d: rendered %d", w, n)
		}
		if n := lipgloss.Width(renderFooterRule(w)); n != w {
			t.Fatalf("rule width %d: rendered %d", w, n)
		}
	}
}

func TestXStartsKillConfirm(t *testing.T) {
	m := newModel(false, false, false, "")
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

func TestTogglePIDFilter(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 80
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{
		{Proto: listen.TCP, Port: 80, PID: 100, Name: "nginx"},
		{Proto: listen.TCP, Port: 22, PID: 0, Name: "-"},
		{Proto: listen.UDP, Port: 53, PID: 200, Name: "named"},
	}
	m.applyFilter()
	if len(m.rows) != 3 {
		t.Fatalf("initial rows=%d, want 3", len(m.rows))
	}
	// press p to hide rows with an unknown PID
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	got := next.(model)
	if !got.onlyPID {
		t.Fatal("expected onlyPID=true after pressing p")
	}
	if len(got.rows) != 2 {
		t.Fatalf("filtered rows=%d, want 2", len(got.rows))
	}
	for _, r := range got.rows {
		if r.e.PID <= 0 {
			t.Fatalf("non-positive PID leaked: %+v", r)
		}
	}
	view := got.View()
	if !strings.Contains(view, "pid") {
		t.Fatalf("expected header meta to indicate pid filter: %q", view)
	}
	// press p again to toggle off
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	got = next.(model)
	if got.onlyPID {
		t.Fatal("expected onlyPID=false after pressing p again")
	}
	if len(got.rows) != 3 {
		t.Fatalf("restored rows=%d, want 3", len(got.rows))
	}
}
