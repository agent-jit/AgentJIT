package compile

import (
	"sort"

	"github.com/agent-jit/agentjit/internal/ingest"
	"github.com/agent-jit/agentjit/internal/ir"
	"github.com/agent-jit/agentjit/internal/trace"
)

// BuildGraphWithIR constructs a TraceGraph using IR matching for Bash commands.
func BuildGraphWithIR(events []ingest.Event, matcher *ir.Matcher) *trace.TraceGraph {
	g := &trace.TraceGraph{
		Nodes: make(map[uint64]*trace.Node),
		Edges: make(map[uint64]map[uint64]*trace.Edge),
	}

	sessions := make(map[string][]ingest.Event)
	for _, e := range events {
		if e.EventType != "post_tool_use" || e.ToolName == "" {
			continue
		}
		sessions[e.SessionID] = append(sessions[e.SessionID], e)
	}

	for sessionID, sessionEvents := range sessions {
		sort.Slice(sessionEvents, func(i, j int) bool {
			return sessionEvents[i].Timestamp.Before(sessionEvents[j].Timestamp)
		})

		var prevID uint64
		for i, event := range sessionEvents {
			var id uint64
			var node *trace.Node

			if event.ToolName == "Bash" {
				if cmd, ok := event.ToolInput["command"].(string); ok {
					preprocessed := trace.PreprocessBashCommand(cmd)
					if match := matcher.Match(preprocessed); match != nil {
						id = match.IRNodeID()
						node = &trace.Node{
							ToolName:   event.ToolName,
							InputShape: map[string]string{"command": match.CapabilityID},
							ID:         id,
						}
					}
				}
			}

			if node == nil {
				shape := trace.InputShape(event.ToolName, event.ToolInput)
				id = trace.NodeID(event.ToolName, shape)
				node = &trace.Node{
					ToolName:   event.ToolName,
					InputShape: shape,
					ID:         id,
				}
			}

			if _, ok := g.Nodes[id]; !ok {
				g.Nodes[id] = node
			}

			if i > 0 {
				addEdge(g, prevID, id, sessionID)
			}
			prevID = id
		}
	}

	return g
}

func addEdge(g *trace.TraceGraph, from, to uint64, sessionID string) {
	if g.Edges[from] == nil {
		g.Edges[from] = make(map[uint64]*trace.Edge)
	}

	edge, ok := g.Edges[from][to]
	if !ok {
		edge = &trace.Edge{From: from, To: to, SessionWeights: make(map[string]int)}
		g.Edges[from][to] = edge
	}

	edge.Weight++
	edge.SessionWeights[sessionID]++

	for _, sid := range edge.SessionIDs {
		if sid == sessionID {
			return
		}
	}
	edge.SessionIDs = append(edge.SessionIDs, sessionID)
}
