package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ryokosuge/tmux-dock/internal/rightpane"
)

const (
	noServerGrace = 2 * time.Second
	// activity は秒単位の表示なので、a を最後に使った状態にするため b の作成から空ける。
	activityGap = 1100 * time.Millisecond
)

// harnessEnv は harness サーバが引き継ぐ環境（HOME は使い捨て）。
func harnessEnv(t *testing.T) []string {
	return append(os.Environ(), "HOME="+t.TempDir())
}

func tmuxHarness(t *testing.T, harness string, args ...string) {
	t.Helper()
	cmd := exec.Command("tmux", append([]string{"-L", harness, "-f", "/dev/null"}, args...)...)
	cmd.Env = harnessEnv(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("harness の tmux %s: %v (output=%s)", strings.Join(args, " "), err, out)
	}
}

// waitClientSession は inner の唯一のクライアントが attach するセッション名が want になるまで待つ。
func waitClientSession(t *testing.T, inner, want string) string {
	t.Helper()
	var got string
	deadline := time.Now().Add(paneWaitLimit)
	for {
		got = strings.TrimSpace(tmuxQuiet(inner, "list-clients", "-F", "#{session_name}"))
		if got == want || time.Now().After(deadline) {
			return got
		}
		time.Sleep(paneWaitStep)
	}
}

func TestTmuxDockAttachCommand(t *testing.T) {
	bin := buildBinary(t)

	t.Run("セッションが 0 件の間は待機の表示が出てサーバは立たず、a ができると 5 秒以内にそのペインが a に attach する", func(t *testing.T) {
		harness, _, inner := newSockets(t)
		tmuxHarness(t, harness, "new-session", "-d", "-x", "160", "-y", "40", bin, "attach", "--inner-socket", inner)

		var pane string
		deadline := time.Now().Add(paneWaitLimit)
		for !strings.Contains(pane, rightpane.WaitingMessage) && time.Now().Before(deadline) {
			pane = tmuxQuiet(harness, "capture-pane", "-p", "-t", "0")
			time.Sleep(paneWaitStep)
		}
		if !strings.Contains(pane, rightpane.WaitingMessage) {
			t.Fatalf("ペインの表示 = %q, want %q を含む", pane, rightpane.WaitingMessage)
		}
		time.Sleep(noServerGrace)
		if out := tmuxQuiet(inner, "list-sessions", "-F", "#{session_name}"); out != "" {
			t.Fatalf("待機中の普段のソケットの list-sessions = %q, want サーバが立たず 0 件", out)
		}

		tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", "a")
		if got := waitClientSession(t, inner, "a"); got != "a" {
			t.Errorf("attach したセッション = %q, want %q（%v 以内）", got, "a", paneWaitLimit)
		}
	})

	t.Run("TMUX_PANE が b のペインの id と同じペインから動かしても最後に使った a に繋がる", func(t *testing.T) {
		harness, _, inner := newSockets(t)
		tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", "a") // ペイン %0
		tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", "b") // ペイン %1
		time.Sleep(activityGap)

		// harness の 1 枚目で a に attach して離し、a を最後に使った状態にする。
		tmuxHarness(t, harness, "new-session", "-d", "-x", "160", "-y", "40",
			"tmux -L "+inner+" attach-session -t '=a'; sleep 300")
		if got := waitClientSession(t, inner, "a"); got != "a" {
			t.Fatalf("準備の attach 先 = %q, want %q", got, "a")
		}
		tmuxOut(t, inner, "detach-client", "-s", "a")

		// 2 枚目（harness の TMUX_PANE=%1）で attach サブコマンドを動かす。
		tmuxHarness(t, harness, "split-window", "-d", bin+" attach --inner-socket "+inner)
		if got := tmuxOut(t, harness, "list-panes", "-F", "#{pane_id}"); !strings.Contains(got, "%1") {
			t.Fatalf("harness のペイン = %q, want %%1 を含む", got)
		}
		if got := waitClientSession(t, inner, "a"); got != "a" {
			t.Errorf("attach したセッション = %q, want %q", got, "a")
		}
	})
}
