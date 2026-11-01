package main

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	switchKey         = "F12"
	listPaneIndex     = "0"
	rightPaneIndex    = "1"
	minSendKeysMajor  = 3
	minSendKeysMinor  = 4
	activePaneFormat  = "#{pane_index} #{pane_active}"
	outerWindowTarget = "=dock:"
)

var tmuxVersionPattern = regexp.MustCompile(`(\d+)\.(\d+)`)

// skipUnlessSendKeysClient は tmux はあるが send-keys -K -c（3.4 以上）が無いときだけ skip する。tmux が無いときは何もせず、後続の newSockets が失敗させる。
func skipUnlessSendKeysClient(t *testing.T) {
	t.Helper()
	out, err := exec.Command("tmux", "-V").Output()
	if err != nil {
		return
	}
	m := tmuxVersionPattern.FindStringSubmatch(string(out))
	if m == nil {
		return
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major < minSendKeysMajor || (major == minSendKeysMajor && minor < minSendKeysMinor) {
		t.Skipf("tmux %q は send-keys -K -c（3.4 以上）が無いので打鍵の integration テストを skip する", strings.TrimSpace(string(out)))
	}
}

func outerClient(t *testing.T, outer string) string {
	t.Helper()
	var lines []string
	deadline := time.Now().Add(paneWaitLimit)
	for time.Now().Before(deadline) {
		lines = strings.Fields(tmuxQuiet(outer, "list-clients", "-F", "#{client_name}"))
		if len(lines) == 1 {
			return lines[0]
		}
		time.Sleep(paneWaitStep)
	}
	t.Fatalf("外側のクライアント = %q, want 1 件（%v 以内）", lines, paneWaitLimit)
	return ""
}

func sendKey(t *testing.T, outer, client, key string) {
	t.Helper()
	tmuxOut(t, outer, "send-keys", "-K", "-c", client, key)
}

// activePaneIndex は外側の dock の window で入力先のペインの番号を返す。
func activePaneIndex(t *testing.T, outer string) string {
	t.Helper()
	for _, line := range strings.Split(tmuxOut(t, outer, "list-panes", "-t", outerWindowTarget, "-F", activePaneFormat), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[1] == "1" {
			return f[0]
		}
	}
	return ""
}

func waitActivePane(t *testing.T, outer, want string) {
	t.Helper()
	var got string
	deadline := time.Now().Add(paneWaitLimit)
	for time.Now().Before(deadline) {
		if got = activePaneIndex(t, outer); got == want {
			return
		}
		time.Sleep(paneWaitStep)
	}
	t.Fatalf("入力先のペイン = %q, want %q（%v 以内）", got, want, paneWaitLimit)
}

func windowCount(t *testing.T, socket, target string) int {
	t.Helper()
	return len(strings.Fields(tmuxOut(t, socket, "list-windows", "-t", target, "-F", "#{window_id}")))
}

// startDockForKeys は右が a に繋がった状態で起動し、入力先を一覧（.0）に揃えて外側クライアント名を返す。
func startDockForKeys(t *testing.T, bin string) (outer, inner, client string) {
	t.Helper()
	skipUnlessSendKeysClient(t)
	outer, inner = startDockWithLastUsedA(t, bin)
	client = outerClient(t, outer)
	waitActivePane(t, outer, rightPaneIndex)
	sendKey(t, outer, client, switchKey)
	waitActivePane(t, outer, listPaneIndex)
	return outer, inner, client
}

func TestTmuxDockKeys(t *testing.T) {
	bin := buildBinary(t)

	t.Run("F12 で入力先が一覧から右に移り、もう一度打つと一覧に戻る", func(t *testing.T) {
		outer, _, client := startDockForKeys(t, bin)

		sendKey(t, outer, client, switchKey)
		waitActivePane(t, outer, rightPaneIndex)
		sendKey(t, outer, client, switchKey)
		waitActivePane(t, outer, listPaneIndex)
	})

	t.Run("右に入力先があるとき C-b c で普段のセッションの window が増え、外側の window は増えない", func(t *testing.T) {
		outer, inner, client := startDockForKeys(t, bin)
		outerBefore := windowCount(t, outer, outerWindowTarget)
		innerBefore := windowCount(t, inner, "=a:")
		sendKey(t, outer, client, switchKey)
		waitActivePane(t, outer, rightPaneIndex)

		sendKey(t, outer, client, "C-b")
		sendKey(t, outer, client, "c")

		want := innerBefore + 1
		got := innerBefore
		deadline := time.Now().Add(paneWaitLimit)
		for got != want && time.Now().Before(deadline) {
			time.Sleep(paneWaitStep)
			got = windowCount(t, inner, "=a:")
		}
		if got != want {
			t.Errorf("普段のセッション a の window 数 = %d, want %d（%v 以内）", got, want, paneWaitLimit)
		}
		if after := windowCount(t, outer, outerWindowTarget); after != outerBefore {
			t.Errorf("外側の dock の window 数 = %d, want %d", after, outerBefore)
		}
	})

	t.Run("F12 を繰り返し打っても普段のサーバの構成は変わらない", func(t *testing.T) {
		outer, inner, client := startDockForKeys(t, bin)
		before := innerLayout(t, inner)

		const presses = 4
		for i := 0; i < presses; i++ {
			sendKey(t, outer, client, switchKey)
		}
		want := listPaneIndex
		if presses%2 == 1 {
			want = rightPaneIndex
		}
		waitActivePane(t, outer, want)

		if after := innerLayout(t, inner); after != before {
			t.Errorf("%s を %d 回打った後の普段のサーバの構成 = %q, want %q", switchKey, presses, after, before)
		}
	})
}
