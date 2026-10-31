// Package rightpane は右ペインの待機と attach の判定・引数・環境を組み立てる。
// tmux は呼ばず、実行は呼び出し側が担う。
package rightpane

import "strings"

// WaitingMessage は普段の tmux サーバにセッションが無いときの待機表示。
const WaitingMessage = "普段の tmux サーバにセッションがありません。セッションができるとここに繋がります"

// Action は右ペインがとる動作。
type Action int

const (
	// Wait はセッションができるまで待つ。
	Wait Action = iota
	// Attach は普段のサーバのセッションへ attach する。
	Attach
)

const envTmuxPanePrefix = "TMUX_PANE="

// Decide はセッション数から動作を決める。0 以下は Wait、1 以上は Attach。
func Decide(sessionCount int) Action {
	if sessionCount >= 1 {
		return Attach
	}
	return Wait
}

// AttachArgs は普段のサーバへ attach する tmux の引数（先頭の "tmux" は含まない）を返す。
// -t は付けない。
func AttachArgs(innerSocket string) []string {
	return []string{"-L", innerSocket, "attach-session"}
}

// AttachEnv は environ から TMUX_PANE= で始まる要素だけを除いた新しいスライスを返す。
// 外側のペイン id で普段のサーバのペインを引いてしまうのを避けるため。入力は変更しない。
func AttachEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, e := range environ {
		if !strings.HasPrefix(e, envTmuxPanePrefix) {
			out = append(out, e)
		}
	}
	return out
}
