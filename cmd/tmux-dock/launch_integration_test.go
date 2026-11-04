package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

const launchDeadline = time.Second

// startDockAgain は 1 回目とは別名の harness サーバで tmux-dock をもう一度起動する（同名だと harness の new-session が重複する）。
func startDockAgain(t *testing.T, bin, harness, outer, inner string) {
	t.Helper()
	again := harness + "-2"
	t.Cleanup(func() { _ = exec.Command("tmux", "-L", again, "kill-server").Run() })
	startDock(t, bin, again, outer, inner)
}

// waitOuterClients は外側の dock に attach するクライアントが want 本になるまで待ち、最後に読んだ client_session を返す。
func waitOuterClients(t *testing.T, outer string, want int) []string {
	t.Helper()
	var clients []string
	deadline := time.Now().Add(paneWaitLimit)
	for {
		clients = strings.Fields(tmuxQuiet(outer, "list-clients", "-F", "#{client_session}"))
		if len(clients) == want || time.Now().After(deadline) {
			return clients
		}
		time.Sleep(paneWaitStep)
	}
}

func outerSessions(outer string) []string {
	return strings.Fields(tmuxQuiet(outer, "list-sessions", "-F", "#{session_name}"))
}

func newInnerSessions(t *testing.T, inner string) {
	t.Helper()
	for _, name := range []string{"a", "b"} {
		tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", name, "-n", "w")
	}
}

func TestTmuxDockLaunch(t *testing.T) {
	bin := buildBinary(t)
	hasAB := func(s string) bool { return hasLine(s, "a") && hasLine(s, "b") }

	t.Run("外側が動いていない試験用ソケットで dock を起動すると、外側に dock のセッションが 1 つでき、左に一覧・右にセッションのペインが並ぶ", func(t *testing.T) {
		harness, outer, inner := newSockets(t)
		newInnerSessions(t, inner)

		startDock(t, bin, harness, outer, inner)

		left := waitLeftPane(t, outer, hasAB)
		if !hasAB(left) {
			t.Errorf("左ペインの表示 = %q, want 行 a と b を含む（%v 以内）", left, paneWaitLimit)
		}
		if got := outerSessions(outer); len(got) != 1 || got[0] != "dock" {
			t.Errorf("外側の list-sessions = %v, want [dock] の 1 行", got)
		}
		panes := strings.Fields(tmuxQuiet(outer, "list-panes", "-a", "-F", "#{pane_left}"))
		if len(panes) != 2 || panes[0] == panes[1] {
			t.Errorf("外側の pane_left = %v, want 互いに異なる左右 2 ペイン", panes)
		}
	})

	t.Run("外側が既に動いている試験用ソケットで dock をもう一度起動すると、外側のセッション数は増えず、既存の dock のセッションに繋がる", func(t *testing.T) {
		harness, outer, inner := newSockets(t)
		newInnerSessions(t, inner)

		startDock(t, bin, harness, outer, inner)
		if got := waitOuterClients(t, outer, 1); len(got) != 1 {
			t.Fatalf("1 回目の起動後の外側のクライアント = %v, want 1 本（%v 以内）", got, paneWaitLimit)
		}

		startDockAgain(t, bin, harness, outer, inner)

		got := waitOuterClients(t, outer, 2)
		if len(got) != 2 || got[0] != "dock" || got[1] != "dock" {
			t.Errorf("2 回目の起動後の外側のクライアントの attach 先 = %v, want [dock dock]（%v 以内）", got, paneWaitLimit)
		}
		if s := outerSessions(outer); len(s) != 1 || s[0] != "dock" {
			t.Errorf("2 回目の起動後の外側の list-sessions = %v, want [dock] の 1 行", s)
		}
		if n := outerPaneCount(outer); n != 2 {
			t.Errorf("2 回目の起動後の外側のペイン数 = %d, want 2", n)
		}
	})

	t.Run("外側が動いていない状態から dock を起動して一覧が最初に描かれるまでが 1 秒以内に終わる", func(t *testing.T) {
		harness, outer, inner := newSockets(t)
		newInnerSessions(t, inner)

		start := time.Now()
		startDock(t, bin, harness, outer, inner)
		left := waitLeftPane(t, outer, hasAB)
		elapsed := time.Since(start)

		t.Logf("起動から左ペインの最初の描画までの時間 = %v（上限 %v）", elapsed, launchDeadline)
		if !hasAB(left) {
			t.Fatalf("左ペインの表示 = %q, want 行 a と b を含む（%v 以内）", left, paneWaitLimit)
		}
		if elapsed > launchDeadline {
			t.Errorf("起動から一覧の最初の描画までの時間 = %v, want %v 以内", elapsed, launchDeadline)
		}
	})

	t.Run("dock の起動と繋ぎ直しを行った後も、普段の tmux サーバのセッションと window の構成が起動前と同じである", func(t *testing.T) {
		harness, outer, inner := newSockets(t)
		newInnerSessions(t, inner)
		before := innerLayout(t, inner)

		startDock(t, bin, harness, outer, inner)
		waitLeftPane(t, outer, hasAB)
		if got := waitOuterClients(t, outer, 1); len(got) != 1 {
			t.Fatalf("1 回目の起動後の外側のクライアント = %v, want 1 本", got)
		}
		startDockAgain(t, bin, harness, outer, inner)
		if got := waitOuterClients(t, outer, 2); len(got) != 2 {
			t.Fatalf("2 回目の起動後の外側のクライアント = %v, want 2 本", got)
		}

		if after := innerLayout(t, inner); after != before {
			t.Errorf("起動と繋ぎ直しの後の普段のサーバの構成 = %q, want %q", after, before)
		}
	})
}
