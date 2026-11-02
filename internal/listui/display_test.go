package listui

import (
	"testing"

	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

func TestDisplayStateEqual(t *testing.T) {
	baseSessions := func() []sessionlist.Session {
		return []sessionlist.Session{
			{ID: "$1", Name: "a", Windows: 1, Activity: 100},
			{ID: "$2", Name: "b", Windows: 2, Activity: 200},
		}
	}
	type input struct {
		sessions     []sessionlist.Session
		cursor       int
		viewing      string
		viewingKnown bool
		marked       map[string]bool
		loadNotice   string
	}
	base := func() input {
		return input{sessions: baseSessions(), cursor: 0, viewing: "$1", viewingKnown: true, marked: map[string]bool{"$2": true}}
	}
	build := func(in input) displayState {
		return newDisplayState(in.sessions, in.cursor, in.viewing, in.viewingKnown, in.marked, in.loadNotice)
	}
	tests := []struct {
		name   string
		mutate func(in *input)
		want   bool
	}{
		{"同じ値から作った状態は同じ", func(in *input) {}, true},
		{"生の Activity だけが違い、印の集合が同じなら同じ", func(in *input) { in.sessions[0].Activity = 999; in.sessions[1].Activity = 1 }, true},
		{"見ているセッションだけが違えば違う", func(in *input) { in.viewing = "$2" }, false},
		{"見ているセッションが分からなくなれば違う", func(in *input) { in.viewingKnown = false }, false},
		{"window 数だけが違えば違う", func(in *input) { in.sessions[0].Windows = 3 }, false},
		{"activity の印の集合だけが違えば違う", func(in *input) { in.marked = map[string]bool{"$1": true, "$2": true} }, false},
		{"名前だけが違えば違う", func(in *input) { in.sessions[0].Name = "renamed" }, false},
		{"セッションが増えれば違う", func(in *input) {
			in.sessions = append(in.sessions, sessionlist.Session{ID: "$3", Name: "c", Windows: 1})
		}, false},
		{"セッションが減れば違う", func(in *input) { in.sessions = in.sessions[:1] }, false},
		{"並びだけが違えば違う", func(in *input) { in.sessions[0], in.sessions[1] = in.sessions[1], in.sessions[0] }, false},
		{"カーソル位置だけが違えば違う", func(in *input) { in.cursor = 1 }, false},
		{"失敗の理由の行が付けば違う", func(in *input) { in.loadNotice = "取得に失敗" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := base(), base()
			tt.mutate(&b)
			if got := build(a).equal(build(b)); got != tt.want {
				t.Errorf("equal = %v, want %v", got, tt.want)
			}
		})
	}
}
