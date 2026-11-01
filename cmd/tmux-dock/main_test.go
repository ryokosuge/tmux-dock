package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ryokosuge/tmux-dock/internal/rightpane"
)

const (
	paneWaitLimit = 5 * time.Second
	paneWaitStep  = 50 * time.Millisecond
	maxNameLen    = 16
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "tmux-dock")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// newSockets は使い捨ての harness・外側・普段のソケット名を返し、終了時に 3 つのサーバを止める。
func newSockets(t *testing.T) (harness, outer, inner string) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Fatalf("tmux が PATH に無い（integration テストは skip せず失敗させる）: %v", err)
	}
	base := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	if len(base) > maxNameLen {
		base = base[:maxNameLen]
	}
	id := fmt.Sprintf("%d-%x", os.Getpid(), time.Now().UnixNano()&0xffffffff)
	harness, outer, inner = "tdk-h-"+id+"-"+base, "tdk-o-"+id+"-"+base, "tdk-i-"+id+"-"+base
	t.Cleanup(func() {
		for _, s := range []string{harness, outer, inner} {
			_ = exec.Command("tmux", "-L", s, "kill-server").Run()
		}
	})
	return harness, outer, inner
}

func tmuxOut(t *testing.T, socket string, args ...string) string {
	t.Helper()
	full := append([]string{"-L", socket}, args...)
	out, err := exec.Command("tmux", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %s: %v (output=%s)", strings.Join(full, " "), err, out)
	}
	return string(out)
}

func innerLayout(t *testing.T, inner string) string {
	t.Helper()
	return tmuxOut(t, inner, "list-sessions", "-F", "#{session_id} #{session_name}") +
		tmuxOut(t, inner, "list-windows", "-a", "-F", "#{session_id} #{window_id} #{window_name}")
}

func TestTmuxDockCommand(t *testing.T) {
	bin := buildBinary(t)

	t.Run("端末で実行すると外側に左右 2 ペインのセッションができ attach され、外側を終えても普段のサーバの構成は同じ", func(t *testing.T) {
		harness, outer, inner := newSockets(t)
		for _, name := range []string{"a", "b"} {
			tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", name)
		}
		before := innerLayout(t, inner)

		startDock(t, bin, harness, outer, inner)

		// 外側のペインが 2 つになり、クライアントが 1 つ attach するまで待つ。
		var panes, clients string
		deadline := time.Now().Add(paneWaitLimit)
		for {
			panes = tmuxQuiet(outer, "list-panes", "-a", "-F", "#{pane_left}")
			clients = tmuxQuiet(outer, "list-clients", "-F", "#{client_session}")
			if len(strings.Fields(panes)) == 2 && len(strings.Fields(clients)) == 1 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("外側の状態が %v 以内に揃わない: list-panes=%q (want 2 ペイン), list-clients=%q (want 1 クライアント)",
					paneWaitLimit, panes, clients)
			}
			time.Sleep(paneWaitStep)
		}
		if f := strings.Fields(panes); f[0] == f[1] {
			t.Errorf("外側のペインの pane_left = %q, want 互いに異なる左右 2 ペイン", panes)
		}
		if want := "dock\n"; clients != want {
			t.Errorf("attach したセッション = %q, want %q", clients, want)
		}

		tmuxOut(t, outer, "kill-server")
		if after := innerLayout(t, inner); after != before {
			t.Errorf("外側を終えた後の普段のサーバの構成 = %q, want %q", after, before)
		}
	})

	t.Run("普段のサーバにセッション a, b があると外側の左ペインに a と b が出て、構成は起動前と同じ", func(t *testing.T) {
		harness, outer, inner := newSockets(t)
		for _, name := range []string{"a", "b"} {
			tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", name)
		}
		before := innerLayout(t, inner)

		startDock(t, bin, harness, outer, inner)

		left := waitLeftPane(t, outer, func(s string) bool { return hasLine(s, "a") && hasLine(s, "b") })
		if !hasLine(left, "a") || !hasLine(left, "b") {
			t.Errorf("左ペインの表示 = %q, want 行 a と b を含む", left)
		}
		if after := innerLayout(t, inner); after != before {
			t.Errorf("一覧を出した後の普段のサーバの構成 = %q, want %q", after, before)
		}
	})

	t.Run("普段のサーバが無いと左ペインに「セッションがありません」が出て、サーバは立たない", func(t *testing.T) {
		harness, outer, inner := newSockets(t)

		startDock(t, bin, harness, outer, inner)

		const want = "セッションがありません"
		left := waitLeftPane(t, outer, func(s string) bool { return strings.Contains(s, want) })
		if !strings.Contains(left, want) {
			t.Errorf("左ペインの表示 = %q, want %q を含む", left, want)
		}
		if out := tmuxQuiet(inner, "list-sessions", "-F", "#{session_name}"); out != "" {
			t.Errorf("普段のソケットの list-sessions = %q, want サーバが立たず 0 件", out)
		}
	})

	t.Run("tmux が PATH に無いとき原因を stderr に出して 0 以外で終わる", func(t *testing.T) {
		cmd := exec.Command(bin)
		cmd.Env = []string{"PATH=" + t.TempDir(), "HOME=" + t.TempDir()}
		var stderr strings.Builder
		cmd.Stderr = &stderr
		err := cmd.Run()
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
			t.Errorf("終了コード: err=%v, want 終了コード 1", err)
		}
		if want := "tmux が PATH に無い"; !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr = %q, want %q を含む", stderr.String(), want)
		}
	})
}

// startDock は harness サーバのペイン（pty）でバイナリを動かす。
// サーバは起動した tmux クライアントの環境を引き継ぐので、ここで環境を渡す。
func startDock(t *testing.T, bin, harness, outer, inner string) {
	t.Helper()
	cmd := exec.Command("tmux", "-L", harness, "-f", "/dev/null",
		"new-session", "-d", "-x", "160", "-y", "40", bin)
	cmd.Env = append(os.Environ(),
		"HOME="+t.TempDir(),
		"TMUX_DOCK_INNER_SOCKET="+inner,
		"TMUX_DOCK_OUTER_SOCKET="+outer,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("harness の起動: %v (output=%s)", err, out)
	}
}

// waitLeftPane は外側の左ペイン（=dock:.0）の表示が ok を満たすまで待ち、最後に読んだ表示を返す。
func waitLeftPane(t *testing.T, outer string, ok func(string) bool) string {
	t.Helper()
	var left string
	deadline := time.Now().Add(paneWaitLimit)
	for {
		left = tmuxQuiet(outer, "capture-pane", "-p", "-t", "=dock:.0")
		if ok(left) || time.Now().After(deadline) {
			return left
		}
		time.Sleep(paneWaitStep)
	}
}

func hasLine(s, line string) bool {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), ">")) == line {
			return true
		}
	}
	return false
}

// tmuxQuiet は socket の tmux に args を流し、stdout を返す。サーバが無い間の失敗は空文字で返す。
func tmuxQuiet(socket string, args ...string) string {
	out, _ := exec.Command("tmux", append([]string{"-L", socket}, args...)...).Output()
	return string(out)
}

const (
	rightPaneTarget = "=dock:.1"
	rightPaneMarker = "tdk-right-pane-marker"
)

// startDockWithLastUsedA は普段のサーバにセッション a, b を作って a を最後に使った状態にし、tmux-dock を起動して
// 右ペインのクライアントが a に繋がるまで待つ。a を先に作るので、普段のサーバの %1 は b のペインになる。
func startDockWithLastUsedA(t *testing.T, bin string) (outer, inner string) {
	t.Helper()
	harness, outer, inner := newSockets(t)
	tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", "a", "-n", "w")
	tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", "b", "-n", "w")
	time.Sleep(activityGap)

	// 準備の attach は別の harness サーバで動かす（harness は起動時の環境を引き継ぐので、tmux-dock 用とは分ける）。
	prep := harness + "-p"
	t.Cleanup(func() { _ = exec.Command("tmux", "-L", prep, "kill-server").Run() })
	tmuxHarness(t, prep, "new-session", "-d", "-x", "160", "-y", "40",
		"tmux -L "+inner+" attach-session -t '=a'; sleep 300")
	if got := waitClientSession(t, inner, "a"); got != "a" {
		t.Fatalf("準備の attach 先 = %q, want %q", got, "a")
	}
	tmuxOut(t, inner, "detach-client", "-s", "a")
	if got := waitClientSession(t, inner, ""); got != "" {
		t.Fatalf("準備の detach 後のクライアント = %q, want なし", got)
	}

	startDock(t, bin, harness, outer, inner)
	if got := waitClientSession(t, inner, "a"); got != "a" {
		t.Fatalf("右ペインのクライアントの attach 先 = %q, want %q（%v 以内）", got, "a", paneWaitLimit)
	}
	return outer, inner
}

func TestTmuxDockRightPane(t *testing.T) {
	bin := buildBinary(t)

	t.Run("最後に a を使った状態で起動すると外側の右ペインのクライアントが a に繋がる", func(t *testing.T) {
		_, inner := startDockWithLastUsedA(t, bin)
		if got := strings.TrimSpace(tmuxQuiet(inner, "list-clients", "-F", "#{client_session}")); got != "a" {
			t.Errorf("右ペインのクライアントのセッション = %q, want %q", got, "a")
		}
	})

	t.Run("外側の右ペインに送った文字列が普段のサーバのセッション a のペインに現れる", func(t *testing.T) {
		outer, inner := startDockWithLastUsedA(t, bin)

		tmuxOut(t, outer, "send-keys", "-t", rightPaneTarget, "echo "+rightPaneMarker, "Enter")

		var pane string
		deadline := time.Now().Add(paneWaitLimit)
		for !strings.Contains(pane, rightPaneMarker) && time.Now().Before(deadline) {
			pane = tmuxQuiet(inner, "capture-pane", "-p", "-t", "=a:")
			time.Sleep(paneWaitStep)
		}
		if !strings.Contains(pane, rightPaneMarker) {
			t.Errorf("セッション a のペインの内容 = %q, want %q を含む（%v 以内）", pane, rightPaneMarker, paneWaitLimit)
		}
	})

	t.Run("普段のサーバが無いと右ペインに待機の表示が出て、サーバは立たない", func(t *testing.T) {
		harness, outer, inner := newSockets(t)

		startDock(t, bin, harness, outer, inner)

		var pane string
		deadline := time.Now().Add(paneWaitLimit)
		for !showsWaitingMessage(pane) && time.Now().Before(deadline) {
			pane = tmuxQuiet(outer, "capture-pane", "-p", "-J", "-t", rightPaneTarget)
			time.Sleep(paneWaitStep)
		}
		if !showsWaitingMessage(pane) {
			t.Fatalf("右ペインの表示 = %q, want %q を含む（%v 以内）", pane, rightpane.WaitingMessage, paneWaitLimit)
		}
		time.Sleep(noServerGrace)
		if out := tmuxQuiet(inner, "list-sessions", "-F", "#{session_name}"); out != "" {
			t.Errorf("普段のソケットの list-sessions = %q, want サーバが立たず 0 件", out)
		}
	})

	t.Run("右ペインを a に繋いだ後に外側を kill-server しても普段のサーバの構成は起動前と同じ", func(t *testing.T) {
		harness, outer, inner := newSockets(t)
		tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", "a", "-n", "w")
		tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", "b", "-n", "w")
		before := innerLayout(t, inner)

		startDock(t, bin, harness, outer, inner)
		if got := waitClientSession(t, inner, "a"); got == "" {
			t.Fatalf("右ペインのクライアントが %v 以内に繋がらない", paneWaitLimit)
		}

		tmuxOut(t, outer, "kill-server")
		if got := waitClientSession(t, inner, ""); got != "" {
			t.Fatalf("外側を終えた後も普段のサーバにクライアント %q が残る", got)
		}
		if after := innerLayout(t, inner); after != before {
			t.Errorf("外側を終えた後の普段のサーバの構成 = %q, want %q", after, before)
		}
	})
}

// showsWaitingMessage は pane に待機の文言が出ているかを返す。
// 右ペインの幅で文言が折り返されると、tmux の版によっては折り返し位置に空白が入るため、空白を除いて比べる。
func showsWaitingMessage(pane string) bool {
	return strings.Contains(removeSpaces(pane), removeSpaces(rightpane.WaitingMessage))
}

func removeSpaces(s string) string {
	return strings.Join(strings.Fields(s), "")
}
