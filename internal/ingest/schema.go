package ingest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Event is the normalized AgentJIT event schema.
type Event struct {
	Timestamp           time.Time              `json:"timestamp"`
	SessionID           string                 `json:"session_id"`
	Harness             string                 `json:"harness"`
	EventType           string                 `json:"event_type"`
	Source              string                 `json:"source,omitempty"`
	ToolName            string                 `json:"tool_name,omitempty"`
	ToolInput           map[string]interface{} `json:"tool_input,omitempty"`
	ToolResponseSummary string                 `json:"tool_response_summary,omitempty"`
	ExitCode            *int                   `json:"exit_code,omitempty"`
	WorkingDirectory    string                 `json:"working_directory"`
	Error               string                 `json:"error,omitempty"`
	Reason              string                 `json:"reason,omitempty"`
	SessionDurationMs   int64                  `json:"session_duration_ms,omitempty"`
	NumTurns            int                    `json:"num_turns,omitempty"`
	Metrics             *EventMetrics          `json:"metrics,omitempty"`
	BootstrapSource     string                 `json:"source_type,omitempty"`
}

// EventMetrics holds optional performance metrics for an event.
type EventMetrics struct {
	ExecutionDurationMs int64 `json:"execution_duration_ms,omitempty"`
}

// hookPayload is the raw JSON structure from Claude Code hooks.
type hookPayload struct {
	SessionID         string                 `json:"session_id"`
	HookEventName     string                 `json:"hook_event_name"`
	CWD               string                 `json:"cwd"`
	ToolName          string                 `json:"tool_name"`
	ToolInput         map[string]interface{} `json:"tool_input"`
	ToolResponse      interface{}            `json:"tool_response"`
	ToolUseID         string                 `json:"tool_use_id"`
	Error             string                 `json:"error"`
	Source            string                 `json:"source"`
	Reason            string                 `json:"reason"`
	SessionDurationMs int64                  `json:"session_duration_ms"`
	NumTurns          int                    `json:"num_turns"`
}

// eventNameMap maps Claude Code hook event names to our normalized event types.
var eventNameMap = map[string]string{
	"PostToolUse":        "post_tool_use",
	"PostToolUseFailure": "post_tool_use_failure",
	"SessionStart":       "session_start",
	"SessionEnd":         "session_end",
}

// NormalizeEvent converts a raw Claude Code hook JSON payload into a normalized Event.
// maxResponseBytes controls truncation of tool_response.
func NormalizeEvent(raw []byte, maxResponseBytes int) (Event, error) {
	var payload hookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Event{}, fmt.Errorf("parsing hook payload: %w", err)
	}

	eventType, ok := eventNameMap[payload.HookEventName]
	if !ok {
		eventType = strings.ToLower(payload.HookEventName)
	}

	event := Event{
		Timestamp:         time.Now().UTC(),
		SessionID:         payload.SessionID,
		Harness:           "claude-code",
		EventType:         eventType,
		ToolName:          payload.ToolName,
		ToolInput:         payload.ToolInput,
		WorkingDirectory:  payload.CWD,
		Error:             payload.Error,
		Source:            payload.Source,
		Reason:            payload.Reason,
		SessionDurationMs: payload.SessionDurationMs,
		NumTurns:          payload.NumTurns,
	}

	// Truncate tool response
	if payload.ToolResponse != nil {
		var summary string
		switch v := payload.ToolResponse.(type) {
		case string:
			summary = v
		default:
			data, _ := json.Marshal(v)
			summary = string(data)
		}
		if len(summary) > maxResponseBytes {
			summary = summary[:maxResponseBytes]
		}
		event.ToolResponseSummary = summary
	}

	return event, nil
}

// DateKey returns the date string (YYYY-MM-DD) for partitioning logs.
func (e Event) DateKey() string {
	return e.Timestamp.Format("2006-01-02")
}
