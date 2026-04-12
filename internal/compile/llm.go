// internal/compile/llm.go
package compile

import (
	"context"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/trace"
)

// LLMBackendConfig holds configuration for the LLM-based compiler backend.
type LLMBackendConfig struct {
	Paths          config.Paths
	Cfg            config.Config
	PromptTemplate string
}

// LLMBackend delegates compilation to Claude via the existing RunCompile flow.
type LLMBackend struct {
	config LLMBackendConfig
}

// NewLLMBackend creates a new LLM compiler backend.
func NewLLMBackend(cfg LLMBackendConfig) *LLMBackend {
	return &LLMBackend{config: cfg}
}

// Name returns the backend identifier.
func (b *LLMBackend) Name() string { return "aj-llm" }

// Compile invokes the existing Claude-based compilation for the given patterns.
// In the LLM backend, patterns are informational — the LLM does its own
// pattern detection from logs. This method runs the full RunCompile flow.
func (b *LLMBackend) Compile(ctx context.Context, patterns []trace.Pattern) ([]SkillResult, error) {
	err := RunCompile(b.config.Paths, b.config.Cfg, b.config.PromptTemplate)
	if err != nil {
		return nil, err
	}
	return nil, nil // Results tracked via skill watcher
}
