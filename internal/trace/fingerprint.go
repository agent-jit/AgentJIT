package trace

import (
	"crypto/sha256"
	"encoding/binary"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// flagGroup represents a CLI flag and optional value for normalized sorting.
type flagGroup struct {
	repr    string // display form, e.g. "--org {VAR}" or "-v"
	sortKey string // the flag itself for sorting
}

// Token represents a single token from a parsed Bash command.
type Token struct {
	Value   string // the raw token text
	Literal bool   // true = keep as-is in template; false = parameterizable
}

var (
	// pathPattern matches:
	//   - Absolute paths starting with / or \
	//   - Paths containing at least two separators (e.g. foo/bar/baz)
	//   - Relative paths starting with ./ or ../
	//   - Paths with a separator and a file extension (e.g. src/main.go)
	pathPattern = regexp.MustCompile(`^[/\\]|[/\\].*[/\\]|^\.\.?[/\\]|.*[/\\].*\.[a-zA-Z]{1,5}$`)
	uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	ipPattern   = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)
	hexPattern  = regexp.MustCompile(`^[0-9a-fA-F]{12,}$`)

	// Preprocessing patterns for stripping shell noise.
	cdPrefixPattern      = regexp.MustCompile(`^\s*cd\s+\S+\s*&&\s*`)
	envVarPrefixPattern  = regexp.MustCompile(`^\s*[A-Za-z_][A-Za-z0-9_]*=("([^"]*)"|\S+)\s+`)
	timeoutPrefixPattern = regexp.MustCompile(`^\s*timeout\s+\d+\s+`)
	redirectionPattern   = regexp.MustCompile(`\s+[12]?>&?[12]?(/dev/null)?`)
)

// shellFields splits a string on whitespace but respects double and single
// quotes, keeping quoted segments as a single field. Quote characters are
// stripped from the resulting tokens.
func shellFields(s string) []string {
	var fields []string
	var cur strings.Builder
	inDouble := false
	inSingle := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case (c == ' ' || c == '\t' || c == '\n' || c == '\r') && !inDouble && !inSingle:
			if cur.Len() > 0 {
				fields = append(fields, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		fields = append(fields, cur.String())
	}
	return fields
}

// extractFirstPipeSegment returns the portion of cmd before the first
// unquoted pipe character. It tracks quote state and $() nesting depth
// so pipes inside quoted strings or command substitutions are preserved.
func extractFirstPipeSegment(cmd string) string {
	depth := 0 // $() nesting depth
	inDouble := false
	inSingle := false

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case inSingle:
			// Inside single quotes, everything is literal.
			continue
		case c == '"':
			inDouble = !inDouble
		case c == '$' && i+1 < len(cmd) && cmd[i+1] == '(':
			depth++
			i++ // skip '('
		case c == ')' && depth > 0:
			depth--
		case c == '|' && !inDouble && depth == 0:
			return strings.TrimSpace(cmd[:i])
		}
	}
	return cmd
}

// PreprocessBashCommand strips shell noise from a command before fingerprinting.
// It removes cd prefixes, env var prefixes, timeout prefixes, pipe chains (keeping
// only the first command), and output redirections. The goal is to isolate the core
// tool invocation for structural matching.
func PreprocessBashCommand(cmd string) string {
	// 1. Extract first command in pipe chain.
	cmd = extractFirstPipeSegment(cmd)

	// 2. Strip cd <path> && prefixes (loop for chained cd's).
	for {
		loc := cdPrefixPattern.FindStringIndex(cmd)
		if loc == nil {
			break
		}
		cmd = cmd[loc[1]:]
	}

	// 3. Strip VAR=value env var prefixes (loop for multiple vars).
	for {
		loc := envVarPrefixPattern.FindStringIndex(cmd)
		if loc == nil {
			break
		}
		cmd = cmd[loc[1]:]
	}

	// 4. Strip timeout N prefix.
	if loc := timeoutPrefixPattern.FindStringIndex(cmd); loc != nil {
		cmd = cmd[loc[1]:]
	}

	// 5. Strip output redirections (2>&1, 2>/dev/null, etc.).
	cmd = redirectionPattern.ReplaceAllString(cmd, "")

	return strings.TrimSpace(cmd)
}

// TokenizeBashCommand splits a Bash command into tokens and classifies each
// as literal (structural) or variable (parameterizable).
//
// Flag handling: any --long-flag is kept literal. If the next token is not
// itself a flag, it's treated as the flag's value (variable). This covers
// most CLI tools without needing a per-tool allow-list. Short flags (-x)
// are treated as literal boolean flags; known short value flags (-n, -f, etc.)
// consume the next token as a value.
//
// Splitting uses shellFields which respects double/single quoting, so
// "foo bar" stays as one token.
func TokenizeBashCommand(cmd string) []Token {
	parts := shellFields(cmd)
	tokens := make([]Token, 0, len(parts))

	// Short flags known to take a value argument.
	shortValueFlags := map[string]bool{
		"-n": true, "-f": true, "-o": true, "-l": true,
		"-p": true, "-c": true, "-i": true, "-t": true, "-g": true,
		"-m": true, "-e": true, "-C": true,
	}

	nextIsValue := false
	for _, part := range parts {
		if nextIsValue {
			// If the "value" starts with -, it's actually another flag,
			// meaning the previous flag was boolean. Re-process this token.
			if strings.HasPrefix(part, "-") {
				nextIsValue = false
			} else {
				tokens = append(tokens, Token{Value: part, Literal: false})
				nextIsValue = false
				continue
			}
		}

		// Handle --flag=value syntax: always treat as flag + variable value.
		if eqIdx := strings.Index(part, "="); eqIdx > 0 && strings.HasPrefix(part, "--") {
			flagName := part[:eqIdx]
			tokens = append(tokens, Token{Value: flagName, Literal: true})
			tokens = append(tokens, Token{Value: part[eqIdx+1:], Literal: false})
			continue
		}

		// Long flags (--foo): keep flag literal, next non-flag token is its value.
		if strings.HasPrefix(part, "--") {
			tokens = append(tokens, Token{Value: part, Literal: true})
			nextIsValue = true
			continue
		}

		// Short value flags (-n, -f, etc.): next token is the value.
		if shortValueFlags[part] {
			tokens = append(tokens, Token{Value: part, Literal: true})
			nextIsValue = true
			continue
		}

		// Other short flags (-v, --): literal, no value consumed.
		if strings.HasPrefix(part, "-") {
			tokens = append(tokens, Token{Value: part, Literal: true})
			continue
		}

		if isVariableToken(part) {
			tokens = append(tokens, Token{Value: part, Literal: false})
			continue
		}

		tokens = append(tokens, Token{Value: part, Literal: true})
	}

	return tokens
}

// normalizedBashShape produces a flag-order-invariant shape string from tokens.
// It extracts the subcommand prefix (leading non-flag literals), collects flag
// groups (flag + optional value), sorts them alphabetically, and appends a single
// {VAR} for any positional variable arguments.
func normalizedBashShape(tokens []Token) string {
	if len(tokens) == 0 {
		return ""
	}

	var subcommand []string
	var flags []flagGroup
	positionalVars := 0
	sawHeredoc := false

	i := 0
	// Extract subcommand prefix: leading literal non-flag tokens.
	for i < len(tokens) && tokens[i].Literal && !strings.HasPrefix(tokens[i].Value, "-") && !isHeredocMarker(tokens[i].Value) {
		subcommand = append(subcommand, tokens[i].Value)
		i++
	}

	// Process remaining tokens: flags, positional args, heredocs.
	for i < len(tokens) {
		tok := tokens[i]

		if tok.Literal && isHeredocMarker(tok.Value) {
			sawHeredoc = true
			break
		}

		if tok.Literal && strings.HasPrefix(tok.Value, "-") {
			// Flag token. Check if next token is its value (non-literal).
			if i+1 < len(tokens) && !tokens[i+1].Literal {
				flags = append(flags, flagGroup{
					repr:    tok.Value + " {VAR}",
					sortKey: tok.Value,
				})
				i += 2
			} else {
				flags = append(flags, flagGroup{
					repr:    tok.Value,
					sortKey: tok.Value,
				})
				i++
			}
			continue
		}

		if !tok.Literal {
			// Positional variable argument (not consumed by a flag).
			positionalVars++
			i++
			continue
		}

		// Literal non-flag token after subcommand (e.g., late subcommand word).
		subcommand = append(subcommand, tok.Value)
		i++
	}

	// Sort flag groups alphabetically by flag name.
	sort.Slice(flags, func(i, j int) bool {
		return flags[i].sortKey < flags[j].sortKey
	})

	// Reassemble.
	var parts []string
	parts = append(parts, subcommand...)
	for _, fg := range flags {
		parts = append(parts, fg.repr)
	}
	if positionalVars > 0 {
		parts = append(parts, "{VAR}")
	}
	if sawHeredoc {
		parts = append(parts, "{HEREDOC}")
	}

	return strings.Join(parts, " ")
}

// isHeredocMarker returns true for shell heredoc markers like <<'EOF', <<EOF, <<"EOF".
func isHeredocMarker(s string) bool {
	return strings.HasPrefix(s, "<<")
}

func isVariableToken(s string) bool {
	// Quote fragments from unbalanced shell quoting are always variable.
	if len(s) > 0 && (s[0] == '"' || s[0] == '\'' || s[len(s)-1] == '"' || s[len(s)-1] == '\'') {
		return true
	}
	if pathPattern.MatchString(s) {
		return true
	}
	if uuidPattern.MatchString(s) {
		return true
	}
	if ipPattern.MatchString(s) && isValidIPv4(s) {
		return true
	}
	if hexPattern.MatchString(s) {
		return true
	}
	return false
}

// isValidIPv4 checks that every octet in a dotted-quad string is <= 255.
func isValidIPv4(s string) bool {
	octets := strings.Split(s, ".")
	if len(octets) != 4 {
		return false
	}
	for _, o := range octets {
		v, err := strconv.Atoi(o)
		if err != nil || v < 0 || v > 255 {
			return false
		}
	}
	return true
}

// InputShape produces a structural fingerprint of a tool's input.
// For Bash: tokenizes the command and replaces variable tokens with {VAR}.
// For other tools: replaces values with type tags ({STRING}, {NUMBER}, etc.).
func InputShape(toolName string, input map[string]interface{}) map[string]string {
	shape := make(map[string]string, len(input))

	if toolName == "Bash" {
		if cmd, ok := input["command"].(string); ok {
			cleaned := PreprocessBashCommand(cmd)
			tokens := TokenizeBashCommand(cleaned)
			shape["command"] = normalizedBashShape(tokens)
		}
		for k, v := range input {
			if k == "command" {
				continue
			}
			shape[k] = typeTag(v)
		}
		return shape
	}

	for k, v := range input {
		shape[k] = typeTag(v)
	}
	return shape
}

func typeTag(v interface{}) string {
	switch v.(type) {
	case string:
		return "{STRING}"
	case float64, int, int64:
		return "{NUMBER}"
	case bool:
		return "{BOOL}"
	case nil:
		return "{NULL}"
	default:
		return "{OBJECT}"
	}
}

// NodeID computes a stable hash for a (toolName, inputShape) pair.
func NodeID(toolName string, shape map[string]string) uint64 {
	h := sha256.New()
	h.Write([]byte(toolName))
	h.Write([]byte{0})

	keys := make([]string, 0, len(shape))
	for k := range shape {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(shape[k]))
		h.Write([]byte{0})
	}

	sum := h.Sum(nil)
	return binary.BigEndian.Uint64(sum[:8])
}
