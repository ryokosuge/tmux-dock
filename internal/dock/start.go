package dock

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
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

// EnsureSession は外側のセッションが無ければ p.Create で作る。あれば作らず、p.Reload で同梱 conf を外側に流し直す。
// has-session の終了コード 1（サーバ無しを含む）は「セッション無し」でありエラーにしない。
// 同時起動で p.Create が負けた場合も、確かめ直しの has-session が 0 ならセッションがあるので成功として扱う。
func EnsureSession(ctx context.Context, run Runner, p Plan) error {
	res, err := run(ctx, p.HasSession)
	if err != nil {
		return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(p.HasSession, " "), err)
	}
	switch res.ExitCode {
	case 0:
		return reload(ctx, run, p)
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
		return reload(ctx, run, p)
	}
	return nil
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
