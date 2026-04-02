package dream

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/ingest"
	"github.com/anthropics/agentjit/internal/skills"
)

func TestBuildContext(t *testing.T) {
	cfg := config.DefaultConfig()

	events := []ingest.Event{
		{
			Timestamp:        time.Now(),
			SessionID:        "s1",
			Harness:          "claude-code",
			EventType:        "post_tool_use",
			ToolName:         "Bash",
			ToolInput:        map[string]interface{}{"command": "ls"},
			WorkingDirectory: "/dev",
		},
	}

	existingSkills := []skills.SkillMeta{
		{Name: "get-logs", Description: "Fetch logs", GeneratedBy: "agentjit"},
	}

	context, err := BuildContext(events, existingSkills, cfg)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	if !strings.Contains(context, "s1") {
		t.Error("context should contain session ID")
	}
	if !strings.Contains(context, "get-logs") {
		t.Error("context should contain existing skill name")
	}
	if !strings.Contains(context, "EXECUTION LOGS") {
		t.Error("context should have EXECUTION LOGS section header")
	}
	if !strings.Contains(context, "EXISTING SKILLS") {
		t.Error("context should have EXISTING SKILLS section header")
	}
}

func TestBuildContextNoSkills(t *testing.T) {
	cfg := config.DefaultConfig()

	events := []ingest.Event{
		{
			Timestamp:        time.Now(),
			SessionID:        "s2",
			Harness:          "claude-code",
			EventType:        "session_start",
			WorkingDirectory: "/tmp",
		},
	}

	context, err := BuildContext(events, nil, cfg)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	if !strings.Contains(context, "No existing skills.") {
		t.Error("context should indicate no existing skills")
	}
}

func TestBuildPrompt(t *testing.T) {
	cfg := config.DefaultConfig()

	// Create a temp prompt file
	dir := t.TempDir()
	promptPath := dir + "/compiler.md"
	os.WriteFile(promptPath, []byte("Template with {{MIN_PATTERN_FREQUENCY}} and {{GLOBAL_SKILLS_DIR}}"), 0644)

	prompt, err := BuildPrompt(promptPath, cfg, "/home/user/.agentjit/skills")
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if strings.Contains(prompt, "{{MIN_PATTERN_FREQUENCY}}") {
		t.Error("template variable not replaced")
	}
	if !strings.Contains(prompt, "3") {
		t.Error("expected min_pattern_frequency default of 3")
	}
	if !strings.Contains(prompt, "/home/user/.agentjit/skills") {
		t.Error("expected global skills dir to be substituted")
	}
}

func TestBuildPromptMissingFile(t *testing.T) {
	cfg := config.DefaultConfig()

	_, err := BuildPrompt("/nonexistent/path/prompt.md", cfg, "/tmp/skills")
	if err == nil {
		t.Error("expected error for missing prompt file")
	}
}
