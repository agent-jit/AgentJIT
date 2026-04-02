package compile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/agentjit/internal/config"
)

func TestBuildManifest(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	paths.EnsureDirs()

	// Create a session log file
	dateDir := filepath.Join(paths.Logs, "2026-04-01")
	os.MkdirAll(dateDir, 0755)

	events := []string{
		`{"timestamp":"2026-04-01T10:00:00Z","session_id":"s1","event_type":"post_tool_use","tool_name":"Bash","tool_input":{"command":"ls"},"working_directory":"/dev"}`,
		`{"timestamp":"2026-04-01T10:01:00Z","session_id":"s1","event_type":"post_tool_use","tool_name":"Read","tool_input":{"file_path":"/dev/main.go"},"working_directory":"/dev"}`,
	}
	os.WriteFile(filepath.Join(dateDir, "s1.jsonl"), []byte(strings.Join(events, "\n")), 0644)

	manifest, err := BuildManifest(paths)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}

	if manifest.TotalSessions != 1 {
		t.Errorf("total_sessions = %d, want 1", manifest.TotalSessions)
	}
	if manifest.TotalEvents != 2 {
		t.Errorf("total_events = %d, want 2", manifest.TotalEvents)
	}
	if manifest.DateRange[0] != "2026-04-01" {
		t.Errorf("date_range[0] = %q, want 2026-04-01", manifest.DateRange[0])
	}
	if len(manifest.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(manifest.Sessions))
	}

	s := manifest.Sessions[0]
	if s.SessionID != "s1" {
		t.Errorf("session_id = %q, want s1", s.SessionID)
	}
	if s.EventCount != 2 {
		t.Errorf("event_count = %d, want 2", s.EventCount)
	}
	if s.WorkingDirectory != "/dev" {
		t.Errorf("working_directory = %q, want /dev", s.WorkingDirectory)
	}
	// Tool names should include Bash and Read
	toolStr := strings.Join(s.ToolNames, ",")
	if !strings.Contains(toolStr, "Bash") || !strings.Contains(toolStr, "Read") {
		t.Errorf("tool_names = %v, want Bash and Read", s.ToolNames)
	}
}

func TestBuildManifestEmpty(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	paths.EnsureDirs()

	manifest, err := BuildManifest(paths)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}

	if manifest.TotalSessions != 0 {
		t.Errorf("total_sessions = %d, want 0", manifest.TotalSessions)
	}
}

func TestBuildManifestJSON(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	paths.EnsureDirs()

	dateDir := filepath.Join(paths.Logs, "2026-03-15")
	os.MkdirAll(dateDir, 0755)
	os.WriteFile(filepath.Join(dateDir, "abc.jsonl"),
		[]byte(`{"timestamp":"2026-03-15T10:00:00Z","session_id":"abc","tool_name":"Bash","working_directory":"/tmp"}`+"\n"), 0644)

	manifest, err := BuildManifest(paths)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify it's valid JSON with expected fields
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["total_sessions"].(float64) != 1 {
		t.Error("JSON total_sessions should be 1")
	}
}

func TestBuildPrompt(t *testing.T) {
	cfg := config.DefaultConfig()

	template := "Template with {{MIN_PATTERN_FREQUENCY}} and {{GLOBAL_SKILLS_DIR}}"

	prompt := BuildPrompt(template, cfg, "/home/user/.aj/skills")

	if strings.Contains(prompt, "{{MIN_PATTERN_FREQUENCY}}") {
		t.Error("template variable not replaced")
	}
	if !strings.Contains(prompt, "3") {
		t.Error("expected min_pattern_frequency default of 3")
	}
	if !strings.Contains(prompt, "/home/user/.aj/skills") {
		t.Error("expected global skills dir to be substituted")
	}
}

func TestBuildPromptEmpty(t *testing.T) {
	cfg := config.DefaultConfig()

	prompt := BuildPrompt("", cfg, "/tmp/skills")
	if prompt != "" {
		t.Error("expected empty prompt for empty template")
	}
}
