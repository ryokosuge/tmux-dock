package main

import (
	"strings"
	"testing"
)

func TestTmuxDockListCreate(t *testing.T) {
	bin := buildBinary(t)

	t.Run("一覧で n と新しい名前 c を入れて Enter を押すと普段のサーバにセッション c が 1 つでき右のクライアントが c に移る", func(t *testing.T) {
		outer, inner := startDockWithLastUsedA(t, bin)
		createFromList(t, outer, "c")

		if got := waitClientSession(t, inner, "c"); got != "c" {
			t.Fatalf("作成後の右ペインのクライアントの attach 先 = %q, want %q（%v 以内）", got, "c", paneWaitLimit)
		}
		names := strings.Fields(tmuxOut(t, inner, "list-sessions", "-F", "#{session_name}"))
		if len(names) != 3 || strings.Count(strings.Join(names, " "), "c") != 1 {
			t.Errorf("作成後の普段のサーバのセッション = %v, want a, b, c の 3 件", names)
		}
	})

	t.Run("一覧で既にある名前 b を入れて Enter を押すとセッション数は変わらず右のクライアントが b に移る", func(t *testing.T) {
		outer, inner := startDockWithLastUsedA(t, bin)
		createFromList(t, outer, "b")

		if got := waitClientSession(t, inner, "b"); got != "b" {
			t.Fatalf("既存の名前を入れた後の右ペインのクライアントの attach 先 = %q, want %q（%v 以内）", got, "b", paneWaitLimit)
		}
		names := strings.Fields(tmuxOut(t, inner, "list-sessions", "-F", "#{session_name}"))
		if len(names) != 2 {
			t.Errorf("既存の名前を入れた後の普段のサーバのセッション = %v, want a, b の 2 件", names)
		}
	})

	t.Run("一覧から c を作った後の構成は作る前に c とその window の行が足されただけ", func(t *testing.T) {
		outer, inner := startDockWithLastUsedA(t, bin)
		before := innerLayout(t, inner)
		createFromList(t, outer, "c")

		if got := waitClientSession(t, inner, "c"); got != "c" {
			t.Fatalf("作成後の右ペインのクライアントの attach 先 = %q, want %q（%v 以内）", got, "c", paneWaitLimit)
		}
		after := innerLayout(t, inner)
		afterLines := strings.Split(strings.TrimSpace(after), "\n")
		inAfter := map[string]bool{}
		for _, l := range afterLines {
			inAfter[l] = true
		}
		beforeLines := strings.Split(strings.TrimSpace(before), "\n")
		inBefore := map[string]bool{}
		for _, l := range beforeLines {
			inBefore[l] = true
			if !inAfter[l] {
				t.Errorf("作成前の行 %q が作成後に無い: 作成前 = %q, 作成後 = %q", l, before, after)
			}
		}
		var added []string
		for _, l := range afterLines {
			if !inBefore[l] {
				added = append(added, l)
			}
		}
		if len(added) != 2 {
			t.Fatalf("作成後に足された行 = %q, want c のセッションの行と window の行の 2 行（作成後 = %q）", added, after)
		}
		sessionLine := ""
		for _, l := range added {
			if strings.HasSuffix(l, " c") {
				sessionLine = l
			}
		}
		if sessionLine == "" {
			t.Errorf("足された行 = %q, want c のセッションの行を含む", added)
		}
	})
}

// createFromList は一覧が出てから n → name → Enter を外側の左ペインに送る。
// 取りこぼさないよう、一覧の行と入力の行が出たのを待ってから送る。
func createFromList(t *testing.T, outer, name string) {
	t.Helper()
	const leftPane = "=dock:.0"
	waitLeftPane(t, outer, func(s string) bool { return hasLine(s, "a") && hasLine(s, "b") })
	tmuxOut(t, outer, "send-keys", "-t", leftPane, "n")
	left := waitLeftPane(t, outer, func(s string) bool { return strings.Contains(s, "新規セッション名:") })
	if !strings.Contains(left, "新規セッション名:") {
		t.Fatalf("n を押しても名前の入力の行が出ない: 左ペインの表示 = %q", left)
	}
	tmuxOut(t, outer, "send-keys", "-t", leftPane, "-l", name)
	tmuxOut(t, outer, "send-keys", "-t", leftPane, "Enter")
}
