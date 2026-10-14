package trace

import "strings"

type shellWord struct {
	start, end int
	value string
}

// literalShellWords accepts literal simple-command arguments only. Expansions,
// assignments, operators and comments require a shell AST and are deliberately
// declined instead of being silently converted into literal arguments.
func literalShellWords(s string) ([]shellWord, bool) {
	var words []shellWord
	for i := 0; i < len(s); {
		if s[i] == ' ' || s[i] == '\t' { i++; continue }
		start := i
		var value strings.Builder
		var quote byte
		for i < len(s) {
			c := s[i]
			if quote == 0 && (c == ' ' || c == '\t') { break }
			if c == '\n' || c == '\r' { return nil, false }
			if quote == '\'' {
				if c == '\'' { quote = 0 } else { value.WriteByte(c) }
				i++; continue
			}
			if c == '\\' {
				i++
				if i == len(s) || s[i] == '\n' { return nil, false }
				if quote == '"' && !strings.ContainsRune("$`\"\\", rune(s[i])) { value.WriteByte('\\') }
				value.WriteByte(s[i]); i++; continue
			}
			if c == '$' || c == '`' { return nil, false }
			if quote == '"' {
				if c == '"' { quote = 0 } else { value.WriteByte(c) }
				i++; continue
			}
			if c == '\'' || c == '"' { quote = c; i++; continue }
			if strings.ContainsRune("|&;()<>{}*?[]~#", rune(c)) { return nil, false }
			value.WriteByte(c); i++
		}
		if quote != 0 { return nil, false }
		words = append(words, shellWord{start: start, end: i, value: value.String()})
	}
	if len(words) == 0 || strings.Contains(words[0].value, "=") { return nil, false }
	switch words[0].value {
	case "if", "then", "else", "elif", "fi", "for", "while", "until", "do", "done", "case", "esac", "select", "function", "time", "!", "[[", "]]":
		return nil, false
	}
	return words, true
}
