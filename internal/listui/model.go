// Package listui は左ペインに出すセッション一覧の Bubble Tea の画面。
package listui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

type fetchFunc func(ctx context.Context) ([]sessionlist.Session, error)

// switchFunc は Enter で選んだセッションに右ペインを切り替える。
type switchFunc func(ctx context.Context, s sessionlist.Session) error

// createFunc は入力した名前で新しいセッションを作る。
type createFunc func(ctx context.Context, name string) error

const (
	cursorMark   = "> "
	noCursorMark = "  "
	inputPrompt  = "新規セッション名: "
)

type sessionsMsg struct {
	sessions []sessionlist.Session
	err      error
}

type switchedMsg struct {
	session sessionlist.Session
	err     error
}

type createdMsg struct {
	err error
}

// Model は起動時に一覧を 1 回だけ取り、カーソル付きで描く。Enter で選んだセッションを switchTo に渡す。
// n で名前の入力に入り、確定した名前を create に渡す。入力中はカーソルを動かさない。
type Model struct {
	fetch    fetchFunc
	switchTo switchFunc
	create   createFunc
	sessions []sessionlist.Session
	cursor   int
	// notice は一覧の代わりに（取得の失敗）または一覧の下に（切り替えの失敗）出す 1 行。
	notice    string
	loadError bool
	// inputting は名前の入力中。input は入力中の名前、inputError は作成に失敗したときの理由の 1 行。
	// creating は作成関数の結果待ちで、その間の打鍵は名前に足さない。
	inputting  bool
	input      string
	inputError string
	creating   bool
}

// New の switchTo は nil でもよく、そのとき Enter は何もしない。create も nil でもよく、そのとき n は何もしない。
func New(fetch fetchFunc, switchTo switchFunc, create createFunc) Model {
	return Model{fetch: fetch, switchTo: switchTo, create: create}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		ss, err := m.fetch(context.Background())
		return sessionsMsg{sessions: ss, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionsMsg:
		m.cursor = 0
		m.notice = ""
		m.loadError = msg.err != nil
		if msg.err != nil {
			m.sessions = nil
			m.notice = fmt.Sprintf("一覧を取得できません: %v", msg.err)
		} else {
			m.sessions = msg.sessions
		}
	case switchedMsg:
		if msg.err != nil {
			m.notice = fmt.Sprintf("%s に切り替えられません: %v", msg.session.Name, msg.err)
		} else {
			m.notice = ""
		}
	case createdMsg:
		m.creating = false
		if msg.err != nil {
			m.inputError = fmt.Sprintf("作成できません: %v", msg.err)
		} else {
			m = m.closeInput()
		}
	case tea.KeyPressMsg:
		if m.inputting {
			return m.updateInput(msg)
		}
		switch msg.String() {
		case "n":
			if m.create != nil {
				m.inputting = true
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
			}
		case "enter":
			if m.switchTo == nil || m.cursor >= len(m.sessions) {
				return m, nil
			}
			s, switchTo := m.sessions[m.cursor], m.switchTo
			return m, func() tea.Msg {
				return switchedMsg{session: s, err: switchTo(context.Background(), s)}
			}
		}
	}
	return m, nil
}

func (m Model) closeInput() Model {
	m.inputting = false
	m.input = ""
	m.inputError = ""
	return m
}

// updateInput は入力中のキーを扱う。j / k / n も含め、印字できる文字はすべて名前に足す。
func (m Model) updateInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.creating {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		return m.closeInput(), nil
	case "backspace":
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
	case "enter":
		name := strings.TrimSpace(m.input)
		if name == "" {
			return m.closeInput(), nil
		}
		m.creating = true
		m.inputError = ""
		create := m.create
		return m, func() tea.Msg {
			return createdMsg{err: create(context.Background(), name)}
		}
	default:
		if msg.Text != "" && !msg.Mod.Contains(tea.ModCtrl) && !msg.Mod.Contains(tea.ModAlt) {
			m.input += msg.Text
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	if m.loadError {
		return tea.NewView(m.notice)
	}
	lines := sessionlist.Lines(m.sessions)
	if len(m.sessions) > 0 {
		for i := range lines {
			mark := noCursorMark
			if i == m.cursor {
				mark = cursorMark
			}
			lines[i] = mark + lines[i]
		}
	}
	if m.notice != "" {
		lines = append(lines, m.notice)
	}
	if m.inputting {
		lines = append(lines, inputPrompt+m.input)
		if m.inputError != "" {
			lines = append(lines, m.inputError)
		}
	}
	return tea.NewView(strings.Join(lines, "\n"))
}
