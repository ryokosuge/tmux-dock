package main

import (
	"strings"
	"testing"
	"time"
)

const switchMarker = "tdk-switch-marker"

func TestTmuxDockListSwitch(t *testing.T) {
	bin := buildBinary(t)

	t.Run("一覧で b を選ぶと右ペインのクライアントが b に移り、右に送った文字列が b のペインに現れる", func(t *testing.T) {
		outer, inner := startDockWithLastUsedA(t, bin)
		waitLeftPane(t, outer, func(s string) bool { return hasLine(s, "a") && hasLine(s, "b") })

		selectListRow(t, outer, "b")

		if got := waitClientSession(t, inner, "b"); got != "b" {
			t.Fatalf("切り替え後の右ペインのクライアントの attach 先 = %q, want %q（%v 以内）", got, "b", paneWaitLimit)
		}
		tmuxOut(t, outer, "send-keys", "-t", rightPaneTarget, "echo "+switchMarker, "Enter")
		var pane string
		deadline := time.Now().Add(paneWaitLimit)
		for !strings.Contains(pane, switchMarker) && time.Now().Before(deadline) {
			pane = tmuxQuiet(inner, "capture-pane", "-p", "-t", "=b:")
			time.Sleep(paneWaitStep)
		}
		if !strings.Contains(pane, switchMarker) {
			t.Errorf("セッション b のペインの内容 = %q, want %q を含む（%v 以内）", pane, switchMarker, paneWaitLimit)
		}
	})

	t.Run("一覧で切り替えた後も普段のサーバのセッションと window の構成は切り替え前と同じ", func(t *testing.T) {
		outer, inner := startDockWithLastUsedA(t, bin)
		waitLeftPane(t, outer, func(s string) bool { return hasLine(s, "a") && hasLine(s, "b") })
		before := innerLayout(t, inner)

		selectListRow(t, outer, "b")

		if got := waitClientSession(t, inner, "b"); got != "b" {
			t.Fatalf("切り替え後の右ペインのクライアントの attach 先 = %q, want %q（%v 以内）", got, "b", paneWaitLimit)
		}
		if after := innerLayout(t, inner); after != before {
			t.Errorf("切り替え後の普段のサーバの構成 = %q, want %q", after, before)
		}
	})
}

// selectListRow は外側の左ペインの一覧で row の行にカーソルを動かして Enter を押す。
// 一覧の行の並びに依らないよう、カーソル行（先頭の >）が row になるまで Down を送る。
func selectListRow(t *testing.T, outer, row string) {
	t.Helper()
	const leftPane = "=dock:.0"
	for i := 0; i < 5; i++ {
		left := tmuxQuiet(outer, "capture-pane", "-p", "-t", leftPane)
		if cursorOn(left, row) {
			tmuxOut(t, outer, "send-keys", "-t", leftPane, "Enter")
			return
		}
		tmuxOut(t, outer, "send-keys", "-t", leftPane, "Down")
		time.Sleep(paneWaitStep * 4)
	}
	t.Fatalf("一覧のカーソルが行 %q に乗らない: 左ペインの表示 = %q", row, tmuxQuiet(outer, "capture-pane", "-p", "-t", leftPane))
}

func cursorOn(left, row string) bool {
	for _, l := range strings.Split(left, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, ">") && strings.TrimSpace(strings.TrimPrefix(l, ">")) == row {
			return true
		}
	}
	return false
}
