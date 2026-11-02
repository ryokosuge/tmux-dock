package listui

import (
	"slices"

	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
)

// displayRow は一覧の 1 行の描画に出る項目。生の Activity は印の集合に畳まれて描画に出ないので持たない。
type displayRow struct {
	id      string
	name    string
	windows int
	viewing bool
	marked  bool
}

// displayState は一覧の描画に出る項目だけを集めた値。前回と同じなら描き直さない。
type displayState struct {
	rows         []displayRow
	cursor       int
	viewingKnown bool
	loadNotice   string
}

// newDisplayState は一覧 sessions（list-sessions の並び）・カーソル位置・右が見ているセッションの id・
// activity の印を付けるセッションの集合・取得の失敗の理由の行から表示の状態を作る。
// viewingKnown が false のときは右が見ているセッションが分からないことを表し、どの行も見ている印を持たない。
func newDisplayState(sessions []sessionlist.Session, cursor int, viewing string, viewingKnown bool, marked map[string]bool, loadNotice string) displayState {
	rows := make([]displayRow, len(sessions))
	for i, s := range sessions {
		rows[i] = displayRow{
			id:      s.ID,
			name:    s.Name,
			windows: s.Windows,
			viewing: viewingKnown && s.ID == viewing,
			marked:  marked[s.ID],
		}
	}
	return displayState{rows: rows, cursor: cursor, viewingKnown: viewingKnown, loadNotice: loadNotice}
}

// equal は 2 つの表示の状態が同じ（描画が変わらない）かを返す。
func (d displayState) equal(o displayState) bool {
	return d.cursor == o.cursor && d.viewingKnown == o.viewingKnown && d.loadNotice == o.loadNotice &&
		slices.Equal(d.rows, o.rows)
}
