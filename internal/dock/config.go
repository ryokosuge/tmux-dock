package dock

const (
	envInnerSocket = "TMUX_DOCK_INNER_SOCKET"
	envOuterSocket = "TMUX_DOCK_OUTER_SOCKET"

	defaultInnerSocket = "default"
	defaultOuterSocket = "tmux-dock"

	// SessionName は外側の tmux に作るセッションの名前。
	SessionName = "dock"
)

// Config は外側 / 普段の tmux サーバを指すソケット名とセッション名を持つ。
type Config struct {
	InnerSocket string
	OuterSocket string
	SessionName string
}

// ConfigFromEnv は getenv から値を読み、未設定または空のものは既定値にする。
func ConfigFromEnv(getenv func(string) string) Config {
	return Config{
		InnerSocket: envOrDefault(getenv, envInnerSocket, defaultInnerSocket),
		OuterSocket: envOrDefault(getenv, envOuterSocket, defaultOuterSocket),
		SessionName: SessionName,
	}
}

func envOrDefault(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}
