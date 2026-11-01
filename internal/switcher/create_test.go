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

func TestDecide(t *testing.T) {
	existing := []sessionlist.Session{{ID: "$1", Name: "work"}, {ID: "$2", Name: "my work"}}
	tests := []struct {
		name       string
		input      string
		wantAction Action
		wantID     string
	}{
		{"空は何もしない", "", ActionNone, ""},
		{"空白だけは何もしない", "  \t ", ActionNone, ""},
		{"ドットを含む名前は弾く", "a.b", ActionReject, ""},
		{"コロンを含む名前は弾く", "c:d", ActionReject, ""},
		{"既存と完全一致なら切り替える", "work", ActionSwitch, "$1"},
		{"前後の空白を除いて既存と一致すれば切り替える", " my work ", ActionSwitch, "$2"},
		{"前方一致だけの別名は作る", "wor", ActionCreate, ""},
		{"新しい名前は作る", "new", ActionCreate, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.input, existing)
			if got.Action != tt.wantAction || got.Existing.ID != tt.wantID {
				t.Errorf("Decide(%q) = %+v, want action %v id %q", tt.input, got, tt.wantAction, tt.wantID)
			}
			if tt.wantAction == ActionReject && got.Reason == "" {
				t.Errorf("Decide(%q) の Reason が空", tt.input)
			}
		})
	}
}

func TestCreateArgs(t *testing.T) {
	// 末尾が \ の名前の往復は追わない（限界）。
	tests := []struct{ name, wantName string }{
		{"my work", "my work"},
		{"work;", `work\;`},
		{";", `\;`},
		{"a;b", "a;b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := []string{"-L", "inner-x", "new-session", "-d", "-P", "-F", "#{session_id}", "-s", tt.wantName}
			if got := CreateArgs("inner-x", tt.name); !reflect.DeepEqual(got, want) {
				t.Errorf("CreateArgs(%q) = %q, want %q", tt.name, got, want)
			}
		})
	}
}

func TestOpen(t *testing.T) {
	ctx := context.Background()
	listOut := dock.Result{Stdout: []byte("$1 work\n")}
	listArgs := sessionlist.ListArgs("inner-x")

	t.Run("新しい名前は一覧、new-session、tty、switch-client の順で session_id へ切り替える", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{listOut, {Stdout: []byte("$9\n")}, okTTY(), {}}}
		if err := Open(ctx, f.run, cfg, "my work"); err != nil {
			t.Fatalf("Open() = %v", err)
		}
		want := [][]string{
			listArgs,
			CreateArgs("inner-x", "my work"),
			PaneTTYArgs("outer-x", "dock"),
			SwitchArgs("inner-x", "/dev/ttys007", "$9"),
		}
		if !reflect.DeepEqual(f.calls, want) {
			t.Errorf("calls = %q, want %q", f.calls, want)
		}
	})

	t.Run("普段のサーバが無ければ new-session は打つが switch-client は打たない", func(t *testing.T) {
		noServer := dock.Result{ExitCode: 1, Stderr: "no server running on /tmp/x\n"}
		f := &fakeRunner{results: []dock.Result{noServer, {Stdout: []byte("$0\n")}}}
		if err := Open(ctx, f.run, cfg, "a"); err != nil {
			t.Fatalf("Open() = %v", err)
		}
		want := [][]string{listArgs, CreateArgs("inner-x", "a")}
		if !reflect.DeepEqual(f.calls, want) {
			t.Errorf("calls = %q, want %q", f.calls, want)
		}
	})

	t.Run("既存と完全一致なら作らずそのセッションへ切り替える", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{listOut, okTTY(), {}}}
		if err := Open(ctx, f.run, cfg, "work"); err != nil {
			t.Fatalf("Open() = %v", err)
		}
		want := [][]string{listArgs, PaneTTYArgs("outer-x", "dock"), SwitchArgs("inner-x", "/dev/ttys007", "$1")}
		if !reflect.DeepEqual(f.calls, want) {
			t.Errorf("calls = %q, want %q", f.calls, want)
		}
	})

	t.Run("空の名前は new-session も switch-client も呼ばず nil", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{listOut}}
		if err := Open(ctx, f.run, cfg, "  "); err != nil {
			t.Fatalf("Open() = %v", err)
		}
		if !reflect.DeepEqual(f.calls, [][]string{listArgs}) {
			t.Errorf("calls = %q, want list-sessions only", f.calls)
		}
	})

	t.Run("ドットかコロンの名前は呼ばずに使えない旨のエラー", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{listOut}}
		err := Open(ctx, f.run, cfg, "a:b")
		if err == nil || !strings.Contains(err.Error(), ". と : は使えない") {
			t.Fatalf("Open() = %v, want . と : は使えない", err)
		}
		if !reflect.DeepEqual(f.calls, [][]string{listArgs}) {
			t.Errorf("calls = %q, want list-sessions only", f.calls)
		}
	})

	t.Run("一覧の取得失敗はコマンドと stderr を含むエラーで new-session を呼ばない", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{{ExitCode: 1, Stderr: "boom"}}}
		err := Open(ctx, f.run, cfg, "a")
		if err == nil || !strings.Contains(err.Error(), "list-sessions") || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("Open() = %v, want list-sessions と boom", err)
		}
		if len(f.calls) != 1 {
			t.Errorf("calls = %q, want 1 call", f.calls)
		}
	})

	t.Run("new-session の失敗はコマンドと stderr を含むエラーで switch しない", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{listOut, {ExitCode: 1, Stderr: "duplicate session: a"}}}
		err := Open(ctx, f.run, cfg, "a")
		if err == nil || !strings.Contains(err.Error(), "new-session") || !strings.Contains(err.Error(), "duplicate session: a") {
			t.Fatalf("Open() = %v, want new-session と stderr", err)
		}
		if len(f.calls) != 2 {
			t.Errorf("calls = %q, want 2 calls", f.calls)
		}
	})

	t.Run("new-session を起動できないときはエラー", func(t *testing.T) {
		f := &fakeRunner{err: errors.New("no tmux")}
		if err := Open(ctx, f.run, cfg, "a"); err == nil || !strings.Contains(err.Error(), "no tmux") {
			t.Fatalf("Open() = %v, want no tmux", err)
		}
	})

	t.Run("作れたあとの切り替え失敗は切り替えだけが失敗したと分かる", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{listOut, {Stdout: []byte("$9\n")}, {ExitCode: 1, Stderr: "can't find pane"}}}
		err := Open(ctx, f.run, cfg, "a")
		if err == nil || !strings.Contains(err.Error(), "作れたが右ペインの切り替えに失敗") || !strings.Contains(err.Error(), "can't find pane") {
			t.Fatalf("Open() = %v, want 切り替え失敗", err)
		}
	})

	t.Run("既存への切り替え失敗は stderr を含むエラー", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{listOut, okTTY(), {ExitCode: 1, Stderr: "no such session"}}}
		err := Open(ctx, f.run, cfg, "work")
		if err == nil || !strings.Contains(err.Error(), "no such session") {
			t.Fatalf("Open() = %v, want no such session", err)
		}
	})
}
