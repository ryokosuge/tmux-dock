package listui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

// load は Init が返す Cmd を実行し、その結果を Update に渡した後の Model と Update の Cmd を返す。
func load(t *testing.T, fetch fetchFunc) (Model, tea.Cmd) {
	t.Helper()
	m := New(fetch)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() = nil, want 取得の Cmd")
	}
	next, cmd := m.Update(cmd())
	return next.(Model), cmd
}

func TestModel(t *testing.T) {
	t.Run("取得したセッション a, b が View に 1 行ずつ出る", func(t *testing.T) {
		m, _ := load(t, func(context.Context) ([]sessionlist.Session, error) {
			return []sessionlist.Session{{Name: "a"}, {Name: "b"}}, nil
		})
		if got, want := m.View().Content, "a\nb"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("Init の Cmd は取得を 1 回だけ呼ぶ", func(t *testing.T) {
		calls := 0
		load(t, func(context.Context) ([]sessionlist.Session, error) {
			calls++
			return nil, nil
		})
		if calls != 1 {
			t.Errorf("fetch の呼び出し回数 = %d, want 1", calls)
		}
	})

	t.Run("取得が失敗したとき View に理由付きのメッセージが出る", func(t *testing.T) {
		m, _ := load(t, func(context.Context) ([]sessionlist.Session, error) {
			return nil, errors.New("boom")
		})
		if got, want := m.View().Content, "一覧を取得できません: boom"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("取得が失敗しても Update は終了のコマンドを返さない", func(t *testing.T) {
		_, cmd := load(t, func(context.Context) ([]sessionlist.Session, error) {
			return nil, errors.New("boom")
		})
		if cmd != nil {
			if _, quit := cmd().(tea.QuitMsg); quit {
				t.Error("Update が終了のコマンドを返した, want 返さない")
			}
		}
	})

	t.Run("0 件のとき sessionlist.Lines と同じ案内の行が出る", func(t *testing.T) {
		m, _ := load(t, func(context.Context) ([]sessionlist.Session, error) { return nil, nil })
		want := strings.Join(sessionlist.Lines(nil), "\n")
		if got := m.View().Content; !reflect.DeepEqual(got, want) {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})
}
