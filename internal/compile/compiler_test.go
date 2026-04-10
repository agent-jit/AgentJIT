package compile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/agentjit/internal/config"
)

func TestBuildManifest(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	// Create a session log file
	dateDir := filepath.Join(paths.Logs, "2026-04-01")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	events := []string{
		`{"timestamp":"2026-04-01T10:00:00Z","session_id":"s1","event_type":"post_tool_use","tool_name":"Bash","tool_input":{"command":"ls"},"working_directory":"/dev"}`,
		`{"timestamp":"2026-04-01T10:01:00Z","session_id":"s1","event_type":"post_tool_use","tool_name":"Read","tool_input":{"file_path":"/dev/main.go"},"working_directory":"/dev"}`,
	}
	if err := os.WriteFile(filepath.Join(dateDir, "s1.jsonl"), []byte(strings.Join(events, "\n")), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

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
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	manifest, err := BuildManifest(paths)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}

	if manifest.TotalSessions != 0 {
		t.Errorf("total_sessions = %d, want 0", manifest.TotalSessions)
	}
}

func TestBuildManifest_SkipsBeforeMarker(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	// Create logs across multiple days
	for _, date := range []string{"2026-03-15", "2026-03-20", "2026-04-01", "2026-04-08"} {
		dateDir := filepath.Join(paths.Logs, date)
		if err := os.MkdirAll(dateDir, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", date, err)
		}
		event := `{"timestamp":"` + date + `T10:00:00Z","session_id":"s-` + date + `","tool_name":"Bash","working_directory":"/tmp"}` + "\n"
		if err := os.WriteFile(filepath.Join(dateDir, "s-"+date+".jsonl"), []byte(event), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", date, err)
		}
	}

	// Set marker to April 1 — should skip March dates, include April 1 and April 8
	marker := time.Date(2026, 4, 1, 15, 30, 0, 0, time.UTC)
	if err := WriteMarker(paths.CompileMarker, marker); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	manifest, err := BuildManifest(paths)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}

	// Should only include 2026-04-01 (marker day) and 2026-04-08 (after marker)
	if manifest.TotalSessions != 2 {
		var dates []string
		for _, s := range manifest.Sessions {
			dates = append(dates, s.Date)
		}
		t.Errorf("TotalSessions = %d, want 2 (included dates: %v)", manifest.TotalSessions, dates)
	}
}

func TestBuildManifest_AfterBootstrapAndCompile(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	// Simulate bootstrap: write events with historical timestamps across 30 days
	for _, date := range []string{"2026-03-09", "2026-03-15", "2026-03-20", "2026-03-25", "2026-04-01", "2026-04-07"} {
		dateDir := filepath.Join(paths.Logs, date)
		if err := os.MkdirAll(dateDir, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", date, err)
		}
		event := `{"timestamp":"` + date + `T10:00:00Z","session_id":"s-` + date + `","tool_name":"Bash","working_directory":"/tmp"}` + "\n"
		if err := os.WriteFile(filepath.Join(dateDir, "s-"+date+".jsonl"), []byte(event), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", date, err)
		}
	}

	// First compile — no marker, should include all 6 sessions
	m1, err := BuildManifest(paths)
	if err != nil {
		t.Fatalf("first BuildManifest: %v", err)
	}
	if m1.TotalSessions != 6 {
		t.Errorf("first compile: TotalSessions = %d, want 6", m1.TotalSessions)
	}

	// Simulate compile finishing: write marker at "now"
	compileTime := time.Date(2026, 4, 8, 7, 1, 29, 0, time.UTC)
	if err := WriteMarker(paths.CompileMarker, compileTime); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	// Second compile — marker set, should include 0 sessions (all before marker date)
	m2, err := BuildManifest(paths)
	if err != nil {
		t.Fatalf("second BuildManifest: %v", err)
	}
	if m2.TotalSessions != 0 {
		var dates []string
		for _, s := range m2.Sessions {
			dates = append(dates, s.Date)
		}
		t.Errorf("second compile: TotalSessions = %d, want 0 (included dates: %v)", m2.TotalSessions, dates)
	}
}

func TestBuildManifest_MarkerSameDayIncludesAll(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	// Events from multiple days including marker day
	for _, date := range []string{"2026-03-15", "2026-04-08"} {
		dateDir := filepath.Join(paths.Logs, date)
		if err := os.MkdirAll(dateDir, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", date, err)
		}
		event := `{"timestamp":"` + date + `T10:00:00Z","session_id":"s-` + date + `","tool_name":"Bash","working_directory":"/tmp"}` + "\n"
		if err := os.WriteFile(filepath.Join(dateDir, "s-"+date+".jsonl"), []byte(event), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", date, err)
		}
	}

	// Marker is on April 8 at 7am — the 2026-04-08 dir should still be included
	// (it has the same date as the marker), but 2026-03-15 should be excluded
	marker := time.Date(2026, 4, 8, 7, 0, 0, 0, time.UTC)
	if err := WriteMarker(paths.CompileMarker, marker); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	m, err := BuildManifest(paths)
	if err != nil {
		t.Fatalf("BuildManifest: %v", err)
	}
	// April 8 dir is NOT before marker date (same day) so it's included
	// This means already-compiled sessions from that day get re-included
	if m.TotalSessions != 1 {
		var dates []string
		for _, s := range m.Sessions {
			dates = append(dates, s.Date)
		}
		t.Errorf("TotalSessions = %d, want 1 (dates: %v)", m.TotalSessions, dates)
	}
	if len(m.Sessions) > 0 && m.Sessions[0].Date != "2026-04-08" {
		t.Errorf("expected 2026-04-08, got %s", m.Sessions[0].Date)
	}
}

func TestBuildManifestJSON(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	dateDir := filepath.Join(paths.Logs, "2026-03-15")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dateDir, "abc.jsonl"),
		[]byte(`{"timestamp":"2026-03-15T10:00:00Z","session_id":"abc","tool_name":"Bash","working_directory":"/tmp"}`+"\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

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

func TestBuildPrompt_IncludesPlatform(t *testing.T) {
	cfg := config.DefaultConfig()

	template := "Platform is {{PLATFORM}} with {{SHELL}}"
	prompt := BuildPrompt(template, cfg, "/tmp/skills")

	if strings.Contains(prompt, "{{PLATFORM}}") {
		t.Error("{{PLATFORM}} was not replaced")
	}
	if strings.Contains(prompt, "{{SHELL}}") {
		t.Error("{{SHELL}} was not replaced")
	}
}

func TestBuildPrompt_WindowsShellVariable(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Compile.Platform = "windows"

	template := "Platform={{PLATFORM}} Shell={{SHELL}}"
	prompt := BuildPrompt(template, cfg, "/tmp/skills")

	if !strings.Contains(prompt, "Platform=windows") {
		t.Errorf("expected Platform=windows, got %q", prompt)
	}
	if !strings.Contains(prompt, "Shell=powershell") {
		t.Errorf("expected Shell=powershell, got %q", prompt)
	}
}

func TestBuildPrompt_UnixShellVariable(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Compile.Platform = "linux"

	template := "Platform={{PLATFORM}} Shell={{SHELL}}"
	prompt := BuildPrompt(template, cfg, "/tmp/skills")

	if !strings.Contains(prompt, "Platform=linux") {
		t.Errorf("expected Platform=linux, got %q", prompt)
	}
	if !strings.Contains(prompt, "Shell=bash") {
		t.Errorf("expected Shell=bash, got %q", prompt)
	}
}
