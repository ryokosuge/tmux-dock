package switcher

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ryokosuge/tmux-dock/internal/dock"
	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

// listClientsNoServer は list-clients がサーバ無しを伝える終了コード。
const listClientsNoServer = 1

// clientFormat は list-clients の出力形式。tty と session_id をタブで区切る。
const clientFormat = "#{client_tty}\t#{session_id}"

// Client は普段のサーバに繋がっているクライアント 1 つ分。
type Client struct {
	TTY       string
	SessionID string
}

// ClientsArgs は普段のサーバ（innerSocket）のクライアント一覧を読む list-clients の引数。
// 先頭の "tmux" は含まない。tmux は呼ばない。
func ClientsArgs(innerSocket string) []string {
	return []string{"-L", innerSocket, "list-clients", "-F", clientFormat}
}

// ParseClients は list-clients の出力を行ごとに最初のタブで tty と session_id に分ける。
// タブの無い行と空行は読み飛ばす。純粋関数。
func ParseClients(out string) []Client {
	var clients []Client
	for _, line := range strings.Split(out, "\n") {
		tty, id, ok := strings.Cut(strings.TrimRight(line, "\r"), "\t")
		if !ok || tty == "" {
			continue
		}
		clients = append(clients, Client{TTY: tty, SessionID: id})
	}
	return clients
}

// Viewing は paneTTY のクライアントが見ているセッションの id を返す。
// 該当するクライアントが無い、または paneTTY が空のときは ok=false。純粋関数。
func Viewing(clients []Client, paneTTY string) (sessionID string, ok bool) {
	if paneTTY == "" {
		return "", false
	}
	for _, c := range clients {
		if c.TTY == paneTTY {
			return c.SessionID, true
		}
	}
	return "", false
}

// Viewer は右ペインの tty を一度取れたら持ち続け、右が見ているセッションの id を読む。
type Viewer struct {
	run dock.Runner
	cfg dock.Config
	// mu は tty の読み書きを守る。即時の取り直しと定期の取り直しが並走して Viewing を呼ぶことがある。
	mu  sync.Mutex
	tty string
}

// NewViewer は run と cfg から Viewer を作る。
func NewViewer(run dock.Runner, cfg dock.Config) *Viewer {
	return &Viewer{run: run, cfg: cfg}
}

// Viewing は右が見ているセッションの id を返す。分からないとき（普段のサーバが無い・右ペインの tty が
// まだ取れない・該当クライアントが無い）は ok=false でエラーにしない。
// tty が取れるまでは呼ぶたびに外側へ取りに行き、取れた後は普段のサーバへの list-clients だけを打つ。
func (v *Viewer) Viewing(ctx context.Context) (string, bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.tty == "" {
		args := PaneTTYArgs(v.cfg.OuterSocket, v.cfg.SessionName)
		res, err := v.run(ctx, args)
		if err != nil {
			return "", false, fmt.Errorf("tmux %s を実行できない: %w", strings.Join(args, " "), err)
		}
		if res.ExitCode != 0 {
			return "", false, fmt.Errorf("tmux %s が終了コード %d で失敗: %s", strings.Join(args, " "), res.ExitCode, res.Stderr)
		}
		v.tty = strings.TrimSpace(string(res.Stdout))
		if v.tty == "" {
			return "", false, nil
		}
	}

	args := ClientsArgs(v.cfg.InnerSocket)
	res, err := v.run(ctx, args)
	if err != nil {
		return "", false, fmt.Errorf("tmux %s を実行できない: %w", strings.Join(args, " "), err)
	}
	if res.ExitCode == listClientsNoServer && sessionlist.IsNoServer(res.Stderr) {
		return "", false, nil
	}
	if res.ExitCode != 0 {
		return "", false, fmt.Errorf("tmux %s が終了コード %d で失敗: %s", strings.Join(args, " "), res.ExitCode, res.Stderr)
	}
	id, ok := Viewing(ParseClients(string(res.Stdout)), v.tty)
	return id, ok, nil
}
