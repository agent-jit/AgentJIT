// internal/compile/tiered_test.go
package compile

import (
	"context"
	"testing"

	"github.com/agent-jit/agentjit/internal/trace"
)

func TestTieredCompiler_RoutesToCorrectBackend(t *testing.T) {
	det := &testBackend{name: "deterministic"}
	llm := &testBackend{name: "llm"}

	tc := NewTieredCompiler(det, llm, 0.6)

	// All-Bash pattern → high confidence → deterministic
	highConf := trace.Pattern{
		Steps: []trace.PatternStep{
			{ToolName: "Bash", Template: "git status"},
			{ToolName: "Bash", Template: "git diff"},
		},
		Frequency: 5,
	}
	// Mixed tools → low confidence → LLM
	lowConf := trace.Pattern{
		Steps: []trace.PatternStep{
			{ToolName: "Bash"}, {ToolName: "Read"}, {ToolName: "Edit"},
			{ToolName: "Bash"}, {ToolName: "Read"}, {ToolName: "Edit"},
			{ToolName: "Bash"}, {ToolName: "Read"}, {ToolName: "Edit"},
			{ToolName: "Bash"}, {ToolName: "Read"}, {ToolName: "Edit"},
			{ToolName: "Bash"},
		},
		Frequency: 3,
	}

	results, err := tc.Compile(context.Background(), []trace.Pattern{highConf, lowConf})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if len(det.compiled) != 1 {
		t.Errorf("deterministic backend got %d patterns, want 1", len(det.compiled))
	}
	if len(llm.compiled) != 1 {
		t.Errorf("LLM backend got %d patterns, want 1", len(llm.compiled))
	}
	if len(results) != 2 {
		t.Errorf("got %d total results, want 2", len(results))
	}
}

func TestTieredCompiler_AllDeterministic(t *testing.T) {
	det := &testBackend{name: "deterministic"}
	llm := &testBackend{name: "llm"}

	tc := NewTieredCompiler(det, llm, 0.6)

	patterns := []trace.Pattern{
		{Steps: []trace.PatternStep{{ToolName: "Bash", Template: "ls"}}, Frequency: 3},
	}

	_, err := tc.Compile(context.Background(), patterns)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if len(det.compiled) != 1 {
		t.Errorf("deterministic backend got %d, want 1", len(det.compiled))
	}
	if len(llm.compiled) != 0 {
		t.Errorf("LLM backend got %d, want 0", len(llm.compiled))
	}
}

func TestTieredCompiler_EmptyPatterns(t *testing.T) {
	det := &testBackend{name: "deterministic"}
	llm := &testBackend{name: "llm"}

	tc := NewTieredCompiler(det, llm, 0.6)

	results, err := tc.Compile(context.Background(), nil)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results for empty input, want 0", len(results))
	}
}
