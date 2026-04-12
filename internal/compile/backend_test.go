// internal/compile/backend_test.go
package compile

import (
	"context"
	"testing"

	"github.com/agent-jit/agentjit/internal/trace"
)

// testBackend is a mock backend for testing the interface contract.
type testBackend struct {
	name     string
	compiled []trace.Pattern
}

func (b *testBackend) Name() string { return b.name }
func (b *testBackend) Compile(ctx context.Context, patterns []trace.Pattern) ([]SkillResult, error) {
	b.compiled = patterns
	var results []SkillResult
	for range patterns {
		results = append(results, SkillResult{
			Name:      "skill-from-" + b.name,
			Path:      "/tmp/skills/" + b.name,
			CreatedBy: b.name,
		})
	}
	return results, nil
}

func TestCompilerBackend_InterfaceCompliance(t *testing.T) {
	var backend CompilerBackend = &testBackend{name: "test"}

	if backend.Name() != "test" {
		t.Errorf("Name() = %q, want test", backend.Name())
	}

	results, err := backend.Compile(context.Background(), []trace.Pattern{
		{Steps: []trace.PatternStep{{ToolName: "Bash", Template: "ls"}}, Frequency: 3},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}
