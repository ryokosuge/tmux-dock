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

type attachCall struct{ args, env []string }

// fakeDeps は results を順に返す Fetch と、呼び出しを記録する Attach を持つ Deps を返す。
// onAttach は attach の呼び出しごとに（1 回目が 0 番）呼ばれ、返した error が Attach の戻り値になる。
func fakeDeps(results []fetchResult, onAttach func(n int) error) (Deps, *strings.Builder, *[]attachCall, *int) {
	var out strings.Builder
	var calls []attachCall
	fetches := 0
	d := Deps{
		Fetch: func(context.Context) ([]sessionlist.Session, error) {
			r := results[fetches]
			fetches++
			return r.sessions, r.err
		},
		Attach: func(_ context.Context, args, env []string) error {
			calls = append(calls, attachCall{args, env})
			if onAttach == nil {
				return nil
			}
			return onAttach(len(calls) - 1)
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
	t.Run("0 件、0 件、1 件の順だと待機の表示が 1 回だけ出て 3 回目の後に attach が引数と環境付きで呼ばれ、終わった後は ctx の終了で Run が終わる", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		d, out, calls, fetches := fakeDeps([]fetchResult{{}, {}, {sessions: one}}, func(int) error { cancel(); return nil })
		if err := Run(ctx, "inner", d); !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() = %v, want context.Canceled", err)
		}
		if got := strings.Count(out.String(), WaitingMessage); got != 1 {
			t.Errorf("待機の表示の回数 = %d, want 1 (out=%q)", got, out.String())
		}
		if *fetches != 3 {
			t.Errorf("取得の回数 = %d, want 3", *fetches)
		}
		want := []attachCall{{args: []string{"-L", "inner", "attach-session"}, env: []string{"PATH=/usr/bin"}}}
		if !reflect.DeepEqual(*calls, want) {
			t.Errorf("attach の呼び出し = %v, want %v", *calls, want)
		}
		if strings.Contains(out.String(), ClearScreen) {
			t.Errorf("最初の待機で画面を消している (out=%q)", out.String())
		}
	})

	t.Run("取得がエラーを返すと「一覧を取得できません: <理由>」を出して取り直す", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		d, out, calls, fetches := fakeDeps([]fetchResult{{err: errors.New("boom")}, {sessions: one}}, func(int) error { cancel(); return nil })
		_ = Run(ctx, "inner", d)
		if want := "一覧を取得できません: boom"; !strings.Contains(out.String(), want) {
			t.Errorf("出力 = %q, want %q を含む", out.String(), want)
		}
		if *fetches != 2 || len(*calls) != 1 {
			t.Errorf("取得 %d 回 / attach %d 回, want 2 回 / 1 回", *fetches, len(*calls))
		}
	})

	t.Run("attach が終わった後に 1 件以上なら attach をもう一度呼ぶ", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		d, _, calls, _ := fakeDeps([]fetchResult{{sessions: one}, {sessions: one}}, func(n int) error {
			if n == 1 {
				cancel()
			}
			return nil
		})
		_ = Run(ctx, "inner", d)
		if len(*calls) != 2 {
			t.Errorf("attach の回数 = %d, want 2", len(*calls))
		}
	})

	t.Run("attach が終了コード非 0 相当でも（error にならず）取り直して 0 件なら attach を呼ばず画面を消してから待機の表示を 1 回出す", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		fetches := 0
		d, out, calls, _ := fakeDeps([]fetchResult{{sessions: one}, {}, {}, {}}, nil)
		inner := d.Fetch
		d.Fetch = func(c context.Context) ([]sessionlist.Session, error) {
			fetches++
			if fetches == 4 {
				cancel()
			}
			return inner(c)
		}
		_ = Run(ctx, "inner", d)
		if len(*calls) != 1 {
			t.Errorf("attach の回数 = %d, want 1", len(*calls))
		}
		if want := ClearScreen + WaitingMessage + "\n"; out.String() != want {
			t.Errorf("出力 = %q, want %q", out.String(), want)
		}
	})

	t.Run("待機、attach、待機と続くと 2 回目の待機の頭でも画面を消して文言が 1 回出る", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		fetches := 0
		d, out, _, _ := fakeDeps([]fetchResult{{}, {sessions: one}, {}, {}}, nil)
		inner := d.Fetch
		d.Fetch = func(c context.Context) ([]sessionlist.Session, error) {
			fetches++
			if fetches == 4 {
				cancel()
			}
			return inner(c)
		}
		_ = Run(ctx, "inner", d)
		if want := WaitingMessage + "\n" + ClearScreen + WaitingMessage + "\n"; out.String() != want {
			t.Errorf("出力 = %q, want %q", out.String(), want)
		}
	})

	t.Run("attach の後の取得に失敗しても終了せず理由を出して取り直す", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		d, out, calls, _ := fakeDeps([]fetchResult{{sessions: one}, {err: errors.New("boom")}, {sessions: one}}, func(n int) error {
			if n == 1 {
				cancel()
			}
			return nil
		})
		_ = Run(ctx, "inner", d)
		if want := "一覧を取得できません: boom"; !strings.Contains(out.String(), want) {
			t.Errorf("出力 = %q, want %q を含む", out.String(), want)
		}
		if len(*calls) != 2 {
			t.Errorf("attach の回数 = %d, want 2", len(*calls))
		}
	})

	t.Run("attach を起動できないと理由付きのエラーが返る", func(t *testing.T) {
		startErr := errors.New("no such file")
		d, _, _, _ := fakeDeps([]fetchResult{{sessions: one}}, func(int) error { return startErr })
		if err := Run(context.Background(), "inner", d); !errors.Is(err, startErr) {
			t.Errorf("Run() = %v, want %v を包む", err, startErr)
		}
	})

	t.Run("attach の最中に ctx が終わると ctx のエラーを返す", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		d, _, _, _ := fakeDeps([]fetchResult{{sessions: one}}, func(int) error { cancel(); return errors.New("signal: terminated") })
		if err := Run(ctx, "inner", d); !errors.Is(err, context.Canceled) {
			t.Errorf("Run() = %v, want context.Canceled", err)
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
			t.Errorf("attach の呼び出し = %v, want なし", *calls)
		}
	})
}
