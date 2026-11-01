package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ryokosuge/tmux-dock/internal/rightpane"
)

// withoutSession は innerLayout の出力から、session_id が sessionID の行を除く。
func withoutSession(layout, sessionID string) string {
	var kept []string
	for _, l := range strings.SplitAfter(layout, "\n") {
		if l != "" && !strings.HasPrefix(l, sessionID+" ") {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, "")
}

func outerPaneCount(outer string) int {
	return len(strings.Fields(tmuxQuiet(outer, "list-panes", "-t", "=dock:", "-F", "#{pane_id}")))
}

// innerAttachProcesses は tmux-dock の子 attach（コマンドが inner の attach-session で終わる行）を返す。
func innerAttachProcesses(t *testing.T, inner string) []string {
	t.Helper()
	out, err := exec.Command("ps", "-axo", "command").Output()
	if err != nil {
		t.Fatalf("ps: %v", err)
	}
	var found []string
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasSuffix(l, "-L "+inner+" attach-session") {
			found = append(found, l)
		}
	}
	return found
}

func TestTmuxDockRightPaneFollowsKilledSession(t *testing.T) {
	bin := buildBinary(t)
	outer, inner := startDockWithLastUsedA(t, bin)
	before := innerLayout(t, inner)
	aID := strings.TrimSpace(tmuxOut(t, inner, "display-message", "-p", "-t", "=a:", "#{session_id}"))

	tmuxOut(t, inner, "kill-session", "-t", "=a")
	if got := waitClientSession(t, inner, "b"); got != "b" {
		t.Fatalf("a を消した後の右のクライアントの attach 先 = %q, want %q（%v 以内）", got, "b", paneWaitLimit)
	}
	if n := outerPaneCount(outer); n != 2 {
		t.Errorf("a を消した後の外側のペイン数 = %d, want 2（右が閉じていない）", n)
	}
	if got, want := innerLayout(t, inner), withoutSession(before, aID); got != want {
		t.Errorf("a を消した後の普段のサーバの構成 = %q, want %q（消す前から a の行を除いたもの）", got, want)
	}

	tmuxOut(t, inner, "kill-session", "-t", "=b")
	var pane string
	deadline := time.Now().Add(paneWaitLimit)
	for !showsWaitingMessage(pane) && time.Now().Before(deadline) {
		pane = tmuxQuiet(outer, "capture-pane", "-p", "-J", "-t", rightPaneTarget)
		time.Sleep(paneWaitStep)
	}
	if !showsWaitingMessage(pane) {
		t.Fatalf("最後のセッションを消した後の右ペインの表示 = %q, want %q を含む（%v 以内）", pane, rightpane.WaitingMessage, paneWaitLimit)
	}
	if n := outerPaneCount(outer); n != 2 {
		t.Errorf("最後のセッションを消した後の外側のペイン数 = %d, want 2（右が閉じていない）", n)
	}
	if out := tmuxQuiet(inner, "list-sessions", "-F", "#{session_name}"); out != "" {
		t.Errorf("最後のセッションを消した後の list-sessions = %q, want 0 件", out)
	}

	tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", "c")
	if got := waitClientSession(t, inner, "c"); got != "c" {
		t.Errorf("c を作った後の右のクライアントの attach 先 = %q, want %q（%v 以内）", got, "c", paneWaitLimit)
	}
	if got, want := strings.TrimSpace(tmuxOut(t, inner, "list-sessions", "-F", "#{session_name}")), "c"; got != want {
		t.Errorf("c を作った後の普段のサーバのセッション = %q, want %q（tmux-dock が作ったセッションが無い）", got, want)
	}
	if procs := innerAttachProcesses(t, inner); len(procs) != 1 {
		t.Errorf("右が移った後の attach-session のプロセス = %q, want 1 本（孤児が残っていない）", procs)
	}

	tmuxOut(t, outer, "kill-server")
	if got := waitClientSession(t, inner, ""); got != "" {
		t.Errorf("外側を終えた後も普段のサーバにクライアント %q が残る", got)
	}
	deadline = time.Now().Add(paneWaitLimit)
	procs := innerAttachProcesses(t, inner)
	for len(procs) > 0 && time.Now().Before(deadline) {
		time.Sleep(paneWaitStep)
		procs = innerAttachProcesses(t, inner)
	}
	if len(procs) > 0 {
		t.Errorf("外側を終えた後の attach-session のプロセス = %q, want 0 本", procs)
	}
}
