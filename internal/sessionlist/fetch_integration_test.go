package sessionlist

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ryokosuge/tmux-dock/internal/dock"
)

// newInnerSocket は使い捨ての普段のソケット名を返し、終了時にそのサーバを止める。
func newInnerSocket(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Fatalf("tmux が PATH に無い（integration テストは skip せず失敗させる）: %v", err)
	}
	// 別プロセスの並行実行でも衝突しないよう pid と nanotime を混ぜる。
	id := fmt.Sprintf("%d-%x", os.Getpid(), time.Now().UnixNano()&0xffffffff)
	inner := "tdk-sl-" + id
	t.Cleanup(func() { _ = exec.Command("tmux", "-L", inner, "kill-server").Run() })
	return inner
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

func TestFetchIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("セッション a, b があるとき a と b の 2 件が返り構成は取得前と同じ", func(t *testing.T) {
		inner := newInnerSocket(t)
		for _, name := range []string{"a", "b"} {
			tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", name)
		}
		before := innerLayout(t, inner)

		got, err := Fetch(ctx, dock.ExecRunner, inner)
		if err != nil {
			t.Fatalf("Fetch() = %v", err)
		}
		names := map[string]bool{}
		for _, s := range got {
			names[s.Name] = true
		}
		if want := map[string]bool{"a": true, "b": true}; len(got) != 2 || !reflect.DeepEqual(names, want) {
			t.Errorf("Fetch() = %v, want a と b の 2 件", got)
		}
		if after := innerLayout(t, inner); after != before {
			t.Errorf("取得後の構成 = %q, want %q", after, before)
		}
	})

	t.Run("サーバが無いとき 0 件でエラーにならずサーバも立たない", func(t *testing.T) {
		inner := newInnerSocket(t)

		got, err := Fetch(ctx, dock.ExecRunner, inner)
		if err != nil || len(got) != 0 {
			t.Fatalf("Fetch() = %v, %v, want 0 件と nil", got, err)
		}
		if err := exec.Command("tmux", "-L", inner, "has-session").Run(); err == nil {
			t.Error("取得の後にサーバが立っている（has-session が成功した）, want サーバ無し")
		}
	})
}
