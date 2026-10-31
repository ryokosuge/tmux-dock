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

type sessionsMsg struct {
	sessions []sessionlist.Session
	err      error
}

// Model は起動時に一覧を 1 回だけ取り、その行を描く。キー操作は持たない。
type Model struct {
	fetch fetchFunc
	lines []string
}

func New(fetch fetchFunc) Model {
	return Model{fetch: fetch}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		ss, err := m.fetch(context.Background())
		return sessionsMsg{sessions: ss, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(sessionsMsg); ok {
		if msg.err != nil {
			m.lines = []string{fmt.Sprintf("一覧を取得できません: %v", msg.err)}
		} else {
			m.lines = sessionlist.Lines(msg.sessions)
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	return tea.NewView(strings.Join(m.lines, "\n"))
}
