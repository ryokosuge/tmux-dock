package listui

import "github.com/ryokosuge/tmux-dock/internal/sessionlist"

// follow は取り直した一覧 fetched を次の一覧にし、カーソルのあったセッションを追ったカーソル位置を返す。
// 一覧の並びは fetched（list-sessions の順）のままにする。セッションは改名されても同じと分かるよう ID で突き合わせる。
// カーソルのセッションが fetched に無い（消えた）ときは、元の位置を一覧の範囲に収める。0 件なら位置は 0。
func follow(prev []sessionlist.Session, cursor int, fetched []sessionlist.Session) ([]sessionlist.Session, int) {
	if cursor >= 0 && cursor < len(prev) {
		for i, s := range fetched {
			if s.ID == prev[cursor].ID {
				return fetched, i
			}
		}
	}
	return fetched, clamp(cursor, len(fetched))
}

func clamp(cursor, n int) int {
	if cursor >= n {
		cursor = n - 1
	}
	if cursor < 0 {
		cursor = 0
	}
	return cursor
}
