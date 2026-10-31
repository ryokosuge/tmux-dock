package dock

import (
	"context"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// tmuxOut は socket の tmux サーバに args を流し、stdout を返す。
func tmuxOut(t *testing.T, socket string, args ...string) string {
	t.Helper()
	full := append([]string{"-L", socket}, args...)
	out, err := exec.Command("tmux", full...).Output()
	if err != nil {
		t.Fatalf("tmux %s: %v (stderr=%s)", strings.Join(full, " "), err, stderrOf(err))
	}
	return string(out)
}

func stderrOf(err error) string {
	if ee, ok := err.(*exec.ExitError); ok {
		return string(ee.Stderr)
	}
	return ""
}

// newSockets は使い捨ての外側・普段のソケット名を返し、終了時に両方のサーバを止める。
func newSockets(t *testing.T) (outer, inner string) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Fatalf("tmux が PATH に無い（integration テストは skip せず失敗させる）: %v", err)
	}
	base := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	outer, inner = "tdk-o-"+base, "tdk-i-"+base
	t.Cleanup(func() {
		for _, s := range []string{outer, inner} {
			_ = exec.Command("tmux", "-L", s, "kill-server").Run()
		}
	})
	return outer, inner
}

func innerLayout(t *testing.T, inner string) string {
	t.Helper()
	return tmuxOut(t, inner, "list-sessions", "-F", "#{session_id} #{session_name}") +
		tmuxOut(t, inner, "list-windows", "-a", "-F", "#{session_id} #{window_id} #{window_name}")
}

func TestEnsureSessionIntegration(t *testing.T) {
	outer, inner := newSockets(t)
	for _, name := range []string{"a", "b"} {
		tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", name)
	}
	before := innerLayout(t, inner)

	p := NewPlan(Config{InnerSocket: inner, OuterSocket: outer, SessionName: SessionName}, "/dev/null")
	ctx := context.Background()

	if err := EnsureSession(ctx, ExecRunner, p); err != nil {
		t.Fatalf("EnsureSession() 1 回目 = %v", err)
	}
	snapshot := func() []string {
		return []string{
			tmuxOut(t, outer, "list-sessions", "-F", "#{session_name}"),
			tmuxOut(t, outer, "list-windows", "-a", "-F", "#{window_id}"),
			tmuxOut(t, outer, "list-panes", "-a", "-F", "#{pane_id} #{pane_left}"),
		}
	}
	first := snapshot()

	t.Run("外側に左右 2 ペインの window を 1 つ持つセッションが 1 つできる", func(t *testing.T) {
		if want := SessionName + "\n"; first[0] != want {
			t.Errorf("list-sessions = %q, want %q", first[0], want)
		}
		if n := len(strings.Fields(first[1])); n != 1 {
			t.Errorf("window 数 = %d, want 1 (list-windows=%q)", n, first[1])
		}
		lefts := map[string]bool{}
		panes := strings.Split(strings.TrimSpace(first[2]), "\n")
		for _, line := range panes {
			lefts[strings.Fields(line)[1]] = true
		}
		if len(panes) != 2 || len(lefts) != 2 {
			t.Errorf("ペイン = %q, want 2 つで pane_left が互いに異なる", first[2])
		}
	})

	t.Run("既にあるときもう一度実行してもセッション・window・ペインが増えない", func(t *testing.T) {
		if err := EnsureSession(ctx, ExecRunner, p); err != nil {
			t.Fatalf("EnsureSession() 2 回目 = %v", err)
		}
		if second := snapshot(); !reflect.DeepEqual(second, first) {
			t.Errorf("2 回目の後の構成 = %q, want %q", second, first)
		}
	})

	t.Run("外側を終えても普段のサーバの構成が実行前と同じ", func(t *testing.T) {
		tmuxOut(t, outer, "kill-server")
		if after := innerLayout(t, inner); after != before {
			t.Errorf("普段のサーバの構成 = %q, want %q", after, before)
		}
	})
}
