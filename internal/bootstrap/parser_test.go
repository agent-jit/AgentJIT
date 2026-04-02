package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTranscript(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "test-session.jsonl")

	// Write a realistic Claude Code transcript JSONL
	lines := []string{
		`{"type":"user","sessionId":"abc123","timestamp":"2026-03-30T10:00:00.000Z","cwd":"/Users/dev/project","message":{"role":"user","content":"list files"}}`,
		`{"type":"assistant","sessionId":"abc123","timestamp":"2026-03-30T10:00:01.000Z","cwd":"/Users/dev/project","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"Bash","input":{"command":"ls -la","description":"List files"}}]}}`,
		`{"type":"progress","sessionId":"abc123","timestamp":"2026-03-30T10:00:02.000Z","cwd":"/Users/dev/project","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"total 8\ndrwxr-xr-x  3 dev staff  96 Mar 30 10:00 .\n"}]}}`,
		`{"type":"assistant","sessionId":"abc123","timestamp":"2026-03-30T10:00:03.000Z","cwd":"/Users/dev/project","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_2","name":"Read","input":{"file_path":"/Users/dev/project/main.go"}}]}}`,
		`{"type":"progress","sessionId":"abc123","timestamp":"2026-03-30T10:00:04.000Z","cwd":"/Users/dev/project","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_2","content":"package main\nfunc main() {}"}]}}`,
	}

	f, _ := os.Create(transcriptPath)
	for _, line := range lines {
		f.WriteString(line + "\n")
	}
	f.Close()

	events, err := ParseTranscript(transcriptPath, 512)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 tool uses", len(events))
	}

	// First event: Bash ls -la
	if events[0].ToolName != "Bash" {
		t.Errorf("event[0].ToolName = %q, want Bash", events[0].ToolName)
	}
	cmd, _ := events[0].ToolInput["command"].(string)
	if cmd != "ls -la" {
		t.Errorf("event[0] command = %q, want 'ls -la'", cmd)
	}
	if events[0].SessionID != "abc123" {
		t.Errorf("event[0].SessionID = %q, want abc123", events[0].SessionID)
	}
	if events[0].BootstrapSource != "bootstrap" {
		t.Errorf("event[0].BootstrapSource = %q, want bootstrap", events[0].BootstrapSource)
	}

	// Second event: Read
	if events[1].ToolName != "Read" {
		t.Errorf("event[1].ToolName = %q, want Read", events[1].ToolName)
	}
}

func TestParseTranscriptEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(path, []byte(""), 0644)

	events, err := ParseTranscript(path, 512)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
	}
}

func TestParseTranscriptNonExistent(t *testing.T) {
	_, err := ParseTranscript("/nonexistent/file.jsonl", 512)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
