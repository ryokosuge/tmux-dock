package dock

// Plan は外側の tmux を起動する 3 つの tmux 呼び出しの引数。
// 先頭の "tmux" は含まない。
type Plan struct {
	HasSession []string
	Create     []string
	Attach     []string
}

// NewPlan は外側のソケットだけを指す起動手順を組み立てる。tmux は呼ばない。
// セッション名は =name で完全一致させ、前方一致・glob で別のセッションに当たらないようにする。
// leftCommand / rightCommand は左右のペインで動かすコマンドで、1 要素ずつ別の引数として渡す（tmux がシェルを通さずそのまま実行する）。
func NewPlan(cfg Config, confPath string, leftCommand, rightCommand []string) Plan {
	outer := []string{"-L", cfg.OuterSocket}
	target := "=" + cfg.SessionName

	return Plan{
		HasSession: join(outer, "has-session", "-t", target),
		Create: join(join(outer, "-f", confPath,
			"new-session", "-d", "-s", cfg.SessionName), append(append(append([]string{}, leftCommand...),
			";",
			"split-window", "-h", "-t", target+":"), rightCommand...)...),
		Attach: join(outer, "attach-session", "-t", target),
	}
}

func join(prefix []string, rest ...string) []string {
	return append(append([]string{}, prefix...), rest...)
}
