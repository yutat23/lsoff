# lsoff

[English](README.md) | [日本語](README.ja.md)

<img width="842" height="469" alt="image" src="https://github.com/user-attachments/assets/1b13713d-8510-4e23-816f-730a31fdaee6" />

CLI / TUI that lists listening TCP/UDP ports on Windows, Linux, and macOS.

Think of it as a port-focused `lsof`: quickly see who is holding a port, then kill the process if you need to.

## Install

Binaries are attached to [GitHub Releases](https://github.com/yutat23/lsoff/releases) (`lsoff-darwin-arm64`, `lsoff-linux-amd64`, `lsoff-windows-amd64.exe`, …).

```bash
go install github.com/yutat23/lsoff@latest
```

Or:

```bash
git clone https://github.com/yutat23/lsoff
cd lsoff
go build -o lsoff .
```

A tagged push (`git tag v0.1.0 && git push origin v0.1.0`) builds those binaries and publishes the release.

macOS uses libproc, so CGO is required (enabled by a normal `go build`).

## Usage

```bash
lsoff                 # TUI (prints a table if stdout is not a TTY)
lsoff 8080            # show processes holding port 8080
lsoff nginx           # search by name, path, PID, cmdline, project, service…
lsoff -q "node 8080"  # space-separated words are AND
lsoff -t              # TCP only (TUI)
lsoff -u 53           # UDP port 53
lsoff --json nginx    # search results as JSON
lsoff -k 8080         # kill those processes (with confirmation)
lsoff -k -y 8080      # skip confirmation (required when piped)
lsoff -h
lsoff -v
```

Example output for a port query:

```
PROTO  PORT  ADDRESS      PID    PROJECT  PROCESS  PATH                 CMD                   CWD
tcp    8080  127.0.0.1    41233  lsoff    node     /usr/local/bin/node  /usr/local/bin/node   ~/mywork/lsoff
```

### TUI

| Key / action | What it does |
|------|------|
| `/` / click `Search:` / `ctrl+f` | Search (port, PID, name, project, path, cmdline; spaces are AND) |
| `↑` / `↓` / `j` / `k` / click / wheel | Move and select (works while searching too) |
| Click a header | Sort by that column (click again for descending) |
| `s` / `S` | Cycle sort column / toggle ascending-descending |
| `y` | Copy the selected `addr:port` |
| `a` | Auto-refresh every 2 seconds |
| `enter` / `space` / click `▸` | Expand or collapse sockets for the same PID |
| `h` / `l` | Collapse / expand |
| `esc` / `ctrl+c` | Clear the search |
| `r` | Reload |
| `x` | Kill the selected process (asks first) |
| `q` | Quit |

In the TUI, `tcp` is green and `udp` is amber. The selected row uses the highlight background instead. The CLI table stays uncolored so it stays script-friendly. Process names, command lines, and paths are sanitized for the terminal (control characters and ANSI/OSC sequences). JSON keeps the original strings.

Sockets that share a PID (typical IPv4 + IPv6) start collapsed as one row with `▸` and a `+N` count. `enter` expands them.

Known service names (http, postgres, redis, vite, …) are searchable and shown on the `SVC` footer line. Ambiguous ports such as 3000 are aliases-only and have no single display name. Historic ports (echo, chargen) are not included. JSON may include `"service"` when a display name exists.

After `x`, press `y` to confirm, or `n` / `esc` to cancel. Kill verifies that the process is still the same one that was listed: Linux uses `pidfd_open` (the fd refers to that process, not the PID number), macOS checks `pbi_start_tvsec`/`usec` then signals, and Windows checks `CreationTime` on an open process handle. On Unix it sends SIGTERM first, then SIGKILL if that same process is still alive after 2 seconds. On Windows it uses `TerminateProcess`. pid 1 and the lsoff process itself are never killed.

CLI `-k` follows the same rules. If the same PID appears twice (IPv4 and IPv6), it is killed only once. When stdin is not a TTY, `-y` is required so a pipe cannot kill by accident.

## Limitations

- **macOS kill is not atomic.** There is no pidfd. lsoff re-reads `pbi_start_tvsec`/`usec` immediately before `kill(2)`, but a PID can still be reused in that tiny window. Linux pidfd and Windows process handles do not have this gap.
- **Windows cmdline/cwd** use `NtQueryInformationProcess`, which Microsoft documents as an internal API that may change. A 64-bit build reads WOW64 (32-bit) process strings from the 32-bit PEB; `ProcessCommandLineInformation` is parsed with the *caller's* pointer size, then PEB `CommandLine` is used as a target-bitness fallback. A 32-bit lsoff cannot inspect 64-bit processes' cmdline or cwd.
- **Version** is baked in at `go build` time. Rebuild after pulling if `lsoff -v` disagrees with `main.go`.

## How it collects data

No external commands (`lsof` / `ss` / `netstat`).

| OS | API |
|----|-----|
| Linux | `/proc/net/{tcp,tcp6,udp,udp6}`, `/proc/<pid>/{fd,cmdline,cwd}` |
| macOS | `libproc` (sockets and cwd) and `sysctl kern.procargs2` |
| Windows | IP Helper, `QueryFullProcessImageName`, `NtQueryInformationProcess` (cmdline and cwd) |

Without permission, PID, path, and cmdline may be empty. Run as root / Administrator in that case.

UDP has no LISTEN state, so sockets bound to a port with no remote peer are shown.

## License

MIT
