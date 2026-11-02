package switcher

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ryokosuge/tmux-dock/internal/dock"
)

func TestClientsArgs(t *testing.T) {
	want := []string{"-L", "inner-x", "list-clients", "-F", "#{client_tty}\t#{session_id}"}
	if got := ClientsArgs("inner-x"); !reflect.DeepEqual(got, want) {
		t.Errorf("ClientsArgs() = %q, want %q", got, want)
	}
}

func TestParseClients(t *testing.T) {
	got := ParseClients("/dev/ttys001\t$1\n/dev/ttys007\t$3\n\n")
	want := []Client{{TTY: "/dev/ttys001", SessionID: "$1"}, {TTY: "/dev/ttys007", SessionID: "$3"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseClients() = %v, want %v", got, want)
	}
}

func TestViewing(t *testing.T) {
	others := []Client{{TTY: "/dev/ttys001", SessionID: "$1"}, {TTY: "/dev/ttys002", SessionID: "$2"}}
	tests := []struct {
		name    string
		clients []Client
		tty     string
		wantID  string
		wantOK  bool
	}{
		{"一致する 1 件の session_id を返す", []Client{{"/dev/ttys007", "$3"}}, "/dev/ttys007", "$3", true},
		{"別端末のクライアントが混ざっても右ペインの tty のものを選ぶ", append(others, Client{"/dev/ttys007", "$3"}), "/dev/ttys007", "$3", true},
		{"該当するクライアントが無いと分からない", others, "/dev/ttys007", "", false},
		{"クライアントが空だと分からない", nil, "/dev/ttys007", "", false},
		{"右ペインの tty が空だと分からない", others, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := Viewing(tt.clients, tt.tty)
			if id != tt.wantID || ok != tt.wantOK {
				t.Errorf("Viewing() = (%q, %v), want (%q, %v)", id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

func clients(out string) dock.Result { return dock.Result{Stdout: []byte(out)} }

func TestViewerViewing(t *testing.T) {
	ctx := context.Background()
	ttyCall := PaneTTYArgs("outer-x", "dock")
	listCall := ClientsArgs("inner-x")

	t.Run("tty が取れた後は取り直さず list-clients だけを呼び、右の切り替えに追随する", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{
			okTTY(), clients("/dev/ttys001\t$1\n/dev/ttys007\t$3\n"),
			clients("/dev/ttys001\t$1\n/dev/ttys007\t$5\n"),
		}}
		v := NewViewer(f.run, cfg)
		for _, want := range []string{"$3", "$5"} {
			id, ok, err := v.Viewing(ctx)
			if err != nil || !ok || id != want {
				t.Fatalf("Viewing() = (%q, %v, %v), want (%q, true, nil)", id, ok, err, want)
			}
		}
		wantCalls := [][]string{ttyCall, listCall, listCall}
		if !reflect.DeepEqual(f.calls, wantCalls) {
			t.Errorf("calls = %q, want %q", f.calls, wantCalls)
		}
	})

	t.Run("tty が空の間は分からないで list-clients を打たず、取れたら以後は持ち続ける", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{{}, okTTY(), clients("/dev/ttys007\t$3\n")}}
		v := NewViewer(f.run, cfg)
		if id, ok, err := v.Viewing(ctx); err != nil || ok || id != "" {
			t.Fatalf("1 回目 Viewing() = (%q, %v, %v), want (\"\", false, nil)", id, ok, err)
		}
		if id, ok, err := v.Viewing(ctx); err != nil || !ok || id != "$3" {
			t.Fatalf("2 回目 Viewing() = (%q, %v, %v), want (\"$3\", true, nil)", id, ok, err)
		}
		wantCalls := [][]string{ttyCall, ttyCall, listCall}
		if !reflect.DeepEqual(f.calls, wantCalls) {
			t.Errorf("calls = %q, want %q", f.calls, wantCalls)
		}
	})

	noServer := []dock.Result{
		{ExitCode: 1, Stderr: "no server running on /private/tmp/tmux-501/inner-x"},
		{ExitCode: 1, Stderr: "error connecting to /private/tmp/tmux-501/inner-x (No such file or directory)"},
	}
	for _, r := range noServer {
		t.Run("普段のサーバが無いと分からないでエラーにしない: "+r.Stderr, func(t *testing.T) {
			f := &fakeRunner{results: []dock.Result{okTTY(), r}}
			id, ok, err := NewViewer(f.run, cfg).Viewing(ctx)
			if err != nil || ok || id != "" {
				t.Errorf("Viewing() = (%q, %v, %v), want (\"\", false, nil)", id, ok, err)
			}
		})
	}

	t.Run("サーバ無し以外の非 0 終了は失敗したコマンドと stderr を含むエラー", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{okTTY(),
			{ExitCode: 1, Stderr: "error connecting to /x (Permission denied)"}}}
		_, _, err := NewViewer(f.run, cfg).Viewing(ctx)
		if err == nil || !strings.Contains(err.Error(), "list-clients") || !strings.Contains(err.Error(), "Permission denied") {
			t.Errorf("Viewing() err = %v, want list-clients と stderr を含む", err)
		}
	})

	t.Run("外側の tty 取得が非 0 終了ならエラー", func(t *testing.T) {
		f := &fakeRunner{results: []dock.Result{{ExitCode: 1, Stderr: "can't find pane"}}}
		_, _, err := NewViewer(f.run, cfg).Viewing(ctx)
		if err == nil || !strings.Contains(err.Error(), "display-message") || !strings.Contains(err.Error(), "can't find pane") {
			t.Errorf("Viewing() err = %v, want display-message と stderr を含む", err)
		}
	})

	t.Run("tmux を起動できないとコマンドを含むエラー", func(t *testing.T) {
		f := &fakeRunner{err: errors.New("exec: not found")}
		_, _, err := NewViewer(f.run, cfg).Viewing(ctx)
		if err == nil || !strings.Contains(err.Error(), "display-message") || !strings.Contains(err.Error(), "not found") {
			t.Errorf("Viewing() err = %v, want コマンドと原因を含む", err)
		}
	})
}
