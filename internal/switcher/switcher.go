package switcher

import (
	"context"
	"fmt"
	"strings"

	"github.com/ryokosuge/tmux-dock/internal/dock"
	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

// rightPane は外側のセッションの右ペイン（pane-base-index 未設定なので .1）。
const rightPane = ":.1"

// PaneTTYArgs は外側のサーバ（outerSocket）の右ペインの tty を読む display-message の引数。
// 先頭の "tmux" は含まない。tmux は呼ばない。
func PaneTTYArgs(outerSocket, sessionName string) []string {
	return []string{"-L", outerSocket, "display-message", "-p", "-t", "=" + sessionName + rightPane, "#{pane_tty}"}
}

// SwitchArgs は普段のサーバ（innerSocket）で tty のクライアントを sessionID へ切り替える引数。
// -c を省くと tmux が別のクライアントを推定しうるので必ず付ける。tmux は呼ばない。
func SwitchArgs(innerSocket, tty, sessionID string) []string {
	return []string{"-L", innerSocket, "switch-client", "-c", tty, "-t", sessionID}
}

// Switch は右ペインのクライアントを s へ切り替える。tty の取得、switch-client の順に tmux を 2 回だけ呼ぶ。
// 普段のサーバに打つのは switch-client だけで、セッションと window は作らず消さない。
func Switch(ctx context.Context, run dock.Runner, cfg dock.Config, s sessionlist.Session) error {
	ttyArgs := PaneTTYArgs(cfg.OuterSocket, cfg.SessionName)
	res, err := run(ctx, ttyArgs)
	if err != nil {
		return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(ttyArgs, " "), err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("tmux %s が終了コード %d で失敗: %s", strings.Join(ttyArgs, " "), res.ExitCode, res.Stderr)
	}
	tty := strings.TrimSpace(string(res.Stdout))
	if tty == "" {
		return fmt.Errorf("tmux %s の出力が空で右ペインの tty を取れない", strings.Join(ttyArgs, " "))
	}

	args := SwitchArgs(cfg.InnerSocket, tty, s.ID)
	res, err = run(ctx, args)
	if err != nil {
		return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(args, " "), err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("tmux %s が終了コード %d で失敗: %s", strings.Join(args, " "), res.ExitCode, res.Stderr)
	}
	return nil
}
