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

const (
	cursorMark   = "> "
	noCursorMark = "  "
)

type sessionsMsg struct {
	sessions []sessionlist.Session
	err      error
}

type switchedMsg struct {
	session sessionlist.Session
	err     error
}

// Model は起動時に一覧を 1 回だけ取り、カーソル付きで描く。Enter で選んだセッションを switchTo に渡す。
type Model struct {
	fetch    fetchFunc
	switchTo switchFunc
	sessions []sessionlist.Session
	cursor   int
	// notice は一覧の代わりに（取得の失敗）または一覧の下に（切り替えの失敗）出す 1 行。
	notice    string
	loadError bool
}

// New の switchTo は nil でもよく、そのとき Enter は何もしない。
func New(fetch fetchFunc, switchTo switchFunc) Model {
	return Model{fetch: fetch, switchTo: switchTo}
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
	case tea.KeyPressMsg:
		switch msg.String() {
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
	return tea.NewView(strings.Join(lines, "\n"))
}
