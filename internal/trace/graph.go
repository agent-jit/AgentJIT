package trace

import (
	"sort"

	"github.com/agent-jit/agentjit/internal/ingest"
)

// Node represents a unique tool-call shape in the trace graph.
type Node struct {
	ToolName   string            // e.g. "Bash", "Read", "Edit"
	InputShape map[string]string // structural fingerprint
	ID         uint64            // hash of (ToolName, InputShape)
}

// Edge represents a transition between two nodes.
type Edge struct {
	From, To       uint64            // node IDs
	Weight         int               // number of times this transition observed
	SessionIDs     []string          // which sessions contributed
	SessionWeights map[string]int    // per-session transition counts
	DataFlow    bool     // true if output of From feeds into input of To
	FlowTokens []string  // shared tokens indicating data flow
}

// TraceGraph is a directed graph of tool-call sequences.
type TraceGraph struct {
	Nodes map[uint64]*Node
	Edges map[uint64]map[uint64]*Edge // adjacency list: from -> to -> edge
}

// BuildGraph constructs a TraceGraph from a flat list of events.
// Events are grouped by session and sorted by timestamp within each session.
// Only post_tool_use events contribute nodes and edges.
func BuildGraph(events []ingest.Event) *TraceGraph {
	g := &TraceGraph{
		Nodes: make(map[uint64]*Node),
		Edges: make(map[uint64]map[uint64]*Edge),
	}

	sessions := make(map[string][]ingest.Event)
	for _, e := range events {
		if e.EventType != "post_tool_use" {
			continue
		}
		if e.ToolName == "" {
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
			shape := InputShape(event.ToolName, event.ToolInput)
			id := NodeID(event.ToolName, shape)

			if _, ok := g.Nodes[id]; !ok {
				g.Nodes[id] = &Node{
					ToolName:   event.ToolName,
					InputShape: shape,
					ID:         id,
				}
			}

			if i > 0 {
				g.addEdge(prevID, id, sessionID)
			}

			prevID = id
		}
	}

	return g
}

func (g *TraceGraph) addEdge(from, to uint64, sessionID string) {
	if g.Edges[from] == nil {
		g.Edges[from] = make(map[uint64]*Edge)
	}

	edge, ok := g.Edges[from][to]
	if !ok {
		edge = &Edge{From: from, To: to, SessionWeights: make(map[string]int)}
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
