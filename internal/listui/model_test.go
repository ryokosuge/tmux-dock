package listui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

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
	m := New(fetch, switchTo, nil, nil)
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
		if got, want := m.View().Content, ">   0 a\n    0 b"; got != want {
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
			{[]string{"down"}, "    0 a\n>   0 b\n    0 c"},
			{[]string{"j", "j"}, "    0 a\n    0 b\n>   0 c"},
			{[]string{"down", "down", "up"}, "    0 a\n>   0 b\n    0 c"},
			{[]string{"j", "j", "k"}, "    0 a\n>   0 b\n    0 c"},
		}
		for _, tt := range tests {
			if got := press(m, tt.keys...).View().Content; got != tt.want {
				t.Errorf("%v の後の View() = %q, want %q", tt.keys, got, tt.want)
			}
		}
	})

	t.Run("末尾でさらに下、先頭でさらに上に動かしても範囲内に留まる", func(t *testing.T) {
		m, _ := load(t, names("a", "b"))
		if got, want := press(m, "down", "down", "down").View().Content, "    0 a\n>   0 b"; got != want {
			t.Errorf("末尾で ↓ した後の View() = %q, want %q", got, want)
		}
		if got, want := press(m, "up", "up").View().Content, ">   0 a\n    0 b"; got != want {
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
		if got, want := m.View().Content, "    0 a\n>   0 b\nb に切り替えられません: boom"; got != want {
			t.Errorf("失敗後の View() = %q, want %q", got, want)
		}
		if got, want := press(m, "up").View().Content, ">   0 a\n    0 b\nb に切り替えられません: boom"; got != want {
			t.Errorf("失敗後に ↑ した View() = %q, want %q", got, want)
		}
	})
}

func loadWithCreate(t *testing.T, fetch fetchFunc, create createFunc) Model {
	t.Helper()
	m := New(fetch, nil, create, nil)
	next, _ := m.Update(m.Init()())
	return next.(Model)
}

// typeText は text を 1 文字ずつ印字できるキーとして Update に渡す。
func typeText(m Model, text string) Model {
	for _, r := range text {
		next, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = next.(Model)
	}
	return m
}

func pressKey(m Model, code rune) (Model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyPressMsg{Code: code})
	return next.(Model), cmd
}

func recordCreate(names *[]string, err error) createFunc {
	return func(_ context.Context, name string) error {
		*names = append(*names, name)
		return err
	}
}

func TestModelCreateInput(t *testing.T) {
	t.Run("n を押して文字を打つたびに入力中の名前が 1 行に伸び、Backspace で日本語も 1 文字ずつ縮む", func(t *testing.T) {
		var created []string
		m := loadWithCreate(t, names("a", "b"), recordCreate(&created, nil))
		m = typeText(m, "n")
		if got, want := m.View().Content, ">   0 a\n    0 b\n新規セッション名: "; got != want {
			t.Errorf("n の後の View() = %q, want %q", got, want)
		}
		m = typeText(m, "ab日本")
		if got, want := m.View().Content, ">   0 a\n    0 b\n新規セッション名: ab日本"; got != want {
			t.Errorf("打鍵の後の View() = %q, want %q", got, want)
		}
		m, _ = pressKey(m, tea.KeyBackspace)
		if got, want := m.View().Content, ">   0 a\n    0 b\n新規セッション名: ab日"; got != want {
			t.Errorf("Backspace の後の View() = %q, want %q", got, want)
		}
	})

	t.Run("入力中に j / k / n を打つとカーソルは動かず名前に足される", func(t *testing.T) {
		m := loadWithCreate(t, names("a", "b"), recordCreate(new([]string), nil))
		m = typeText(m, "njkn")
		if got, want := m.View().Content, ">   0 a\n    0 b\n新規セッション名: jkn"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("Esc で取り消すと作成関数は呼ばれず、入力前のカーソル位置の一覧に戻る", func(t *testing.T) {
		var created []string
		m := loadWithCreate(t, names("a", "b"), recordCreate(&created, nil))
		m = press(m, "down")
		m = typeText(m, "nxy")
		m, _ = pressKey(m, tea.KeyEscape)
		if got, want := m.View().Content, "    0 a\n>   0 b"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
		if len(created) != 0 {
			t.Errorf("作成関数に渡った名前 = %v, want なし", created)
		}
	})

	t.Run("空と空白だけの名前で Enter しても作成関数は呼ばれず、入力前のカーソル位置の一覧に戻る", func(t *testing.T) {
		for _, text := range []string{"n", "n  "} {
			var created []string
			m := loadWithCreate(t, names("a", "b"), recordCreate(&created, nil))
			m = press(m, "down")
			m = typeText(m, text)
			m, cmd := pressKey(m, tea.KeyEnter)
			if cmd != nil {
				cmd()
			}
			if got, want := m.View().Content, "    0 a\n>   0 b"; got != want {
				t.Errorf("%q の後の View() = %q, want %q", text, got, want)
			}
			if len(created) != 0 {
				t.Errorf("%q の後に作成関数に渡った名前 = %v, want なし", text, created)
			}
		}
	})

	t.Run("名前を入れて Enter すると作成関数に名前が渡り、成功すると入力前のカーソル位置の一覧に戻る", func(t *testing.T) {
		var created []string
		m := loadWithCreate(t, names("a", "b"), recordCreate(&created, nil))
		m = press(m, "down")
		m = typeText(m, "nwork")
		m, cmd := pressKey(m, tea.KeyEnter)
		if cmd == nil {
			t.Fatal("Enter の Update = nil, want 作成の Cmd")
		}
		if len(created) != 0 {
			t.Fatalf("Cmd を実行する前に作成関数が呼ばれた: %v", created)
		}
		next, _ := m.Update(cmd())
		if want := []string{"work"}; !reflect.DeepEqual(created, want) {
			t.Errorf("作成関数に渡った名前 = %v, want %v", created, want)
		}
		if got, want := next.(Model).View().Content, "    0 a\n>   0 b"; got != want {
			t.Errorf("成功後の View() = %q, want %q", got, want)
		}
	})

	t.Run("作成が失敗したら入力を閉じず、理由の 1 行を出して名前の入力を続けられる", func(t *testing.T) {
		var created []string
		m := loadWithCreate(t, names("a", "b"), recordCreate(&created, errors.New("boom")))
		m = typeText(m, "nwork")
		m, cmd := pressKey(m, tea.KeyEnter)
		next, _ := m.Update(cmd())
		m = next.(Model)
		if got, want := m.View().Content, ">   0 a\n    0 b\n新規セッション名: work\n作成できません: boom"; got != want {
			t.Errorf("失敗後の View() = %q, want %q", got, want)
		}
		m = typeText(m, "2")
		if got, want := m.View().Content, ">   0 a\n    0 b\n新規セッション名: work2\n作成できません: boom"; got != want {
			t.Errorf("続けて入力した後の View() = %q, want %q", got, want)
		}
	})

	t.Run("作成関数の結果が返るまでの打鍵は名前に足されない", func(t *testing.T) {
		m := loadWithCreate(t, names("a"), recordCreate(new([]string), nil))
		m = typeText(m, "nw")
		m, _ = pressKey(m, tea.KeyEnter)
		m = typeText(m, "x")
		if got, want := m.View().Content, ">   0 a\n新規セッション名: w"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("一覧の取得が失敗していても、失敗の行の下に入力行が出る", func(t *testing.T) {
		m := loadWithCreate(t, func(context.Context) ([]sessionlist.Session, error) {
			return nil, errors.New("boom")
		}, recordCreate(new([]string), nil))
		m = typeText(m, "nab")
		if got, want := m.View().Content, "一覧を取得できません: boom\n新規セッション名: ab"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("作成関数が nil のとき n を押しても入力は始まらない", func(t *testing.T) {
		m := loadWithCreate(t, names("a", "b"), nil)
		if got, want := typeText(m, "n").View().Content, ">   0 a\n    0 b"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})
}

type result struct {
	names []string
	err   error
}

// sequence は呼ばれるたびに responses を先頭から 1 つずつ返す取得関数。

func sequence(calls *int, responses ...result) fetchFunc {
	return func(ctx context.Context) ([]sessionlist.Session, error) {
		r := responses[*calls]
		*calls++
		if r.err != nil {
			return nil, r.err
		}
		return names(r.names...)(ctx)
	}
}

func TestModelRefresh(t *testing.T) {
	t.Run("取得結果を受けた Update は次の tick を張る Cmd を返し、取得はまだ呼ばない", func(t *testing.T) {
		calls := 0
		m := New(sequence(&calls, result{names: []string{"a"}}), nil, nil, nil)
		m.interval = time.Nanosecond
		_, cmd := m.Update(m.Init()())
		if cmd == nil {
			t.Fatal("取得結果の Update = nil, want 次の tick の Cmd")
		}
		if calls != 1 {
			t.Errorf("tick の前の取得回数 = %d, want 1", calls)
		}
		if _, ok := cmd().(tickMsg); !ok {
			t.Errorf("tick の Cmd の結果 = %T, want tickMsg", cmd())
		}
	})

	t.Run("tick のメッセージで取得が呼ばれ、その結果で一覧が差し替わる", func(t *testing.T) {
		calls := 0
		m := New(sequence(&calls, result{names: []string{"a", "b"}}, result{names: []string{"a", "c", "d"}}), nil, nil, nil)
		next, _ := m.Update(m.Init()())
		next, cmd := next.Update(tickMsg{})
		if cmd == nil {
			t.Fatal("tick の Update = nil, want 取得の Cmd")
		}
		if calls != 1 {
			t.Fatalf("Cmd を実行する前の取得回数 = %d, want 1", calls)
		}
		next, _ = next.Update(cmd())
		if calls != 2 {
			t.Errorf("取得回数 = %d, want 2", calls)
		}
		if got, want := next.(Model).View().Content, ">   0 a\n    0 c\n    0 d"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("取り直しの前後でカーソルは同じセッションを追う", func(t *testing.T) {
		fetch := func(ss ...sessionlist.Session) fetchFunc {
			return func(context.Context) ([]sessionlist.Session, error) { return ss, nil }
		}
		m := New(fetch(sessionlist.Session{ID: "$1", Name: "a"}, sessionlist.Session{ID: "$2", Name: "b"}), nil, nil, nil)
		next, _ := m.Update(m.Init()())
		m = press(next.(Model), "down")
		m.fetch = fetch(sessionlist.Session{ID: "$0", Name: "z"}, sessionlist.Session{ID: "$2", Name: "b2"})
		next, _ = m.Update(m.fetchCmd()())
		if got, want := next.(Model).View().Content, "    0 z\n>   0 b2"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("取り直しが失敗すると前回の一覧を残して理由の 1 行を出し、次に取れたらその行が消える", func(t *testing.T) {
		calls := 0
		m := New(sequence(&calls,
			result{names: []string{"a", "b"}},
			result{err: errors.New("boom")},
			result{names: []string{"a", "b", "c"}},
		), nil, nil, nil)
		next, _ := m.Update(m.Init()())
		next, cmd := next.Update(tickMsg{})
		next, cmd = next.Update(cmd())
		if cmd == nil {
			t.Fatal("失敗後の Update = nil, want 次の tick の Cmd")
		}
		if got, want := next.(Model).View().Content, ">   0 a\n    0 b\n一覧を取得できません: boom"; got != want {
			t.Errorf("失敗後の View() = %q, want %q", got, want)
		}
		next, cmd = next.Update(tickMsg{})
		next, _ = next.Update(cmd())
		if got, want := next.(Model).View().Content, ">   0 a\n    0 b\n    0 c"; got != want {
			t.Errorf("次に取れた後の View() = %q, want %q", got, want)
		}
	})

	t.Run("名前の入力中に取り直しが来ても入力中の名前は変わらない", func(t *testing.T) {
		calls := 0
		m := New(sequence(&calls, result{names: []string{"a", "b"}}, result{names: []string{"a", "b", "c"}}), nil, recordCreate(new([]string), nil), nil)
		next, _ := m.Update(m.Init()())
		m = typeText(next.(Model), "nwo")
		next, cmd := m.Update(tickMsg{})
		next, _ = next.Update(cmd())
		if got, want := next.(Model).View().Content, ">   0 a\n    0 b\n    0 c\n新規セッション名: wo"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})
}

type viewingResult struct {
	id    string
	known bool
	err   error
}

// viewingSeq は呼ばれるたびに responses を先頭から 1 つずつ返す見ている id の取得関数。
func viewingSeq(responses ...viewingResult) viewingFunc {
	i := 0
	return func(context.Context) (string, bool, error) {
		r := responses[i]
		i++
		return r.id, r.known, r.err
	}
}

func sessionsWithID(ids ...string) fetchFunc {
	return func(context.Context) ([]sessionlist.Session, error) {
		ss := make([]sessionlist.Session, len(ids))
		for i, id := range ids {
			ss[i] = sessionlist.Session{ID: id, Name: "s" + id}
		}
		return ss, nil
	}
}

// refresh は取り直しの tick を 1 回進めた Model を返す。
func refresh(t *testing.T, m Model) Model {
	t.Helper()
	_, cmd := m.Update(tickMsg{})
	next, _ := m.Update(cmd())
	return next.(Model)
}

func TestModelViewing(t *testing.T) {
	t.Run("見ている id の行にだけ 2 桁目の印が付き、カーソルの印とは別の桁に出る", func(t *testing.T) {
		m := New(sessionsWithID("$1", "$2", "$3"), nil, nil, viewingSeq(viewingResult{id: "$2", known: true}))
		next, _ := m.Update(m.Init()())
		if got, want := next.(Model).View().Content, ">   0 s$1\n *  0 s$2\n    0 s$3"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("カーソルの行が見ている行のときは 1 桁目と 2 桁目の両方に印が付く", func(t *testing.T) {
		m := New(sessionsWithID("$1", "$2"), nil, nil, viewingSeq(viewingResult{id: "$1", known: true}))
		next, _ := m.Update(m.Init()())
		if got, want := next.(Model).View().Content, ">*  0 s$1\n    0 s$2"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("分からないとき、または見ている id が一覧に無いときはどの行にも印が付かない", func(t *testing.T) {
		for name, r := range map[string]viewingResult{
			"分からない":    {known: false},
			"一覧に無い id": {id: "$9", known: true},
		} {
			m := New(sessionsWithID("$1", "$2"), nil, nil, viewingSeq(r))
			next, _ := m.Update(m.Init()())
			if got, want := next.(Model).View().Content, ">   0 s$1\n    0 s$2"; got != want {
				t.Errorf("%s のとき View() = %q, want %q", name, got, want)
			}
		}
	})

	t.Run("失敗の行と 0 件の案内の行には印が付かない", func(t *testing.T) {
		failing := func(context.Context) ([]sessionlist.Session, error) { return nil, errors.New("boom") }
		m := New(failing, nil, nil, viewingSeq(viewingResult{id: "", known: true}))
		next, _ := m.Update(m.Init()())
		if got, want := next.(Model).View().Content, "一覧を取得できません: boom"; got != want {
			t.Errorf("失敗のとき View() = %q, want %q", got, want)
		}
		m = New(names(), nil, nil, viewingSeq(viewingResult{id: "", known: true}))
		next, _ = m.Update(m.Init()())
		if got, want := next.(Model).View().Content, strings.Join(sessionlist.Lines(nil), "\n"); got != want {
			t.Errorf("0 件のとき View() = %q, want %q", got, want)
		}
	})

	t.Run("取り直しのたびに見ている id も取り直され、印が移る", func(t *testing.T) {
		m := New(sessionsWithID("$1", "$2"), nil, nil, viewingSeq(
			viewingResult{id: "$1", known: true},
			viewingResult{id: "$2", known: true},
		))
		next, _ := m.Update(m.Init()())
		m = refresh(t, next.(Model))
		if got, want := m.View().Content, ">   0 s$1\n *  0 s$2"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("見ている id の取得が失敗しても一覧は残って理由の 1 行が出て、次に取れたらその行が消える", func(t *testing.T) {
		m := New(sessionsWithID("$1", "$2"), nil, nil, viewingSeq(
			viewingResult{id: "$1", known: true},
			viewingResult{err: errors.New("boom")},
			viewingResult{id: "$2", known: true},
		))
		next, _ := m.Update(m.Init()())
		m = refresh(t, next.(Model))
		if got, want := m.View().Content, ">   0 s$1\n    0 s$2\n右が見ているセッションを取得できません: boom"; got != want {
			t.Errorf("失敗後の View() = %q, want %q", got, want)
		}
		m = refresh(t, m)
		if got, want := m.View().Content, ">   0 s$1\n *  0 s$2"; got != want {
			t.Errorf("次に取れた後の View() = %q, want %q", got, want)
		}
	})
}

func TestModelWindowsAndActivity(t *testing.T) {
	t.Run("window 数 3 のセッションの行に window 数 3 が出る", func(t *testing.T) {
		m, _ := load(t, func(context.Context) ([]sessionlist.Session, error) {
			return []sessionlist.Session{{ID: "$1", Name: "a", Windows: 3}}, nil
		})
		if got, want := m.View().Content, ">   3 a"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("名前に空白・タブ・記号があっても名前と window 数が崩れない", func(t *testing.T) {
		m, _ := load(t, func(context.Context) ([]sessionlist.Session, error) {
			return []sessionlist.Session{{ID: "$1", Name: "a  b\t3 #{x},y:z", Windows: 3}}, nil
		})
		if got, want := m.View().Content, ">   3 a  b\t3 #{x},y:z"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("失敗の理由の行と 0 件の案内の行には window 数も印も付かない", func(t *testing.T) {
		m, _ := load(t, func(context.Context) ([]sessionlist.Session, error) { return nil, errors.New("boom") })
		if got, want := m.View().Content, "一覧を取得できません: boom"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("右で見ていないセッションの activity が進むと印が付き、右が見ると消える", func(t *testing.T) {
		acts := []map[string]int64{
			{"$1": 10, "$2": 10},
			{"$1": 10, "$2": 20},
			{"$1": 10, "$2": 20},
		}
		n := 0
		fetch := func(context.Context) ([]sessionlist.Session, error) {
			a := acts[n]
			return []sessionlist.Session{
				{ID: "$1", Name: "a", Windows: 1, Activity: a["$1"]},
				{ID: "$2", Name: "b", Windows: 1, Activity: a["$2"]},
			}, nil
		}
		viewing := func(context.Context) (string, bool, error) {
			if n == 2 {
				return "$2", true, nil
			}
			return "$1", true, nil
		}
		m := New(fetch, nil, nil, viewing)
		next, _ := m.Update(m.Init()())
		m = next.(Model)
		if got, w := m.View().Content, ">*  1 a\n    1 b"; got != w {
			t.Errorf("1 回目 View() = %q, want %q", got, w)
		}
		n = 1
		m = refresh(t, m)
		if got, w := m.View().Content, ">*  1 a\n  ! 1 b"; got != w {
			t.Errorf("activity が進んだ後 View() = %q, want %q", got, w)
		}
		n = 2
		m = refresh(t, m)
		if got, w := m.View().Content, ">   1 a\n *  1 b"; got != w {
			t.Errorf("右が見た後 View() = %q, want %q", got, w)
		}
	})
}

func TestModelMarksShareColumns(t *testing.T) {
	t.Run("カーソルと activity の印が付く行と見ている印が付く行で、window 数と名前の桁が揃う", func(t *testing.T) {
		n := 0
		fetch := func(context.Context) ([]sessionlist.Session, error) {
			return []sessionlist.Session{
				{ID: "$1", Name: "a", Windows: 1, Activity: int64(10 + n)},
				{ID: "$2", Name: "b", Windows: 12, Activity: 10},
			}, nil
		}
		viewing := func(context.Context) (string, bool, error) { return "$2", true, nil }
		m := New(fetch, nil, nil, viewing)
		next, _ := m.Update(m.Init()())
		n = 1
		m = refresh(t, next.(Model))
		if got, want := m.View().Content, "> ! 1 a\n * 12 b"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})
}

func TestModelSkipRedraw(t *testing.T) {
	base := func() sessionsMsg {
		return sessionsMsg{
			sessions: []sessionlist.Session{
				{ID: "$1", Name: "a", Windows: 1, Activity: 10},
				{ID: "$2", Name: "b", Windows: 1, Activity: 10},
			},
			viewingFetched: true, viewingID: "$1", viewingKnown: true,
		}
	}
	update := func(m Model, msg tea.Msg) (Model, tea.Cmd) {
		next, cmd := m.Update(msg)
		return next.(Model), cmd
	}
	// isTick は cmd が次の取り直しの tick だけを返すことを確かめる。
	isTick := func(t *testing.T, cmd tea.Cmd) {
		t.Helper()
		if cmd == nil {
			t.Fatal("Cmd = nil, want 次の取り直しの tick")
		}
		m := New(nil, nil, nil, nil)
		m.interval = time.Millisecond
		_, tick := m.Update(sessionsMsg{})
		if got, want := reflect.TypeOf(cmd()), reflect.TypeOf(tick()); got != want {
			t.Errorf("Cmd() の型 = %v, want %v", got, want)
		}
	}
	loaded := func(t *testing.T) Model {
		t.Helper()
		m := New(nil, nil, nil, nil)
		m.interval = time.Millisecond
		m, _ = update(m, base())
		return m
	}

	t.Run("前回と同じ取得結果を渡すと Model も画面も変わらず Cmd は次の tick だけ", func(t *testing.T) {
		m := loaded(t)
		next, cmd := update(m, base())
		if !reflect.DeepEqual(next, m) {
			t.Errorf("Model = %+v, want %+v", next, m)
		}
		if got, want := next.View().Content, m.View().Content; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
		isTick(t, cmd)
	})

	diffs := map[string]func(*sessionsMsg){
		"見ている印":       func(s *sessionsMsg) { s.viewingID = "$2" },
		"window 数":    func(s *sessionsMsg) { s.sessions[1].Windows = 2 },
		"activity の印": func(s *sessionsMsg) { s.sessions[1].Activity = 20 },
	}
	wants := map[string]string{
		"見ている印":       " *  1 b",
		"window 数":    "    2 b",
		"activity の印": "  ! 1 b",
	}
	for name, change := range diffs {
		t.Run(name+"だけが違う取得結果を渡すと Model が変わり画面にその違いが出る", func(t *testing.T) {
			m := loaded(t)
			msg := base()
			change(&msg)
			next, cmd := update(m, msg)
			if reflect.DeepEqual(next, m) {
				t.Error("Model が変わらなかった, want 変わる")
			}
			if got := next.View().Content; !strings.Contains(got, wants[name]) {
				t.Errorf("View() = %q, want %q を含む", got, wants[name])
			}
			isTick(t, cmd)
		})
	}

	t.Run("右が見ているセッションの activity だけが進んでも描き直さず、基準は進む", func(t *testing.T) {
		m := loaded(t)
		msg := base()
		msg.sessions[0].Activity = 30
		next, cmd := update(m, msg)
		if got, want := next.View().Content, m.View().Content; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
		if !reflect.DeepEqual(next.sessions, m.sessions) || !reflect.DeepEqual(next.activityMarked, m.activityMarked) {
			t.Errorf("表示に出る値が入れ替わった: sessions=%+v marked=%+v", next.sessions, next.activityMarked)
		}
		if got := next.activityBase["$1"]; got != 30 {
			t.Errorf("activityBase[$1] = %d, want 30", got)
		}
		isTick(t, cmd)
	})

	t.Run("取り直しの失敗が続く間は理由の行が同じなら描き直さず、取れて理由が消えると描き直す", func(t *testing.T) {
		m := loaded(t)
		fail := sessionsMsg{err: errors.New("boom")}
		failed, _ := update(m, fail)
		if !strings.HasSuffix(failed.View().Content, "一覧を取得できません: boom") {
			t.Fatalf("View() = %q, want 理由の行で終わる", failed.View().Content)
		}
		again, cmd := update(failed, fail)
		if !reflect.DeepEqual(again, failed) {
			t.Errorf("Model = %+v, want %+v", again, failed)
		}
		isTick(t, cmd)
		recovered, _ := update(again, base())
		if got, want := recovered.View().Content, m.View().Content; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})
}

func TestModelRefetchAfterCreate(t *testing.T) {
	update := func(m Model, msg tea.Msg) (Model, tea.Cmd) {
		next, cmd := m.Update(msg)
		return next.(Model), cmd
	}
	// create は n で名前を確定して createdMsg を Update に流した後の Model と Cmd を返す。
	create := func(t *testing.T, m Model, createErr error) (Model, tea.Cmd) {
		t.Helper()
		m.create = recordCreate(new([]string), createErr)
		m = typeText(m, "nnew")
		m, cmd := pressKey(m, tea.KeyEnter)
		if cmd == nil {
			t.Fatal("Enter の Update = nil, want 作成の Cmd")
		}
		return update(m, cmd())
	}
	isTick := func(cmd tea.Cmd) bool {
		if cmd == nil {
			return false
		}
		_, ok := cmd().(tickMsg)
		return ok
	}

	t.Run("作成が成功すると次の取り直しを待たずに作ったセッションが一覧に出る", func(t *testing.T) {
		calls := 0
		m := New(sequence(&calls, result{names: []string{"a"}}, result{names: []string{"a", "new"}}), nil, nil, nil)
		m.interval = time.Hour
		m, _ = update(m, m.Init()())
		m, cmd := create(t, m, nil)
		if cmd == nil {
			t.Fatal("作成成功後の Cmd = nil, want 即時の取得")
		}
		m, _ = update(m, cmd())
		if got, want := m.View().Content, ">   0 a\n    0 new"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})

	t.Run("作成が成功した後も取り直しの鎖は 1 本のまま", func(t *testing.T) {
		calls := 0
		m := New(sequence(&calls, result{names: []string{"a"}}, result{names: []string{"a", "new"}}), nil, nil, nil)
		m.interval = time.Millisecond
		m, _ = update(m, m.Init()())
		m, cmd := create(t, m, nil)
		_, after := update(m, cmd())
		if after != nil {
			t.Errorf("即時の結果の後の Cmd = %T, want nil（次の tick を張らない）", after())
		}
	})

	t.Run("作成より前に始まった定期の取得の結果が後から届いても作ったセッションは消えず、鎖は続く", func(t *testing.T) {
		calls := 0
		m := New(sequence(&calls, result{names: []string{"a"}}, result{names: []string{"a"}}, result{names: []string{"a", "new"}}), nil, nil, nil)
		m.interval = time.Millisecond
		m, _ = update(m, m.Init()())
		_, tickCmd := update(m, tickMsg{})
		stale := tickCmd() // 作成の前に始まった定期の取得（作ったセッションを含まない）
		m, cmd := create(t, m, nil)
		m, _ = update(m, cmd())
		m, after := update(m, stale)
		if got, want := m.View().Content, ">   0 a\n    0 new"; got != want {
			t.Errorf("古い結果の後の View() = %q, want %q", got, want)
		}
		if !isTick(after) {
			t.Error("古い定期の結果の後の Cmd は次の tick であるべき（鎖を止めない）")
		}
	})

	t.Run("古い世代の即時の結果は表示に使わず、次の tick も張らない", func(t *testing.T) {
		m := New(nil, nil, nil, nil)
		m.interval = time.Millisecond
		m.gen = 2
		m, cmd := update(m, sessionsMsg{sessions: []sessionlist.Session{{ID: "$1", Name: "old"}}, gen: 1, immediate: true})
		if cmd != nil {
			t.Errorf("Cmd = %T, want nil", cmd())
		}
		if got := m.View().Content; strings.Contains(got, "old") {
			t.Errorf("View() = %q, want 古い結果の old を含まない", got)
		}
	})

	t.Run("作成が失敗したときは理由の 1 行を出し、取り直しの Cmd を返さない", func(t *testing.T) {
		m := New(names("a"), nil, nil, nil)
		m, _ = update(m, m.Init()())
		m, cmd := create(t, m, errors.New("boom"))
		if cmd != nil {
			t.Errorf("Cmd = %T, want nil", cmd())
		}
		if got, want := m.View().Content, ">   0 a\n新規セッション名: new\n作成できません: boom"; got != want {
			t.Errorf("View() = %q, want %q", got, want)
		}
	})
}
