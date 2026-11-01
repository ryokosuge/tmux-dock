package outerconf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteToWritesEmbeddedContent(t *testing.T) {
	base := t.TempDir()
	path, err := WriteTo(base)
	if err != nil {
		t.Fatalf("WriteTo(%q) error = %v", base, err)
	}
	if want := filepath.Join(base, "tmux-dock", "outer.tmux.conf"); path != want {
		t.Errorf("WriteTo path = %q, want %q", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) || len(got) == 0 {
		t.Errorf("written content = %q, want embedded %q", got, content)
	}
}

func TestWriteToTwiceSucceedsWithSameContent(t *testing.T) {
	base := t.TempDir()
	if _, err := WriteTo(base); err != nil {
		t.Fatalf("first WriteTo error = %v", err)
	}
	path, err := WriteTo(base)
	if err != nil {
		t.Fatalf("second WriteTo error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("content after second write = %q, want %q", got, content)
	}
}

func TestWriteToFailsWhenBaseDirIsFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if path, err := WriteTo(blocker); err == nil {
		t.Errorf("WriteTo(%q) = %q, nil error; want error", blocker, path)
	}
}

func TestContentHasKeyBindings(t *testing.T) {
	for _, line := range []string{
		"set -g prefix None",
		"set -g prefix2 None",
		"bind -n F12 select-pane -t :.+",
	} {
		if !strings.Contains(string(content), line+"\n") {
			t.Errorf("同梱 conf = %q, want 行 %q を含む", content, line)
		}
	}
}
