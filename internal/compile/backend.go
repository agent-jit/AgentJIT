// internal/compile/backend.go
package compile

import (
	"context"

	"github.com/agent-jit/agentjit/internal/trace"
)

// SkillResult represents a skill produced by a compiler backend.
type SkillResult struct {
	Name      string // skill directory name
	Path      string // full path to the skill directory
	CreatedBy string // backend name that produced this skill
}

// CompilerBackend is the interface for pluggable compilation strategies.
type CompilerBackend interface {
	// Name returns the backend's identifier (e.g. "aj-deterministic", "aj-llm").
	Name() string

	// Compile takes parameterized patterns and produces skills.
	Compile(ctx context.Context, patterns []trace.Pattern) ([]SkillResult, error)
}
