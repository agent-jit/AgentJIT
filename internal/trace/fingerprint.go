package trace

import (
	"regexp"
	"strconv"
	"strings"
)

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

	valueFlagSet = map[string]bool{
		"-n": true, "--namespace": true,
		"-f": true, "--file": true, "--filename": true,
		"-o": true, "--output": true,
		"-l": true, "--label": true, "--selector": true,
		"-p": true, "--port": true,
		"-c": true, "--container": true,
		"-i": true, "--image": true,
		"-t": true, "--tag": true,
		"--name": true, "--context": true, "--cluster": true,
		"--region": true, "--zone": true, "--project": true,
		"--profile": true, "--subscription": true,
		"--resource-group": true, "-g": true,
	}
)

// TokenizeBashCommand splits a Bash command into tokens and classifies each
// as literal (structural) or variable (parameterizable).
//
// NOTE: This uses strings.Fields for splitting, so shell quoting (e.g.,
// "foo bar"), escape sequences, and metacharacters (|, &&, >, etc.) are
// not handled. Each whitespace-delimited field is treated as one token.
func TokenizeBashCommand(cmd string) []Token {
	parts := strings.Fields(cmd)
	tokens := make([]Token, 0, len(parts))

	nextIsValue := false
	for _, part := range parts {
		if nextIsValue {
			tokens = append(tokens, Token{Value: part, Literal: false})
			nextIsValue = false
			continue
		}

		// Handle --flag=value syntax: split on '=' and check if the
		// prefix is a known value-bearing flag.
		if eqIdx := strings.Index(part, "="); eqIdx > 0 && strings.HasPrefix(part, "-") {
			flagName := part[:eqIdx]
			if valueFlagSet[flagName] {
				tokens = append(tokens, Token{Value: flagName, Literal: true})
				tokens = append(tokens, Token{Value: part[eqIdx+1:], Literal: false})
				continue
			}
		}

		if valueFlagSet[part] {
			tokens = append(tokens, Token{Value: part, Literal: true})
			nextIsValue = true
			continue
		}

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

func isVariableToken(s string) bool {
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
