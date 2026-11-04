package dock

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// recordingRunner は呼ばれた引数を記録し、has-session / create の結果を差し替える。
// recheck を入れると、2 回目以降の has-session はそちらを返す（create 後の確かめ直し用）。
type recordingRunner struct {
	has, create Result
	recheck     *Result
	hasErr      error
	calls       [][]string
	hasCalls    int
}

func (r *recordingRunner) run(_ context.Context, args []string) (Result, error) {
	r.calls = append(r.calls, args)
	if reflect.DeepEqual(args, testPlan().HasSession) {
		r.hasCalls++
		if r.recheck != nil && r.hasCalls > 1 {
			return *r.recheck, r.hasErr
		}
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

	t.Run("作成が失敗しても確かめ直しの has-session が 0 ならエラーにしない", func(t *testing.T) {
		r := &recordingRunner{
			has:     Result{ExitCode: 1},
			create:  Result{ExitCode: 1, Stderr: "duplicate session: dock"},
			recheck: &Result{ExitCode: 0},
		}
		if err := EnsureSession(context.Background(), r.run, p); err != nil {
			t.Fatalf("EnsureSession() = %v, want nil", err)
		}
		if want := [][]string{p.HasSession, p.Create, p.HasSession}; !reflect.DeepEqual(r.calls, want) {
			t.Errorf("tmux の呼び出し = %v, want %v", r.calls, want)
		}
	})

	t.Run("作成が失敗し確かめ直しも 0 以外なら作成の失敗を返す", func(t *testing.T) {
		r := &recordingRunner{
			has:     Result{ExitCode: 1},
			create:  Result{ExitCode: 1, Stderr: "create failed: no space"},
			recheck: &Result{ExitCode: 1, Stderr: "no server running"},
		}
		err := EnsureSession(context.Background(), r.run, p)
		if err == nil || !strings.Contains(err.Error(), "create failed: no space") {
			t.Errorf("EnsureSession() = %v, want 作成の stderr %q を含むエラー", err, "create failed: no space")
		}
		if want := [][]string{p.HasSession, p.Create, p.HasSession}; !reflect.DeepEqual(r.calls, want) {
			t.Errorf("tmux の呼び出し = %v, want %v", r.calls, want)
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
