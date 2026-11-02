package main

import (
	"strings"
	"testing"
)

// hasViewingLine は見ている印（2 桁目の "*"）が付いた name の行が s にあるかを返す。1 桁目はカーソルの印。
func hasViewingLine(s, name string) bool {
	for _, l := range strings.Split(s, "\n") {
		r := []rune(l)
		if len(r) > 1 && r[1] == '*' && strings.Fields(string(r[2:]))[0] == name {
			return true
		}
	}
	return false
}

func TestTmuxDockViewingMark(t *testing.T) {
	bin := buildBinary(t)

	t.Run("右のペインのクライアントを b に切り替えると、見ている印が a の行から b の行へ移る", func(t *testing.T) {
		outer, inner := startDockWithLastUsedA(t, bin)

		left := waitLeftPane(t, outer, func(s string) bool { return hasViewingLine(s, "a") })
		if !hasViewingLine(left, "a") || hasViewingLine(left, "b") {
			t.Fatalf("切り替え前の左ペインの表示 = %q, want a の行だけに見ている印", left)
		}

		tty := strings.TrimSpace(tmuxOut(t, inner, "list-clients", "-F", "#{client_tty}"))
		tmuxOut(t, inner, "switch-client", "-c", tty, "-t", "=b")

		left = waitLeftPane(t, outer, func(s string) bool { return hasViewingLine(s, "b") && !hasViewingLine(s, "a") })
		if !hasViewingLine(left, "b") || hasViewingLine(left, "a") {
			t.Errorf("切り替え後の左ペインの表示 = %q, want b の行だけに見ている印", left)
		}
	})
}
