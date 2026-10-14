package trace

import "testing"

func TestDiffShellPreservesSemantics(t *testing.T) {
	for _, tc := range []struct { a, b, want string; count int }{
		{`cat -- "./a.txt"`, `cat -- "./b.txt"`, `cat -- "$FILE_PATH"`, 1},
		{`printf '%s\n' 'a b'`, `printf '%s\n' 'c d'`, `printf '%s\n' "$ARG_1"`, 1},
		{`cp ./a ./b`, `cp ./c ./d`, `cp "$FILE_PATH" "$FILE_PATH_2"`, 2},
		{`echo "a b" | cat`, `echo "a b" | cat`, `echo "a b" | cat`, 0},
		{`cat ./a | wc -l`, `cat ./b | wc -l`, "", 0},
		{`cat "$HOME/a"`, `cat "$HOME/b"`, "", 0},
		{`cat ./a`, `cat ./b ./c`, "", 0},
		{`cat ./a`, `rm ./a`, "", 0},
	} {
		got, ps := diffBashInstances([]map[string]interface{}{{"command": tc.a}, {"command": tc.b}})
		if got != tc.want || len(ps) != tc.count { t.Errorf("%q / %q: got %q (%d params), want %q (%d)", tc.a, tc.b, got, len(ps), tc.want, tc.count) }
	}
}
