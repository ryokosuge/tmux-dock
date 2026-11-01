# AGENTS.md

tmux-dock を実装する担当（人間・agent）向けの手引き。いま決着していることだけを書く。後続の Task で増えたら育てる。

## 前提の道具

- Go 1.27 系（`go.mod` は `go 1.27`、module は `github.com/ryokosuge/tmux-dock`）
- tmux（手元の確認は 3.7c。CI（`.github/workflows/ci.yml`）は macOS ランナーで `brew install tmux` した最新版を使う。tmux の版は固定していない）
- make

## 検査の入口

手元の検査の入口は `make check` で、次の 3 つを順に流す。

1. gofmt の確認（`gofmt -l .` の出力が空であること）
2. `go vet ./...`
3. `go test ./...`

入口は repo 直下の `Makefile`（`fmt-check` → `vet` → `test`）。gofmt は未整形ファイルがあっても `gofmt -l` の終了コードが 0 なので、出力が空でなければファイル名を出して失敗させる。golangci-lint は足さない。

## 言語とライブラリの方針

- 言語は Go、TUI は Bubble Tea v2（`charm.land/bubbletea/v2`）。bubbles / lipgloss は足さない
- 決まっていない依存を足さない。必要と感じたら足さずに質問として返す
- tmux とのやりとりは `os/exec` で tmux を呼ぶ。control mode は使わない
- tmux 出力の解析・コマンド引数の組み立て・判定は、tmux を呼ぶ部分から分けた純粋関数にする（例: `internal/dock` の `NewPlan` は引数だけを組み立て、tmux を呼ばない）

## 設定（環境変数）

`internal/dock` の `ConfigFromEnv` が読む。未設定または空なら既定値になる。

| 環境変数 | 既定 | 意味 |
|---|---|---|
| `TMUX_DOCK_INNER_SOCKET` | `default` | 普段の tmux サーバのソケット名 |
| `TMUX_DOCK_OUTER_SOCKET` | `tmux-dock` | 外側の tmux サーバのソケット名 |

外側の tmux は `tmux -L <outer socket> -f <conf>` で起動する。普段のサーバの一覧は `tmux -L <inner socket> list-sessions -F ...` で読む（サーバが無いと終了コード 1 になり、0 件として扱う）。

## テストの作法

- 純粋関数は unit テストで確かめる（`internal/dock/config_test.go` / `plan_test.go` の流儀に合わせる）
- tmux を要る integration テストは、`TMUX_DOCK_INNER_SOCKET` / `TMUX_DOCK_OUTER_SOCKET` に使い捨てのソケット名を指し、`-f /dev/null` で試験用サーバを立てる
- テストの後は立てた試験用サーバを片付ける

## 守る不変条件

普段の tmux サーバ（既定ソケット）のセッションと window の構成は、本人が一覧で新規作成か切り替えを選んだとき以外に変わらない。

- テスト・動作確認のどちらでも、既定ソケットの tmux サーバを操作しない。確認が要るときは使い捨てのソケット名を指す
- 外側の tmux の操作は外側のソケット（`-L <outer socket>`）だけに向ける

## Verify

PR を出す前に通す（CI と同じ判定）:

```
make check
```

環境の疎通（Go と tmux が使えるか）は次で確かめる。専用の doctor コマンドはまだ無い。

```
go version
tmux -V
```

Linux（Ubuntu）の Go 1.27.3 / tmux 3.2a では、既存の `TestTmuxDockRightPane` の待機メッセージ検査が失敗する（`capture-pane -p -J` の結果に空白が入り、期待文字列と一致しない）。fmt-check / vet は通るが、この環境で `make check` が通るとは扱わない。

1 パッケージだけを短く回すときは `go test ./internal/<パッケージ> -run <テスト名>`。integration テストも同じ `go test` で走り、tmux が PATH に無いと skip せず失敗する。

## Conventions

- このファイルや README と既存コードの書き方が食い違うときは既存コードに従い、動いているコードをドキュメントに合わせて書き換えず、見つけた食い違いを PR 本文に列挙する
- テストの置き場・名前・使い捨てソケットの作り方は、既存の `*_test.go` / `*_integration_test.go` の流儀に合わせる（`tdk-o-` / `tdk-i-` / `tdk-h-` / `tdk-sl-` + pid + nanotime の短い hex、macOS のソケットパス上限 104 バイトに収める）

## Code Review Rules

<!-- レビューで見つけるべき判断の規約を書く（実装の作法は Conventions へ、機械的に検査できるものは lint / CI へ） -->

<!-- evidence: AGENTS.md（既存）, Makefile, .github/workflows/ci.yml, go.mod, internal/*/*_test.go, cmd/tmux-dock/main_test.go -->
