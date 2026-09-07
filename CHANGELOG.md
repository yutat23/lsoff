# Changelog

[English](CHANGELOG.md) | [日本語](CHANGELOG.ja.md)

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[0.1.4]: https://github.com/yutat23/lsoff/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/yutat23/lsoff/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/yutat23/lsoff/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/yutat23/lsoff/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/yutat23/lsoff/releases/tag/v0.1.0
