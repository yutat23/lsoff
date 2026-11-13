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

// TestSearchLineFitsWidth: the search input was sized without the prompt
// and the cursor cell, and a query set before the first resize kept its
// unscrolled window, so a long query rendered wider than the terminal and
// the wrap shifted every line below it off the mouse Y offsets.
func TestSearchLineFitsWidth(t *testing.T) {
	queries := map[string]string{
		"ascii": strings.Repeat("q", 80),
		"wide":  strings.Repeat("日", 80),
	}
	key := func(mm tea.Model, msg tea.Msg) tea.Model {
		next, _ := mm.Update(msg)
		return next
	}
	right := tea.KeyMsg{Type: tea.KeyRight}
	home := tea.KeyMsg{Type: tea.KeyHome}
	states := []struct {
		name string
		set  func(mm tea.Model, w int) tea.Model
	}{
		{"focused at end", func(mm tea.Model, w int) tea.Model { return mm }},
		{"focused at start", func(mm tea.Model, w int) tea.Model { return key(mm, home) }},
		{"unfocused", func(mm tea.Model, w int) tea.Model { return key(mm, tea.KeyMsg{Type: tea.KeyEnter}) }},
		// Scroll with the cursor mid-query, then shrink the terminal.
		{"shrunk mid-query", func(mm tea.Model, w int) tea.Model {
			mm = key(mm, home)
			for i := 0; i < 40; i++ {
				mm = key(mm, right)
			}
			return key(mm, tea.WindowSizeMsg{Width: w, Height: 24})
		}},
	}
	for _, w := range []int{80, 40, 20} {
		for qname, q := range queries {
			for _, st := range states {
				var mm tea.Model = newModel(false, false, false, q)
				start := w
				if st.name == "shrunk mid-query" {
					start = 300
				}
				mm = key(mm, tea.WindowSizeMsg{Width: start, Height: 24})
				mm = st.set(mm, w)
				line := strings.Split(mm.View(), "\n")[viewFilterY]
				if n := lipgloss.Width(line); n > w {
					t.Fatalf("width %d, %s query, %s: search line is %d cells: %q", w, qname, st.name, n, listen.SanitizeDisplay(line))
				}
			}
		}
		// The placeholder shown for an empty query must fit as well.
		var mm tea.Model = newModel(false, false, false, "")
		mm = key(mm, tea.WindowSizeMsg{Width: w, Height: 24})
		line := strings.Split(mm.View(), "\n")[viewFilterY]
		if n := lipgloss.Width(line); n > w {
			t.Fatalf("width %d: placeholder line is %d cells: %q", w, n, listen.SanitizeDisplay(line))
		}
	}
}

func TestViewSanitizesShortCwd(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 100
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{{
		Proto: listen.TCP,
		Port:  8080,
		Addr:  "127.0.0.1",
		PID:   1,
		Name:  "server",
		Cwd:   "/tmp/ok\x1b[31m-danger",
	}}
	m.applyFilter()

	view := m.View()
	plain := listen.SanitizeDisplay(view)
	if strings.Contains(view, "\x1b[31m-danger") {
		t.Fatalf("raw CWD escape reached view: %q", view)
	}
	if !strings.Contains(plain, "CWD   /tmp/ok-danger") {
		t.Fatalf("sanitized CWD missing from view: %q", plain)
	}
}

func TestIPFamilyFiltersToggleAndSwitch(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 100
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{
		{Proto: listen.TCP, Addr: "127.0.0.1", Port: 3000, Name: "v4-process", PID: 10},
		{Proto: listen.TCP, Addr: "0.0.0.0", Port: 3001, Name: "v4-docker", Source: listen.SourceDocker},
		{Proto: listen.TCP, Addr: "::1", Port: 3000, Name: "v6-process", PID: 11},
		{Proto: listen.TCP, Addr: "::", Port: 3001, Name: "v6-docker", Source: listen.SourceDocker},
	}
	m.applyFilter()
	if len(m.rows) != 4 {
		t.Fatalf("all rows=%d, want 4", len(m.rows))
	}

	press := func(key string) model {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = next.(model)
		return m
	}
	if got := press("4"); got.ipFamily != IPFamilyV4 || len(got.rows) != 2 {
		t.Fatalf("IPv4 filter: family=%v rows=%d", got.ipFamily, len(got.rows))
	}
	if !strings.Contains(m.View(), "ipv4") {
		t.Fatal("active IPv4 filter is not visible")
	}
	if got := press("4"); got.ipFamily != IPFamilyAll || len(got.rows) != 4 {
		t.Fatalf("IPv4 toggle off: family=%v rows=%d", got.ipFamily, len(got.rows))
	}
	if got := press("6"); got.ipFamily != IPFamilyV6 || len(got.rows) != 2 {
		t.Fatalf("IPv6 filter: family=%v rows=%d", got.ipFamily, len(got.rows))
	}
	if got := press("4"); got.ipFamily != IPFamilyV4 || len(got.rows) != 2 {
		t.Fatalf("direct IPv6 to IPv4 switch: family=%v rows=%d", got.ipFamily, len(got.rows))
	}
	if got := press("6"); got.ipFamily != IPFamilyV6 || len(got.rows) != 2 {
		t.Fatalf("direct IPv4 to IPv6 switch: family=%v rows=%d", got.ipFamily, len(got.rows))
	}
	if got := press("6"); got.ipFamily != IPFamilyAll || len(got.rows) != 4 {
		t.Fatalf("IPv6 toggle off: family=%v rows=%d", got.ipFamily, len(got.rows))
	}
}

func TestIPFamilyFilterComposesWithProtocolPIDAndSearch(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 100
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{
		{Proto: listen.TCP, Addr: "0.0.0.0", Port: 3000, Name: "docker:web", Source: listen.SourceDocker},
		{Proto: listen.TCP, Addr: "::", Port: 3000, Name: "docker:web", Source: listen.SourceDocker},
		{Proto: listen.UDP, Addr: "0.0.0.0", Port: 3000, Name: "docker:web", Source: listen.SourceDocker},
		{Proto: listen.TCP, Addr: "127.0.0.1", Port: 3000, Name: "other", PID: 12},
	}
	m.wantTCP = true
	m.all = listen.FilterProto(m.all, m.wantTCP, false)
	m.onlyPID = true
	m.filter.SetValue("docker")
	m.applyFilter()
	if len(m.rows) != 0 {
		t.Fatalf("PID filter should hide Docker rows, got %d", len(m.rows))
	}
	m.onlyPID = false
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	m = next.(model)
	if m.ipFamily != IPFamilyV4 || len(m.rows) != 1 || m.rows[0].e.Addr != "0.0.0.0" {
		t.Fatalf("combined Docker/search/protocol/family filter: family=%v rows=%+v", m.ipFamily, m.rows)
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

// TestReloadStatusRule: a completed reload shows the socket count only in
// place of nothing, the "refreshing..." placeholder or an earlier count, so
// the count stays current across reloads but never overwrites action feedback.
func TestReloadStatusRule(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 80
	m.height = 24
	entries := []listen.Entry{
		{Proto: listen.TCP, Port: 80, Addr: "127.0.0.1", PID: 10, Name: "nginx"},
		{Proto: listen.TCP, Port: 443, Addr: "127.0.0.1", PID: 10, Name: "nginx"},
		{Proto: listen.UDP, Port: 53, Addr: "127.0.0.1", PID: 11, Name: "named"},
	}
	var mm tea.Model = m
	load := func(n int) string {
		t.Helper()
		next, _ := mm.Update(loadedMsg{gen: mm.(model).loadGen, entries: entries[:n]})
		mm = next
		return mm.(model).status
	}
	press := func(key string) string {
		t.Helper()
		next, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		mm = next
		return mm.(model).status
	}

	if got := load(2); got != "2 listening sockets" {
		t.Fatalf("initial load status=%q", got)
	}
	// Successive reloads (auto-refresh) keep the count current.
	if got := load(3); got != "3 listening sockets" {
		t.Fatalf("second load status=%q", got)
	}
	if got := press("r"); got != "refreshing..." {
		t.Fatalf("r status=%q", got)
	}
	if got := load(1); got != "1 listening sockets" {
		t.Fatalf("manual reload status=%q", got)
	}
	// Action feedback survives the reloads that follow it.
	if got := press("a"); got != "auto-refresh on" {
		t.Fatalf("a status=%q", got)
	}
	for i := 0; i < 2; i++ {
		if got := load(2); got != "auto-refresh on" {
			t.Fatalf("reload %d replaced action feedback: status=%q", i, got)
		}
	}
	if got := press("r"); got != "refreshing..." {
		t.Fatalf("r status=%q", got)
	}
	if got := load(3); got != "3 listening sockets" {
		t.Fatalf("reload after action status=%q", got)
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

func TestCtrlDAndCtrlUPageTableAndSearch(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 80
	m.height = 20
	m.loading = false
	m.all = make([]listen.Entry, 20)
	for i := range m.all {
		m.all[i] = listen.Entry{Proto: listen.TCP, Port: uint16(i + 1), Name: "listener"}
	}
	m.applyFilter()
	m.cursor = 0

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = next.(model)
	if m.cursor != m.pageSize() {
		t.Fatalf("ctrl+d cursor=%d, want %d", m.cursor, m.pageSize())
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = next.(model)
	if m.cursor != 0 {
		t.Fatalf("ctrl+u cursor=%d, want 0", m.cursor)
	}

	m.filtering = true
	m.filter.Focus()
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = next.(model)
	if m.cursor != m.pageSize() {
		t.Fatalf("search ctrl+d cursor=%d, want %d", m.cursor, m.pageSize())
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = next.(model)
	if m.cursor != 0 {
		t.Fatalf("search ctrl+u cursor=%d, want 0", m.cursor)
	}
}

func TestShortcutBarAtBottom(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 120
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) < 3 {
		t.Fatalf("too few lines: %q", view)
	}
	if !strings.Contains(lines[0], "lsoff") {
		t.Fatalf("title should be first: %q", lines[0])
	}
	bar := lines[len(lines)-1]
	for _, want := range []string{"search", "move", "expand", "pid", "copy", "auto-refresh", "reload", "sort", "kill", "quit"} {
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

// stubKill replaces killOne for the test and records every identity it is
// asked to kill, so no real process is ever signalled.
func stubKill(t *testing.T) *[]listen.Ident {
	t.Helper()
	var got []listen.Ident
	prev := killOne
	killOne = func(id listen.Ident) error {
		got = append(got, id)
		return nil
	}
	t.Cleanup(func() { killOne = prev })
	return &got
}

// TestKillConfirmSurvivesReload: a reload already in flight when x was pressed
// used to land during the dialog, drop the confirmed row, leave the cursor on
// the next process, and make both the box and y switch to that process.
func TestKillConfirmSurvivesReload(t *testing.T) {
	killed := stubKill(t)
	m := newModel(false, false, false, "")
	m.width = 100
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{
		{Proto: listen.TCP, Port: 8000, Addr: "127.0.0.1", PID: 100, Start: 1000, Name: "alpha"},
		{Proto: listen.TCP, Port: 8001, Addr: "127.0.0.1", PID: 101, Start: 1001, Name: "bravo"},
		{Proto: listen.TCP, Port: 8002, Addr: "127.0.0.1", PID: 102, Start: 1002, Name: "charlie"},
	}
	m.applyFilter()
	m.cursor = 0

	// A reload is in flight before x is pressed.
	m, _ = m.beginLoad()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = next.(model)
	if !m.confirm {
		t.Fatal("expected confirm after x")
	}

	// The reload lands while the dialog is open, without the confirmed process.
	next, _ = m.Update(loadedMsg{gen: m.loadGen, entries: []listen.Entry{m.all[1], m.all[2]}})
	m = next.(model)
	if !m.confirm {
		t.Fatal("reload closed the confirmation")
	}
	if e, ok := m.selected(); !ok || e.PID != 101 {
		t.Fatalf("precondition: cursor should now be on pid 101, got %+v ok=%v", e, ok)
	}

	view := listen.SanitizeDisplay(m.View())
	if !strings.Contains(view, "Kill alpha (pid 100)?") {
		t.Fatalf("confirm box retargeted after reload: %q", view)
	}
	if strings.Contains(view, "Kill bravo") {
		t.Fatalf("confirm box names the row under the cursor: %q", view)
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = next.(model)
	if m.confirm || m.confirmID != (listen.Ident{}) || m.confirmName != "" {
		t.Fatalf("y did not clear the dialog: confirm=%v id=%+v name=%q", m.confirm, m.confirmID, m.confirmName)
	}
	if cmd == nil {
		t.Fatal("y should return a kill command")
	}
	raw := cmd()
	msg, ok := raw.(killedMsg)
	if !ok {
		t.Fatalf("kill command returned %T", raw)
	}
	want := listen.Ident{PID: 100, Start: 1000}
	if len(*killed) != 1 || (*killed)[0] != want {
		t.Fatalf("killed %+v, want exactly %+v", *killed, want)
	}
	if msg.pid != 100 {
		t.Fatalf("killedMsg pid=%d, want 100", msg.pid)
	}
}

// TestKillStatusSurvivesReload: the reload started after a successful kill
// used to replace "killed pid N" with the socket count as soon as it landed,
// so the confirmation was on screen for only a few milliseconds.
func TestKillStatusSurvivesReload(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 100
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{
		{Proto: listen.TCP, Port: 8000, Addr: "127.0.0.1", PID: 100, Start: 1000, Name: "alpha"},
		{Proto: listen.TCP, Port: 8001, Addr: "127.0.0.1", PID: 101, Start: 1001, Name: "bravo"},
	}
	m.applyFilter()

	next, cmd := m.Update(killedMsg{pid: 100})
	m = next.(model)
	if cmd == nil || !m.loading {
		t.Fatal("a successful kill should start a reload")
	}
	next, _ = m.Update(loadedMsg{gen: m.loadGen, entries: []listen.Entry{m.all[1]}})
	m = next.(model)
	if len(m.all) != 1 || m.all[0].PID != 101 {
		t.Fatalf("post-kill reload not applied: %+v", m.all)
	}
	if m.status != "killed pid 100" {
		t.Fatalf("status=%q, want the kill to still be reported", m.status)
	}
	if view := listen.SanitizeDisplay(m.View()); !strings.Contains(view, "killed pid 100") {
		t.Fatalf("kill feedback missing from view: %q", view)
	}
}

// stubClipboard replaces writeClipboard for the test so y never touches the
// real clipboard; it fails with err when err is non-nil.
func stubClipboard(t *testing.T, err error) {
	t.Helper()
	prev := writeClipboard
	writeClipboard = func(string) error { return err }
	t.Cleanup(func() { writeClipboard = prev })
}

// TestActionErrorSurvivesReload: every completed reload used to set m.err to
// its own (nil) error, so with auto-refresh a failed kill or copy vanished
// within two seconds while success feedback stayed. An action error now stays
// until the next action that reports something, or r.
func TestActionErrorSurvivesReload(t *testing.T) {
	entries := []listen.Entry{
		{Proto: listen.TCP, Port: 8000, Addr: "127.0.0.1", PID: 100, Start: 1000, Name: "alpha"},
		{Proto: listen.TCP, Port: 8001, Addr: "127.0.0.1", PID: 101, Start: 1001, Name: "bravo"},
	}
	cases := []struct {
		name string
		fail func(tea.Model) tea.Model
		want string
	}{
		{"kill", func(mm tea.Model) tea.Model {
			next, _ := mm.Update(killedMsg{pid: 100, err: fmt.Errorf("operation not permitted")})
			return next
		}, "operation not permitted"},
		{"copy", func(mm tea.Model) tea.Model {
			stubClipboard(t, fmt.Errorf("no clipboard utility"))
			next, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
			return next
		}, "copy: no clipboard utility"},
	}
	for _, c := range cases {
		var mm tea.Model = newModel(false, false, false, "")
		mm, _ = mm.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
		load := func() {
			t.Helper()
			mm, _ = mm.Update(loadedMsg{gen: mm.(model).loadGen, entries: entries})
		}
		view := func() string { return listen.SanitizeDisplay(mm.(model).View()) }
		load()
		mm = c.fail(mm)
		if !strings.Contains(view(), c.want) {
			t.Fatalf("%s: error not shown: %q", c.name, view())
		}
		// Several reloads, as auto-refresh does every two seconds.
		for i := 0; i < 3; i++ {
			load()
			if !strings.Contains(view(), c.want) {
				t.Fatalf("%s: reload %d erased the action error: %q", c.name, i, view())
			}
		}
		// The next action that reports something replaces the stale error.
		mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
		if got := mm.(model); got.err != nil || got.status != "sort name" {
			t.Fatalf("%s: s left err=%v status=%q", c.name, got.err, got.status)
		}
		if v := view(); strings.Contains(v, c.want) || !strings.Contains(v, "sort name") {
			t.Fatalf("%s: s feedback hidden by the stale error: %q", c.name, v)
		}

		// r clears the error and reloads.
		mm = c.fail(mm)
		next, cmd := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
		mm = next
		if got := mm.(model); cmd == nil || !got.loading || got.err != nil {
			t.Fatalf("%s: r cmd=%v loading=%v err=%v", c.name, cmd != nil, got.loading, got.err)
		}
		if v := view(); strings.Contains(v, c.want) || !strings.Contains(v, refreshingStatus) {
			t.Fatalf("%s: r did not replace the error: %q", c.name, v)
		}
	}

	// A successful copy replaces an earlier copy error.
	stubClipboard(t, nil)
	m := newModel(false, false, false, "")
	m.width = 100
	m.height = 24
	m.all = entries
	m.applyFilter()
	m.err = fmt.Errorf("copy: no clipboard utility")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if got := next.(model); got.err != nil || got.status != "copied 127.0.0.1:8000" {
		t.Fatalf("successful copy left err=%v status=%q", got.err, got.status)
	}
}

// TestLoadErrorClearedByNextLoad: a failed reload reports its error, and the
// next successful one clears it, so a transient listing failure under
// auto-refresh heals itself. An action error outranks it on screen.
func TestLoadErrorClearedByNextLoad(t *testing.T) {
	entries := []listen.Entry{{Proto: listen.TCP, Port: 8000, Addr: "127.0.0.1", PID: 100, Start: 1000, Name: "alpha"}}
	var mm tea.Model = newModel(false, false, false, "")
	mm, _ = mm.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	load := func(err error) {
		t.Helper()
		mm, _ = mm.Update(loadedMsg{gen: mm.(model).loadGen, entries: entries, err: err})
	}
	view := func() string { return listen.SanitizeDisplay(mm.(model).View()) }

	load(nil)
	load(fmt.Errorf("listing failed"))
	if !strings.Contains(view(), "listing failed") {
		t.Fatalf("load error not shown: %q", view())
	}
	load(nil)
	if v := view(); strings.Contains(v, "listing failed") || !strings.Contains(v, "1 listening sockets") {
		t.Fatalf("successful load did not clear the load error: %q", v)
	}

	// With both, the action error shows; once an action replaces it, the
	// still-current load error is back, and the next good load clears it.
	mm, _ = mm.Update(killedMsg{pid: 100, err: fmt.Errorf("operation not permitted")})
	load(fmt.Errorf("listing failed"))
	if v := view(); !strings.Contains(v, "operation not permitted") || strings.Contains(v, "listing failed") {
		t.Fatalf("action error should outrank the load error: %q", v)
	}
	mm, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if v := view(); !strings.Contains(v, "listing failed") {
		t.Fatalf("load error lost after the action error was replaced: %q", v)
	}
	load(nil)
	if v := view(); strings.Contains(v, "listing failed") || !strings.Contains(v, "sort name") {
		t.Fatalf("good load after both errors: %q", v)
	}
}

// TestKillConfirmCancelClearsTarget: closing the dialog must drop the captured
// target so it cannot be reused by a later confirmation.
func TestKillConfirmCancelClearsTarget(t *testing.T) {
	killed := stubKill(t)
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("n")},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune("q")},
	} {
		m := newModel(false, false, false, "")
		m.width = 80
		m.height = 24
		m.loading = false
		m.rows = []viewRow{
			{e: listen.Entry{Proto: listen.TCP, Port: 80, PID: 12, Start: 1, Name: "nginx"}},
		}
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		m = next.(model)
		if m.confirmID != (listen.Ident{PID: 12, Start: 1}) || m.confirmName != "nginx" {
			t.Fatalf("x captured id=%+v name=%q", m.confirmID, m.confirmName)
		}
		next, cmd := m.Update(key)
		m = next.(model)
		if cmd != nil {
			t.Fatalf("%s should not return a command", key)
		}
		if m.confirm || m.confirmID != (listen.Ident{}) || m.confirmName != "" {
			t.Fatalf("%s left confirm=%v id=%+v name=%q", key, m.confirm, m.confirmID, m.confirmName)
		}
	}
	if len(*killed) != 0 {
		t.Fatalf("cancel killed %+v", *killed)
	}
}

func TestDockerEntryIsRenderableAndNotKillable(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 100
	m.height = 24
	m.loading = false
	m.all = []listen.Entry{{
		Proto: listen.TCP, Port: 7000, Addr: "0.0.0.0", Name: "docker:web",
		Source: listen.SourceDocker, ContainerID: "abcdef012345", ContainerPort: 4000,
		ContainerProtocol: "tcp",
	}}
	m.applyFilter()
	if len(m.rows) != 1 || !strings.Contains(m.View(), "docker:web") || !strings.Contains(m.View(), "4000/tcp") {
		t.Fatalf("Docker entry did not render: %q", m.View())
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	got := next.(model)
	if got.confirm {
		t.Fatal("Docker entry opened process kill confirmation")
	}
	// Root cannot give a container a PID, so the hint must point at docker.
	if want := "Docker container: lsoff cannot stop it (docker stop web)"; got.status != want {
		t.Fatalf("status=%q, want %q", got.status, want)
	}
}

// TestXOnUnknownPIDSuggestsRoot: a host process whose PID could not be read
// usually needs more privileges, unlike a Docker row.
func TestXOnUnknownPIDSuggestsRoot(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 80
	m.height = 24
	m.loading = false
	m.rows = []viewRow{
		{e: listen.Entry{Proto: listen.TCP, Port: 22, Addr: "0.0.0.0"}},
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	got := next.(model)
	if got.confirm || got.status != "pid unknown (try as root)" {
		t.Fatalf("confirm=%v status=%q", got.confirm, got.status)
	}
}

// TestDockerDetailLinesStartAtColumnZero: the SRC and PORT lines used to put
// "\n" inside Render, so lipgloss padded the empty second line and the next
// detail line (PORT, then ID) started after a run of spaces.
func TestDockerDetailLinesStartAtColumnZero(t *testing.T) {
	var mm tea.Model = newModel(false, false, false, "")
	mm, _ = mm.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	mm, _ = mm.Update(loadedMsg{gen: mm.(model).loadGen, entries: []listen.Entry{{
		Proto: listen.TCP, Port: 8080, Addr: "0.0.0.0", Name: "docker:web",
		Source: listen.SourceDocker, ContainerID: "abcdef123456", ContainerPort: 80,
		ContainerProtocol: "tcp",
	}}})

	// SanitizeDisplay also drops newlines, so split first and strip per line.
	lines := strings.Split(mm.View(), "\n")
	svc := -1
	for i, line := range lines {
		line = listen.SanitizeDisplay(line)
		lines[i] = line
		if strings.HasPrefix(line, "SVC   ") {
			svc = i
		}
		if trimmed := strings.TrimLeft(line, " "); trimmed != line &&
			(strings.HasPrefix(trimmed, "PORT  ") || strings.HasPrefix(trimmed, "ID    ")) {
			t.Fatalf("line %d has leading padding before a detail label: %q", i, line)
		}
	}
	if svc < 0 || svc+3 >= len(lines) {
		t.Fatalf("SVC detail line missing or block truncated:\n%s", strings.Join(lines, "\n"))
	}
	for i, want := range []string{"SRC   docker", "PORT  80/tcp", "ID    abcdef123456"} {
		if got := strings.TrimRight(lines[svc+1+i], " "); got != want {
			t.Fatalf("detail line %d = %q, want %q", svc+1+i, got, want)
		}
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

// TestViewFitsWidth pins the detail block to the terminal width: an over-long
// PATH used to wrap and push the footer off screen.
func TestViewFitsWidth(t *testing.T) {
	const longPath = "/Applications/Google Chrome.app/Contents/Frameworks/Google Chrome Framework.framework/Versions/140.0.7339.80/Helpers/Google Chrome Helper.app/Contents/MacOS/Google Chrome Helper"
	m := newModel(false, false, false, "")
	m.width = 80
	m.height = 24
	m.loading = false
	m.filter.Width = max(10, m.width-4)
	m.all = []listen.Entry{{
		Proto:   listen.TCP,
		Port:    8080,
		Addr:    "127.0.0.1",
		PID:     900,
		Name:    "Google Chrome Helper",
		Path:    longPath,
		Cmdline: strings.Repeat("x", 300),
		Cwd:     "/" + strings.Repeat("y", 299),
		Project: "lsoff",
	}}
	m.applyFilter()
	m.status = strings.Repeat("s", 200)

	if n := lipgloss.Width(longPath); n < 120 {
		t.Fatalf("test path too short to be interesting: %d cells", n)
	}
	for i, line := range strings.Split(m.View(), "\n") {
		if n := lipgloss.Width(line); n > m.width {
			t.Fatalf("line %d is %d cells wide, want <= %d: %q", i, n, m.width, listen.SanitizeDisplay(line))
		}
	}

	m.status = ""
	m.err = fmt.Errorf("%s", strings.Repeat("e", 200))
	for i, line := range strings.Split(m.View(), "\n") {
		if n := lipgloss.Width(line); n > m.width {
			t.Fatalf("error view line %d is %d cells wide, want <= %d", i, n, m.width)
		}
	}
}

// minViewHeight is the smallest terminal the fixed chrome (title, search,
// header, detail block, status or confirm box, footer) fits in with one row.
const minViewHeight = 13

// TestViewFitsHeight: the kill confirmation box is three lines where the
// status line is one, so with a full page of rows the view used to be one
// line taller than the terminal and Bubble Tea dropped the title line.
func TestViewFitsHeight(t *testing.T) {
	entries := make([]listen.Entry, 60)
	for i := range entries {
		entries[i] = listen.Entry{Proto: listen.TCP, Port: uint16(1000 + i), Addr: "127.0.0.1", PID: 100 + i, Start: 1, Name: "listener"}
	}
	entries[59] = listen.Entry{
		Proto: listen.TCP, Port: 1059, Addr: "0.0.0.0", Name: "docker:web",
		Source: listen.SourceDocker, ContainerID: "abcdef012345", ContainerPort: 80, ContainerProtocol: "tcp",
	}
	key := func(mm tea.Model, k tea.KeyMsg) tea.Model {
		next, _ := mm.Update(k)
		return next
	}
	runes := func(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
	states := []struct {
		name string
		set  func(tea.Model) tea.Model
	}{
		{"normal", func(mm tea.Model) tea.Model { return mm }},
		// Select the last row of the first page so the box must not hide it
		// (stopping short of the Docker row, which x refuses).
		{"confirming", func(mm tea.Model) tea.Model {
			m := mm.(model)
			for i := 1; i < min(m.pageSize(), len(m.rows)-1); i++ {
				mm = key(mm, runes("j"))
			}
			return key(mm, runes("x"))
		}},
		{"confirming at end", func(mm tea.Model) tea.Model {
			mm = key(mm, runes("G"))
			mm = key(mm, runes("k"))
			return key(mm, runes("x"))
		}},
		{"error", func(mm tea.Model) tea.Model {
			m := mm.(model)
			m.err = fmt.Errorf("permission denied")
			return m
		}},
		{"docker selected", func(mm tea.Model) tea.Model { return key(mm, runes("G")) }},
		{"empty", func(mm tea.Model) tea.Model {
			mm = key(mm, runes("/"))
			mm = key(mm, runes("zzz"))
			return key(mm, tea.KeyMsg{Type: tea.KeyEnter})
		}},
		{"searching", func(mm tea.Model) tea.Model {
			mm = key(mm, runes("/"))
			return key(mm, runes("listener"))
		}},
	}
	for _, h := range []int{minViewHeight, minViewHeight + 1, 16, 20, 24, 30, 40, 50, 80} {
		for _, st := range states {
			var mm tea.Model = newModel(false, false, false, "")
			mm, _ = mm.Update(tea.WindowSizeMsg{Width: 100, Height: h})
			mm, _ = mm.Update(loadedMsg{gen: mm.(model).loadGen, entries: entries})
			mm = st.set(mm)
			m := mm.(model)
			if st.name == "confirming" || st.name == "confirming at end" {
				if !m.confirm {
					t.Fatalf("h=%d %s: precondition: confirmation not open", h, st.name)
				}
			}

			lines := strings.Split(m.View(), "\n")
			for i := range lines {
				lines[i] = listen.SanitizeDisplay(lines[i])
			}
			if len(lines) > h {
				t.Fatalf("h=%d %s: view is %d lines, want <= %d:\n%s", h, st.name, len(lines), h, strings.Join(lines, "\n"))
			}
			if !strings.HasPrefix(lines[0], "lsoff") {
				t.Fatalf("h=%d %s: title is not the first line: %q", h, st.name, lines[0])
			}
			// The rows View draws must be exactly the rows rowIndexAt maps
			// clicks to, and the selected row must be among them.
			drawn := min(m.pageSize(), len(m.rows)-m.offset)
			for k := 0; k < drawn; k++ {
				y := viewRowsY + k
				i, ok := m.rowIndexAt(y)
				want := fmt.Sprintf("  %5d  ", m.rows[m.offset+k].e.Port)
				if !ok || i != m.offset+k || !strings.Contains(lines[y], want) {
					t.Fatalf("h=%d %s: line %d = %q, rowIndexAt=(%d, %v), want row %d (port%s)", h, st.name, y, lines[y], i, ok, m.offset+k, want)
				}
			}
			if _, ok := m.rowIndexAt(viewRowsY + drawn); ok {
				t.Fatalf("h=%d %s: rowIndexAt hits line %d below the drawn rows", h, st.name, viewRowsY+drawn)
			}
			if len(m.rows) > 0 && (m.cursor < m.offset || m.cursor >= m.offset+drawn) {
				t.Fatalf("h=%d %s: cursor %d not among drawn rows [%d, %d)", h, st.name, m.cursor, m.offset, m.offset+drawn)
			}
		}
	}
}

// TestRowsAlignWithHeaderWide is TestRowsAlignWithHeader with full-width text:
// CJK characters take two cells, so padding by rune count shifts every column
// after PROJECT.
func TestRowsAlignWithHeaderWide(t *testing.T) {
	m := model{width: 120}
	header := m.formatHeader()
	e := listen.Entry{Proto: listen.TCP, Port: 8080, Addr: "127.0.0.1", PID: 900, Name: "メモ帳", Project: "日本語プロジェクト"}
	for _, selected := range []bool{false, true} {
		line := m.formatRow(viewRow{e: e}, selected)
		// "日本語プロジェクト" is 18 cells, so the column shows a truncated
		// prefix; its first characters must still start the PROJECT column.
		for _, c := range []struct{ label, value string }{
			{"PROJECT", "日本語"},
			{"PROCESS", "メモ帳"},
		} {
			want := cellX(t, header, c.label)
			if got := cellX(t, line, c.value); got != want {
				t.Fatalf("selected=%v: %s starts at column %d, header %s at %d\nheader: %q\nrow:    %q",
					selected, c.value, got, c.label, want, header, listen.SanitizeDisplay(line))
			}
		}
	}
}

// TestWideProjectTruncatedToColumn checks that a full-width value wider than
// its column is cut to the column width in cells, never spilling into PROCESS.
func TestWideProjectTruncatedToColumn(t *testing.T) {
	m := model{width: 120}
	header := m.formatHeader()
	e := listen.Entry{
		Proto:   listen.TCP,
		Port:    8080,
		Addr:    "127.0.0.1",
		PID:     900,
		Name:    "node",
		Project: strings.Repeat("日", 30),
	}
	line := listen.SanitizeDisplay(m.formatRow(viewRow{e: e}, false))
	projX := cellX(t, header, "PROJECT")
	procX := cellX(t, header, "PROCESS")
	if got := cellX(t, line, "node"); got != procX {
		t.Fatalf("PROCESS starts at %d, want %d: %q", got, procX, line)
	}
	// The rendered project cell, in cells, must fit the 14-cell column.
	cell := line[len(truncateCells(line, projX)) : len(line)-len("  node")]
	if n := lipgloss.Width(cell); n != 14 {
		t.Fatalf("project cell is %d cells wide, want 14: %q", n, cell)
	}
}

// truncateCells returns the prefix of s that is n cells wide.
func truncateCells(s string, n int) string {
	w := 0
	for i, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > n {
			return s[:i]
		}
		w += rw
	}
	return s
}

func TestTruncateByDisplayWidth(t *testing.T) {
	cases := []struct {
		in string
		n  int
	}{
		{strings.Repeat("日", 10), 14},
		{strings.Repeat("日", 10), 5},
		{strings.Repeat("日", 10), 1},
		{"日a日a日", 4},
		{"abcdef", 3},
	}
	for _, c := range cases {
		got := truncate(c.in, c.n)
		if n := lipgloss.Width(got); n > c.n {
			t.Fatalf("truncate(%q, %d) = %q (%d cells), want <= %d", c.in, c.n, got, n, c.n)
		}
	}
	if got := truncate("abc", 5); got != "abc" {
		t.Fatalf("short strings must pass through: %q", got)
	}
}

// TestAutoToggleDoesNotStackTickChains: on/off/on used to leave the first
// chain alive as well, halving the refresh interval with every toggle.
func TestAutoToggleDoesNotStackTickChains(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 80
	m.height = 24
	m.loading = false

	var mm tea.Model = m
	press := func() {
		next, _ := mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
		mm = next
	}
	press() // on: chain 1
	first := mm.(model).autoGen
	press() // off
	press() // on: chain 2
	got := mm.(model)
	if !got.auto {
		t.Fatal("auto should be on after three toggles")
	}
	if got.autoGen == first {
		t.Fatalf("autoGen not bumped: %d", got.autoGen)
	}

	if _, cmd := got.Update(tickMsg{gen: first}); cmd != nil {
		t.Fatal("a stale tick must not re-arm a second chain")
	}
	if _, cmd := got.Update(tickMsg{gen: got.autoGen}); cmd == nil {
		t.Fatal("the current chain must keep ticking")
	}
}

func TestTickIgnoredWhenAutoOff(t *testing.T) {
	m := newModel(false, false, false, "")
	m.width = 80
	m.height = 24
	if _, cmd := m.Update(tickMsg{gen: m.autoGen}); cmd != nil {
		t.Fatal("tick with auto off should do nothing")
	}
}
