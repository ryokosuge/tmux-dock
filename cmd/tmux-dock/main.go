// Command tmux-dock は外側の tmux を用意し、その画面に attach する。
//
// 終了コード: 0 は attach 先の tmux に置き換わった（このプロセスは戻らない）、
// 1 は起動の準備か attach に失敗した（原因は stderr に出す）。
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/ryokosuge/tmux-dock/internal/dock"
	"github.com/ryokosuge/tmux-dock/internal/outerconf"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "tmux-dock: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux が PATH に無いため起動できない。tmux をインストールして PATH に通すこと: %w", err)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("同梱 conf の書き出し先（ユーザーのキャッシュ dir）を決められない: %w", err)
	}
	confPath, err := outerconf.WriteTo(cacheDir)
	if err != nil {
		return err
	}
	plan := dock.NewPlan(dock.ConfigFromEnv(os.Getenv), confPath)
	if err := dock.EnsureSession(context.Background(), dock.ExecRunner, plan); err != nil {
		return err
	}
	argv := append([]string{"tmux"}, plan.Attach...)
	if err := syscall.Exec(tmuxPath, argv, os.Environ()); err != nil {
		return fmt.Errorf("%s への置き換え（exec）に失敗した: %w", tmuxPath, err)
	}
	return nil
}
