package trace

import (
	"crypto/sha256"
	"encoding/binary"
	"regexp"
	"sort"
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

// InputShape produces a structural fingerprint of a tool's input.
// For Bash: tokenizes the command and replaces variable tokens with {VAR}.
// For other tools: replaces values with type tags ({STRING}, {NUMBER}, etc.).
func InputShape(toolName string, input map[string]interface{}) map[string]string {
	shape := make(map[string]string, len(input))

	if toolName == "Bash" {
		if cmd, ok := input["command"].(string); ok {
			tokens := TokenizeBashCommand(cmd)
			var parts []string
			for _, tok := range tokens {
				if tok.Literal {
					parts = append(parts, tok.Value)
				} else {
					parts = append(parts, "{VAR}")
				}
			}
			shape["command"] = strings.Join(parts, " ")
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
