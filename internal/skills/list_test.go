package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func createTestSkill(t *testing.T, dir, name string, savingsPerInvocation int, frequency int) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	os.MkdirAll(skillDir, 0755)

	skillContent := `---
name: ` + name + `
description: Test skill
---

## Usage
Test skill.
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644)

	meta := map[string]interface{}{
		"generated_by":        "aj",
		"version":             1,
		"scope":               "global",
		"source_pattern_hash": name + "-v1",
		"roi": map[string]interface{}{
			"stochastic_tokens_avg":  18500,
			"deterministic_tokens_avg": 200,
			"savings_per_invocation":  savingsPerInvocation,
			"observed_frequency":      frequency,
			"total_projected_savings": savingsPerInvocation * frequency,
		},
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(filepath.Join(skillDir, "metadata.json"), metaJSON, 0644)
}

func TestListSkills(t *testing.T) {
	dir := t.TempDir()
	createTestSkill(t, dir, "get-logs", 18300, 7)
	createTestSkill(t, dir, "run-tests", 5000, 12)

	skills, err := ListSkills(dir)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}

	// Find get-logs skill and verify ROI was read from metadata.json
	for _, s := range skills {
		if s.Name == "get-logs" {
			if s.SavingsPerInvocation != 18300 {
				t.Errorf("SavingsPerInvocation = %d, want 18300", s.SavingsPerInvocation)
			}
			if s.ObservedFrequency != 7 {
				t.Errorf("ObservedFrequency = %d, want 7", s.ObservedFrequency)
			}
			if s.TotalSavings != 18300*7 {
				t.Errorf("TotalSavings = %d, want %d", s.TotalSavings, 18300*7)
			}
			if s.Scope != "global" {
				t.Errorf("Scope = %q, want global", s.Scope)
			}
			if s.Version != fmt.Sprintf("%d", 1) {
				t.Errorf("Version = %q, want 1", s.Version)
			}
		}
	}
}

func TestListSkillsEmpty(t *testing.T) {
	dir := t.TempDir()

	skills, err := ListSkills(dir)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}
