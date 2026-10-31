package rightpane

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

type execCall struct{ args, env []string }

// fakeDeps は results を順に返す Fetch と、呼び出しを記録する Exec を持つ Deps を返す。
func fakeDeps(results []fetchResult, execErr error) (Deps, *strings.Builder, *[]execCall, *int) {
	var out strings.Builder
	var calls []execCall
	fetches := 0
	d := Deps{
		Fetch: func(context.Context) ([]sessionlist.Session, error) {
			r := results[fetches]
			fetches++
			return r.sessions, r.err
		},
		Exec: func(args, env []string) error {
			calls = append(calls, execCall{args, env})
			return execErr
		},
		Environ:  func() []string { return []string{"PATH=/usr/bin", "TMUX_PANE=%1"} },
		Out:      &out,
		Interval: time.Millisecond,
	}
	return d, &out, &calls, &fetches
}

type fetchResult struct {
	sessions []sessionlist.Session
	err      error
}

var one = []sessionlist.Session{{Name: "a"}}

func TestRun(t *testing.T) {
	t.Run("0 件、0 件、1 件の順だと待機の表示が 1 回だけ出て 3 回目の後に exec が引数と環境付きで 1 回呼ばれる", func(t *testing.T) {
		d, out, calls, fetches := fakeDeps([]fetchResult{{}, {}, {sessions: one}}, nil)
		if err := Run(context.Background(), "inner", d); err != nil {
			t.Fatalf("Run() = %v", err)
		}
		if got := strings.Count(out.String(), WaitingMessage); got != 1 {
			t.Errorf("待機の表示の回数 = %d, want 1 (out=%q)", got, out.String())
		}
		if *fetches != 3 {
			t.Errorf("取得の回数 = %d, want 3", *fetches)
		}
		want := []execCall{{args: []string{"-L", "inner", "attach-session"}, env: []string{"PATH=/usr/bin"}}}
		if !reflect.DeepEqual(*calls, want) {
			t.Errorf("exec の呼び出し = %v, want %v", *calls, want)
		}
	})

	t.Run("取得がエラーを返すと「一覧を取得できません: <理由>」を出して取り直す", func(t *testing.T) {
		d, out, calls, fetches := fakeDeps([]fetchResult{{err: errors.New("boom")}, {sessions: one}}, nil)
		if err := Run(context.Background(), "inner", d); err != nil {
			t.Fatalf("Run() = %v", err)
		}
		if want := "一覧を取得できません: boom"; !strings.Contains(out.String(), want) {
			t.Errorf("出力 = %q, want %q を含む", out.String(), want)
		}
		if *fetches != 2 || len(*calls) != 1 {
			t.Errorf("取得 %d 回 / exec %d 回, want 2 回 / 1 回", *fetches, len(*calls))
		}
	})

	t.Run("exec が失敗するとエラーが返る", func(t *testing.T) {
		execErr := errors.New("no such file")
		d, _, _, _ := fakeDeps([]fetchResult{{sessions: one}}, execErr)
		if err := Run(context.Background(), "inner", d); !errors.Is(err, execErr) {
			t.Errorf("Run() = %v, want %v を包む", err, execErr)
		}
	})

	t.Run("ctx が終わると待機を止めて ctx のエラーを返す", func(t *testing.T) {
		d, _, calls, _ := fakeDeps([]fetchResult{{}, {}, {}, {}, {}, {}, {}, {}, {}, {}}, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := Run(ctx, "inner", d); !errors.Is(err, context.Canceled) {
			t.Errorf("Run() = %v, want context.Canceled", err)
		}
		if len(*calls) != 0 {
			t.Errorf("exec の呼び出し = %v, want なし", *calls)
		}
	})
}
