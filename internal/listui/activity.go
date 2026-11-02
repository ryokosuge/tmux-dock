package listui

import "github.com/ryokosuge/tmux-dock/internal/sessionlist"

// activityMarks は取り直した一覧 fetched から、activity の印を付けるセッションの集合と次の基準を返す。
// 基準 base は session_id → 最後に右で見た時点の Activity。名前ではなく ID で突き合わせるので、改名しても印は変わらない。
// viewing は右が見ているセッションの id で、viewingKnown が false のときは分からないことを表し、どの基準も進めない。
//   - 右が見ているセッションは基準を今の値へ進め、印を付けない
//   - 基準を持つそれ以外のセッションは基準を変えず、Activity が基準より大きいときだけ印を付ける
//   - 基準を持たないセッション（起動時の最初の取得・新しく現れたセッション）は今の値を基準にし、印を付けない
//
// fetched に無いセッションの基準は次の基準に含めない。
func activityMarks(base map[string]int64, fetched []sessionlist.Session, viewing string, viewingKnown bool) (next map[string]int64, marked map[string]bool) {
	next = make(map[string]int64, len(fetched))
	marked = make(map[string]bool)
	for _, s := range fetched {
		prev, ok := base[s.ID]
		switch {
		case viewingKnown && s.ID == viewing, !ok:
			next[s.ID] = s.Activity
		default:
			next[s.ID] = prev
			if s.Activity > prev {
				marked[s.ID] = true
			}
		}
	}
	return next, marked
}
