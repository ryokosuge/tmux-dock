package listui

import (
	"reflect"
	"testing"

	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

func TestActivityMarks(t *testing.T) {
	tests := []struct {
		name         string
		base         map[string]int64
		fetched      []sessionlist.Session
		viewing      string
		viewingKnown bool
		wantNext     map[string]int64
		wantMarked   map[string]bool
	}{
		{
			name:       "基準より activity が大きいセッションにだけ印が付き、同じセッションには付かない",
			base:       map[string]int64{"$1": 100, "$2": 100},
			fetched:    []sessionlist.Session{{ID: "$1", Activity: 150}, {ID: "$2", Activity: 100}},
			wantNext:   map[string]int64{"$1": 100, "$2": 100},
			wantMarked: map[string]bool{"$1": true},
		},
		{
			name:         "右が見ているセッションは基準が今の値へ進み、印が付かない",
			base:         map[string]int64{"$1": 100, "$2": 100},
			fetched:      []sessionlist.Session{{ID: "$1", Activity: 150}, {ID: "$2", Activity: 160}},
			viewing:      "$1",
			viewingKnown: true,
			wantNext:     map[string]int64{"$1": 150, "$2": 100},
			wantMarked:   map[string]bool{"$2": true},
		},
		{
			name:       "基準を持たないセッションは今の値を基準にし、印を付けない",
			base:       map[string]int64{"$1": 100},
			fetched:    []sessionlist.Session{{ID: "$1", Activity: 100}, {ID: "$2", Activity: 200}},
			wantNext:   map[string]int64{"$1": 100, "$2": 200},
			wantMarked: map[string]bool{},
		},
		{
			name:       "起動時の最初の取得（基準が nil）はどのセッションにも印を付けない",
			base:       nil,
			fetched:    []sessionlist.Session{{ID: "$1", Activity: 100}, {ID: "$2", Activity: 200}},
			wantNext:   map[string]int64{"$1": 100, "$2": 200},
			wantMarked: map[string]bool{},
		},
		{
			name:       "取得結果から消えたセッションの基準は次の基準から除かれる",
			base:       map[string]int64{"$1": 100, "$2": 100},
			fetched:    []sessionlist.Session{{ID: "$1", Activity: 100}},
			wantNext:   map[string]int64{"$1": 100},
			wantMarked: map[string]bool{},
		},
		{
			name:         "右が見ているセッションが分からないときは、viewing の値が何でもどの基準も進めない",
			base:         map[string]int64{"$1": 100},
			fetched:      []sessionlist.Session{{ID: "$1", Activity: 150}},
			viewing:      "$1",
			viewingKnown: false,
			wantNext:     map[string]int64{"$1": 100},
			wantMarked:   map[string]bool{"$1": true},
		},
		{
			name:       "改名されたセッションも ID で突き合わせ、基準を引き継ぐ",
			base:       map[string]int64{"$1": 100},
			fetched:    []sessionlist.Session{{ID: "$1", Name: "renamed", Activity: 150}},
			wantNext:   map[string]int64{"$1": 100},
			wantMarked: map[string]bool{"$1": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, marked := activityMarks(tt.base, tt.fetched, tt.viewing, tt.viewingKnown)
			if !reflect.DeepEqual(next, tt.wantNext) {
				t.Errorf("next = %v, want %v", next, tt.wantNext)
			}
			if !reflect.DeepEqual(marked, tt.wantMarked) {
				t.Errorf("marked = %v, want %v", marked, tt.wantMarked)
			}
		})
	}
}

func TestActivityMarksAcrossFetches(t *testing.T) {
	at := func(a int64) []sessionlist.Session { return []sessionlist.Session{{ID: "$1", Activity: a}} }

	t.Run("右が見ている間の動きは、離れた後に動きが無ければ印にならない", func(t *testing.T) {
		base, _ := activityMarks(nil, at(100), "", false)
		base, marked := activityMarks(base, at(150), "$1", true)
		if len(marked) != 0 {
			t.Fatalf("見ている間に印が付いた: %v", marked)
		}
		_, marked = activityMarks(base, at(150), "$2", true)
		if len(marked) != 0 {
			t.Errorf("離れた後に動きが無いのに印が付いた: %v", marked)
		}
	})

	t.Run("右が見ているセッションを離れた後に動きがあると印が付く", func(t *testing.T) {
		base, _ := activityMarks(nil, at(100), "", false)
		base, _ = activityMarks(base, at(150), "$1", true)
		_, marked := activityMarks(base, at(200), "$2", true)
		if want := map[string]bool{"$1": true}; !reflect.DeepEqual(marked, want) {
			t.Errorf("marked = %v, want %v", marked, want)
		}
	})
}
