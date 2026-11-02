package main

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// listRow は一覧の 1 行（カーソル・見ている・activity の 3 桁の印、window 数、名前）。
var listRow = regexp.MustCompile(`^...( ?\d+) (.+)$`)

// rowOf は name の行の activity の印（3 桁目）の有無と window 数を返す。行が無いときは found が false。
func rowOf(s, name string) (marked bool, windows int, found bool) {
	for _, l := range strings.Split(s, "\n") {
		m := listRow.FindStringSubmatch(strings.TrimRight(l, " "))
		if m == nil || m[2] != name {
			continue
		}
		n, _ := strconv.Atoi(strings.TrimSpace(m[1]))
		return []rune(l)[2] == '!', n, true
	}
	return false, 0, false
}

func TestTmuxDockActivityIntegration(t *testing.T) {
	bin := buildBinary(t)

	t.Run("右で見ていないセッションに出力を出すと、その行だけに activity の印が付き、window を足すと window 数が増える", func(t *testing.T) {
		outer, inner := startDockWithLastUsedA(t, bin)
		waitLeftPane(t, outer, func(s string) bool {
			_, _, a := rowOf(s, "a")
			_, _, b := rowOf(s, "b")
			return a && b
		})

		time.Sleep(activityGap) // window_activity は秒単位なので、最初の取得と別の秒にする
		tmuxOut(t, inner, "send-keys", "-t", "=b:", "echo x", "Enter")

		left := waitLeftPane(t, outer, func(s string) bool {
			bMarked, _, _ := rowOf(s, "b")
			return bMarked
		})
		bMarked, _, _ := rowOf(left, "b")
		aMarked, _, _ := rowOf(left, "a")
		if !bMarked || aMarked {
			t.Fatalf("出力後の左ペインの表示 = %q, want b の行だけに activity の印", left)
		}

		_, before, _ := rowOf(left, "b")
		tmuxOut(t, inner, "new-window", "-d", "-t", "=b:")
		left = waitLeftPane(t, outer, func(s string) bool {
			_, w, _ := rowOf(s, "b")
			return w == before+1
		})
		if _, w, _ := rowOf(left, "b"); w != before+1 {
			t.Errorf("window 追加後の b の行の window 数 = %d, want %d（表示 = %q）", w, before+1, left)
		}
	})
}
