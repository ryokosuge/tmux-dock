package listui

import (
	"reflect"
	"testing"

	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

func TestFollow(t *testing.T) {
	prev := []sessionlist.Session{{ID: "$1", Name: "a"}, {ID: "$2", Name: "b"}, {ID: "$3", Name: "c"}}

	tests := []struct {
		name       string
		cursor     int
		fetched    []sessionlist.Session
		wantList   []sessionlist.Session
		wantCursor int
	}{
		{
			name:       "増えたセッションが加わり、消えたセッションが除かれ、並びは取得結果どおり",
			cursor:     0,
			fetched:    []sessionlist.Session{{ID: "$4", Name: "d"}, {ID: "$1", Name: "a"}, {ID: "$3", Name: "c"}},
			wantList:   []sessionlist.Session{{ID: "$4", Name: "d"}, {ID: "$1", Name: "a"}, {ID: "$3", Name: "c"}},
			wantCursor: 1,
		},
		{
			name:       "改名されたセッションは新しい名前で出て、カーソルはその行に残る",
			cursor:     1,
			fetched:    []sessionlist.Session{{ID: "$1", Name: "a"}, {ID: "$2", Name: "renamed"}, {ID: "$3", Name: "c"}},
			wantList:   []sessionlist.Session{{ID: "$1", Name: "a"}, {ID: "$2", Name: "renamed"}, {ID: "$3", Name: "c"}},
			wantCursor: 1,
		},
		{
			name:       "改名されて並びの位置が動いたセッションにカーソルがあると、カーソルは改名後のその行へ移る",
			cursor:     0,
			fetched:    []sessionlist.Session{{ID: "$2", Name: "b"}, {ID: "$3", Name: "c"}, {ID: "$1", Name: "renamed"}},
			wantList:   []sessionlist.Session{{ID: "$2", Name: "b"}, {ID: "$3", Name: "c"}, {ID: "$1", Name: "renamed"}},
			wantCursor: 2,
		},
		{
			name:       "カーソルのセッションが消えて末尾を越えるときは最後の行に収まる",
			cursor:     2,
			fetched:    []sessionlist.Session{{ID: "$1", Name: "a"}, {ID: "$2", Name: "b"}},
			wantList:   []sessionlist.Session{{ID: "$1", Name: "a"}, {ID: "$2", Name: "b"}},
			wantCursor: 1,
		},
		{
			name:       "カーソルのセッションが消えて位置が範囲内なら同じ位置のまま",
			cursor:     1,
			fetched:    []sessionlist.Session{{ID: "$1", Name: "a"}, {ID: "$3", Name: "c"}},
			wantList:   []sessionlist.Session{{ID: "$1", Name: "a"}, {ID: "$3", Name: "c"}},
			wantCursor: 1,
		},
		{
			name:       "取得結果が 0 件でも panic せずカーソルは 0",
			cursor:     2,
			fetched:    nil,
			wantList:   nil,
			wantCursor: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotList, gotCursor := follow(prev, tt.cursor, tt.fetched)
			if !reflect.DeepEqual(gotList, tt.wantList) {
				t.Errorf("follow() list = %v, want %v", gotList, tt.wantList)
			}
			if gotCursor != tt.wantCursor {
				t.Errorf("follow() cursor = %d, want %d", gotCursor, tt.wantCursor)
			}
		})
	}

	t.Run("前の一覧が 0 件のとき、取得結果が出てカーソルは先頭", func(t *testing.T) {
		fetched := []sessionlist.Session{{ID: "$1", Name: "a"}}
		gotList, gotCursor := follow(nil, 0, fetched)
		if !reflect.DeepEqual(gotList, fetched) || gotCursor != 0 {
			t.Errorf("follow(nil, 0, %v) = %v, %d, want %v, 0", fetched, gotList, gotCursor, fetched)
		}
	})
}
