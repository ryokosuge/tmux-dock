package dock

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// hasSessionMissing は has-session が「セッション無し」を伝える終了コード。
const hasSessionMissing = 1

// Result は tmux を 1 回呼んだ結果。ExitCode は tmux を起動できて終了した場合の終了コード。
// Stdout は tmux の標準出力そのまま（list-sessions のように出力を読む呼び出しが使う）。
type Result struct {
	ExitCode int
	Stdout   []byte
	Stderr   string
}

// Runner は tmux を args（先頭の "tmux" を含まない）で呼ぶ。
// tmux を起動できなかったときだけ error を返し、tmux が非 0 で終わったことは Result で返す。
type Runner func(ctx context.Context, args []string) (Result, error)

// ExecRunner は os/exec で PATH 上の tmux を呼ぶ Runner。
func ExecRunner(ctx context.Context, args []string) (Result, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: strings.TrimSpace(stderr.String())}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	return res, err
}

// healthyWait は崩れと判断する前に健全になるのを待つ上限、healthyPoll はその間の確かめの間隔。
// 並行起動の相手が new-session と split-window の間にいる間の 1 ペインを崩れと見間違えないための待ちで、
// 親 AC の 1 秒以内に収まる値にする。unit テストは 0 に差し替える。
var (
	healthyWait = 200 * time.Millisecond
	healthyPoll = 20 * time.Millisecond
)

// 健全な dock のセッションの形。
const (
	healthyWindows = 1
	healthyPanes   = 2
)

// EnsureSession は外側のセッションが無ければ p.Create で作る。あれば作らず、p.Reload で同梱 conf を外側に流し直す。
// has-session の終了コード 1（サーバ無しを含む）は「セッション無し」でありエラーにしない。
// 同時起動で p.Create が負けた場合も、確かめ直しの has-session が 0 ならセッションがあるものとして扱う。
// セッションがあるときは、window が 1 つ・ペインが 2 つ・どのペインも終わっていないこと（健全）を p.ListPanes で確かめる。
// 相手が作成の途中かもしれないので healthyWait まで待ち直し、それでも崩れていれば p.Kill で消して p.Create を 1 回だけ流し直す。
// 流し直しの後も健全にならないときはエラーを返す（繰り返すと毎回失敗する部分作成で起動が終わらなくなるため）。
func EnsureSession(ctx context.Context, run Runner, p Plan) error {
	res, err := run(ctx, p.HasSession)
	if err != nil {
		return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(p.HasSession, " "), err)
	}
	switch res.ExitCode {
	case 0:
		return reloadIfHealthy(ctx, run, p, nil)
	case hasSessionMissing:
	default:
		return execFailure(p.HasSession, res)
	}

	res, err = run(ctx, p.Create)
	if err != nil {
		return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(p.Create, " "), err)
	}
	if res.ExitCode != 0 {
		created := res
		res, err = run(ctx, p.HasSession)
		if err != nil {
			return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(p.HasSession, " "), err)
		}
		if res.ExitCode != 0 {
			return execFailure(p.Create, created)
		}
		return reloadIfHealthy(ctx, run, p, &created)
	}
	return nil
}

// reloadIfHealthy は dock のセッションが健全なら conf を流し直し、崩れていれば作り直す。
// created は直前の p.Create の失敗で、作り直しても健全にならないときのエラーに手がかりとして含める。
func reloadIfHealthy(ctx context.Context, run Runner, p Plan, created *Result) error {
	ok, _, err := waitHealthy(ctx, run, p)
	if err != nil {
		return err
	}
	if ok {
		return reload(ctx, run, p)
	}
	return rebuild(ctx, run, p, created)
}

// rebuild は崩れた dock のセッションを消して p.Create を流し直し、健全になったかを確かめる。1 回だけ試す。
// p.Kill が非 0 のときはセッションが既に無いだけなので続ける。
func rebuild(ctx context.Context, run Runner, p Plan, created *Result) error {
	if _, err := run(ctx, p.Kill); err != nil {
		return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(p.Kill, " "), err)
	}
	res, err := run(ctx, p.Create)
	if err != nil {
		return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(p.Create, " "), err)
	}
	ok, panes, err := waitHealthy(ctx, run, p)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	if res.ExitCode != 0 {
		return execFailure(p.Create, res)
	}
	if created != nil {
		return fmt.Errorf("dock のセッションを作り直しても window %d・ペイン %d の健全な状態にならない（直前の作成の失敗: %w; list-panes: %q）",
			healthyWindows, healthyPanes, execFailure(p.Create, *created), panes)
	}
	return fmt.Errorf("dock のセッションを作り直しても window %d・ペイン %d の健全な状態にならない（list-panes: %q）",
		healthyWindows, healthyPanes, panes)
}

// waitHealthy は dock のセッションが健全になるまで healthyWait を上限に確かめる。最後に見た list-panes の出力も返す。
func waitHealthy(ctx context.Context, run Runner, p Plan) (bool, string, error) {
	deadline := time.Now().Add(healthyWait)
	for {
		res, err := run(ctx, p.ListPanes)
		if err != nil {
			return false, "", fmt.Errorf("tmux %s を実行できない: %w", strings.Join(p.ListPanes, " "), err)
		}
		// 非 0 はセッションが無いことを表すので、崩れとして扱い作り直しに進ませる。
		if res.ExitCode == 0 && isHealthy(string(res.Stdout)) {
			return true, string(res.Stdout), nil
		}
		if !time.Now().Before(deadline) {
			return false, strings.TrimSpace(string(res.Stdout) + " " + res.Stderr), nil
		}
		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		case <-time.After(healthyPoll):
		}
	}
}

// isHealthy は list-panes の出力（"<window_index> <pane_dead>" の行）が window 1・ペイン 2・全ペイン生存かを返す。
func isHealthy(out string) bool {
	windows := map[string]bool{}
	panes := 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != "0" {
			return false
		}
		windows[fields[0]] = true
		panes++
	}
	return len(windows) == healthyWindows && panes == healthyPanes
}

// reload は既存の外側に同梱 conf を流し直す。set -g / bind は何度流しても結果が同じ。
func reload(ctx context.Context, run Runner, p Plan) error {
	res, err := run(ctx, p.Reload)
	if err != nil {
		return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(p.Reload, " "), err)
	}
	if res.ExitCode != 0 {
		return execFailure(p.Reload, res)
	}
	return nil
}

func execFailure(args []string, res Result) error {
	return fmt.Errorf("tmux %s が終了コード %d で失敗: %s",
		strings.Join(args, " "), res.ExitCode, res.Stderr)
}
