package dock

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// recordingRunner は呼ばれた引数を記録し、has-session / create / reload の結果を差し替える。
// recheck を入れると、2 回目以降の has-session はそちらを返す（create 後の確かめ直し用）。
type recordingRunner struct {
	has, create, reload Result
	reloadErr           error
	recheck             *Result
	hasErr              error
	calls               [][]string
	hasCalls            int
	// panes は list-panes の結果を呼ばれた順に返す。尽きたら最後を返す。無ければ健全な 2 ペイン。
	panes     []Result
	paneCalls int
}

const (
	healthyPanesOut = "0 0\n0 0\n"
	singlePaneOut   = "0 0\n"
)

func (r *recordingRunner) run(_ context.Context, args []string) (Result, error) {
	r.calls = append(r.calls, args)
	if reflect.DeepEqual(args, testPlan().HasSession) {
		r.hasCalls++
		if r.recheck != nil && r.hasCalls > 1 {
			return *r.recheck, r.hasErr
		}
		return r.has, r.hasErr
	}
	if reflect.DeepEqual(args, testPlan().ListPanes) {
		i := r.paneCalls
		r.paneCalls++
		if len(r.panes) == 0 {
			return Result{Stdout: []byte(healthyPanesOut)}, nil
		}
		return r.panes[min(i, len(r.panes)-1)], nil
	}
	if reflect.DeepEqual(args, testPlan().Kill) {
		return Result{}, nil
	}
	if reflect.DeepEqual(args, testPlan().Reload) {
		return r.reload, r.reloadErr
	}
	return r.create, nil
}

func TestEnsureSession(t *testing.T) {
	p := testPlan()

	t.Run("セッションがあるときは作成を流さず conf を流し直す", func(t *testing.T) {
		r := &recordingRunner{}
		if err := EnsureSession(context.Background(), r.run, p); err != nil {
			t.Fatalf("EnsureSession() = %v, want nil", err)
		}
		if want := [][]string{p.HasSession, p.ListPanes, p.Reload}; !reflect.DeepEqual(r.calls, want) {
			t.Errorf("tmux の呼び出し = %v, want %v", r.calls, want)
		}
	})

	t.Run("流し直しが非 0 で終わったら stderr をエラーに含める", func(t *testing.T) {
		r := &recordingRunner{reload: Result{ExitCode: 1, Stderr: "no such file: dock.conf"}}
		err := EnsureSession(context.Background(), r.run, p)
		if err == nil || !strings.Contains(err.Error(), "no such file: dock.conf") {
			t.Errorf("EnsureSession() = %v, want stderr %q を含むエラー", err, "no such file: dock.conf")
		}
	})

	t.Run("流し直しの tmux を起動できなかったら元のエラーを包んで返す", func(t *testing.T) {
		cause := errors.New("fork failed")
		r := &recordingRunner{reloadErr: cause}
		if err := EnsureSession(context.Background(), r.run, p); !errors.Is(err, cause) {
			t.Errorf("EnsureSession() = %v, want %v を包むエラー", err, cause)
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
		if want := [][]string{p.HasSession, p.Create, p.HasSession, p.ListPanes, p.Reload}; !reflect.DeepEqual(r.calls, want) {
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

	t.Run("セッションが健全なら作り直さない", func(t *testing.T) {
		r := &recordingRunner{}
		if err := EnsureSession(context.Background(), r.run, p); err != nil {
			t.Fatalf("EnsureSession() = %v, want nil", err)
		}
		for _, c := range r.calls {
			if reflect.DeepEqual(c, p.Kill) || reflect.DeepEqual(c, p.Create) {
				t.Errorf("健全なのに作り直しを流した: %v", r.calls)
			}
		}
	})

	t.Run("ペインが 1 つなら kill して作り直し、健全になれば成功する", func(t *testing.T) {
		r := &recordingRunner{panes: []Result{{Stdout: []byte(singlePaneOut)}, {Stdout: []byte(healthyPanesOut)}}}
		if err := EnsureSession(context.Background(), r.run, p); err != nil {
			t.Fatalf("EnsureSession() = %v, want nil", err)
		}
		if want := [][]string{p.HasSession, p.ListPanes, p.Kill, p.Create, p.ListPanes, p.Reload}; !reflect.DeepEqual(r.calls, want) {
			t.Errorf("tmux の呼び出し = %v, want %v", r.calls, want)
		}
	})

	t.Run("ペインが終わっているものを含むなら作り直す", func(t *testing.T) {
		r := &recordingRunner{panes: []Result{{Stdout: []byte("0 0\n0 1\n")}, {Stdout: []byte(healthyPanesOut)}}}
		if err := EnsureSession(context.Background(), r.run, p); err != nil {
			t.Fatalf("EnsureSession() = %v, want nil", err)
		}
		if want := [][]string{p.HasSession, p.ListPanes, p.Kill, p.Create, p.ListPanes, p.Reload}; !reflect.DeepEqual(r.calls, want) {
			t.Errorf("tmux の呼び出し = %v, want %v", r.calls, want)
		}
	})

	t.Run("作成が失敗し確かめ直しが 0 でも 1 ペインのままなら作り直しを 1 回だけ試してエラーを返す", func(t *testing.T) {
		r := &recordingRunner{
			has:     Result{ExitCode: 1},
			create:  Result{ExitCode: 1, Stderr: "split failed: bad size"},
			recheck: &Result{ExitCode: 0},
			panes:   []Result{{Stdout: []byte(singlePaneOut)}},
		}
		err := EnsureSession(context.Background(), r.run, p)
		if err == nil || !strings.Contains(err.Error(), "split failed: bad size") {
			t.Errorf("EnsureSession() = %v, want 作成の stderr %q を含むエラー", err, "split failed: bad size")
		}
		if want := [][]string{p.HasSession, p.Create, p.HasSession, p.ListPanes, p.Kill, p.Create, p.ListPanes}; !reflect.DeepEqual(r.calls, want) {
			t.Errorf("tmux の呼び出し = %v, want %v", r.calls, want)
		}
	})

	t.Run("作り直しの Create が負けても相手が健全にしていれば conf を流し直して成功する", func(t *testing.T) {
		r := &recordingRunner{
			create: Result{ExitCode: 1, Stderr: "duplicate session: dock"},
			panes:  []Result{{Stdout: []byte(singlePaneOut)}, {Stdout: []byte(healthyPanesOut)}},
		}
		if err := EnsureSession(context.Background(), r.run, p); err != nil {
			t.Fatalf("EnsureSession() = %v, want nil", err)
		}
		if want := [][]string{p.HasSession, p.ListPanes, p.Kill, p.Create, p.ListPanes, p.Reload}; !reflect.DeepEqual(r.calls, want) {
			t.Errorf("tmux の呼び出し = %v, want %v", r.calls, want)
		}
	})

	t.Run("作り直した後の conf の流し直しが非 0 で終わったら stderr をエラーに含める", func(t *testing.T) {
		r := &recordingRunner{
			reload: Result{ExitCode: 1, Stderr: "no such file: dock.conf"},
			panes:  []Result{{Stdout: []byte(singlePaneOut)}, {Stdout: []byte(healthyPanesOut)}},
		}
		err := EnsureSession(context.Background(), r.run, p)
		if err == nil || !strings.Contains(err.Error(), "no such file: dock.conf") {
			t.Errorf("EnsureSession() = %v, want stderr %q を含むエラー", err, "no such file: dock.conf")
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
