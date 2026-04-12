package compile

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/trace"
)

func TestGenerateBashScript_NoParams(t *testing.T) {
	steps := []trace.PatternStep{
		{ToolName: "Bash", Template: "git status"},
		{ToolName: "Bash", Template: "git diff"},
	}

	script := generateBashScript(steps)

	if !strings.HasPrefix(script, "#!/usr/bin/env bash") {
		t.Error("missing shebang")
	}
	if !strings.Contains(script, "set -euo pipefail") {
		t.Error("missing strict mode")
	}
	if !strings.Contains(script, "git status") {
		t.Error("missing 'git status' step")
	}
	if !strings.Contains(script, "git diff") {
		t.Error("missing 'git diff' step")
	}
}

func TestGenerateBashScript_WithParams(t *testing.T) {
	steps := []trace.PatternStep{
		{
			ToolName: "Bash",
			Template: "kubectl get pods -n $NAMESPACE",
			Parameters: []trace.Parameter{
				{Name: "NAMESPACE", Position: 4},
			},
		},
	}

	script := generateBashScript(steps)

	if !strings.Contains(script, "NAMESPACE") {
		t.Error("script should reference NAMESPACE parameter")
	}
	if !strings.Contains(script, "Usage:") || !strings.Contains(script, "NAMESPACE") {
		t.Error("script should include usage message with NAMESPACE")
	}
}

func TestGenerateBashScript_SkipsNonBash(t *testing.T) {
	steps := []trace.PatternStep{
		{ToolName: "Bash", Template: "ls"},
		{ToolName: "Read", Template: "Read call"},
		{ToolName: "Bash", Template: "cat file.txt"},
	}

	script := generateBashScript(steps)

	if !strings.Contains(script, "ls") {
		t.Error("missing 'ls' step")
	}
	if strings.Contains(script, "Read call") {
		t.Error("non-Bash step should be skipped")
	}
	if !strings.Contains(script, "cat file.txt") {
		t.Error("missing 'cat file.txt' step")
	}
}

func TestGeneratePowerShellScript_NoParams(t *testing.T) {
	steps := []trace.PatternStep{
		{ToolName: "Bash", Template: "git status"},
	}

	script := generatePowerShellScript(steps)

	if !strings.Contains(script, "$ErrorActionPreference") {
		t.Error("missing error preference")
	}
	if !strings.Contains(script, "git status") {
		t.Error("missing command")
	}
}

func TestDeterministicBackend_Compile(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	backend := NewDeterministicBackend(paths, "linux")

	patterns := []trace.Pattern{
		{
			Steps: []trace.PatternStep{
				{
					ToolName: "Bash",
					Template: "kubectl get pods -n $NAMESPACE",
					Parameters: []trace.Parameter{
						{Name: "NAMESPACE", Position: 4, Values: []string{"staging", "production"}},
					},
				},
				{
					ToolName: "Bash",
					Template: "kubectl logs -n $NAMESPACE pod/$ARG_1",
					Parameters: []trace.Parameter{
						{Name: "NAMESPACE", Position: 3, Values: []string{"staging", "production"}},
						{Name: "ARG_1", Position: 4, Values: []string{"pod-a", "pod-b"}},
					},
				},
			},
			Frequency:  5,
			SessionIDs: []string{"s1", "s2", "s3", "s4", "s5"},
		},
	}

	results, err := backend.Compile(context.Background(), patterns)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	r := results[0]
	if r.CreatedBy != "aj-deterministic" {
		t.Errorf("CreatedBy = %q, want aj-deterministic", r.CreatedBy)
	}

	// Verify files exist
	skillDir := r.Path
	for _, file := range []string{"SKILL.md", "metadata.json"} {
		if _, err := os.Stat(filepath.Join(skillDir, file)); err != nil {
			t.Errorf("missing %s: %v", file, err)
		}
	}

	// Verify bash script exists
	scriptPath := filepath.Join(skillDir, "scripts", r.Name+".sh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("missing bash script: %v", err)
	}

	// Verify metadata.json content
	metaData, _ := os.ReadFile(filepath.Join(skillDir, "metadata.json"))
	var meta map[string]interface{}
	if err := json.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("parsing metadata.json: %v", err)
	}
	if meta["generated_by"] != "aj-deterministic" {
		t.Errorf("metadata generated_by = %q", meta["generated_by"])
	}
}
