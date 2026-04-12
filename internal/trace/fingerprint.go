package trace

import (
	"regexp"
	"strings"
)

// Token represents a single token from a parsed Bash command.
type Token struct {
	Value   string // the raw token text
	Literal bool   // true = keep as-is in template; false = parameterizable
}

var (
	pathPattern = regexp.MustCompile(`^[/\\]|[/\\].*[/\\]|.*\.[a-zA-Z]{1,5}$`)
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
	if ipPattern.MatchString(s) {
		return true
	}
	if hexPattern.MatchString(s) {
		return true
	}
	return false
}
