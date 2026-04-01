package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHookTemplates(t *testing.T) {
	hooks := AgentJITHooks()

	if len(hooks) != 4 {
		t.Fatalf("expected 4 hook events, got %d", len(hooks))
	}

	// Verify PostToolUse is async
	ptHooks, ok := hooks["PostToolUse"]
	if !ok {
		t.Fatal("missing PostToolUse")
	}
	group := ptHooks[0]
	handler := group.Hooks[0]
	if !handler.Async {
		t.Error("PostToolUse should be async")
	}

	// Verify SessionStart is synchronous
	ssHooks := hooks["SessionStart"]
	ssHandler := ssHooks[0].Hooks[0]
	if ssHandler.Async {
		t.Error("SessionStart should be synchronous")
	}
}

func TestInstallHooksIntoEmptySettings(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Write empty settings
	os.WriteFile(settingsPath, []byte("{}"), 0644)

	if err := InstallHooks(settingsPath); err != nil {
		t.Fatalf("InstallHooks: %v", err)
	}

	// Read and verify
	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	hooks, ok := settings["hooks"]
	if !ok {
		t.Fatal("hooks key missing from settings")
	}

	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		t.Fatal("hooks is not an object")
	}

	for _, event := range []string{"PostToolUse", "PostToolUseFailure", "SessionStart", "SessionEnd"} {
		if _, ok := hooksMap[event]; !ok {
			t.Errorf("missing hook event: %s", event)
		}
	}
}

func TestInstallHooksPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Write settings with an existing hook
	existing := `{
		"hooks": {
			"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo existing"}]}]
		},
		"model": "opus"
	}`
	os.WriteFile(settingsPath, []byte(existing), 0644)

	if err := InstallHooks(settingsPath); err != nil {
		t.Fatalf("InstallHooks: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	// Verify existing hook is preserved
	hooksMap := settings["hooks"].(map[string]interface{})
	if _, ok := hooksMap["PreToolUse"]; !ok {
		t.Error("existing PreToolUse hook was clobbered")
	}

	// Verify new hooks were added
	if _, ok := hooksMap["PostToolUse"]; !ok {
		t.Error("PostToolUse hook not added")
	}

	// Verify non-hook settings preserved
	if settings["model"] != "opus" {
		t.Error("model setting was clobbered")
	}
}

func TestInstallHooksIdempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsPath, []byte("{}"), 0644)

	// Install twice
	InstallHooks(settingsPath)
	InstallHooks(settingsPath)

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	// PostToolUse should have exactly 1 matcher group, not 2
	hooksMap := settings["hooks"].(map[string]interface{})
	ptHooks := hooksMap["PostToolUse"].([]interface{})
	if len(ptHooks) != 1 {
		t.Errorf("PostToolUse has %d groups after double install, want 1", len(ptHooks))
	}
}

func TestUninstallHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	existing := `{
		"hooks": {
			"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo existing"}]}],
			"PostToolUse": [{"hooks": [{"type": "command", "command": "agentjit ingest", "async": true}]}]
		},
		"model": "opus"
	}`
	os.WriteFile(settingsPath, []byte(existing), 0644)

	if err := UninstallHooks(settingsPath); err != nil {
		t.Fatalf("UninstallHooks: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	hooksMap := settings["hooks"].(map[string]interface{})

	// AgentJIT hooks should be removed
	if _, ok := hooksMap["PostToolUse"]; ok {
		t.Error("PostToolUse should have been removed")
	}

	// Existing non-AgentJIT hooks preserved
	if _, ok := hooksMap["PreToolUse"]; !ok {
		t.Error("PreToolUse should have been preserved")
	}

	// Non-hook settings preserved
	if settings["model"] != "opus" {
		t.Error("model was clobbered")
	}
}
