package trace

import (
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/ingest"
)

func TestParameterize_BasicBashPattern(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	// Both commands use -n (namespace) flag whose value is variable.
	// The second command also uses --name flag so the pod name is variable.
	// This ensures both commands produce the same InputShape across sessions.
	events := []ingest.Event{
		makeEvent("s1", "Bash", map[string]interface{}{"command": "kubectl get pods -n staging"}, t0),
		makeEvent("s1", "Bash", map[string]interface{}{"command": "kubectl logs -n staging --name pod-x"}, t0.Add(time.Minute)),
		makeEvent("s2", "Bash", map[string]interface{}{"command": "kubectl get pods -n production"}, t0.Add(time.Hour)),
		makeEvent("s2", "Bash", map[string]interface{}{"command": "kubectl logs -n production --name pod-y"}, t0.Add(time.Hour+time.Minute)),
		makeEvent("s3", "Bash", map[string]interface{}{"command": "kubectl get pods -n dev"}, t0.Add(2*time.Hour)),
		makeEvent("s3", "Bash", map[string]interface{}{"command": "kubectl logs -n dev --name pod-z"}, t0.Add(2*time.Hour+time.Minute)),
	}

	g := BuildGraph(events)
	hotPaths := FindHotPaths(g, 3, 2, 20)
	if len(hotPaths) == 0 {
		t.Fatal("expected at least one hot path")
	}

	patterns := Parameterize(hotPaths, events, g)
	if len(patterns) == 0 {
		t.Fatal("expected at least one parameterized pattern")
	}

	p := patterns[0]
	if len(p.Steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(p.Steps))
	}

	// First step should have NAMESPACE parameter
	foundNS := false
	for _, param := range p.Steps[0].Parameters {
		if param.Name == "NAMESPACE" {
			foundNS = true
			if len(param.Values) != 3 {
				t.Errorf("NAMESPACE has %d values, want 3", len(param.Values))
			}
		}
	}
	if !foundNS {
		t.Errorf("expected NAMESPACE parameter in step 0, got params: %+v", p.Steps[0].Parameters)
	}
}

func TestParameterize_LiteralsPreserved(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	events := []ingest.Event{
		makeEvent("s1", "Bash", map[string]interface{}{"command": "git status"}, t0),
		makeEvent("s2", "Bash", map[string]interface{}{"command": "git status"}, t0.Add(time.Hour)),
		makeEvent("s3", "Bash", map[string]interface{}{"command": "git status"}, t0.Add(2*time.Hour)),
	}

	g := BuildGraph(events)

	// Single-node patterns won't pass minLength=2 for hot paths,
	// so we test parameterization directly with a synthetic hot path.
	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(g.Nodes))
	}
	var nodeID uint64
	for id := range g.Nodes {
		nodeID = id
	}
	hp := HotPath{
		NodeIDs:    []uint64{nodeID},
		Frequency:  3,
		SessionIDs: []string{"s1", "s2", "s3"},
	}

	patterns := Parameterize([]HotPath{hp}, events, g)
	if len(patterns) == 0 {
		t.Fatal("expected one pattern")
	}

	// "git status" has no variable tokens — should have 0 parameters
	if len(patterns[0].Steps[0].Parameters) != 0 {
		t.Errorf("expected 0 parameters for identical commands, got %d", len(patterns[0].Steps[0].Parameters))
	}
}
