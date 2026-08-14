# lsoff

[English](README.md) | [日本語](README.ja.md)

Windows / Linux / macOS で LISTEN 中の TCP/UDP ポートを一覧する CLI / TUI。

`lsof` のポート版、というより「今誰がこのポートを掴んでいるか」をすぐ見つけて、必要ならプロセスを終了するための小さなツールです。

## インストール

バイナリは [GitHub Releases](https://github.com/yutat23/lsoff/releases) に付いています（`lsoff-darwin-arm64`、`lsoff-linux-amd64`、`lsoff-windows-amd64.exe` など）。

```bash
go install github.com/yutat23/lsoff@latest
```

または:

```bash
git clone https://github.com/yutat23/lsoff
cd lsoff
go build -o lsoff .
```

タグを push すると（`git tag v0.1.0 && git push origin v0.1.0`）バイナリをビルドして Release に添付します。

macOS では libproc を使うため CGO が必要です（通常の `go build` で有効です）。

## 使い方

```bash
lsoff                 # TUI（stdout が TTY でないときは表を出す）
lsoff 8080            # ポート 8080 を掴んでいるプロセスを表示
lsoff nginx           # 名前・パス・PID・cmdline・プロジェクト・サービス名などで検索
lsoff -q "node 8080"  # 空白区切りは AND
lsoff -t              # TCP だけ（TUI）
lsoff -u 53           # UDP の 53 番
lsoff --json nginx    # 検索結果を JSON で
lsoff -k 8080         # そのポートのプロセスを終了（確認あり）
lsoff -k -y 8080      # 確認なし（パイプ経由では必須）
lsoff -h
lsoff -v
```

ポート指定時の出力例:

```
PROTO  PORT  ADDRESS      PID    PROJECT  PROCESS  PATH                 CMD                   CWD
tcp    8080  127.0.0.1    41233  lsoff    node     /usr/local/bin/node  /usr/local/bin/node   ~/mywork/lsoff
```

### TUI

| 操作 | 動作 |
|------|------|
| `/` / `Search:` をクリック / `ctrl+f` | 検索（ポート・PID・名前・プロジェクト・パス・cmdline。空白は AND） |
| `↑` / `↓` / クリック / ホイール | 移動・選択（検索中も可） |
| ヘッダークリック | その列でソート（もう一度で降順） |
| `s` / `S` | ソート列を切り替え / 昇降順 |
| `y` | 選択中の `addr:port` をコピー |
| `a` | 2 秒ごとの自動更新 |
| `enter` / `space` / click `▸` | 同じ PID のポートを展開・折りたたみ |
| `h` / `l` | 折りたたみ / 展開 |
| `esc` / `ctrl+c` | 検索をクリア |
| `r` | 再読み込み |
| `k` | 選択中プロセスを終了（確認あり） |
| `q` | 終了 |

TUI では `tcp` を緑、`udp` をアンバーで色分けします。選択中の行は背景色が優先されます。CLI の表はスクリプト向けのため着色しません。プロセス名、cmdline、パスは端末向けに制御文字と ANSI/OSC を除去します。JSON は元の文字列を保持します。

同じ PID のソケット（よくある IPv4 + IPv6）は最初 `▸` と `+N` で折りたたみます。`enter` で展開します。

よく使うサービス名（http, postgres, redis, vite, …）は検索でき、フッターの `SVC` に出ます。3000 番のように名前が一つに決まらないポートは検索エイリアスのみです。echo / chargen などの歴史的ポートは入れていません。JSON は表示名があるとき `"service"` を付けます。

`k` のあとに `y` で実行、`n` または `esc` でキャンセルします。kill は一覧時のプロセスと同一かを検証します。Linux は `pidfd_open`（fd が PID 番号ではなくそのプロセスを指す）、macOS は `pbi_start_tvsec` / `usec` を確認してから signal、Windows は開いたハンドルの `CreationTime` です。Unix ではまず SIGTERM、2 秒残っていたら同じプロセスへ SIGKILL です。Windows は `TerminateProcess` です。pid 1 と自身は殺しません。

CLI の `-k` も同じです。同じ PID が IPv4/IPv6 で二重に出ていても一度だけ終了します。パイプから呼ぶときは誤爆防止のため `-y` が必要です。

## 制限

- **macOS の kill は atomic ではない。** pidfd 相当がない。`pbi_start_tvsec` / `usec` を `kill(2)` の直前に再確認するが、その隙間で PID が再利用される余地は残る。Linux の pidfd と Windows のプロセスハンドルにはこの隙間がない。
- **Windows の cmdline / cwd** は `NtQueryInformationProcess` を使う。Microsoft は内部 API であり将来変わり得ると説明している。64-bit ビルドは WOW64（32-bit）プロセスの文字列を 32-bit PEB から読む。`ProcessCommandLineInformation` は *呼び出し側* のポインタ幅で解釈し、取れなければ対象の bitness で PEB の `CommandLine` にフォールバックする。32-bit の lsoff から 64-bit プロセスの cmdline / cwd は取れない。
- **バージョン** は `go build` 時に埋め込まれる。`lsoff -v` が `main.go` と違うときはビルドし直す。

## OS ごとの取得方法

外部コマンド（`lsof` / `ss` / `netstat`）には依存しません。

| OS | API |
|----|-----|
| Linux | `/proc/net/{tcp,tcp6,udp,udp6}`、`/proc/<pid>/{fd,cmdline,cwd}` |
| macOS | `libproc`（ソケットと cwd）と `sysctl kern.procargs2` |
| Windows | IP Helper、`QueryFullProcessImageName`、`NtQueryInformationProcess`（cmdline と cwd） |

権限のないプロセスは PID やパス、cmdline が空になることがあります。その場合は root / Administrator で実行してください。

UDP に LISTEN 状態はないため、リモート未接続でポートに bind されているソケットを表示します。

## ライセンス

MIT
