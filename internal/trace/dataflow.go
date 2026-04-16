package trace

import (
	"sort"
	"strings"

	"github.com/agent-jit/agentjit/internal/ingest"
)

// stopwords are common tokens filtered from data-flow detection.
var stopwords = map[string]bool{
	"true": true, "false": true, "error": true, "null": true,
	"the": true, "and": true, "file": true, "line": true,
	"name": true, "type": true, "test": true, "data": true,
	"with": true, "from": true, "that": true, "this": true,
	"have": true, "will": true, "been": true, "were": true,
	"stdout": true, "stderr": true,
}

// tokenizeForFlow splits a string into tokens for data-flow overlap detection.
func tokenizeForFlow(s string) map[string]bool {
	tokens := make(map[string]bool)
	r := strings.NewReplacer("/", " ", ":", " ", "\n", " ", "\r", " ", "\t", " ")
	normalized := r.Replace(s)

	for _, tok := range strings.Fields(normalized) {
		tok = strings.TrimSpace(tok)
		if len(tok) < 4 {
			continue
		}
		lower := strings.ToLower(tok)
		if stopwords[lower] {
			continue
		}
		tokens[tok] = true
	}
	return tokens
}

// extractInputTokens extracts tokens from all values in a ToolInput map.
func extractInputTokens(input map[string]interface{}) map[string]bool {
	tokens := make(map[string]bool)
	for _, v := range input {
		s, ok := v.(string)
		if !ok {
			continue
		}
		for tok := range tokenizeForFlow(s) {
			tokens[tok] = true
		}
	}
	return tokens
}

// DetectDataFlowEdges annotates existing graph edges with data-flow information.
func DetectDataFlowEdges(g *TraceGraph, events []ingest.Event, windowSize int) {
	sessions := make(map[string][]ingest.Event)
	for _, e := range events {
		if e.EventType != "post_tool_use" || e.ToolName == "" {
			continue
		}
		sessions[e.SessionID] = append(sessions[e.SessionID], e)
	}
	for sid := range sessions {
		sort.Slice(sessions[sid], func(i, j int) bool {
			return sessions[sid][i].Timestamp.Before(sessions[sid][j].Timestamp)
		})
	}

	for _, sessionEvents := range sessions {
		for i := 1; i < len(sessionEvents); i++ {
			eventB := sessionEvents[i]
			inputTokens := extractInputTokens(eventB.ToolInput)
			if len(inputTokens) == 0 {
				continue
			}

			shapeB := InputShape(eventB.ToolName, eventB.ToolInput)
			idB := NodeID(eventB.ToolName, shapeB)

			start := i - windowSize
			if start < 0 {
				start = 0
			}

			for j := start; j < i; j++ {
				eventA := sessionEvents[j]
				if eventA.ToolResponseSummary == "" {
					continue
				}

				outputTokens := tokenizeForFlow(eventA.ToolResponseSummary)
				if len(outputTokens) == 0 {
					continue
				}

				var overlap []string
				for tok := range inputTokens {
					if outputTokens[tok] {
						overlap = append(overlap, tok)
					}
				}

				if len(overlap) == 0 {
					continue
				}

				shapeA := InputShape(eventA.ToolName, eventA.ToolInput)
				idA := NodeID(eventA.ToolName, shapeA)

				if adj, ok := g.Edges[idA]; ok {
					if edge, ok := adj[idB]; ok {
						edge.DataFlow = true
						existing := make(map[string]bool, len(edge.FlowTokens))
						for _, t := range edge.FlowTokens {
							existing[t] = true
						}
						for _, t := range overlap {
							if !existing[t] {
								edge.FlowTokens = append(edge.FlowTokens, t)
							}
						}
					}
				}
			}
		}
	}

	for _, adj := range g.Edges {
		for _, edge := range adj {
			if len(edge.FlowTokens) > 1 {
				sort.Strings(edge.FlowTokens)
			}
		}
	}
}
