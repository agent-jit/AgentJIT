package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func createTestSkill(t *testing.T, dir, name string, savingsPerInvocation int, frequency int) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	os.MkdirAll(skillDir, 0755)

	content := `---
name: ` + name + `
description: Test skill
generated_by: agentjit
version: 1
created: 2026-04-01T06:00:00Z
scope: global
roi:
  stochastic_tokens_avg: 18500
  deterministic_tokens_avg: 200
  savings_per_invocation: ` + fmt.Sprintf("%d", savingsPerInvocation) + `
  observed_frequency: ` + fmt.Sprintf("%d", frequency) + `
  total_projected_savings: ` + fmt.Sprintf("%d", savingsPerInvocation*frequency) + `
---

## Usage
Test skill.
`
	os.WriteFile(filepath.Join(skillDir, "skill.md"), []byte(content), 0644)
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
