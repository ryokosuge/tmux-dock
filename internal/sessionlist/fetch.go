package sessionlist

import (
	"context"
	"fmt"
	"strings"

	"github.com/ryokosuge/tmux-dock/internal/dock"
)

// listSessionsNoServer は list-sessions がサーバ無しを伝える終了コード。
const listSessionsNoServer = 1

const (
	// noServerPrefix は接続できるソケットにサーバが居ないときの stderr の先頭。
	noServerPrefix = "no server running on"
	// noSocketPrefix / noSocketSuffix はソケットのファイルが無いときの stderr の先頭と末尾。
	// 権限なし・ソケットでない・パスが長すぎる等も同じ先頭になるので、末尾まで見て区別する。
	noSocketPrefix = "error connecting to"
	noSocketSuffix = "(No such file or directory)"
)

// ListArgs は普段のサーバ（innerSocket）だけを指す list-sessions の引数を組み立てる。
// 先頭の "tmux" は含まない。tmux は呼ばない。
func ListArgs(innerSocket string) []string {
	return []string{"-L", innerSocket, "list-sessions", "-F", Format}
}

// Fetch は普段の tmux サーバ（innerSocket）のセッション一覧を list-sessions だけで読む。
// サーバが無いこと（終了コード 1 かつ isNoServer）は 0 件でありエラーにしない。
// list-sessions はサーバを起動しないので、普段のサーバの構成は変わらない。
func Fetch(ctx context.Context, run dock.Runner, innerSocket string) ([]Session, error) {
	args := ListArgs(innerSocket)
	res, err := run(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("tmux %s を実行できない: %w", strings.Join(args, " "), err)
	}
	if res.ExitCode == 0 {
		return Parse(res.Stdout), nil
	}
	if res.ExitCode == listSessionsNoServer && isNoServer(res.Stderr) {
		return nil, nil
	}
	return nil, fmt.Errorf("tmux %s が終了コード %d で失敗: %s",
		strings.Join(args, " "), res.ExitCode, res.Stderr)
}

// isNoServer は list-sessions の stderr がサーバ無し（tmux 3.7c の 2 形）かを返す。純粋関数。
func isNoServer(stderr string) bool {
	msg := strings.TrimSpace(stderr)
	return strings.HasPrefix(msg, noServerPrefix) ||
		(strings.HasPrefix(msg, noSocketPrefix) && strings.HasSuffix(msg, noSocketSuffix))
}
