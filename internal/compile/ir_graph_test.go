package compile

import (
	"fmt"
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/ingest"
	"github.com/agent-jit/agentjit/internal/ir"
	"github.com/agent-jit/agentjit/internal/trace"
)

func TestIRGraphIdentityUsedByParameterizationAndDataFlow(t *testing.T) {
	cat, err := ir.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var events []ingest.Event
	for i := 0; i < 3; i++ {
		file := fmt.Sprintf("./sample%d.txt", i)
		for step, command := range []string{"git status", "git diff -- " + file, "cat -- " + file} {
			events = append(events, ingest.Event{
				Timestamp: time.Unix(int64(i*10+step), 0),
				SessionID: fmt.Sprintf("s%d", i),
				EventType: "post_tool_use",
				ToolName: "Bash",
				ToolInput: map[string]interface{}{"command": command},
				ToolResponseSummary: file,
			})
		}
	}
	g := BuildGraphWithIR(events, ir.NewMatcher(cat))
	ids := make([]uint64, 3)
	for i := range ids {
		ids[i] = g.EventNodeID(events[i])
		if g.Nodes[ids[i]] == nil {
			t.Fatalf("resolved event %d to a missing node", i)
		}
	}
	lexicalID := trace.NodeID(events[0].ToolName, trace.InputShape(events[0].ToolName, events[0].ToolInput))
	if ids[0] == lexicalID {
		t.Fatal("test requires an IR node distinct from the lexical node")
	}
	patterns := trace.Parameterize([]trace.HotPath{{
		NodeIDs: ids, Frequency: 3, SessionIDs: []string{"s0", "s1", "s2"},
	}}, events, g)
	if len(patterns) != 1 || len(patterns[0].Steps) != 3 {
		t.Fatalf("lost IR-backed pattern: %+v", patterns)
	}
	for i, want := range []string{"git status", `git diff -- "$FILE_PATH"`, `cat -- "$FILE_PATH"`} {
		if got := patterns[0].Steps[i].Template; got != want {
			t.Errorf("step %d: got %q, want %q", i, got, want)
		}
	}
	trace.DetectDataFlowEdges(g, events, 5)
	for i := 0; i < 2; i++ {
		edge := g.Edges[ids[i]][ids[i+1]]
		if edge == nil || !edge.DataFlow || len(edge.FlowTokens) == 0 {
			t.Errorf("missing data flow on edge %d: %+v", i, edge)
		}
	}
}
