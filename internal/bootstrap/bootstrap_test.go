package bootstrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-jit/agentjit/internal/config"
)

func createFakeTranscript(t *testing.T, dir, sessionID string) string {
	t.Helper()
	path := filepath.Join(dir, sessionID+".jsonl")
	lines := []string{
		`{"type":"assistant","sessionId":"` + sessionID + `","timestamp":"2026-03-30T10:00:01.000Z","cwd":"/dev","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"Bash","input":{"command":"echo hi"}}]}}`,
		`{"type":"progress","sessionId":"` + sessionID + `","timestamp":"2026-03-30T10:00:02.000Z","cwd":"/dev","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"hi"}]}}`,
	}
	f, _ := os.Create(path)
	for _, l := range lines {
		_, _ = f.WriteString(l + "\n")
	}
	f.Close()
	return path
}

func TestRunBootstrap(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	_ = paths.EnsureDirs()
	cfg := config.DefaultConfig()

	// Create fake Claude projects directory
	claudeProjectsDir := filepath.Join(root, "claude-projects")
	projectDir := filepath.Join(claudeProjectsDir, "-Users-dev-project")
	_ = os.MkdirAll(projectDir, 0755)

	createFakeTranscript(t, projectDir, "session-001")
	createFakeTranscript(t, projectDir, "session-002")

	result, err := RunBootstrap(paths, cfg, claudeProjectsDir, BootstrapOptions{})
	if err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}

	if result.SessionsProcessed != 2 {
		t.Errorf("SessionsProcessed = %d, want 2", result.SessionsProcessed)
	}
	if result.EventsImported < 2 {
		t.Errorf("EventsImported = %d, want >= 2", result.EventsImported)
	}

	// Verify log files were created
	entries, _ := os.ReadDir(paths.Logs)
	if len(entries) == 0 {
		t.Error("no log date directories created")
	}

	// Verify processed tracker was updated
	data, err := os.ReadFile(paths.BootstrapProcessed)
	if err != nil {
		t.Fatalf("reading bootstrap_processed: %v", err)
	}
	var processed ProcessedFiles
	_ = json.Unmarshal(data, &processed)
	if len(processed.Files) != 2 {
		t.Errorf("processed files = %d, want 2", len(processed.Files))
	}
}

func TestRunBootstrapIdempotent(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	_ = paths.EnsureDirs()
	cfg := config.DefaultConfig()

	claudeProjectsDir := filepath.Join(root, "claude-projects")
	projectDir := filepath.Join(claudeProjectsDir, "-Users-dev-project")
	_ = os.MkdirAll(projectDir, 0755)

	createFakeTranscript(t, projectDir, "session-001")

	// Run twice
	_, _ = RunBootstrap(paths, cfg, claudeProjectsDir, BootstrapOptions{})
	result, _ := RunBootstrap(paths, cfg, claudeProjectsDir, BootstrapOptions{})

	if result.SessionsProcessed != 0 {
		t.Errorf("second run should process 0 sessions, got %d", result.SessionsProcessed)
	}
}

func TestRunBootstrapDryRun(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	_ = paths.EnsureDirs()
	cfg := config.DefaultConfig()

	claudeProjectsDir := filepath.Join(root, "claude-projects")
	projectDir := filepath.Join(claudeProjectsDir, "-Users-dev-project")
	_ = os.MkdirAll(projectDir, 0755)

	createFakeTranscript(t, projectDir, "session-001")

	result, err := RunBootstrap(paths, cfg, claudeProjectsDir, BootstrapOptions{DryRun: true})
	if err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}

	if result.SessionsProcessed != 1 {
		t.Errorf("SessionsProcessed = %d, want 1", result.SessionsProcessed)
	}

	// Verify nothing was written
	entries, _ := os.ReadDir(paths.Logs)
	if len(entries) != 0 {
		t.Error("dry run should not create log files")
	}
}
