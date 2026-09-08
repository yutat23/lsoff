# Changelog

[English](CHANGELOG.md) | [日本語](CHANGELOG.ja.md)

このプロジェクトの主な変更はこのファイルに記録します。

形式は [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) に従い、バージョンは [Semantic Versioning](https://semver.org/lang/ja/) です。

## [Unreleased]

## [0.1.5] - 2026-09-09

### Added

- Docker: `/var/run/docker.sock` 経由でローカルの Docker Engine API から TCP/UDP の公開ポート情報を取得し、ホストのリスナー一覧に追加。`PROCESS` にコンテナ名、TUI の詳細欄にコンテナ側のポート・プロトコルとコンテナ ID を表示する。
- TUI: `4` / `6` で IPv4 または IPv6 の行だけ表示するフィルタを追加。同じキーをもう一度押すと解除する。
- TUI: `ctrl+u` / `ctrl+d` で 1 ページ上 / 下へ移動できるようにした。検索中も利用できる。

### Changed

- TUI: プロセスまたは Docker コンテナ、プロトコル、ホスト側ポートを単位にグループ化するようにした。同じグループの IPv4/IPv6 の行はまとめて展開でき、ポートやプロトコルが異なるものは別グループになる。

### Fixed

- macOS: root なしで実行すると、他ユーザー（`sshd` や `kdc`、`screensharingd` などの root デーモン）のリスナーが一覧から丸ごと消えていた。`sysctl net.inet.{tcp,udp}.pcblist64` で全リスニングソケットを列挙し、プロセスを覗けない行は Linux と同様に PID / PROCESS を `-` で表示するようにした。
- macOS: 既に終了したプロセスを kill すると、成功扱いにならず `proc_pidinfo bsdinfo: 0` というエラーになっていた（Linux とは挙動が違っていた）。
- TUI: 詳細欄の PATH 行が端末幅で切り詰められておらず、長い macOS のアプリパスで折り返してフッターがずれていた。
- TUI と CLI の表: 全角文字（日本語のプロジェクト名やプロセス名）を含むと、その右側の列がずれていた。表示幅でパディングするようにした。
- TUI: 自動更新を off → on と切り替えるたびに更新ループが 1 本増え、更新間隔がどんどん短くなっていた。
- TUI: PID 不明の行が同じアドレス・ポートに 2 つあると（Linux で root なし、`SO_REUSEPORT` のリスナーなど）、片方が 2 回表示されもう片方が消えていた。
- CLI: `lsoff 65536` のように 0-65535 に収まらない数値を検索語として扱っていたのをやめ、不正なポートとしてエラーにした。
- CLI: `-k` の結果を PID ごとに報告するようにした（`killed pid N` または `pid N: error`）。1 つの PID で失敗しても、kill できた分が隠れない。
- Windows: サイズ問い合わせと取得の間にソケット表が増えると一覧全体が失敗していたのを、再取得するようにした。
- Linux: `/proc/net` のアドレス解釈をビッグエンディアン環境でも正しくした。

## [0.1.4] - 2026-08-25

### Added

- PID が不明な行（`PID <= 0`）を隠す機能を追加（TUI の `p` キーまたは `-p` / `--pid` フラグ）。`-t` / `-u` と同じ表示フィルタなので、0 件でもエラーにはせず、空の表または `[]` を返す。

### Changed

- TUI でグループを展開したとき、子ソケットをツリー表示にした。先頭は `▾`、最後以外の子は `├─`、最後の子は `└─`。マーク列の幅は固定なので、どの行でも列がヘッダーと揃う。
- TUI フッターを `a auto-refresh` と `r reload` に変更（`r` はこれまでバーに出ていなかった）。

## [0.1.3] - 2026-08-16

### Changed

- TUI 下部のショートカットを、キー色とセパレータだけ残した控えめな表示にし、フッターの上にラインを引いた。

## [0.1.2] - 2026-08-16

### Changed

- TUI ヘルプの `enter fold` を `enter expand` に変えた。

### Added

- リリース時に Homebrew tap の formula を更新するようにした。

## [0.1.1] - 2026-08-15

### Changed

- TUI の移動に矢印キーに加えて `j` / `k` を使えるようにした。
- TUI の kill を `k` から `x` に変更し、`k` を上移動に使った。

## [0.1.0] - 2026-08-14

### Added

- 初回リリース。Windows / Linux / macOS で LISTEN 中の TCP/UDP ポートを一覧し、TUI と任意の kill を提供する。

[Unreleased]: https://github.com/yutat23/lsoff/compare/v0.1.5...HEAD
[0.1.5]: https://github.com/yutat23/lsoff/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/yutat23/lsoff/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/yutat23/lsoff/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/yutat23/lsoff/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/yutat23/lsoff/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/yutat23/lsoff/releases/tag/v0.1.0
