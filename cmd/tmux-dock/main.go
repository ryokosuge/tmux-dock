// Command tmux-dock は外側の tmux を用意し、その画面に attach する。
//
// サブコマンド list は左ペイン用の一覧画面（--inner-socket <名前> --outer-socket <名前>）を出す。
// サブコマンド attach は右ペイン用で、普段のサーバにセッションができるまで待ち、attach-session を
// 子プロセスで動かす。attach が終わったら（終了コードによらず）待機に戻って繋ぎ直し続ける。
//
// 終了コード: 0 は SIGHUP / SIGINT / SIGTERM で止められた（子の attach も止めてから終わる）、
// 1 は起動の準備か attach の起動に失敗した（原因は stderr に出す）。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ryokosuge/tmux-dock/internal/dock"
	"github.com/ryokosuge/tmux-dock/internal/listui"
	"github.com/ryokosuge/tmux-dock/internal/outerconf"
	"github.com/ryokosuge/tmux-dock/internal/rightpane"
	"github.com/ryokosuge/tmux-dock/internal/sessionlist"
	"github.com/ryokosuge/tmux-dock/internal/switcher"
)

func main() {
	var err error
	switch {
	case len(os.Args) > 1 && os.Args[1] == "list":
		err = runList(os.Args[2:])
	case len(os.Args) > 1 && os.Args[1] == "attach":
		err = runAttach(os.Args[2:])
	default:
		err = run()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "tmux-dock: %v\n", err)
		os.Exit(1)
	}
}

// runList は左ペインで動く一覧の画面。--inner-socket / --outer-socket が無ければ
// TMUX_DOCK_INNER_SOCKET / TMUX_DOCK_OUTER_SOCKET（未設定なら既定）を使う。
func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	envCfg := dock.ConfigFromEnv(os.Getenv)
	inner := fs.String("inner-socket", envCfg.InnerSocket, "普段の tmux サーバのソケット名")
	outer := fs.String("outer-socket", envCfg.OuterSocket, "外側の tmux サーバのソケット名")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fetch := func(ctx context.Context) ([]sessionlist.Session, error) {
		return sessionlist.Fetch(ctx, dock.ExecRunner, *inner)
	}
	cfg := dock.Config{InnerSocket: *inner, OuterSocket: *outer, SessionName: dock.SessionName}
	switchTo := func(ctx context.Context, s sessionlist.Session) error {
		return switcher.Switch(ctx, dock.ExecRunner, cfg, s)
	}
	if _, err := tea.NewProgram(listui.New(fetch, switchTo, nil)).Run(); err != nil {
		return fmt.Errorf("一覧の画面を動かせなかった: %w", err)
	}
	return nil
}

// attachInterval は普段のサーバのセッションを取り直す間隔。
const attachInterval = time.Second

// runAttach は右ペインで動く。--inner-socket が無ければ TMUX_DOCK_INNER_SOCKET（未設定なら default）を使う。
// attach-session は子プロセスで動かし、止める合図（SIGHUP / SIGINT / SIGTERM）で子を止めてから終わる。
func runAttach(args []string) error {
	fs := flag.NewFlagSet("attach", flag.ContinueOnError)
	inner := fs.String("inner-socket", dock.ConfigFromEnv(os.Getenv).InnerSocket, "普段の tmux サーバのソケット名")
	if err := fs.Parse(args); err != nil {
		return err
	}
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux が PATH に無いため attach できない。tmux をインストールして PATH に通すこと: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	err = rightpane.Run(ctx, *inner, rightpane.Deps{
		Fetch: func(ctx context.Context) ([]sessionlist.Session, error) {
			return sessionlist.Fetch(ctx, dock.ExecRunner, *inner)
		},
		Attach: func(ctx context.Context, args, env []string) error {
			return runChild(ctx, tmuxPath, args, env)
		},
		Environ:  os.Environ,
		Out:      os.Stdout,
		Interval: attachInterval,
	})
	if errors.Is(err, context.Canceled) {
		return nil // 止める合図による終了は正常終了
	}
	return err
}

// attachWaitDelay は子へ SIGTERM を送ってから、終わるのを待つ上限。
const attachWaitDelay = 3 * time.Second

// runChild は tmux を右ペインの端末につないだ子プロセスとして起動し、終わるまで待つ。
// 起動できないときだけ error を返し、非 0 の終了は nil にする。ctx が終わると SIGTERM で止める
// （SIGKILL だと tmux クライアントが端末を raw のまま残すため）。
func runChild(ctx context.Context, tmuxPath string, args, env []string) error {
	cmd := exec.CommandContext(ctx, tmuxPath, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = env
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = attachWaitDelay
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil
	}
	return err
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
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("左右のペインで動かす自分自身の実行ファイルのパスを決められない: %w", err)
	}
	cfg := dock.ConfigFromEnv(os.Getenv)
	plan := dock.NewPlan(cfg, confPath,
		[]string{self, "list", "--inner-socket", cfg.InnerSocket, "--outer-socket", cfg.OuterSocket},
		[]string{self, "attach", "--inner-socket", cfg.InnerSocket})
	if err := dock.EnsureSession(context.Background(), dock.ExecRunner, plan); err != nil {
		return err
	}
	argv := append([]string{"tmux"}, plan.Attach...)
	if err := syscall.Exec(tmuxPath, argv, os.Environ()); err != nil {
		return fmt.Errorf("%s への置き換え（exec）に失敗した: %w", tmuxPath, err)
	}
	return nil
}
