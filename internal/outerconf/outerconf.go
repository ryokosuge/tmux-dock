// Package outerconf は外側の tmux に渡す同梱 conf を埋め込み、ディスクへ書き出す。
package outerconf

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed outer.tmux.conf
var content []byte

const (
	dirName  = "tmux-dock"
	fileName = "outer.tmux.conf"
)

// WriteTo は <baseDir>/tmux-dock/outer.tmux.conf に埋め込んだ conf を書き出し、そのパスを返す。
// 既にあれば上書きする。
func WriteTo(baseDir string) (string, error) {
	dir := filepath.Join(baseDir, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("outerconf: create directory %q: %w", dir, err)
	}
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", fmt.Errorf("outerconf: write %q: %w", path, err)
	}
	return path, nil
}
