package dock

import "testing"

func fakeEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Config
	}{
		{
			name: "環境変数が未設定なら既定のソケット名になる",
			env:  map[string]string{},
			want: Config{InnerSocket: "default", OuterSocket: "tmux-dock", SessionName: "dock"},
		},
		{
			name: "環境変数が空文字なら既定のソケット名になる",
			env:  map[string]string{"TMUX_DOCK_INNER_SOCKET": "", "TMUX_DOCK_OUTER_SOCKET": ""},
			want: Config{InnerSocket: "default", OuterSocket: "tmux-dock", SessionName: "dock"},
		},
		{
			name: "環境変数を設定するとその値になる",
			env:  map[string]string{"TMUX_DOCK_INNER_SOCKET": "work", "TMUX_DOCK_OUTER_SOCKET": "outer-x"},
			want: Config{InnerSocket: "work", OuterSocket: "outer-x", SessionName: "dock"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConfigFromEnv(fakeEnv(tt.env))
			if got != tt.want {
				t.Errorf("ConfigFromEnv(%v) = %+v, want %+v", tt.env, got, tt.want)
			}
		})
	}
}
