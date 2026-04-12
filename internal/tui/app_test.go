package tui

import (
	"testing"

	"github.com/agent-jit/agentjit/internal/trace"
)

func TestNewModel_Init(t *testing.T) {
	paths := []AnnotatedPath{
		{
			Path:       trace.HotPath{NodeIDs: []uint64{1, 2}, Frequency: 5, SessionIDs: []string{"s1", "s2", "s3", "s4", "s5"}},
			Labels:     []string{"kubectl get pods", "kubectl logs"},
			Confidence: 0.9,
			Savings:    800,
		},
	}

	m := NewModel(paths, nil)

	if len(m.paths) != 1 {
		t.Errorf("got %d paths, want 1", len(m.paths))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestNewModel_Empty(t *testing.T) {
	m := NewModel(nil, nil)
	if len(m.paths) != 0 {
		t.Errorf("got %d paths, want 0", len(m.paths))
	}
}
