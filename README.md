# tmux-dock
tmux のセッション一覧を左に常駐させ、右のペインで選んだセッションを直接操作する TUI

## インストール

clone した repo で `go install` する。

```sh
go install ./cmd/tmux-dock
```

Ghostty の起動コマンドはログインシェル経由で `tmux-dock` を探すので、`$(go env GOPATH)/bin` をログインシェルの PATH に通しておく。

## Ghostty の設定

`~/.config/ghostty/config` に次の 2 行を足す。

起動コマンドの行。本人のログインシェルの絶対パスで包む（Finder から開いたときの PATH は最小で、`tmux-dock` だけでは見つからないため）。

```
# fish の例
initial-command = /opt/homebrew/bin/fish -l -c tmux-dock
# zsh の例
initial-command = /bin/zsh -l -c tmux-dock
```

どちらか 1 行だけを書く。`initial-command` は最初の 1 枚の tab にだけ効くので、2 枚目以降の Ghostty の tab では tmux-dock は起動しない。

一覧と右を行き来するキーの行。Cmd+\ で F12 を送る。

```
keybind = cmd+backslash=text:\x1b[24~
```

## 普段の tmux の設定

必須の設定。コピーした文字を外側へ届けるため、普段の tmux の `set-clipboard` を `external` にする。既定値なので、off にしていなければ足さなくてよい。

```tmux
set -s set-clipboard external
```

任意の設定。右で見ているセッションが消えたとき、tmux が別のセッションへ移す。

```tmux
set -g detach-on-destroy off
```

この設定が無い（既定の on の）ときは、右の attach が終わって右ペインが待機へ戻り、セッションができると繋ぎ直す。

## 一覧と右を行き来するキー

Ghostty で Cmd+\ を押すと F12 が送られ、外側の tmux が F12 で一覧と右を往復する。F12 は右の普段のセッションの中では使えなくなる。
