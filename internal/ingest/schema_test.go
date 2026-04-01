package ingest

import (
	"encoding/json"
	"testing"
)

func TestNormalizePostToolUse(t *testing.T) {
	raw := `{
		"session_id": "abc123",
		"hook_event_name": "PostToolUse",
		"cwd": "/Users/dev/project",
		"tool_name": "Bash",
		"tool_input": {"command": "kubectl logs -n ns pod/x", "description": "Get logs"},
		"tool_response": "some very long output that should be truncated..."
	}`

	event, err := NormalizeEvent([]byte(raw), 20)
	if err != nil {
		t.Fatalf("NormalizeEvent: %v", err)
	}

	if event.SessionID != "abc123" {
		t.Errorf("SessionID = %q, want abc123", event.SessionID)
	}
	if event.EventType != "post_tool_use" {
		t.Errorf("EventType = %q, want post_tool_use", event.EventType)
	}
	if event.Harness != "claude-code" {
		t.Errorf("Harness = %q, want claude-code", event.Harness)
	}
	if event.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", event.ToolName)
	}
	if event.WorkingDirectory != "/Users/dev/project" {
		t.Errorf("WorkingDirectory = %q", event.WorkingDirectory)
	}
	if len(event.ToolResponseSummary) > 20 {
		t.Errorf("ToolResponseSummary not truncated: len=%d", len(event.ToolResponseSummary))
	}
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestNormalizeSessionStart(t *testing.T) {
	raw := `{
		"session_id": "abc123",
		"hook_event_name": "SessionStart",
		"cwd": "/Users/dev/project",
		"source": "startup"
	}`

	event, err := NormalizeEvent([]byte(raw), 512)
	if err != nil {
		t.Fatalf("NormalizeEvent: %v", err)
	}

	if event.EventType != "session_start" {
		t.Errorf("EventType = %q, want session_start", event.EventType)
	}
	if event.Source != "startup" {
		t.Errorf("Source = %q, want startup", event.Source)
	}
}

func TestNormalizeSessionEnd(t *testing.T) {
	raw := `{
		"session_id": "abc123",
		"hook_event_name": "SessionEnd",
		"cwd": "/Users/dev/project",
		"reason": "prompt_input_exit",
		"session_duration_ms": 45000,
		"num_turns": 12
	}`

	event, err := NormalizeEvent([]byte(raw), 512)
	if err != nil {
		t.Fatalf("NormalizeEvent: %v", err)
	}

	if event.EventType != "session_end" {
		t.Errorf("EventType = %q, want session_end", event.EventType)
	}
	if event.Reason != "prompt_input_exit" {
		t.Errorf("Reason = %q, want prompt_input_exit", event.Reason)
	}
	if event.SessionDurationMs != 45000 {
		t.Errorf("SessionDurationMs = %d, want 45000", event.SessionDurationMs)
	}
	if event.NumTurns != 12 {
		t.Errorf("NumTurns = %d, want 12", event.NumTurns)
	}
}

func TestNormalizePostToolUseFailure(t *testing.T) {
	raw := `{
		"session_id": "abc123",
		"hook_event_name": "PostToolUseFailure",
		"cwd": "/Users/dev/project",
		"tool_name": "Bash",
		"tool_input": {"command": "bad-command"},
		"error": "command not found"
	}`

	event, err := NormalizeEvent([]byte(raw), 512)
	if err != nil {
		t.Fatalf("NormalizeEvent: %v", err)
	}

	if event.EventType != "post_tool_use_failure" {
		t.Errorf("EventType = %q, want post_tool_use_failure", event.EventType)
	}
	if event.Error != "command not found" {
		t.Errorf("Error = %q, want 'command not found'", event.Error)
	}
}

func TestEventToJSON(t *testing.T) {
	raw := `{
		"session_id": "abc123",
		"hook_event_name": "PostToolUse",
		"cwd": "/Users/dev",
		"tool_name": "Bash",
		"tool_input": {"command": "ls"}
	}`

	event, err := NormalizeEvent([]byte(raw), 512)
	if err != nil {
		t.Fatalf("NormalizeEvent: %v", err)
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var roundtrip Event
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if roundtrip.SessionID != "abc123" {
		t.Errorf("roundtrip SessionID = %q", roundtrip.SessionID)
	}
}
