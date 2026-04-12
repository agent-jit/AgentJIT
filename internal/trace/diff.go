package trace

import (
	"fmt"
	"sort"
	"strings"

	"github.com/agent-jit/agentjit/internal/ingest"
)

// Parameter represents a variable extracted from diffing multiple instances.
type Parameter struct {
	Name     string   // e.g. "NAMESPACE", "FILE_PATH", "ARG_1"
	Position int      // token position in the command
	Values   []string // observed concrete values across instances
}

// PatternStep is one step in a parameterized pattern.
type PatternStep struct {
	ToolName   string
	Template   string      // command with $PARAM placeholders
	Parameters []Parameter // extracted parameters
	NodeID     uint64
}

// Pattern is a fully parameterized hot path ready for compilation.
type Pattern struct {
	Steps      []PatternStep
	Frequency  int
	SessionIDs []string
}

// Parameterize takes hot paths and raw events, aligns instances to the
// node sequence, and diffs tool inputs to extract parameters.
func Parameterize(hotPaths []HotPath, events []ingest.Event, g *TraceGraph) []Pattern {
	// Index events by session, keeping only post_tool_use events with a tool name.
	bySession := make(map[string][]ingest.Event)
	for _, e := range events {
		if e.EventType == "post_tool_use" && e.ToolName != "" {
			bySession[e.SessionID] = append(bySession[e.SessionID], e)
		}
	}
	for sid := range bySession {
		sort.Slice(bySession[sid], func(i, j int) bool {
			return bySession[sid][i].Timestamp.Before(bySession[sid][j].Timestamp)
		})
	}

	var patterns []Pattern
	for _, hp := range hotPaths {
		p := parameterizeHotPath(hp, bySession, g)
		patterns = append(patterns, p)
	}
	return patterns
}

func parameterizeHotPath(hp HotPath, bySession map[string][]ingest.Event, g *TraceGraph) Pattern {
	p := Pattern{
		Frequency:  hp.Frequency,
		SessionIDs: hp.SessionIDs,
	}

	// For each step in the hot path, collect actual commands from each session.
	for stepIdx, nodeID := range hp.NodeIDs {
		node := g.Nodes[nodeID]
		step := PatternStep{
			ToolName: node.ToolName,
			NodeID:   nodeID,
		}

		// Collect concrete tool inputs for this step from each session.
		var instances []map[string]interface{}
		for _, sid := range hp.SessionIDs {
			sessionEvents := bySession[sid]
			if event := findNthMatchingEvent(sessionEvents, stepIdx, hp.NodeIDs); event != nil {
				instances = append(instances, event.ToolInput)
			}
		}

		if node.ToolName == "Bash" {
			step.Template, step.Parameters = diffBashInstances(instances)
		} else {
			step.Template = node.ToolName + " call"
		}

		p.Steps = append(p.Steps, step)
	}

	return p
}

// findNthMatchingEvent walks through a session's events in order and matches
// them against the hot path's node sequence. It returns the event that
// corresponds to the step at stepIdx.
//
// NOTE: This uses greedy, non-backtracking matching. If a session contains
// extra events whose NodeID matches a path step, the greedy match may bind
// the wrong event instance, producing slightly off parameter values. This is
// acceptable for the current use case where sessions closely follow the pattern.
func findNthMatchingEvent(sessionEvents []ingest.Event, stepIdx int, pathNodeIDs []uint64) *ingest.Event {
	matchIdx := 0
	for i := range sessionEvents {
		e := &sessionEvents[i]
		shape := InputShape(e.ToolName, e.ToolInput)
		id := NodeID(e.ToolName, shape)

		if matchIdx < len(pathNodeIDs) && id == pathNodeIDs[matchIdx] {
			if matchIdx == stepIdx {
				return e
			}
			matchIdx++
		}
	}
	return nil
}

// diffBashInstances compares multiple Bash command instances and extracts parameters.
// It tokenizes each instance using TokenizeBashCommand for consistency with InputShape,
// then compares tokens positionally to find variable positions.
func diffBashInstances(instances []map[string]interface{}) (string, []Parameter) {
	if len(instances) == 0 {
		return "", nil
	}

	// Tokenize all instances using TokenizeBashCommand for consistent splitting.
	var tokenSets [][]Token
	for _, inst := range instances {
		cmd, _ := inst["command"].(string)
		tokens := TokenizeBashCommand(cmd)
		tokenSets = append(tokenSets, tokens)
	}

	// Use first instance as reference length.
	refLen := len(tokenSets[0])
	for _, ts := range tokenSets[1:] {
		if len(ts) != refLen {
			// Length mismatch -- fall back to literal first instance.
			cmd, _ := instances[0]["command"].(string)
			return cmd, nil
		}
	}

	var params []Parameter
	var templateParts []string
	argCounter := 0

	for pos := 0; pos < refLen; pos++ {
		// Check if all instances have the same token value at this position.
		allSame := true
		ref := tokenSets[0][pos].Value
		for _, ts := range tokenSets[1:] {
			if ts[pos].Value != ref {
				allSame = false
				break
			}
		}

		if allSame {
			templateParts = append(templateParts, ref)
		} else {
			// Extract parameter.
			name := inferParamName(tokenSets[0], pos, argCounter)
			argCounter++

			var values []string
			for _, ts := range tokenSets {
				values = append(values, ts[pos].Value)
			}

			params = append(params, Parameter{
				Name:     name,
				Position: pos,
				Values:   values,
			})
			templateParts = append(templateParts, "$"+name)
		}
	}

	return strings.Join(templateParts, " "), params
}

// CollectUniqueParams gathers unique parameters across all steps, preserving order.
func CollectUniqueParams(steps []PatternStep) []Parameter {
	seen := make(map[string]bool)
	var params []Parameter
	for _, step := range steps {
		for _, p := range step.Parameters {
			if !seen[p.Name] {
				seen[p.Name] = true
				params = append(params, p)
			}
		}
	}
	return params
}

// inferParamName guesses a descriptive name for a parameter based on context.
// It checks the preceding token for known flags and falls back to a generic name.
func inferParamName(tokens []Token, pos int, fallbackIdx int) string {
	if pos > 0 {
		prev := tokens[pos-1].Value
		switch prev {
		case "-n", "--namespace":
			return "NAMESPACE"
		case "-f", "--file", "--filename":
			return "FILE_PATH"
		case "-o", "--output":
			return "OUTPUT"
		case "-l", "--label", "--selector":
			return "SELECTOR"
		case "-p", "--port":
			return "PORT"
		case "-c", "--container":
			return "CONTAINER"
		case "-i", "--image":
			return "IMAGE"
		case "-t", "--tag":
			return "TAG"
		case "--name":
			return "NAME"
		case "--context":
			return "CONTEXT"
		case "--cluster":
			return "CLUSTER"
		case "--region":
			return "REGION"
		case "--zone":
			return "ZONE"
		case "--project":
			return "PROJECT"
		case "--profile":
			return "PROFILE"
		case "--subscription":
			return "SUBSCRIPTION"
		case "--resource-group", "-g":
			return "RESOURCE_GROUP"
		}
	}

	// Check if the reference value looks like a path.
	val := tokens[pos].Value
	if strings.Contains(val, "/") || strings.Contains(val, "\\") {
		return "FILE_PATH"
	}

	return fmt.Sprintf("ARG_%d", fallbackIdx+1)
}
