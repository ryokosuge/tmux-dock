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
	return loadWith(t, fetch, nil)
}

func loadWith(t *testing.T, fetch fetchFunc, switchTo switchFunc) (Model, tea.Cmd) {
	t.Helper()
	m := New(fetch, switchTo)
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
		if got, want := m.View().Content, "> a\n  b"; got != want {
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

func names(ns ...string) fetchFunc {
	return func(context.Context) ([]sessionlist.Session, error) {
		ss := make([]sessionlist.Session, len(ns))
		for i, n := range ns {
			ss[i] = sessionlist.Session{Name: n}
		}
		return ss, nil
	}
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		var kp tea.KeyPressMsg
		switch k {
		case "up":
			kp = tea.KeyPressMsg{Code: tea.KeyUp}
		case "down":
			kp = tea.KeyPressMsg{Code: tea.KeyDown}
		case "enter":
			kp = tea.KeyPressMsg{Code: tea.KeyEnter}
		default:
			kp = tea.KeyPressMsg{Code: rune(k[0]), Text: k}
		}
		next, _ := m.Update(kp)
		m = next.(Model)
	}
	return m
}

func TestModelCursor(t *testing.T) {
	t.Run("↓ と j でカーソルが 1 つ下、↑ と k で 1 つ上に動く", func(t *testing.T) {
		m, _ := load(t, names("a", "b", "c"))
		tests := []struct {
			keys []string
			want string
		}{
			{[]string{"down"}, "  a\n> b\n  c"},
			{[]string{"j", "j"}, "  a\n  b\n> c"},
			{[]string{"down", "down", "up"}, "  a\n> b\n  c"},
			{[]string{"j", "j", "k"}, "  a\n> b\n  c"},
		}
		for _, tt := range tests {
			if got := press(m, tt.keys...).View().Content; got != tt.want {
				t.Errorf("%v の後の View() = %q, want %q", tt.keys, got, tt.want)
			}
		}
	})

	t.Run("末尾でさらに下、先頭でさらに上に動かしても範囲内に留まる", func(t *testing.T) {
		m, _ := load(t, names("a", "b"))
		if got, want := press(m, "down", "down", "down").View().Content, "  a\n> b"; got != want {
			t.Errorf("末尾で ↓ した後の View() = %q, want %q", got, want)
		}
		if got, want := press(m, "up", "up").View().Content, "> a\n  b"; got != want {
			t.Errorf("先頭で ↑ した後の View() = %q, want %q", got, want)
		}
	})

	t.Run("0 件のとき上下と Enter を押しても panic せず切り替え関数は呼ばれない", func(t *testing.T) {
		calls := 0
		m, _ := loadWith(t, names(), func(context.Context, sessionlist.Session) error {
			calls++
			return nil
		})
		m = press(m, "down", "up", "j", "k")
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd != nil {
			cmd()
		}
		if calls != 0 {
			t.Errorf("切り替え関数の呼び出し回数 = %d, want 0", calls)
		}
	})
}

func TestModelSwitch(t *testing.T) {
	t.Run("3 件でカーソルを 2 つ下へ動かして Enter すると 3 件目が切り替え関数に渡る", func(t *testing.T) {
		var got []sessionlist.Session
		m, _ := loadWith(t, names("a", "b", "c"), func(_ context.Context, s sessionlist.Session) error {
			got = append(got, s)
			return nil
		})
		m = press(m, "down", "down")
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("Enter の Update = nil, want 切り替えの Cmd")
		}
		cmd()
		if want := []sessionlist.Session{{Name: "c"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("切り替え関数に渡ったセッション = %v, want %v", got, want)
		}
	})

	t.Run("カーソルを動かしただけでは切り替え関数は呼ばれない", func(t *testing.T) {
		calls := 0
		m, _ := loadWith(t, names("a", "b"), func(context.Context, sessionlist.Session) error {
			calls++
			return nil
		})
		press(m, "down", "up", "j", "k")
		if calls != 0 {
			t.Errorf("切り替え関数の呼び出し回数 = %d, want 0", calls)
		}
	})

	t.Run("切り替えが失敗しても終了せず、理由付きの 1 行を出したまま操作を続けられる", func(t *testing.T) {
		m, _ := loadWith(t, names("a", "b"), func(context.Context, sessionlist.Session) error {
			return errors.New("boom")
		})
		m = press(m, "down")
		next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		next, cmd = next.(Model).Update(cmd())
		if cmd != nil {
			if _, quit := cmd().(tea.QuitMsg); quit {
				t.Error("Update が終了のコマンドを返した, want 返さない")
			}
		}
		m = next.(Model)
		if got, want := m.View().Content, "  a\n> b\nb に切り替えられません: boom"; got != want {
			t.Errorf("失敗後の View() = %q, want %q", got, want)
		}
		if got, want := press(m, "up").View().Content, "> a\n  b\nb に切り替えられません: boom"; got != want {
			t.Errorf("失敗後に ↑ した View() = %q, want %q", got, want)
		}
	})
}
