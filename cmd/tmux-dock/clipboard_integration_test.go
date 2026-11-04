package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTmuxDockClipboard(t *testing.T) {
	bin := buildBinary(t)

	t.Run("起動した外側の set-clipboard が on になっている", func(t *testing.T) {
		outer, _ := startDockWithLastUsedA(t, bin)

		if got := strings.TrimSpace(tmuxOut(t, outer, "show", "-s", "set-clipboard")); got != "set-clipboard on" {
			t.Errorf("外側の show -s set-clipboard = %q, want %q", got, "set-clipboard on")
		}
	})

	t.Run("右の内側のセッションでクリップボードへ書くコピーをすると外側の paste buffer に同じ文字が届き、普段のサーバの構成は起動前と同じ", func(t *testing.T) {
		outer, inner := startDockWithLastUsedA(t, bin)
		before := innerLayout(t, inner)
		marker := fmt.Sprintf("tdk-clip-%d", time.Now().UnixNano())
		if buffers := tmuxQuiet(outer, "list-buffers"); strings.Contains(buffers, marker) {
			t.Fatalf("コピー前の外側の list-buffers = %q, want %q を含まない", buffers, marker)
		}

		tmuxOut(t, inner, "set-buffer", "-w", marker)

		var got string
		deadline := time.Now().Add(paneWaitLimit)
		for got != marker && time.Now().Before(deadline) {
			got = tmuxQuiet(outer, "show-buffer")
			time.Sleep(paneWaitStep)
		}
		if got != marker {
			t.Errorf("外側の show-buffer = %q, want %q（%v 以内）", got, marker, paneWaitLimit)
		}
		if after := innerLayout(t, inner); after != before {
			t.Errorf("コピーの後の普段のサーバの構成 = %q, want %q", after, before)
		}
	})
}
