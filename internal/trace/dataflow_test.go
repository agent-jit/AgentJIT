package trace

import (
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/ingest"
)

func TestDetectDataFlowEdges_PodNameFlow(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []ingest.Event{
		{
			Timestamp:           t0,
			SessionID:           "s1",
			EventType:           "post_tool_use",
			ToolName:            "Bash",
			ToolInput:           map[string]interface{}{"command": "kubectl get pods -n prod"},
			ToolResponseSummary: "NAME READY STATUS\nmy-app-pod-7f8b9 1/1 Running",
		},
		{
			Timestamp: t0.Add(time.Minute),
			SessionID: "s1",
			EventType: "post_tool_use",
			ToolName:  "Bash",
			ToolInput: map[string]interface{}{"command": "kubectl logs my-app-pod-7f8b9 -n prod"},
		},
	}

	g := BuildGraph(events)
	DetectDataFlowEdges(g, events, 5)

	found := false
	for _, adj := range g.Edges {
		for _, edge := range adj {
			if edge.DataFlow {
				found = true
				if len(edge.FlowTokens) == 0 {
					t.Error("DataFlow edge should have FlowTokens")
				}
				hasToken := false
				for _, tok := range edge.FlowTokens {
					if tok == "my-app-pod-7f8b9" {
						hasToken = true
					}
				}
				if !hasToken {
					t.Errorf("expected 'my-app-pod-7f8b9' in FlowTokens, got %v", edge.FlowTokens)
				}
			}
		}
	}
	if !found {
		t.Error("expected a DataFlow edge between get pods and logs")
	}
}

func TestDetectDataFlowEdges_NoOverlap(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []ingest.Event{
		{
			Timestamp:           t0,
			SessionID:           "s1",
			EventType:           "post_tool_use",
			ToolName:            "Bash",
			ToolInput:           map[string]interface{}{"command": "git status"},
			ToolResponseSummary: "On branch main\nnothing to commit",
		},
		{
			Timestamp: t0.Add(time.Minute),
			SessionID: "s1",
			EventType: "post_tool_use",
			ToolName:  "Bash",
			ToolInput: map[string]interface{}{"command": "kubectl get pods -n prod"},
		},
	}

	g := BuildGraph(events)
	DetectDataFlowEdges(g, events, 5)

	for _, adj := range g.Edges {
		for _, edge := range adj {
			if edge.DataFlow {
				t.Error("no data-flow edge expected when output and input don't overlap")
			}
		}
	}
}

func TestDetectDataFlowEdges_StopwordsFiltered(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []ingest.Event{
		{
			Timestamp:           t0,
			SessionID:           "s1",
			EventType:           "post_tool_use",
			ToolName:            "Bash",
			ToolInput:           map[string]interface{}{"command": "ls"},
			ToolResponseSummary: "file test data true name",
		},
		{
			Timestamp: t0.Add(time.Minute),
			SessionID: "s1",
			EventType: "post_tool_use",
			ToolName:  "Bash",
			ToolInput: map[string]interface{}{"command": "echo file test data true name"},
		},
	}

	g := BuildGraph(events)
	DetectDataFlowEdges(g, events, 5)

	for _, adj := range g.Edges {
		for _, edge := range adj {
			if edge.DataFlow {
				t.Errorf("stopwords should be filtered, but got DataFlow edge with tokens: %v", edge.FlowTokens)
			}
		}
	}
}

func TestDetectDataFlowEdges_WindowRespected(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []ingest.Event{
		{Timestamp: t0, SessionID: "s1", EventType: "post_tool_use", ToolName: "Bash",
			ToolInput: map[string]interface{}{"command": "cmd0"}, ToolResponseSummary: "unique-token-xyz"},
		{Timestamp: t0.Add(1 * time.Minute), SessionID: "s1", EventType: "post_tool_use", ToolName: "Bash",
			ToolInput: map[string]interface{}{"command": "cmd1"}},
		{Timestamp: t0.Add(2 * time.Minute), SessionID: "s1", EventType: "post_tool_use", ToolName: "Bash",
			ToolInput: map[string]interface{}{"command": "cmd2"}},
		{Timestamp: t0.Add(3 * time.Minute), SessionID: "s1", EventType: "post_tool_use", ToolName: "Bash",
			ToolInput: map[string]interface{}{"command": "cmd3"}},
		{Timestamp: t0.Add(4 * time.Minute), SessionID: "s1", EventType: "post_tool_use", ToolName: "Bash",
			ToolInput: map[string]interface{}{"command": "cmd4"}},
		{Timestamp: t0.Add(5 * time.Minute), SessionID: "s1", EventType: "post_tool_use", ToolName: "Bash",
			ToolInput: map[string]interface{}{"command": "cmd5"}},
		{Timestamp: t0.Add(6 * time.Minute), SessionID: "s1", EventType: "post_tool_use", ToolName: "Bash",
			ToolInput: map[string]interface{}{"command": "echo unique-token-xyz"}},
	}

	g := BuildGraph(events)
	DetectDataFlowEdges(g, events, 5)

	for _, adj := range g.Edges {
		for _, edge := range adj {
			if edge.DataFlow {
				t.Errorf("window=5 should prevent flow detection across 6 steps, got flow tokens: %v", edge.FlowTokens)
			}
		}
	}
}

func TestTokenizeForFlow(t *testing.T) {
	tokens := tokenizeForFlow("NAME READY STATUS\nmy-app-pod-7f8b9 1/1 Running")
	found := false
	for tok := range tokens {
		if tok == "my-app-pod-7f8b9" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'my-app-pod-7f8b9' in tokenized output")
	}
}
