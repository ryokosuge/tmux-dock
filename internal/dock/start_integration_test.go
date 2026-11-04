package dock

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ryokosuge/tmux-dock/internal/outerconf"
)

// maxNameLen はソケット名に入れる t.Name() 由来の部分の最大バイト数。
const maxNameLen = 16

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
	// 別プロセスの並行実行でも衝突しないよう pid と nanotime を混ぜる。
	// ソケットパスは macOS で 104 バイトが上限なので t.Name() 由来の部分は切り詰める。
	base := strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	if len(base) > maxNameLen {
		base = base[:maxNameLen]
	}
	id := fmt.Sprintf("%d-%x", os.Getpid(), time.Now().UnixNano()&0xffffffff)
	outer, inner = "tdk-o-"+id+"-"+base, "tdk-i-"+id+"-"+base
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

	p := NewPlan(Config{InnerSocket: inner, OuterSocket: outer, SessionName: SessionName}, "/dev/null", []string{"sleep", "300"}, []string{"sleep", "300"})
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

func TestEnsureSessionConcurrentIntegration(t *testing.T) {
	outer, inner := newSockets(t)
	tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", "a")
	before := innerLayout(t, inner)

	p := NewPlan(Config{InnerSocket: inner, OuterSocket: outer, SessionName: SessionName}, "/dev/null", []string{"sleep", "300"}, []string{"sleep", "300"})

	const starters = 2
	errs := make(chan error, starters)
	for range starters {
		go func() { errs <- EnsureSession(context.Background(), ExecRunner, p) }()
	}
	for i := 0; i < starters; i++ {
		if err := <-errs; err != nil {
			t.Errorf("並行の EnsureSession() = %v, want nil", err)
		}
	}

	t.Run("外側のセッションは 1 つで window 1・ペイン 2 のまま", func(t *testing.T) {
		if got, want := tmuxOut(t, outer, "list-sessions", "-F", "#{session_name}"), SessionName+"\n"; got != want {
			t.Errorf("list-sessions = %q, want %q", got, want)
		}
		if got := tmuxOut(t, outer, "list-windows", "-a", "-F", "#{window_id}"); len(strings.Fields(got)) != 1 {
			t.Errorf("list-windows = %q, want window 1 つ", got)
		}
		if got := tmuxOut(t, outer, "list-panes", "-a", "-F", "#{pane_id}"); len(strings.Fields(got)) != 2 {
			t.Errorf("list-panes = %q, want ペイン 2 つ", got)
		}
	})

	t.Run("普段のサーバの構成が実行前と同じ", func(t *testing.T) {
		if after := innerLayout(t, inner); after != before {
			t.Errorf("普段のサーバの構成 = %q, want %q", after, before)
		}
	})
}

func TestEnsureSessionReloadsConfIntegration(t *testing.T) {
	outer, inner := newSockets(t)
	tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", "a")
	before := innerLayout(t, inner)

	// -f /dev/null で立てた外側は同梱 conf を持たず、set-clipboard が既定の external のまま。
	tmuxOut(t, outer, "-f", "/dev/null", "new-session", "-d", "-s", SessionName, "sleep", "300")
	if got := tmuxOut(t, outer, "show", "-s", "-v", "set-clipboard"); strings.TrimSpace(got) != "external" {
		t.Fatalf("先に立てた外側の set-clipboard = %q, want external", got)
	}

	confPath, err := outerconf.WriteTo(t.TempDir())
	if err != nil {
		t.Fatalf("outerconf.WriteTo() = %v", err)
	}
	p := NewPlan(Config{InnerSocket: inner, OuterSocket: outer, SessionName: SessionName}, confPath, []string{"sleep", "300"}, []string{"sleep", "300"})
	if err := EnsureSession(context.Background(), ExecRunner, p); err != nil {
		t.Fatalf("EnsureSession() = %v", err)
	}

	t.Run("既に動いている外側の set-clipboard が on になる", func(t *testing.T) {
		if got := tmuxOut(t, outer, "show", "-s", "-v", "set-clipboard"); strings.TrimSpace(got) != "on" {
			t.Errorf("set-clipboard = %q, want on", got)
		}
	})

	t.Run("外側のセッションは 1 つのまま", func(t *testing.T) {
		if got, want := tmuxOut(t, outer, "list-sessions", "-F", "#{session_name}"), SessionName+"\n"; got != want {
			t.Errorf("list-sessions = %q, want %q", got, want)
		}
	})

	t.Run("普段のサーバの構成が実行前と同じ", func(t *testing.T) {
		if after := innerLayout(t, inner); after != before {
			t.Errorf("普段のサーバの構成 = %q, want %q", after, before)
		}
	})
}
