package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSkillsDir(t *testing.T) {
	dir := t.TempDir()

	// Create a skill directory with skill.md
	skillDir := filepath.Join(dir, "get-logs")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "skill.md"), []byte(`---
name: get-logs
description: Fetch logs from pods
generated_by: agentjit
version: 1
created: 2026-04-01T06:00:00Z
scope: global
roi:
  savings_per_invocation: 18300
  observed_frequency: 7
---

## Usage
Fetch logs.
`), 0644)

	skills, err := ScanSkillsDir(dir)
	if err != nil {
		t.Fatalf("ScanSkillsDir: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}

	s := skills[0]
	if s.Name != "get-logs" {
		t.Errorf("Name = %q, want get-logs", s.Name)
	}
	if s.GeneratedBy != "agentjit" {
		t.Errorf("GeneratedBy = %q, want agentjit", s.GeneratedBy)
	}
}

func TestScanSkillsDirEmpty(t *testing.T) {
	dir := t.TempDir()

	skills, err := ScanSkillsDir(dir)
	if err != nil {
		t.Fatalf("ScanSkillsDir: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}

func TestScanSkillsDirNonExistent(t *testing.T) {
	skills, err := ScanSkillsDir("/nonexistent/dir")
	if err != nil {
		t.Fatalf("ScanSkillsDir: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}
