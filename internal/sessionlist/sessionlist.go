package sessionlist

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// fieldCount は Format の項目数。
const fieldCount = 4

// Format は list-sessions に渡す -F の書式。生のタブ（0x09）区切りで
// session_id / session_windows / 各 window の window_activity（window ごとに末尾の空白付き）/ session_name の順。
// 名前は何でも入りうるので必ず最後に置き、SplitN で割る。先頭や途中に置くと名前の区切りで割れる。
// tmux は -F の中の \t を解釈しないので、区切りは生のタブで書く。
const Format = "#{session_id}\t#{session_windows}\t#{W:#{window_activity} }\t#{session_name}"

// emptyMessage は 0 件のときに左ペインへ出す行。
const emptyMessage = "セッションがありません"

// Session は普段の tmux サーバにある 1 つのセッション。
// Activity は各 window の window_activity（UNIX 時刻の秒）の最大で、window が無ければ 0。
type Session struct {
	ID       string
	Name     string
	Windows  int
	Activity int64
}

// Parse は list-sessions -F Format の出力（改行区切り）を、tmux の出力順のままの並びにする。
// 空の出力は 0 件。解析できない行は飛ばし、行番号と理由を含むエラーにまとめて返す（他の行の結果は残る）。
func Parse(out []byte) ([]Session, error) {
	text := strings.TrimSuffix(string(out), "\n")
	if text == "" {
		return nil, nil
	}
	var ss []Session
	var errs []error
	for i, l := range strings.Split(text, "\n") {
		s, err := parseLine(l)
		if err != nil {
			errs = append(errs, fmt.Errorf("%d 行目 %q: %w", i+1, l, err))
			continue
		}
		ss = append(ss, s)
	}
	return ss, errors.Join(errs...)
}

func parseLine(line string) (Session, error) {
	f := strings.SplitN(line, "\t", fieldCount)
	if len(f) != fieldCount {
		return Session{}, fmt.Errorf("タブ区切りで %d 項目に割れない（%d 項目）", fieldCount, len(f))
	}
	windows, err := strconv.Atoi(f[1])
	if err != nil {
		return Session{}, fmt.Errorf("window 数 %q を数値に読めない", f[1])
	}
	var activity int64
	for _, a := range strings.Fields(f[2]) {
		v, err := strconv.ParseInt(a, 10, 64)
		if err != nil {
			return Session{}, fmt.Errorf("window_activity %q を数値に読めない", a)
		}
		activity = max(activity, v)
	}
	return Session{ID: f[0], Name: f[3], Windows: windows, Activity: activity}, nil
}

// windowsWidth は行の window 数の桁数。名前の前に固定の桁で置き、名前の長さや空白に左右されないようにする。
const windowsWidth = 2

// Lines は左ペインに出す行（window 数と名前）を返す。0 件なら案内の 1 行。tmux は呼ばない。
func Lines(ss []Session) []string {
	if len(ss) == 0 {
		return []string{emptyMessage}
	}
	lines := make([]string, len(ss))
	for i, s := range ss {
		lines[i] = fmt.Sprintf("%*d %s", windowsWidth, s.Windows, s.Name)
	}
	return lines
}
