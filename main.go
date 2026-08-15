package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/yutat23/lsoff/internal/listen"
	"github.com/yutat23/lsoff/internal/tui"
	"golang.org/x/term"
)

var version = "0.1.3"

type config struct {
	tcp     bool
	udp     bool
	json    bool
	kill    bool
	yes     bool
	help    bool
	version bool
	port    *uint16
	query   string
}

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			if ee.msg != "" {
				fmt.Fprintln(os.Stderr, ee.msg)
			}
			os.Exit(ee.code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string, in io.Reader, out, errw io.Writer) error {
	cfg, err := parseArgs(args)
	if err != nil {
		return err
	}
	if cfg.help {
		fmt.Fprint(out, usage())
		return nil
	}
	if cfg.version {
		fmt.Fprintf(out, "lsoff %s\n", version)
		return nil
	}

	useTUI := cfg.port == nil && !cfg.json && !cfg.kill && isTTY(out)
	if useTUI {
		return tui.Run(cfg.tcp, cfg.udp, cfg.query)
	}

	entries, err := listen.List()
	if err != nil {
		return err
	}
	entries = listen.FilterProto(entries, cfg.tcp, cfg.udp)
	if cfg.port != nil {
		entries = listen.FilterPort(entries, *cfg.port)
	}
	if cfg.query != "" {
		entries = listen.FilterQuery(entries, cfg.query)
	}

	if cfg.kill {
		return runKill(cfg, entries, in, out, errw)
	}

	if len(entries) == 0 && (cfg.port != nil || cfg.query != "") {
		return &exitError{code: 1, msg: noneFound(cfg)}
	}
	if cfg.json {
		return listen.FormatJSON(out, entries)
	}
	return listen.FormatTable(out, entries)
}

func runKill(cfg config, entries []listen.Entry, in io.Reader, out, errw io.Writer) error {
	if len(entries) == 0 {
		return &exitError{code: 1, msg: noneFound(cfg)}
	}
	if err := listen.FormatTable(out, entries); err != nil {
		return err
	}
	ids := listen.UniqueIdents(entries)
	if len(ids) == 0 {
		return &exitError{code: 1, msg: "no process ids to kill (try as root)"}
	}
	pids := make([]int, len(ids))
	for i, id := range ids {
		pids[i] = id.PID
	}
	if !cfg.yes {
		ok, err := confirmKill(in, errw, pids)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(errw, "cancelled")
			return &exitError{code: 1}
		}
	}
	if err := listen.KillAll(ids); err != nil {
		return err
	}
	for _, id := range ids {
		fmt.Fprintf(errw, "killed pid %d\n", id.PID)
	}
	return nil
}

func confirmKill(in io.Reader, errw io.Writer, pids []int) (bool, error) {
	if f, ok := in.(*os.File); ok && !term.IsTerminal(int(f.Fd())) {
		return false, fmt.Errorf("refusing to kill without -y (stdin is not a TTY)")
	}
	label := "process"
	if len(pids) != 1 {
		label = "processes"
	}
	fmt.Fprintf(errw, "Kill %d %s %v? [y/N] ", len(pids), label, pids)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	s := strings.ToLower(strings.TrimSpace(line))
	return s == "y" || s == "yes", nil
}

func noneFound(cfg config) string {
	switch {
	case cfg.port != nil && cfg.query != "":
		return fmt.Sprintf("no listeners on port %d matching %q", *cfg.port, cfg.query)
	case cfg.port != nil:
		return fmt.Sprintf("no listeners on port %d", *cfg.port)
	case cfg.query != "":
		return fmt.Sprintf("no listeners matching %q", cfg.query)
	default:
		return "no listeners"
	}
}

func isTTY(out io.Writer) bool {
	f, ok := out.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func parseArgs(args []string) (config, error) {
	var cfg config
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			cfg.help = true
			return cfg, nil
		case a == "-v" || a == "--version":
			cfg.version = true
			return cfg, nil
		case a == "-t" || a == "--tcp":
			cfg.tcp = true
		case a == "-u" || a == "--udp":
			cfg.udp = true
		case a == "-j" || a == "--json":
			cfg.json = true
		case a == "-k" || a == "--kill":
			cfg.kill = true
		case a == "-y" || a == "--yes":
			cfg.yes = true
		case a == "-q" || a == "--query":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("%s requires a value\n\n%s", a, usage())
			}
			i++
			cfg.query = args[i]
		case strings.HasPrefix(a, "--query="):
			cfg.query = strings.TrimPrefix(a, "--query=")
		case strings.HasPrefix(a, "-q="):
			cfg.query = strings.TrimPrefix(a, "-q=")
		case strings.HasPrefix(a, "-"):
			return cfg, fmt.Errorf("unknown flag %s\n\n%s", a, usage())
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) > 1 {
		return cfg, fmt.Errorf("too many arguments\n\n%s", usage())
	}
	if len(positional) == 1 {
		n, err := strconv.ParseUint(positional[0], 10, 16)
		if err != nil {
			if cfg.query != "" {
				return cfg, fmt.Errorf("too many search terms (use quotes: %q)", cfg.query+" "+positional[0])
			}
			cfg.query = positional[0]
		} else {
			p := uint16(n)
			cfg.port = &p
		}
	}
	if cfg.kill && cfg.port == nil {
		return cfg, fmt.Errorf("-k requires a port\n\n%s", usage())
	}
	if cfg.kill && cfg.json {
		return cfg, fmt.Errorf("cannot combine -k and --json")
	}
	if cfg.yes && !cfg.kill {
		return cfg, fmt.Errorf("-y can only be used with -k")
	}
	return cfg, nil
}

func usage() string {
	return `lsoff - list listening TCP/UDP ports

Usage:
  lsoff              interactive TUI (table if stdout is not a TTY)
  lsoff <port>       show processes listening on port
  lsoff <query>      search by name, project, path, pid, or port substring
  lsoff -k <port>    kill those processes (asks for confirmation)
  lsoff -h           help
  lsoff -v           version

Flags:
  -t, --tcp          TCP only
  -u, --udp          UDP only
  -q, --query <str>  search (name, project, path, pid, port); words are AND
  -j, --json         JSON output
  -k, --kill         kill processes on <port>
  -y, --yes          do not ask before -k (required if stdin is not a TTY)

TUI:
  / or click Search      filter as you type
  ↑/↓ / j/k / wheel      move
  click header           sort by column
  y                      copy addr:port
  a                      auto-refresh
  s / S                  sort / reverse
  enter / space          expand or collapse a process
  h / l                  collapse / expand
  esc / ctrl+c           clear search
  r                      refresh
  x                      kill selected process (asks for confirmation)
  q                      quit
`
}
