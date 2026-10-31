package sessionlist

import (
	"context"
	"fmt"
	"strings"

	"github.com/ryokosuge/tmux-dock/internal/dock"
)

// listSessionsNoServer は list-sessions がサーバ無しを伝える終了コード。
const listSessionsNoServer = 1

// noServerPrefixes はサーバ無しのときの stderr の先頭。
// ソケットのファイルが無いと前者、ファイルはあるが接続できないと後者（tmux 3.7c）。
var noServerPrefixes = []string{"error connecting to", "no server running on"}

// ListArgs は普段のサーバ（innerSocket）だけを指す list-sessions の引数を組み立てる。
// 先頭の "tmux" は含まない。tmux は呼ばない。
func ListArgs(innerSocket string) []string {
	return []string{"-L", innerSocket, "list-sessions", "-F", Format}
}

// Fetch は普段の tmux サーバ（innerSocket）のセッション一覧を list-sessions だけで読む。
// サーバが無いこと（終了コード 1 かつ stderr が noServerPrefixes で始まる）は 0 件でありエラーにしない。
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

func isNoServer(stderr string) bool {
	for _, p := range noServerPrefixes {
		if strings.HasPrefix(stderr, p) {
			return true
		}
	}
	return false
}
