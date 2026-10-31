package checkgate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const goMod = "module example.com/fixture\n\ngo 1.27\n"

const cleanSource = "package fixture\n\nfunc Add(a, b int) int { return a + b }\n"

const unformattedSource = "package fixture\n\nfunc Add(a,b int) int {return a+b}\n"

const vetViolationSource = "package fixture\n\nimport \"fmt\"\n\nfunc Show() string { return fmt.Sprintf(\"%d\", \"text\") }\n"

const passingTest = "package fixture\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"want 3\")\n\t}\n}\n"

const failingTest = "package fixture\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 4 {\n\t\tt.Fatal(\"want 4\")\n\t}\n}\n"

// runCheck は files を置いた一時 module で、リポジトリの Makefile を指定して make check を流す。
func runCheck(t *testing.T, files map[string]string) (string, error) {
	t.Helper()
	makefile, err := filepath.Abs("../../Makefile")
	if err != nil {
		t.Fatalf("Makefile のパスを解決できない: %v", err)
	}
	dir := t.TempDir()
	files["go.mod"] = goMod
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("%s を書けない: %v", name, err)
		}
	}
	cmd := exec.Command("make", "-f", makefile, "check")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestMakeCheck(t *testing.T) {
	t.Run("整形済みで vet とテストが通ると終了コード 0 で終わる", func(t *testing.T) {
		out, err := runCheck(t, map[string]string{"a.go": cleanSource, "a_test.go": passingTest})
		if err != nil {
			t.Fatalf("make check が失敗した: %v\n%s", err, out)
		}
	})

	t.Run("gofmt で未整形のファイルがあるとファイル名を出して失敗し整形すると成功する", func(t *testing.T) {
		out, err := runCheck(t, map[string]string{"a.go": unformattedSource, "a_test.go": passingTest})
		if err == nil {
			t.Fatalf("未整形なのに make check が成功した\n%s", out)
		}
		if !strings.Contains(out, "a.go") {
			t.Errorf("出力に未整形のファイル名 a.go が無い:\n%s", out)
		}
		out, err = runCheck(t, map[string]string{"a.go": cleanSource, "a_test.go": passingTest})
		if err != nil {
			t.Fatalf("整形後も make check が失敗した: %v\n%s", err, out)
		}
	})

	t.Run("落ちるテストがあると失敗する", func(t *testing.T) {
		out, err := runCheck(t, map[string]string{"a.go": cleanSource, "a_test.go": failingTest})
		if err == nil {
			t.Fatalf("落ちるテストがあるのに make check が成功した\n%s", out)
		}
	})

	t.Run("go vet が指摘するコードがあると失敗する", func(t *testing.T) {
		out, err := runCheck(t, map[string]string{"a.go": vetViolationSource})
		if err == nil {
			t.Fatalf("vet 違反があるのに make check が成功した\n%s", out)
		}
	})
}
