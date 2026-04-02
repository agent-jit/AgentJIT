package bootstrap

import (
	"bufio"
	"encoding/json"
	"os"
	"time"

	"github.com/anthropics/agentjit/internal/ingest"
)

// transcriptLine represents a single line in a Claude Code transcript JSONL.
type transcriptLine struct {
	Type      string          `json:"type"`
	SessionID string          `json:"sessionId"`
	Timestamp string          `json:"timestamp"`
	CWD       string          `json:"cwd"`
	Message   json.RawMessage `json:"message"`
}

// messageContent represents the message field in a transcript line.
type messageContent struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// toolUseBlock represents a tool_use block in assistant messages.
type toolUseBlock struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// toolResultBlock represents a tool_result block in progress messages.
type toolResultBlock struct {
	Type      string      `json:"type"`
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content"`
}

// ParseTranscript reads a Claude Code transcript JSONL file and extracts
// tool use events into the normalized AgentJIT event schema.
func ParseTranscript(path string, maxResponseBytes int) ([]ingest.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []ingest.Event
	// Map tool_use_id to event index for pairing results
	pendingTools := make(map[string]int)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024) // 4MB buffer for large lines

	for scanner.Scan() {
		var line transcriptLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}

		ts, _ := time.Parse(time.RFC3339Nano, line.Timestamp)
		if ts.IsZero() {
			ts, _ = time.Parse("2006-01-02T15:04:05.000Z", line.Timestamp)
		}

		var msg messageContent
		if err := json.Unmarshal(line.Message, &msg); err != nil {
			continue
		}

		// Assistant messages contain tool_use blocks
		if line.Type == "assistant" && msg.Role == "assistant" {
			var blocks []json.RawMessage
			if err := json.Unmarshal(msg.Content, &blocks); err != nil {
				continue
			}

			for _, block := range blocks {
				var tu toolUseBlock
				if err := json.Unmarshal(block, &tu); err != nil {
					continue
				}
				if tu.Type != "tool_use" {
					continue
				}

				event := ingest.Event{
					Timestamp:        ts,
					SessionID:        line.SessionID,
					Harness:          "claude-code",
					EventType:        "post_tool_use",
					ToolName:         tu.Name,
					ToolInput:        tu.Input,
					WorkingDirectory: line.CWD,
					BootstrapSource:  "bootstrap",
				}

				events = append(events, event)
				pendingTools[tu.ID] = len(events) - 1
			}
		}

		// Progress messages contain tool_result blocks
		if line.Type == "progress" && msg.Role == "user" {
			var blocks []json.RawMessage
			if err := json.Unmarshal(msg.Content, &blocks); err != nil {
				continue
			}

			for _, block := range blocks {
				var tr toolResultBlock
				if err := json.Unmarshal(block, &tr); err != nil {
					continue
				}
				if tr.Type != "tool_result" {
					continue
				}

				idx, ok := pendingTools[tr.ToolUseID]
				if !ok {
					continue
				}

				// Attach truncated response
				var summary string
				switch v := tr.Content.(type) {
				case string:
					summary = v
				default:
					data, _ := json.Marshal(v)
					summary = string(data)
				}
				if len(summary) > maxResponseBytes {
					summary = summary[:maxResponseBytes]
				}
				events[idx].ToolResponseSummary = summary

				delete(pendingTools, tr.ToolUseID)
			}
		}
	}

	return events, scanner.Err()
}
