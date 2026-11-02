package sessionlist

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// row は Format に従う 1 行を作る。
func row(id string, windows int, activity string, name string) string {
	return fmt.Sprintf("%s\t%d\t%s\t%s\n", id, windows, activity, name)
}

func parseLines(t *testing.T, out string) []string {
	t.Helper()
	ss, err := Parse([]byte(out))
	if err != nil {
		t.Fatalf("Parse(%q) = %v", out, err)
	}
	return Lines(ss)
}

func TestLines(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{"a, b, c の 3 行がこの順で並ぶ", row("$0", 1, "1 ", "a") + row("$1", 1, "1 ", "b") + row("$2", 1, "1 ", "c"), []string{" 1 a", " 1 b", " 1 c"}},
		{"空の出力は「セッションがありません」の 1 行", "", []string{"セッションがありません"}},
		{"空白を含む名前は 1 つの名前", row("$0", 1, "1 ", "my work") + row("$1", 1, "1 ", "b"), []string{" 1 my work", " 1 b"}},
		{": を含む名前は 1 つの名前", row("$0", 1, "1 ", "a:b") + row("$1", 1, "1 ", "c"), []string{" 1 a:b", " 1 c"}},
		{"末尾の改行が無くても同じ行数", strings.TrimSuffix(row("$0", 1, "1 ", "a")+row("$1", 1, "1 ", "b"), "\n"), []string{" 1 a", " 1 b"}},
		{"window 数は 2 桁の幅に右寄せで名前の前に出る", row("$0", 3, "1 ", "a") + row("$1", 12, "1 ", "b"), []string{" 3 a", "12 b"}},
		{"名前にタブや空白があっても window 数が崩れない", row("$0", 3, "1 ", "a  b\t9"), []string{" 3 a  b\t9"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLines(t, tt.out); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Lines(Parse(%q)) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []Session
	}{
		{"空の出力は 0 件", "", nil},
		{"id と window 数と activity の最大と名前を取る", row("$0", 2, "100 250 ", "work"), []Session{{ID: "$0", Name: "work", Windows: 2, Activity: 250}}},
		{"window が 1 つなら activity はその値", row("$3", 1, "42 ", "a"), []Session{{ID: "$3", Name: "a", Windows: 1, Activity: 42}}},
		{"activity が空なら 0", row("$3", 0, "", "a"), []Session{{ID: "$3", Name: "a"}}},
		{"空白と : を含む名前を 1 つの名前として解析する", row("$0", 1, "5 ", "my work") + row("$1", 1, "6 ", "a:b"),
			[]Session{{ID: "$0", Name: "my work", Windows: 1, Activity: 5}, {ID: "$1", Name: "a:b", Windows: 1, Activity: 6}}},
		{"名前に空白・タブ・# , : が入っても window 数と activity が崩れない", row("$12", 3, "7 9 8 ", "a  b\t3 9999\t#{x},y:z"),
			[]Session{{ID: "$12", Name: "a  b\t3 9999\t#{x},y:z", Windows: 3, Activity: 9}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.out))
			if err != nil {
				t.Fatalf("Parse(%q) = %v", tt.out, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.out, got, tt.want)
			}
		})
	}

	errTests := []struct {
		name    string
		out     string
		bad     string
		want    []Session
		wantErr []string
	}{
		{"4 項目に割れない行は他の行を残して行番号付きのエラー", row("$0", 1, "5 ", "a") + "$1\t1\tb\n" + row("$2", 1, "6 ", "c"), "",
			[]Session{{ID: "$0", Name: "a", Windows: 1, Activity: 5}, {ID: "$2", Name: "c", Windows: 1, Activity: 6}}, []string{"2 行目", "4 項目"}},
		{"window 数が数値でない行はエラー", row("$0", 1, "5 ", "a") + "$1\tx\t5 \tb\n", "",
			[]Session{{ID: "$0", Name: "a", Windows: 1, Activity: 5}}, []string{"2 行目", "window 数", `"x"`}},
		{"activity が数値でない行はエラー", "$1\t1\t5 z \tb\n" + row("$2", 1, "6 ", "c"), "",
			[]Session{{ID: "$2", Name: "c", Windows: 1, Activity: 6}}, []string{"1 行目", "window_activity", `"z"`}},
	}
	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.out))
			if err == nil {
				t.Fatalf("Parse(%q) err = nil, want エラー", tt.out)
			}
			for _, w := range tt.wantErr {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("Parse(%q) err = %v, want %q を含む", tt.out, err, w)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.out, got, tt.want)
			}
		})
	}
}
