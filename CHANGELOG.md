# Changelog

[English](CHANGELOG.md) | [日本語](CHANGELOG.ja.md)

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.6] - 2026-11-13

### Changed

- Docker: the Engine socket is no longer fixed to `/var/run/docker.sock`. A `unix://` `DOCKER_HOST` is used on its own; otherwise `/var/run/docker.sock`, `$XDG_RUNTIME_DIR/docker.sock` (rootless Docker) and `~/.docker/run/docker.sock` (Docker Desktop) are tried in order within one 750 ms budget. Any other `DOCKER_HOST` (`tcp://`, `ssh://`, …) disables Docker discovery, since a remote engine's ports are not listeners on this host.

### Fixed

- macOS: an IPv6 wildcard listener that also accepts IPv4 (a dual-stack socket, such as `rapportd`) was shown with the address `0.0.0.0` instead of `::`. The TUI `6` filter hid it and `y` copied the wrong address. It is now shown as `::`, as `lsof` and Linux report it.
- TUI: a list reload that finished while the kill confirmation was open could move the selection to another row, and `y` then killed that process instead of the one named when `x` was pressed. The confirmation now keeps the process it was opened for.
- TUI: the `PORT` and `ID` detail lines of a Docker row were indented by 12 spaces instead of starting at the left edge like `SVC` and `SRC`.
- Docker: pressing `x` on a Docker row in the TUI, or running `lsoff -k` on a port that only a container publishes, suggested `try as root`, which cannot help. Both now name the container and point at `docker stop`.
- CLI: the TUI key list in `lsoff -h` had fallen behind the TUI. It now lists `4` / `6`, `ctrl+u` / `ctrl+d`, `ctrl+f`, `g` / `G` and the other keys it was missing; the README key tables gained the same aliases.
- Linux: killing a process that had already exited between listing and the kill returned `no such process` instead of succeeding quietly as on macOS. A PID that now belongs to a different process is still refused.
- TUI: `killed pid N` was replaced by the socket count as soon as the reload after the kill finished, so it was visible for only a moment. A finished reload no longer overwrites feedback about an action (`killed pid N`, `copied …`, `auto-refresh on`); the socket count still updates when it is what the status line shows.
- TUI: with a full page of rows, the kill confirmation box made the view one line taller than the terminal, which pushed the title line off the top while it was open.
- TUI: a long search query made the search line wider than the terminal (89 cells for an 80-character query in 80 columns), so it wrapped and shifted every row below it, including the rows that mouse clicks are mapped to.
- TUI: a failed kill or copy was erased by the next successful reload, so with auto-refresh on the error vanished within 2 seconds. Such an error now stays until the next action reports its own result; a listing error is still cleared by the next successful reload.
- TUI: the `PORT` and `ID` detail lines of a Docker row showed the container protocol and ID without the terminal sanitizing applied to `PATH`, `CMD` and `CWD`.
- TUI: in a terminal narrower than about 74 columns the title line, the header and every row were wider than the terminal and wrapped, which shifted the lines below them and the rows that mouse clicks are mapped to; below 27 columns the shortcut bar wrapped onto extra lines. Lines are now cut at the terminal edge with `…`, from 12 columns up. Output at 74 columns and wider is unchanged.

## [0.1.5] - 2026-09-09

### Added

- Docker: supplement host listeners with published TCP/UDP ports from the local Docker Engine API over `/var/run/docker.sock`, showing container names in `PROCESS` and container endpoint details in the TUI.
- TUI: filter IPv4 or IPv6 rows with `4` / `6`; press the same key again to clear the filter.
- TUI: move one page up / down with `ctrl+u` / `ctrl+d`, including while searching.

### Changed

- TUI: group bindings by process or Docker container, protocol, and host port. IPv4/IPv6 bindings within a group can be expanded together; different ports and protocols remain separate groups.

### Fixed

- macOS: listeners owned by other users (root daemons such as `sshd`, `kdc`, `screensharingd`) were missing entirely when lsoff ran without root. All listening sockets are now enumerated with `sysctl net.inet.{tcp,udp}.pcblist64`, and rows whose process cannot be inspected are shown with `-` for PID / process, as on Linux.
- macOS: killing a process that had already exited returned `proc_pidinfo bsdinfo: 0` instead of succeeding quietly, unlike Linux.
- TUI: the PATH detail line was not truncated to the terminal width, so a long macOS app path wrapped and pushed the footer down.
- TUI and CLI table: full-width characters (Japanese project or process names) shifted the columns to their right. Cells are now padded by display width.
- TUI: toggling auto-refresh off and on again started an extra refresh loop each time, so the refresh interval kept shrinking.
- TUI: two rows with an unknown PID on the same address and port (for example `SO_REUSEPORT` listeners on Linux without root) were shown as one row twice, hiding the other.
- CLI: `lsoff 65536` and other numbers outside 0-65535 are rejected as an invalid port instead of being treated as a search query.
- CLI: `-k` now reports each PID separately (`killed pid N` or `pid N: error`), so a failure on one PID no longer hides the ones that were killed.
- Windows: the socket table is re-fetched if it grows between the size query and the read, instead of failing the whole listing.
- Linux: `/proc/net` address decoding is now correct on big-endian hosts.

## [0.1.4] - 2026-08-25

### Added

- Hide rows with an unknown PID (`PID <= 0`) with `p` in the TUI or the `-p` / `--pid` CLI flag. Like `-t` / `-u`, it is a view filter, so an empty result is an empty table or `[]`, not an error.

### Changed

- Expanded TUI groups render child sockets as a tree: `▾` on the head, `├─` for children before the last, and `└─` for the last one. The mark column is a fixed width, so the columns stay aligned with the header on every row.
- TUI footer shows `a auto-refresh` and `r reload` (reload was missing from the bar).

## [0.1.3] - 2026-08-16

### Changed

- TUI shortcut bar uses muted key colors and separators instead of a dim help line, with a rule above the footer.

## [0.1.2] - 2026-08-16

### Changed

- TUI help says `enter expand` instead of `enter fold`.

### Added

- Releases update the Homebrew tap formula.

## [0.1.1] - 2026-08-15

### Changed

- TUI movement uses `j` / `k` in addition to the arrow keys.
- TUI kill is now `x` (was `k`), so `k` can move up.

## [0.1.0] - 2026-08-14

### Added

- Initial release: list listening TCP/UDP ports on Windows, Linux, and macOS, with an interactive TUI and optional kill.

[Unreleased]: https://github.com/yutat23/lsoff/compare/v0.1.6...HEAD
[0.1.6]: https://github.com/yutat23/lsoff/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/yutat23/lsoff/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/yutat23/lsoff/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/yutat23/lsoff/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/yutat23/lsoff/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/yutat23/lsoff/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/yutat23/lsoff/releases/tag/v0.1.0
