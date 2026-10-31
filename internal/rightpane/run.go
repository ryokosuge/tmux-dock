package rightpane

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

// Deps は Run が外に出る操作。本物は sessionlist.Fetch と syscall.Exec を渡す。
type Deps struct {
	Fetch    func(ctx context.Context) ([]sessionlist.Session, error)
	Exec     func(args, env []string) error // 成功すると戻らない（本物は syscall.Exec）
	Environ  func() []string
	Out      io.Writer
	Interval time.Duration // 本物は 1 秒
}

// Run は待機と attach を行う。0 件の間は WaitingMessage を 1 回だけ出して Interval ごとに Fetch を呼び、
// 1 件以上で Exec(AttachArgs(innerSocket), AttachEnv(Environ())) を呼ぶ。
// 取得の失敗は Out に出して取り直す。ctx が終わると ctx.Err() を返す。
func Run(ctx context.Context, innerSocket string, d Deps) error {
	waiting := false
	for {
		sessions, err := d.Fetch(ctx)
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintf(d.Out, "一覧を取得できません: %v\n", err)
		case Decide(len(sessions)) == Attach:
			if err := d.Exec(AttachArgs(innerSocket), AttachEnv(d.Environ())); err != nil {
				return fmt.Errorf("tmux -L %s attach-session への置き換え（exec）に失敗した: %w", innerSocket, err)
			}
			return nil
		case !waiting:
			fmt.Fprintln(d.Out, WaitingMessage)
			waiting = true
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d.Interval):
		}
	}
}
