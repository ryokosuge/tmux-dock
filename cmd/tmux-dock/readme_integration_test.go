package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const readmePath = "../../README.md"

var tmuxBlockRe = regexp.MustCompile("(?s)```tmux\n(.*?)```")

func TestREADMEの普段のtmux設定を読み込むとset_clipboardがexternalでdetach_on_destroyがoffになる(t *testing.T) {
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("README を読めない path=%s: %v", readmePath, err)
	}
	var conf strings.Builder
	blocks := tmuxBlockRe.FindAllStringSubmatch(string(readme), -1)
	if len(blocks) == 0 {
		t.Fatalf("README に ```tmux のコードブロックが無い path=%s（普段の tmux 向けの設定を ```tmux で書くこと）", readmePath)
	}
	for _, b := range blocks {
		conf.WriteString(b[1])
	}
	confPath := filepath.Join(t.TempDir(), "readme.tmux.conf")
	if err := os.WriteFile(confPath, []byte(conf.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, inner := newSockets(t)
	tmuxOut(t, inner, "-f", "/dev/null", "new-session", "-d", "-s", "base")
	if out, err := exec.Command("tmux", "-L", inner, "source-file", confPath).CombinedOutput(); err != nil {
		t.Fatalf("README の tmux 設定の source-file が失敗 err=%v output=%s conf=\n%s", err, out, conf.String())
	}

	assertLine(t, tmuxOut(t, inner, "show", "-s", "set-clipboard"), "set-clipboard external")
	assertLine(t, tmuxOut(t, inner, "show", "-g", "detach-on-destroy"), "detach-on-destroy off")
}

func assertLine(t *testing.T, out, want string) {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == want {
			return
		}
	}
	t.Errorf("出力に行 %q が無い output=%q", want, out)
}
