package ingest

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
)

func TestWriteEvent(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	w := NewWriter(paths)

	event := Event{
		Timestamp:        time.Date(2026, 4, 1, 5, 28, 45, 0, time.UTC),
		SessionID:        "cld_abc123",
		Harness:          "claude-code",
		EventType:        "post_tool_use",
		ToolName:         "Bash",
		ToolInput:        map[string]interface{}{"command": "ls"},
		WorkingDirectory: "/Users/dev",
	}

	if err := w.Write(event); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify file was created in correct location
	expectedPath := filepath.Join(root, "logs", "2026-04-01", "cld_abc123.jsonl")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected file at %s: %v", expectedPath, err)
	}

	// Read and verify content
	f, err := os.Open(expectedPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatal("expected one line in JSONL")
	}

	var read Event
	if err := json.Unmarshal(scanner.Bytes(), &read); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if read.SessionID != "cld_abc123" {
		t.Errorf("SessionID = %q, want cld_abc123", read.SessionID)
	}
	if read.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", read.ToolName)
	}
}

func TestWriteMultipleEvents(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	_ = paths.EnsureDirs()

	w := NewWriter(paths)

	for i := 0; i < 3; i++ {
		event := Event{
			Timestamp:        time.Date(2026, 4, 1, 5, 28, 45, 0, time.UTC),
			SessionID:        "cld_abc123",
			Harness:          "claude-code",
			EventType:        "post_tool_use",
			ToolName:         "Bash",
			WorkingDirectory: "/Users/dev",
		}
		if err := w.Write(event); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	expectedPath := filepath.Join(root, "logs", "2026-04-01", "cld_abc123.jsonl")
	f, _ := os.Open(expectedPath)
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	if count != 3 {
		t.Errorf("line count = %d, want 3", count)
	}
}

func TestWriteDifferentSessions(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	_ = paths.EnsureDirs()

	w := NewWriter(paths)

	e1 := Event{
		Timestamp: time.Date(2026, 4, 1, 5, 0, 0, 0, time.UTC),
		SessionID: "session_a", Harness: "claude-code", EventType: "post_tool_use",
		WorkingDirectory: "/dev",
	}
	e2 := Event{
		Timestamp: time.Date(2026, 4, 1, 6, 0, 0, 0, time.UTC),
		SessionID: "session_b", Harness: "claude-code", EventType: "post_tool_use",
		WorkingDirectory: "/dev",
	}

	_ = w.Write(e1)
	_ = w.Write(e2)

	fileA := filepath.Join(root, "logs", "2026-04-01", "session_a.jsonl")
	fileB := filepath.Join(root, "logs", "2026-04-01", "session_b.jsonl")

	if _, err := os.Stat(fileA); err != nil {
		t.Errorf("session_a file missing: %v", err)
	}
	if _, err := os.Stat(fileB); err != nil {
		t.Errorf("session_b file missing: %v", err)
	}
}
