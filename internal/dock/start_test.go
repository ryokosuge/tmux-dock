package dock

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// recordingRunner は呼ばれた引数を記録し、has-session / create の結果を差し替える。
type recordingRunner struct {
	has, create Result
	hasErr      error
	calls       [][]string
}

func (r *recordingRunner) run(_ context.Context, args []string) (Result, error) {
	r.calls = append(r.calls, args)
	if reflect.DeepEqual(args, testPlan().HasSession) {
		return r.has, r.hasErr
	}
	return r.create, nil
}

func TestEnsureSession(t *testing.T) {
	p := testPlan()

	t.Run("セッションがあるときは作成を流さない", func(t *testing.T) {
		r := &recordingRunner{}
		if err := EnsureSession(context.Background(), r.run, p); err != nil {
			t.Fatalf("EnsureSession() = %v, want nil", err)
		}
		if want := [][]string{p.HasSession}; !reflect.DeepEqual(r.calls, want) {
			t.Errorf("tmux の呼び出し = %v, want %v", r.calls, want)
		}
	})

	t.Run("has-session が終了コード 1 のときは作成を流しエラーにしない", func(t *testing.T) {
		r := &recordingRunner{has: Result{ExitCode: 1, Stderr: "no server running"}}
		if err := EnsureSession(context.Background(), r.run, p); err != nil {
			t.Fatalf("EnsureSession() = %v, want nil", err)
		}
		if want := [][]string{p.HasSession, p.Create}; !reflect.DeepEqual(r.calls, want) {
			t.Errorf("tmux の呼び出し = %v, want %v", r.calls, want)
		}
	})

	t.Run("作成が失敗したら tmux の stderr をエラーに含める", func(t *testing.T) {
		r := &recordingRunner{
			has:    Result{ExitCode: 1},
			create: Result{ExitCode: 1, Stderr: "duplicate session: dock"},
		}
		err := EnsureSession(context.Background(), r.run, p)
		if err == nil || !strings.Contains(err.Error(), "duplicate session: dock") {
			t.Errorf("EnsureSession() = %v, want stderr %q を含むエラー", err, "duplicate session: dock")
		}
	})

	t.Run("has-session が終了コード 1 以外で失敗したら作成を流さず stderr をエラーに含める", func(t *testing.T) {
		r := &recordingRunner{has: Result{ExitCode: 2, Stderr: "protocol version mismatch"}}
		err := EnsureSession(context.Background(), r.run, p)
		if err == nil || !strings.Contains(err.Error(), "protocol version mismatch") {
			t.Errorf("EnsureSession() = %v, want stderr %q を含むエラー", err, "protocol version mismatch")
		}
		if want := [][]string{p.HasSession}; !reflect.DeepEqual(r.calls, want) {
			t.Errorf("tmux の呼び出し = %v, want %v", r.calls, want)
		}
	})

	t.Run("tmux を起動できなかったら元のエラーを包んで返す", func(t *testing.T) {
		cause := errors.New("executable file not found in $PATH")
		r := &recordingRunner{hasErr: cause}
		if err := EnsureSession(context.Background(), r.run, p); !errors.Is(err, cause) {
			t.Errorf("EnsureSession() = %v, want %v を包むエラー", err, cause)
		}
	})
}
