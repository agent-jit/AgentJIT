package compile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/ingest"
)

func writeTestEvent(t *testing.T, dir, sessionID string, ts time.Time) {
	t.Helper()
	dateKey := ts.Format("2006-01-02")
	dateDir := filepath.Join(dir, dateKey)
	_ = os.MkdirAll(dateDir, 0755)

	event := ingest.Event{
		Timestamp:        ts,
		SessionID:        sessionID,
		Harness:          "claude-code",
		EventType:        "post_tool_use",
		ToolName:         "Bash",
		ToolInput:        map[string]interface{}{"command": "echo test"},
		WorkingDirectory: "/dev",
	}
	data, _ := json.Marshal(event)
	data = append(data, '\n')

	path := filepath.Join(dateDir, sessionID+".jsonl")
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	_, _ = f.Write(data)
	f.Close()
}

func TestGatherUnprocessedLogs(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	_ = paths.EnsureDirs()

	// Write events across dates
	t1 := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	writeTestEvent(t, paths.Logs, "old_session", t1)
	writeTestEvent(t, paths.Logs, "new_session", t2)

	// Set marker to before t2 but after t1
	marker := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	_ = WriteMarker(paths.CompileMarker, marker)

	events, err := GatherUnprocessedLogs(paths, 50000)
	if err != nil {
		t.Fatalf("GatherUnprocessedLogs: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("got %d events, want 1 (only new_session)", len(events))
	}
}

func TestGatherAllLogsNoMarker(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	_ = paths.EnsureDirs()

	t1 := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	writeTestEvent(t, paths.Logs, "session_a", t1)
	writeTestEvent(t, paths.Logs, "session_b", t2)

	events, err := GatherUnprocessedLogs(paths, 50000)
	if err != nil {
		t.Fatalf("GatherUnprocessedLogs: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("got %d events, want 2", len(events))
	}
}

func TestGatherRespectsMaxLines(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	_ = paths.EnsureDirs()

	ts := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		writeTestEvent(t, paths.Logs, "big_session", ts)
	}

	events, err := GatherUnprocessedLogs(paths, 5)
	if err != nil {
		t.Fatalf("GatherUnprocessedLogs: %v", err)
	}

	if len(events) > 5 {
		t.Errorf("got %d events, want <= 5", len(events))
	}
}

func TestWriteAndReadMarker(t *testing.T) {
	dir := t.TempDir()
	markerPath := filepath.Join(dir, "marker")

	ts := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	if err := WriteMarker(markerPath, ts); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	read, err := ReadMarker(markerPath)
	if err != nil {
		t.Fatalf("ReadMarker: %v", err)
	}

	if !read.Equal(ts) {
		t.Errorf("ReadMarker = %v, want %v", read, ts)
	}
}
