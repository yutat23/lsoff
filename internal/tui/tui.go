package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/yutat23/lsoff/internal/listen"
)

const (
	viewFilterY  = 1
	viewHeaderY  = 2
	viewRowsY    = 3
	autoInterval = 2 * time.Second
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	pathStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	headerRow  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("249"))
	tcpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	udpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	selStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("63"))
	confirmBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("203")).
			Padding(0, 1)
)

type loadedMsg struct {
	gen     int
	entries []listen.Entry
	err     error
}

type killedMsg struct {
	pid int
	err error
}

type tickMsg time.Time

type model struct {
	all       []listen.Entry
	rows      []viewRow
	cursor    int
	offset    int
	width     int
	height    int
	filter    textinput.Model
	filtering bool
	confirm   bool
	status    string
	err       error
	loading   bool
	loadGen   int
	wantTCP   bool
	wantUDP   bool
	auto      bool
	sortKey   listen.SortKey
	sortDesc  bool
	expanded  map[int]bool
}

// Run starts the interactive TUI.
func Run(tcp, udp bool, query string) error {
	m := newModel(tcp, udp, query)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

func newModel(tcp, udp bool, query string) model {
	ti := textinput.New()
	ti.Placeholder = "/ to search"
	ti.CharLimit = 80
	ti.Prompt = "Search: "
	ti.SetValue(query)
	m := model{
		filter:   ti,
		loading:  true,
		loadGen:  1,
		wantTCP:  tcp,
		wantUDP:  udp,
		expanded: make(map[int]bool),
	}
	if query != "" {
		m.filtering = true
		m.filter.Focus()
	}
	return m
}

func (m model) Init() tea.Cmd {
	return m.loadCmd(m.loadGen)
}

func (m model) loadCmd(gen int) tea.Cmd {
	tcp, udp := m.wantTCP, m.wantUDP
	return func() tea.Msg {
		entries, err := listen.List()
		if err == nil {
			entries = listen.FilterProto(entries, tcp, udp)
		}
		return loadedMsg{gen: gen, entries: entries, err: err}
	}
}

func (m model) beginLoad() (model, tea.Cmd) {
	m.loadGen++
	m.loading = true
	return m, m.loadCmd(m.loadGen)
}

func (m model) autoTick() tea.Cmd {
	if !m.auto {
		return nil
	}
	return tea.Tick(autoInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func killCmd(id listen.Ident) tea.Cmd {
	return func() tea.Msg {
		return killedMsg{pid: id.PID, err: listen.Kill(id)}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.filter.Width = max(10, msg.Width-4)
		m.clamp()
		return m, nil

	case loadedMsg:
		if msg.gen != m.loadGen {
			return m, nil
		}
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.all = msg.entries
			m.applyFilter()
			m.status = fmt.Sprintf("%d listening sockets", len(m.all))
		}
		return m, nil

	case killedMsg:
		if msg.err != nil {
			m.status = ""
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.status = fmt.Sprintf("killed pid %d", msg.pid)
		return m.beginLoad()

	case tickMsg:
		if !m.auto {
			return m, nil
		}
		if m.confirm {
			return m, m.autoTick()
		}
		if m.loading {
			return m, m.autoTick()
		}
		m, cmd := m.beginLoad()
		return m, tea.Batch(cmd, m.autoTick())

	case tea.MouseMsg:
		return m.updateMouse(msg)

	case tea.KeyMsg:
		if m.confirm {
			return m.updateConfirm(msg)
		}
		if m.filtering {
			return m.updateFilter(msg)
		}
		return m.updateTable(msg)
	}
	return m, nil
}

func (m model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.confirm {
		return m, nil
	}
	switch {
	case msg.Button == tea.MouseButtonWheelUp && msg.Action == tea.MouseActionPress:
		if m.cursor > 0 {
			m.cursor--
			m.clamp()
		}
	case msg.Button == tea.MouseButtonWheelDown && msg.Action == tea.MouseActionPress:
		if m.cursor < len(m.rows)-1 {
			m.cursor++
			m.clamp()
		}
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		if msg.Y == viewFilterY {
			m.filtering = true
			m.filter.Focus()
			return m, textinput.Blink
		}
		if msg.Y == viewHeaderY {
			key := sortKeyAtX(msg.X)
			if key == m.sortKey {
				m.sortDesc = !m.sortDesc
			} else {
				m.sortKey = key
				m.sortDesc = false
			}
			m.applyFilter()
			return m, nil
		}
		if i, ok := m.rowIndexAt(msg.Y); ok {
			if m.filtering {
				m.filtering = false
				m.filter.Blur()
			}
			m.cursor = i
			m.clamp()
			if msg.X < 3 {
				m.toggleFold()
			}
		}
	}
	return m, nil
}

func (m model) rowIndexAt(y int) (int, bool) {
	if y < viewRowsY || len(m.rows) == 0 {
		return 0, false
	}
	i := m.offset + (y - viewRowsY)
	if i < 0 || i >= len(m.rows) {
		return 0, false
	}
	if y-viewRowsY >= m.pageSize() {
		return 0, false
	}
	return i, true
}

func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		e, ok := m.selected()
		m.confirm = false
		if !ok || e.PID <= 0 {
			m.status = "no process to kill"
			return m, nil
		}
		if e.Start == 0 {
			m.status = "process identity unknown (refusing to kill)"
			return m, nil
		}
		return m, killCmd(e.Ident())
	case "n", "N", "esc", "q":
		m.confirm = false
		m.status = "cancelled"
		return m, nil
	}
	return m, nil
}

func (m model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.filtering = false
		m.filter.Blur()
		m.filter.SetValue("")
		m.applyFilter()
		return m, nil
	case "enter":
		m.filtering = false
		m.filter.Blur()
		return m, nil
	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
			m.clamp()
		}
		return m, nil
	case "down", "ctrl+n":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
			m.clamp()
		}
		return m, nil
	case "pgup":
		m.cursor -= m.pageSize()
		m.clamp()
		return m, nil
	case "pgdown":
		m.cursor += m.pageSize()
		m.clamp()
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m model) updateTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "/", "ctrl+f":
		m.filtering = true
		m.filter.Focus()
		return m, textinput.Blink
	case "esc":
		if m.filter.Value() != "" {
			m.filter.SetValue("")
			m.applyFilter()
		}
		return m, nil
	case "r", "ctrl+r":
		m.err = nil
		m.status = "refreshing..."
		return m.beginLoad()
	case "a":
		m.auto = !m.auto
		if m.auto {
			m.status = "auto-refresh on"
			if m.loading {
				return m, m.autoTick()
			}
			m, cmd := m.beginLoad()
			return m, tea.Batch(cmd, m.autoTick())
		}
		m.status = "auto-refresh off"
		return m, nil
	case "s":
		m.cycleSort()
		m.applyFilter()
		m.status = "sort " + m.sortKey.String()
		return m, nil
	case "S":
		m.sortDesc = !m.sortDesc
		m.applyFilter()
		m.status = "sort " + m.sortKey.String()
		return m, nil
	case "y":
		e, ok := m.selected()
		if !ok {
			m.status = "nothing selected"
			return m, nil
		}
		ep := listen.Endpoint(e)
		if err := clipboard.WriteAll(ep); err != nil {
			m.status = ""
			m.err = fmt.Errorf("copy: %w", err)
			return m, nil
		}
		m.err = nil
		m.status = "copied " + ep
		return m, nil
	case "x":
		e, ok := m.selected()
		if !ok {
			m.status = "nothing selected"
			return m, nil
		}
		if e.PID <= 0 {
			m.status = "pid unknown (try as root)"
			return m, nil
		}
		if e.Start == 0 {
			m.status = "process identity unknown (refusing to kill)"
			return m, nil
		}
		m.confirm = true
		return m, nil
	case "enter", " ":
		m.toggleFold()
		return m, nil
	case "right", "l":
		m.setFold(true)
		return m, nil
	case "left", "h":
		m.setFold(false)
		return m, nil
	case "up", "k", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
			m.clamp()
		}
	case "down", "j", "ctrl+n":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
			m.clamp()
		}
	case "pgup":
		m.cursor -= m.pageSize()
		m.clamp()
	case "pgdown":
		m.cursor += m.pageSize()
		m.clamp()
	case "home", "g":
		m.cursor = 0
		m.clamp()
	case "end", "G":
		m.cursor = len(m.rows) - 1
		m.clamp()
	}
	return m, nil
}

func (m *model) cycleSort() {
	order := []listen.SortKey{listen.SortPort, listen.SortName, listen.SortProject, listen.SortPID, listen.SortProto, listen.SortAddr}
	for i, k := range order {
		if m.sortKey == k {
			m.sortKey = order[(i+1)%len(order)]
			m.sortDesc = false
			return
		}
	}
	m.sortKey = listen.SortPort
}

func sortKeyAtX(x int) listen.SortKey {
	switch {
	case x < 9:
		return listen.SortProto
	case x < 16:
		return listen.SortPort
	case x < 39:
		return listen.SortAddr
	case x < 48:
		return listen.SortPID
	case x < 64:
		return listen.SortProject
	default:
		return listen.SortName
	}
}

func (m *model) toggleFold() {
	r, ok := m.selectedRow()
	if !ok || r.e.PID <= 0 {
		return
	}
	if r.fold != foldCollapsed && r.fold != foldExpanded && r.fold != foldChild {
		return
	}
	m.expanded[r.e.PID] = !m.expanded[r.e.PID]
	m.applyFilter()
}

func (m *model) setFold(open bool) {
	r, ok := m.selectedRow()
	if !ok || r.e.PID <= 0 {
		return
	}
	if r.fold == foldNone {
		return
	}
	m.expanded[r.e.PID] = open
	m.applyFilter()
}

func (m *model) applyFilter() {
	keep := make([]string, 0, 3)
	if r, ok := m.selectedRow(); ok {
		keep = append(keep, r.id(), r.e.Key())
		if r.e.PID > 0 {
			keep = append(keep, "p/"+strconv.Itoa(r.e.PID))
		}
	}
	filtered := listen.FilterQuery(m.all, m.filter.Value())
	m.rows = flattenGroups(filtered, m.sortKey, m.sortDesc, m.expanded)
	for _, id := range keep {
		for i, r := range m.rows {
			if r.id() == id || r.e.Key() == id {
				m.cursor = i
				m.clamp()
				return
			}
		}
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clamp()
}

func (m *model) clamp() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if n := len(m.rows); n == 0 {
		m.cursor = 0
		m.offset = 0
		return
	} else if m.cursor >= n {
		m.cursor = n - 1
	}
	ps := m.pageSize()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+ps {
		m.offset = m.cursor - ps + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m model) pageSize() int {
	h := m.height - 11
	if h < 1 {
		return 1
	}
	return h
}

func (m model) selectedRow() (viewRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return viewRow{}, false
	}
	return m.rows[m.cursor], true
}

func (m model) selected() (listen.Entry, bool) {
	r, ok := m.selectedRow()
	if !ok {
		return listen.Entry{}, false
	}
	return r.e, true
}

func (m model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	var b strings.Builder
	title := titleStyle.Render("lsoff")
	arrow := "↑"
	if m.sortDesc {
		arrow = "↓"
	}
	meta := helpStyle.Render(fmt.Sprintf("  %d/%d%s  %s%s", len(m.rows), len(m.all), protoLabel(m.wantTCP, m.wantUDP), m.sortKey.String(), arrow))
	if m.auto {
		meta += helpStyle.Render("  auto")
	}
	if m.loading {
		meta += helpStyle.Render("  loading")
	}
	b.WriteString(title + meta + "\n")

	b.WriteString(m.filter.View() + "\n")

	b.WriteString(headerRow.Render(m.formatHeader()) + "\n")

	ps := m.pageSize()
	end := min(m.offset+ps, len(m.rows))
	if len(m.rows) == 0 {
		b.WriteString(helpStyle.Render("  no matching listeners") + "\n")
	} else {
		for i := m.offset; i < end; i++ {
			b.WriteString(m.formatRow(m.rows[i], i == m.cursor) + "\n")
		}
	}

	b.WriteString("\n")
	if e, ok := m.selected(); ok {
		svc := listen.ServiceName(e.Proto, e.Port)
		if svc == "" {
			svc = strings.Join(listen.SearchTerms(e.Proto, e.Port), ", ")
		}
		b.WriteString(pathStyle.Render("SVC   "+dash(svc)) + "\n")
		b.WriteString(pathStyle.Render("PATH  "+dash(listen.SanitizeDisplay(e.Path))) + "\n")
		b.WriteString(pathStyle.Render("CMD   "+dash(truncate(listen.SanitizeDisplay(e.Cmdline), max(8, m.width-6)))) + "\n")
		b.WriteString(pathStyle.Render("CWD   "+dash(truncate(listen.SanitizeDisplay(listen.ShortCwd(e.Cwd)), max(8, m.width-6)))) + "\n")
	} else {
		b.WriteString("\n\n\n\n")
	}

	switch {
	case m.confirm:
		e, _ := m.selected()
		msg := fmt.Sprintf("Kill %s (pid %d)?  y / n", dash(listen.SanitizeDisplay(e.Name)), e.PID)
		b.WriteString(confirmBox.Render(msg) + "\n")
	case m.err != nil:
		b.WriteString(errStyle.Render(m.err.Error()) + "\n")
	case m.status != "":
		b.WriteString(okStyle.Render(m.status) + "\n")
	default:
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("/ search  j/k move  enter expand  y copy  a auto  s sort  x kill  q quit"))
	return b.String()
}

func protoLabel(tcp, udp bool) string {
	switch {
	case tcp && !udp:
		return "  tcp"
	case udp && !tcp:
		return "  udp"
	default:
		return ""
	}
}

func (m model) formatHeader() string {
	return fmt.Sprintf("   %-4s  %5s  %-21s  %7s  %-14s  %s", "PROTO", "PORT", "ADDRESS", "PID", "PROJECT", "PROCESS")
}

func (m model) formatRow(r viewRow, selected bool) string {
	e := r.e
	name := listen.SanitizeDisplay(e.Name)
	if name == "" {
		name = "-"
	}
	if r.hidden > 0 {
		name = fmt.Sprintf("%s  +%d", name, r.hidden)
	}
	proj := listen.SanitizeDisplay(e.Project)
	if proj == "" {
		proj = "-"
	}
	maxName := max(8, m.width-64)
	name = truncate(name, maxName)
	addr := truncate(listen.SanitizeDisplay(e.Addr), 21)
	proto := fmt.Sprintf("%-4s", e.Proto.String())
	rest := fmt.Sprintf("  %5d  %-21s  %7s  %-14s  %s", e.Port, addr, pidCell(e.PID), truncate(proj, 14), name)
	line := " " + r.mark() + " " + proto + rest
	if selected {
		return selStyle.Render(padRight(line, m.width))
	}
	return " " + r.mark() + " " + protoCell(e.Proto) + rest
}

func protoCell(p listen.Proto) string {
	s := fmt.Sprintf("%-4s", p.String())
	if p == listen.UDP {
		return udpStyle.Render(s)
	}
	return tcpStyle.Render(s)
}

func pidCell(pid int) string {
	if pid <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", pid)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func padRight(s string, n int) string {
	plain := lipgloss.Width(s)
	if plain >= n {
		return s
	}
	return s + strings.Repeat(" ", n-plain)
}
