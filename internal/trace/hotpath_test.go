package trace

import (
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/ingest"
)

func TestFindHotPaths_BasicPattern(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	// 3 sessions all doing: git status -> git diff -> git add
	var events []ingest.Event
	for i, sid := range []string{"s1", "s2", "s3"} {
		base := t0.Add(time.Duration(i) * time.Hour)
		events = append(events,
			makeEvent(sid, "Bash", map[string]interface{}{"command": "git status"}, base),
			makeEvent(sid, "Bash", map[string]interface{}{"command": "git diff"}, base.Add(time.Minute)),
			makeEvent(sid, "Bash", map[string]interface{}{"command": "git add ."}, base.Add(2*time.Minute)),
		)
	}

	g := BuildGraph(events)
	paths := FindHotPaths(g, 3, 2, 20)

	if len(paths) == 0 {
		t.Fatal("expected at least one hot path")
	}
	// The longest path should be 3 steps
	maxLen := 0
	for _, p := range paths {
		if len(p.NodeIDs) > maxLen {
			maxLen = len(p.NodeIDs)
		}
	}
	if maxLen != 3 {
		t.Errorf("longest path = %d, want 3", maxLen)
	}
}

func TestFindHotPaths_BelowThreshold(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	// Only 2 sessions — below minFrequency=3
	events := []ingest.Event{
		makeEvent("s1", "Bash", map[string]interface{}{"command": "git status"}, t0),
		makeEvent("s1", "Bash", map[string]interface{}{"command": "git diff"}, t0.Add(time.Minute)),
		makeEvent("s2", "Bash", map[string]interface{}{"command": "git status"}, t0.Add(time.Hour)),
		makeEvent("s2", "Bash", map[string]interface{}{"command": "git diff"}, t0.Add(time.Hour+time.Minute)),
	}

	g := BuildGraph(events)
	paths := FindHotPaths(g, 3, 2, 20)

	if len(paths) != 0 {
		t.Errorf("expected 0 hot paths below threshold, got %d", len(paths))
	}
}

func TestFindHotPaths_PrunesSubPaths(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	// 3 sessions: A -> B -> C
	var events []ingest.Event
	for i, sid := range []string{"s1", "s2", "s3"} {
		base := t0.Add(time.Duration(i) * time.Hour)
		events = append(events,
			makeEvent(sid, "Bash", map[string]interface{}{"command": "git status"}, base),
			makeEvent(sid, "Bash", map[string]interface{}{"command": "git diff"}, base.Add(time.Minute)),
			makeEvent(sid, "Bash", map[string]interface{}{"command": "git add ."}, base.Add(2*time.Minute)),
		)
	}

	g := BuildGraph(events)
	paths := FindHotPaths(g, 3, 2, 20)

	// Should NOT include A->B as separate path since A->B->C is hot at same frequency
	for _, p := range paths {
		if len(p.NodeIDs) == 2 {
			// Check that the 2-node path doesn't have the same frequency as the 3-node path
			has3 := false
			for _, pp := range paths {
				if len(pp.NodeIDs) == 3 && pp.Frequency >= p.Frequency {
					has3 = true
				}
			}
			if has3 {
				t.Errorf("subpath of length 2 should be pruned when length-3 path has same frequency")
			}
		}
	}
}

func TestFindHotPaths_MinLength(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	var events []ingest.Event
	for i, sid := range []string{"s1", "s2", "s3"} {
		base := t0.Add(time.Duration(i) * time.Hour)
		events = append(events,
			makeEvent(sid, "Bash", map[string]interface{}{"command": "git status"}, base),
			makeEvent(sid, "Bash", map[string]interface{}{"command": "git diff"}, base.Add(time.Minute)),
		)
	}

	g := BuildGraph(events)
	paths := FindHotPaths(g, 3, 2, 20)

	for _, p := range paths {
		if len(p.NodeIDs) < 2 {
			t.Errorf("path length %d < minLength=2", len(p.NodeIDs))
		}
	}
}
