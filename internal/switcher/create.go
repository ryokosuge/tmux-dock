package switcher

import (
	"context"
	"fmt"
	"strings"

	"github.com/ryokosuge/tmux-dock/internal/dock"
	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

// Action は確定した名前に対して取る動作。
type Action int

const (
	// ActionNone は何もしない（空の名前）。
	ActionNone Action = iota
	// ActionCreate は新しいセッションを作る。
	ActionCreate
	// ActionSwitch は既存のセッションへ切り替える。
	ActionSwitch
	// ActionReject は作らずに理由を返す。
	ActionReject
)

// invalidNameChars は tmux がセッション名として扱えない（-t で指せない）文字。
const invalidNameChars = ".:"

// Decision は Decide の結果。Existing は ActionSwitch のとき、Reason は ActionReject のときに入る。
type Decision struct {
	Action   Action
	Existing sessionlist.Session
	Reason   string
}

// Decide は確定した名前と既存のセッションから動作を決める。純粋関数で tmux は呼ばない。
func Decide(name string, existing []sessionlist.Session) Decision {
	name = strings.TrimSpace(name)
	if name == "" {
		return Decision{Action: ActionNone}
	}
	if strings.ContainsAny(name, invalidNameChars) {
		return Decision{Action: ActionReject, Reason: fmt.Sprintf("セッション名 %q は作れない: . と : は使えない", name)}
	}
	for _, s := range existing {
		if s.Name == name {
			return Decision{Action: ActionSwitch, Existing: s}
		}
	}
	return Decision{Action: ActionCreate}
}

// CreateArgs は普段のサーバ（innerSocket）に name のセッションを detached で作る引数。
// 名前は 1 要素のまま渡す。-c は付けず tmux の既定の作業ディレクトリに任せる。tmux は呼ばない。
// tmux は argv 要素の末尾の ; をコマンド区切りと読むので、末尾が ; のときだけ最後の ; の前に \ を足す。
// 末尾が \ の名前の往復は追わない。
func CreateArgs(innerSocket, name string) []string {
	if strings.HasSuffix(name, ";") {
		name = strings.TrimSuffix(name, ";") + `\;`
	}
	return []string{"-L", innerSocket, "new-session", "-d", "-P", "-F", "#{session_id}", "-s", name}
}

// Open は確定した name を実行する。普段のサーバの一覧を取り直して Decide し、
// 作るなら new-session の session_id へ、既存ならそのセッションへ Switch する。
// 作成前の一覧が 0 件（右が待機中で内側クライアントが居ない）なら switch-client は必ず失敗するので
// new-session だけ打ち、右の待機ループ（rightpane.Run）の attach に任せる。
// 何もしないときは nil、弾いたときは理由のエラーを返す。
func Open(ctx context.Context, run dock.Runner, cfg dock.Config, name string) error {
	existing, err := sessionlist.Fetch(ctx, run, cfg.InnerSocket)
	if err != nil {
		return fmt.Errorf("セッション一覧を取得できない: %w", err)
	}
	d := Decide(name, existing)
	switch d.Action {
	case ActionNone:
		return nil
	case ActionReject:
		return fmt.Errorf("%s", d.Reason)
	case ActionSwitch:
		return Switch(ctx, run, cfg, d.Existing)
	}

	name = strings.TrimSpace(name)
	args := CreateArgs(cfg.InnerSocket, name)
	res, err := run(ctx, args)
	if err != nil {
		return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(args, " "), err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("tmux %s が終了コード %d で失敗: %s", strings.Join(args, " "), res.ExitCode, res.Stderr)
	}
	if len(existing) == 0 {
		return nil
	}
	id := strings.TrimSpace(string(res.Stdout))
	if id == "" {
		return fmt.Errorf("tmux %s の出力が空で作ったセッションの session_id を取れない", strings.Join(args, " "))
	}
	if err := Switch(ctx, run, cfg, sessionlist.Session{ID: id, Name: name}); err != nil {
		return fmt.Errorf("セッション %q（%s）は作れたが右ペインの切り替えに失敗: %w", name, id, err)
	}
	return nil
}
