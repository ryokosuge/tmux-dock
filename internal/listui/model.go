// Package listui は左ペインに出すセッション一覧の Bubble Tea の画面。
package listui

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	// refreshInterval は一覧を取り直す間隔の既定。
	refreshInterval = time.Second
)

type sessionsMsg struct {
	sessions []sessionlist.Session
	err      error
}

// tickMsg は取り直しの時刻が来たことを表す。
type tickMsg struct{}

type switchedMsg struct {
	session sessionlist.Session
	err     error
}

type createdMsg struct {
	err error
}

// Model は一覧を interval ごとに取り直し、カーソル付きで描く。前の取得が終わってから次の tick を張る。
// 取り直しに失敗したら前回の一覧を残して理由の 1 行を出し、次に取れたらその行を消す。Enter で選んだセッションを switchTo に渡す。
// n で名前の入力に入り、確定した名前を create に渡す。入力中はカーソルを動かさない。
type Model struct {
	fetch    fetchFunc
	switchTo switchFunc
	create   createFunc
	sessions []sessionlist.Session
	cursor   int
	// notice は一覧の下に出す、切り替えの失敗の 1 行。
	notice string
	// loadNotice は取得の失敗の 1 行。一覧を一度も取れていない間は一覧の代わりに、取れた後は一覧の下に出す。
	loadNotice string
	loaded     bool
	interval   time.Duration
	// inputting は名前の入力中。input は入力中の名前、inputError は作成に失敗したときの理由の 1 行。
	// creating は作成関数の結果待ちで、その間の打鍵は名前に足さない。
	inputting  bool
	input      string
	inputError string
	creating   bool
}

// New の switchTo は nil でもよく、そのとき Enter は何もしない。create も nil でもよく、そのとき n は何もしない。
func New(fetch fetchFunc, switchTo switchFunc, create createFunc) Model {
	return Model{fetch: fetch, switchTo: switchTo, create: create, interval: refreshInterval}
}

func (m Model) Init() tea.Cmd {
	return m.fetchCmd()
}

func (m Model) fetchCmd() tea.Cmd {
	fetch := m.fetch
	return func() tea.Msg {
		ss, err := fetch(context.Background())
		return sessionsMsg{sessions: ss, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionsMsg:
		if msg.err != nil {
			m.loadNotice = fmt.Sprintf("一覧を取得できません: %v", msg.err)
		} else {
			m.loadNotice = ""
			m.loaded = true
			m.sessions, m.cursor = follow(m.sessions, m.cursor, msg.sessions)
		}
		return m, tea.Tick(m.interval, func(time.Time) tea.Msg { return tickMsg{} })
	case tickMsg:
		return m, m.fetchCmd()
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
	var lines []string
	if m.loadNotice != "" && !m.loaded {
		lines = []string{m.loadNotice}
	} else {
		lines = sessionlist.Lines(m.sessions)
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
		if m.loadNotice != "" {
			lines = append(lines, m.loadNotice)
		}
	}
	if m.inputting {
		lines = append(lines, inputPrompt+m.input)
		if m.inputError != "" {
			lines = append(lines, m.inputError)
		}
	}
	return tea.NewView(strings.Join(lines, "\n"))
}
