package sessionlist

import (
	"reflect"
	"testing"
)

func parseLines(t *testing.T, out string) []string {
	t.Helper()
	return Lines(Parse([]byte(out)))
}

func TestLines(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{"a, b, c の 3 行がこの順で並ぶ", "$0 a\n$1 b\n$2 c\n", []string{"a", "b", "c"}},
		{"空の出力は「セッションがありません」の 1 行", "", []string{"セッションがありません"}},
		{"空白を含む名前は 1 つの名前", "$0 my work\n$1 b\n", []string{"my work", "b"}},
		{": を含む名前は 1 つの名前", "$0 a:b\n$1 c\n", []string{"a:b", "c"}},
		{"末尾の改行が無くても同じ行数", "$0 a\n$1 b", []string{"a", "b"}},
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
		{"空白と : を含む名前を 1 つの名前として解析する", "$0 my work\n$1 a:b\n", []Session{{ID: "$0", Name: "my work"}, {ID: "$1", Name: "a:b"}}},
		{"名前に空白が複数あっても先頭の id だけで分ける", "$12 a  b $3\n", []Session{{ID: "$12", Name: "a  b $3"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse([]byte(tt.out)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.out, got, tt.want)
			}
		})
	}
}
