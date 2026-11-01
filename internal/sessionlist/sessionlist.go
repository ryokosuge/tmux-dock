package sessionlist

import "strings"

// Format は list-sessions に渡す -F の書式。1 行 1 セッション、先頭が session_id、空白 1 つを挟んで残り全部がセッション名。
// id は $ と数字だけで空白を含まないので、先頭の 1 つだけで分ければ名前に空白や区切りが入っても割れない。
const Format = "#{session_id} #{session_name}"

// emptyMessage は 0 件のときに左ペインへ出す行。
const emptyMessage = "セッションがありません"

// Session は普段の tmux サーバにある 1 つのセッション。
type Session struct {
	ID   string
	Name string
}

// Parse は list-sessions -F Format の出力（改行区切り）を、tmux の出力順のままの並びにする。
// 空の出力は 0 件。
func Parse(out []byte) []Session {
	text := strings.TrimSuffix(string(out), "\n")
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	ss := make([]Session, len(lines))
	for i, l := range lines {
		id, name, _ := strings.Cut(l, " ")
		ss[i] = Session{ID: id, Name: name}
	}
	return ss
}

// Lines は左ペインに出す行を返す。0 件なら案内の 1 行。tmux は呼ばない。
func Lines(ss []Session) []string {
	if len(ss) == 0 {
		return []string{emptyMessage}
	}
	lines := make([]string, len(ss))
	for i, s := range ss {
		lines[i] = s.Name
	}
	return lines
}
