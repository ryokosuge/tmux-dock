package switcher

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ryokosuge/tmux-dock/internal/dock"
	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

var (
	cfg  = dock.Config{InnerSocket: "inner-x", OuterSocket: "outer-x", SessionName: "dock"}
	sess = sessionlist.Session{ID: "$3", Name: "my work"}
)

// fakeRunner は呼ばれた引数を記録し、呼び出し順に決めた結果を返す。
type fakeRunner struct {
	results []dock.Result
	err     error
	calls   [][]string
}

func (f *fakeRunner) run(_ context.Context, args []string) (dock.Result, error) {
	f.calls = append(f.calls, args)
	if f.err != nil {
		return dock.Result{}, f.err
	}
	return f.results[len(f.calls)-1], nil
}

func okTTY() dock.Result { return dock.Result{Stdout: []byte("/dev/ttys007\n")} }

func TestPaneTTYArgs(t *testing.T) {
	want := []string{"-L", "outer-x", "display-message", "-p", "-t", "=dock:.1", "#{pane_tty}"}
	if got := PaneTTYArgs("outer-x", "dock"); !reflect.DeepEqual(got, want) {
		t.Errorf("PaneTTYArgs() = %q, want %q", got, want)
	}
}

func TestSwitchArgs(t *testing.T) {
	want := []string{"-L", "inner-x", "switch-client", "-c", "/dev/ttys007", "-t", "$3"}
	if got := SwitchArgs("inner-x", "/dev/ttys007", "$3"); !reflect.DeepEqual(got, want) {
		t.Errorf("SwitchArgs() = %q, want %q", got, want)
	}
}

func TestSwitch(t *testing.T) {
	ctx := context.Background()

	t.Run("tty の取得、switch-client の順に 2 回だけ tmux を呼ぶ", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{okTTY(), {}}}
		if err := Switch(ctx, f.run, cfg, sess); err != nil {
			t.Fatalf("Switch() = %v", err)
		}
		want := [][]string{
			PaneTTYArgs("outer-x", "dock"),
			SwitchArgs("inner-x", "/dev/ttys007", "$3"),
		}
		if !reflect.DeepEqual(f.calls, want) {
			t.Errorf("tmux の呼び出し = %q, want %q", f.calls, want)
		}
	})

	failures := []struct {
		name    string
		results []dock.Result
		calls   int
		want    []string
	}{
		{"tty の取得が非 0 終了ならコマンドと stderr を含むエラー", []dock.Result{{ExitCode: 1, Stderr: "can't find pane: 1"}}, 1,
			[]string{"display-message", "can't find pane: 1"}},
		{"tty の出力が空ならコマンドを含むエラーで switch-client を打たない", []dock.Result{{Stdout: []byte("\n")}}, 1,
			[]string{"display-message", "tty"}},
		{"switch-client が非 0 終了ならコマンドと stderr を含むエラー", []dock.Result{okTTY(), {ExitCode: 1, Stderr: "can't find client: /dev/ttys007"}}, 2,
			[]string{"switch-client", "can't find client: /dev/ttys007"}},
	}
	for _, tt := range failures {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeRunner{results: tt.results}
			err := Switch(ctx, f.run, cfg, sess)
			if err == nil {
				t.Fatal("Switch() err = nil, want エラー")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("Switch() err = %q, want %q を含む", err, w)
				}
			}
			if len(f.calls) != tt.calls {
				t.Errorf("tmux の呼び出し回数 = %d, want %d", len(f.calls), tt.calls)
			}
		})
	}

	t.Run("tmux を起動できないときは原因を含むエラー", func(t *testing.T) {
		f := &fakeRunner{err: errors.New("exec: not found")}
		err := Switch(ctx, f.run, cfg, sess)
		if err == nil || !strings.Contains(err.Error(), "exec: not found") {
			t.Errorf("Switch() err = %v, want 起動失敗の原因を含むエラー", err)
		}
	})
}
