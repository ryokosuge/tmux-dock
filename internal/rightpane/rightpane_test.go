package rightpane

import (
	"reflect"
	"testing"
)

func TestDecide(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  Action
	}{
		{name: "セッションが 0 件なら待つ", count: 0, want: Wait},
		{name: "セッションが 1 件なら attach する", count: 1, want: Attach},
		{name: "セッションが複数件なら attach する", count: 3, want: Attach},
		{name: "負の件数は待つ", count: -1, want: Wait},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Decide(tt.count); got != tt.want {
				t.Errorf("Decide(%d) = %v, want %v", tt.count, got, tt.want)
			}
		})
	}
}

func TestAttachArgs(t *testing.T) {
	tests := []struct {
		name   string
		socket string
		want   []string
	}{
		{name: "普段のソケットを -L で指し -t を含まない", socket: "default", want: []string{"-L", "default", "attach-session"}},
		{name: "別名のソケットもそのまま渡す", socket: "work", want: []string{"-L", "work", "attach-session"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AttachArgs(tt.socket); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AttachArgs(%q) = %v, want %v", tt.socket, got, tt.want)
			}
		})
	}
}

func TestAttachEnv(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		want []string
	}{
		{
			name: "TMUX_PANE だけが外れ TMUX などは残る",
			env:  []string{"PATH=/usr/bin", "TMUX=/tmp/tmux-501/tmux-dock,1,0", "TMUX_PANE=%1", "HOME=/home/x"},
			want: []string{"PATH=/usr/bin", "TMUX=/tmp/tmux-501/tmux-dock,1,0", "HOME=/home/x"},
		},
		{
			name: "TMUX_PANE_X のような別名は外れない",
			env:  []string{"TMUX_PANE_X=1", "TMUX_PANE=%2"},
			want: []string{"TMUX_PANE_X=1"},
		},
		{
			name: "TMUX_PANE が無ければそのまま",
			env:  []string{"A=1"},
			want: []string{"A=1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := append([]string(nil), tt.env...)
			if got := AttachEnv(tt.env); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AttachEnv(%v) = %v, want %v", tt.env, got, tt.want)
			}
			if !reflect.DeepEqual(tt.env, orig) {
				t.Errorf("AttachEnv が入力を変更した: got %v, want %v", tt.env, orig)
			}
		})
	}
}

func TestWaitingMessage(t *testing.T) {
	if WaitingMessage == "" {
		t.Error("WaitingMessage が空")
	}
}
