package rightpane

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

// Deps は Run が外に出る操作。本物は sessionlist.Fetch と、attach-session を子プロセスで動かす関数を渡す。
type Deps struct {
	Fetch func(ctx context.Context) ([]sessionlist.Session, error)
	// Attach は tmux を args・env で起動し、終わるまで待つ。起動できないときだけ error を返し、
	// 終了コードは error にしない（どの終了も待機ループへ戻る）。ctx が終わったら子を止めて戻る。
	Attach   func(ctx context.Context, args, env []string) error
	Environ  func() []string
	Out      io.Writer
	Interval time.Duration // 本物は 1 秒
}

// Run は待機と attach を繰り返す。0 件の間は WaitingMessage を待機の区間ごとに 1 回だけ出して
// Interval ごとに Fetch を呼び、1 件以上で Attach(AttachArgs(innerSocket), AttachEnv(Environ())) を呼ぶ。
// attach が終わると（終了コードによらず）取り直して待機ループへ戻り、attach が一度でも終わった後の
// 待機は ClearScreen で画面を消してから文言を出す。attach が Interval より短く終わったときは Interval を挟む。
// 取得の失敗は Out に出して取り直す。attach を起動できなければエラーを返し、ctx が終わると ctx.Err() を返す。
func Run(ctx context.Context, innerSocket string, d Deps) error {
	waiting := false
	attached := false
	for {
		sessions, err := d.Fetch(ctx)
		wait := true
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintf(d.Out, "一覧を取得できません: %v\n", err)
		case Decide(len(sessions)) == Attach:
			started := time.Now()
			if err := d.Attach(ctx, AttachArgs(innerSocket), AttachEnv(d.Environ())); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("tmux -L %s attach-session を子プロセスで起動できなかった: %w", innerSocket, err)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			attached = true
			waiting = false
			wait = time.Since(started) < d.Interval
		case !waiting:
			if attached {
				fmt.Fprint(d.Out, ClearScreen)
			}
			fmt.Fprintln(d.Out, WaitingMessage)
			waiting = true
		}
		if !wait {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d.Interval):
		}
	}
}
