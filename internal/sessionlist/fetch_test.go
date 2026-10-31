package sessionlist

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ryokosuge/tmux-dock/internal/dock"
)

// fakeRunner は呼ばれた引数を記録し、決めた結果を返す。
type fakeRunner struct {
	res   dock.Result
	err   error
	calls [][]string
}

func (f *fakeRunner) run(_ context.Context, args []string) (dock.Result, error) {
	f.calls = append(f.calls, args)
	return f.res, f.err
}

func TestListArgs(t *testing.T) {
	want := []string{"-L", "inner-x", "list-sessions", "-F", "#{session_name}"}
	if got := ListArgs("inner-x"); !reflect.DeepEqual(got, want) {
		t.Errorf("ListArgs() = %q, want %q", got, want)
	}
}

func TestFetch(t *testing.T) {
	ctx := context.Background()

	t.Run("Runner に ListArgs の引数をそのまま流す", func(t *testing.T) {
		f := &fakeRunner{}
		if _, err := Fetch(ctx, f.run, "inner-x"); err != nil {
			t.Fatalf("Fetch() = %v", err)
		}
		want := [][]string{ListArgs("inner-x")}
		if !reflect.DeepEqual(f.calls, want) {
			t.Errorf("tmux の引数 = %q, want %q", f.calls, want)
		}
	})

	t.Run("終了コード 0 の出力をセッション名の並びにする", func(t *testing.T) {
		f := &fakeRunner{res: dock.Result{Stdout: []byte("a\nb\n")}}
		got, err := Fetch(ctx, f.run, "s")
		if err != nil {
			t.Fatalf("Fetch() = %v", err)
		}
		if want := []Session{{Name: "a"}, {Name: "b"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("Fetch() = %v, want %v", got, want)
		}
	})

	for _, stderr := range []string{
		"no server running on /tmp/tmux-501/s",
		"error connecting to /tmp/tmux-501/s (No such file or directory)",
	} {
		t.Run("終了コード 1 でサーバ無しの文言なら 0 件でエラーにしない: "+strings.Fields(stderr)[0]+" "+strings.Fields(stderr)[1], func(t *testing.T) {
			f := &fakeRunner{res: dock.Result{ExitCode: 1, Stderr: stderr}}
			got, err := Fetch(ctx, f.run, "s")
			if err != nil || len(got) != 0 {
				t.Errorf("Fetch() = %v, %v, want 0 件と nil", got, err)
			}
		})
	}

	t.Run("終了コード 1 でもサーバ無しの文言でないときは stderr を含むエラー", func(t *testing.T) {
		f := &fakeRunner{res: dock.Result{ExitCode: 1, Stderr: "protocol version mismatch"}}
		_, err := Fetch(ctx, f.run, "s")
		if err == nil || !strings.Contains(err.Error(), "protocol version mismatch") {
			t.Errorf("Fetch() err = %v, want stderr を含むエラー", err)
		}
	})

	for _, reason := range []string{"Permission denied", "Socket operation on non-socket", "File name too long"} {
		stderr := "error connecting to /tmp/tmux-501/s (" + reason + ")\n"
		t.Run("サーバ無し以外の接続失敗はエラー: "+reason, func(t *testing.T) {
			f := &fakeRunner{res: dock.Result{ExitCode: 1, Stderr: stderr}}
			_, err := Fetch(ctx, f.run, "s")
			if err == nil || !strings.Contains(err.Error(), reason) {
				t.Errorf("Fetch() err = %v, want %q を含むエラー", err, reason)
			}
		})
	}

	t.Run("stderr 末尾の改行があってもサーバ無しと判定する", func(t *testing.T) {
		f := &fakeRunner{res: dock.Result{ExitCode: 1, Stderr: "error connecting to /tmp/x (No such file or directory)\n"}}
		if got, err := Fetch(ctx, f.run, "s"); err != nil || len(got) != 0 {
			t.Errorf("Fetch() = %v, %v, want 0 件と nil", got, err)
		}
	})

	t.Run("サーバ無しの文言でも終了コードが 1 以外ならエラー", func(t *testing.T) {
		f := &fakeRunner{res: dock.Result{ExitCode: 2, Stderr: "no server running on /tmp/x"}}
		if _, err := Fetch(ctx, f.run, "s"); err == nil {
			t.Error("Fetch() err = nil, want エラー")
		}
	})

	t.Run("tmux を起動できないときはエラー", func(t *testing.T) {
		f := &fakeRunner{err: errors.New("exec: not found")}
		_, err := Fetch(ctx, f.run, "s")
		if err == nil || !strings.Contains(err.Error(), "exec: not found") {
			t.Errorf("Fetch() err = %v, want 起動失敗の原因を含むエラー", err)
		}
	})
}
