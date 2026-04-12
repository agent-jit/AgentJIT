package trace

import (
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/ingest"
)

func makeEvent(sessionID, toolName string, input map[string]interface{}, ts time.Time) ingest.Event {
	return ingest.Event{
		Timestamp: ts,
		SessionID: sessionID,
		EventType: "post_tool_use",
		ToolName:  toolName,
		ToolInput: input,
	}
}

func TestBuildGraph_SingleSession(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []ingest.Event{
		makeEvent("s1", "Bash", map[string]interface{}{"command": "kubectl get pods -n staging"}, t0),
		makeEvent("s1", "Bash", map[string]interface{}{"command": "kubectl logs -n staging pod/x"}, t0.Add(time.Minute)),
	}

	g := BuildGraph(events)

	if len(g.Nodes) != 2 {
		t.Errorf("got %d nodes, want 2", len(g.Nodes))
	}
	edgeCount := 0
	for _, adj := range g.Edges {
		edgeCount += len(adj)
	}
	if edgeCount != 1 {
		t.Errorf("got %d edges, want 1", edgeCount)
	}
}

func TestBuildGraph_SameShapeCollapses(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []ingest.Event{
		makeEvent("s1", "Bash", map[string]interface{}{"command": "kubectl get pods -n staging"}, t0),
		makeEvent("s2", "Bash", map[string]interface{}{"command": "kubectl get pods -n production"}, t0.Add(time.Hour)),
	}

	g := BuildGraph(events)

	if len(g.Nodes) != 1 {
		t.Errorf("got %d nodes, want 1 (same shape should collapse)", len(g.Nodes))
	}
}

func TestBuildGraph_MultiSessionEdgeWeight(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []ingest.Event{
		makeEvent("s1", "Bash", map[string]interface{}{"command": "git status"}, t0),
		makeEvent("s1", "Bash", map[string]interface{}{"command": "git diff"}, t0.Add(time.Minute)),
		makeEvent("s2", "Bash", map[string]interface{}{"command": "git status"}, t0.Add(time.Hour)),
		makeEvent("s2", "Bash", map[string]interface{}{"command": "git diff"}, t0.Add(time.Hour+time.Minute)),
	}

	g := BuildGraph(events)

	if len(g.Nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(g.Nodes))
	}

	found := false
	for _, adj := range g.Edges {
		for _, edge := range adj {
			if edge.Weight == 2 && len(edge.SessionIDs) == 2 {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected edge with weight=2 and 2 session IDs")
	}
}

func TestBuildGraph_SkipsNonToolUseEvents(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []ingest.Event{
		{Timestamp: t0, SessionID: "s1", EventType: "session_start"},
		makeEvent("s1", "Bash", map[string]interface{}{"command": "ls"}, t0.Add(time.Minute)),
		{Timestamp: t0.Add(2 * time.Minute), SessionID: "s1", EventType: "session_end"},
	}

	g := BuildGraph(events)

	if len(g.Nodes) != 1 {
		t.Errorf("got %d nodes, want 1 (non-tool-use events skipped)", len(g.Nodes))
	}
}
