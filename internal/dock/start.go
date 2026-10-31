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
type Result struct {
	ExitCode int
	Stderr   string
}

// Runner は tmux を args（先頭の "tmux" を含まない）で呼ぶ。
// tmux を起動できなかったときだけ error を返し、tmux が非 0 で終わったことは Result で返す。
type Runner func(ctx context.Context, args []string) (Result, error)

// ExecRunner は os/exec で PATH 上の tmux を呼ぶ Runner。
func ExecRunner(ctx context.Context, args []string) (Result, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := Result{Stderr: strings.TrimSpace(stderr.String())}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	return res, err
}

// EnsureSession は外側のセッションが無ければ p.Create で作る。あれば何も作らない。
// has-session の終了コード 1（サーバ無しを含む）は「セッション無し」でありエラーにしない。
func EnsureSession(ctx context.Context, run Runner, p Plan) error {
	res, err := run(ctx, p.HasSession)
	if err != nil {
		return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(p.HasSession, " "), err)
	}
	switch res.ExitCode {
	case 0:
		return nil
	case hasSessionMissing:
	default:
		return execFailure(p.HasSession, res)
	}

	res, err = run(ctx, p.Create)
	if err != nil {
		return fmt.Errorf("tmux %s を実行できない: %w", strings.Join(p.Create, " "), err)
	}
	if res.ExitCode != 0 {
		return execFailure(p.Create, res)
	}
	return nil
}

func execFailure(args []string, res Result) error {
	return fmt.Errorf("tmux %s が終了コード %d で失敗: %s",
		strings.Join(args, " "), res.ExitCode, res.Stderr)
}
