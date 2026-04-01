package ingest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/agentjit/internal/config"
)

func TestIngestFromStdin(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()

	input := `{
		"session_id": "test_session",
		"hook_event_name": "PostToolUse",
		"cwd": "/Users/dev/project",
		"tool_name": "Bash",
		"tool_input": {"command": "echo hello"},
		"tool_response": "hello"
	}`

	cfg := config.DefaultConfig()
	reader := bytes.NewReader([]byte(input))

	err := IngestFromReader(reader, paths, cfg)
	if err != nil {
		t.Fatalf("IngestFromReader: %v", err)
	}

	// Check that a log file was created
	entries, err := os.ReadDir(paths.Logs)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no date directories created")
	}

	// Find the JSONL file
	dateDir := filepath.Join(paths.Logs, entries[0].Name())
	files, _ := os.ReadDir(dateDir)
	if len(files) == 0 {
		t.Fatal("no JSONL files created")
	}
	if files[0].Name() != "test_session.jsonl" {
		t.Errorf("unexpected filename: %s", files[0].Name())
	}
}

func TestIngestEmptyInput(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()

	cfg := config.DefaultConfig()
	reader := bytes.NewReader([]byte(""))

	err := IngestFromReader(reader, paths, cfg)
	if err == nil {
		t.Error("expected error for empty input")
	}
}
