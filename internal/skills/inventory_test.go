package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScanSkillsDir(t *testing.T) {
	dir := t.TempDir()

	// Create a skill directory with SKILL.md and metadata.json
	skillDir := filepath.Join(dir, "get-logs")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: get-logs
description: Fetch logs from pods
---

## Usage
Fetch logs.
`), 0644)

	meta := map[string]interface{}{
		"generated_by":        "aj",
		"version":             1,
		"scope":               "global",
		"source_pattern_hash": "get-logs-v1",
	}
	metaJSON, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(skillDir, "metadata.json"), metaJSON, 0644)

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
	if s.GeneratedBy != "aj" {
		t.Errorf("GeneratedBy = %q, want aj", s.GeneratedBy)
	}
	if s.Scope != "global" {
		t.Errorf("Scope = %q, want global", s.Scope)
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
