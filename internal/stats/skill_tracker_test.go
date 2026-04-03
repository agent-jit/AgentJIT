package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/agentjit/internal/config"
)

func TestCheckSkillExecution_AJSkill(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	paths.EnsureDirs()

	// Create a skill directory with metadata
	skillDir := filepath.Join(paths.Skills, "deploy-staging")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: deploy-staging\n---\n"), 0644)
	os.WriteFile(filepath.Join(skillDir, "metadata.json"), []byte(`{"version":1,"scope":"global","roi":{"savings_per_invocation":1300,"observed_frequency":5,"total_projected_savings":6500}}`), 0644)

	CheckSkillExecution("Skill", "post_tool_use", "test-session",
		map[string]interface{}{"skill": "deploy-staging"}, paths)

	records, err := ReadAllRecords(paths.Stats)
	if err != nil {
		t.Fatalf("ReadAllRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Type != RecordSkillExecution {
		t.Errorf("expected type %s, got %s", RecordSkillExecution, records[0].Type)
	}

	var data SkillExecutionData
	json.Unmarshal(records[0].Data, &data)
	if data.SkillName != "deploy-staging" {
		t.Errorf("expected skill name deploy-staging, got %s", data.SkillName)
	}
	if !data.Success {
		t.Error("expected success=true")
	}
	if data.EstimatedTokensSaved != 1300 {
		t.Errorf("expected 1300 tokens saved, got %d", data.EstimatedTokensSaved)
	}
}

func TestCheckSkillExecution_Failure(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	paths.EnsureDirs()

	skillDir := filepath.Join(paths.Skills, "deploy-staging")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "metadata.json"), []byte(`{"roi":{"savings_per_invocation":500}}`), 0644)

	CheckSkillExecution("Skill", "post_tool_use_failure", "",
		map[string]interface{}{"skill": "deploy-staging"}, paths)

	records, _ := ReadAllRecords(paths.Stats)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	var data SkillExecutionData
	json.Unmarshal(records[0].Data, &data)
	if data.Success {
		t.Error("expected success=false for post_tool_use_failure")
	}
}

func TestCheckSkillExecution_NonAJSkill(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	paths.EnsureDirs()

	// No skill directory created — this skill is not AJ-generated
	CheckSkillExecution("Skill", "post_tool_use", "",
		map[string]interface{}{"skill": "user-created-skill"}, paths)

	records, _ := ReadAllRecords(paths.Stats)
	if len(records) != 0 {
		t.Fatalf("expected 0 records for non-AJ skill, got %d", len(records))
	}
}

func TestCheckSkillExecution_NonSkillTool(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)

	CheckSkillExecution("Bash", "post_tool_use", "",
		map[string]interface{}{"command": "ls"}, paths)

	records, _ := ReadAllRecords(paths.Stats)
	if len(records) != 0 {
		t.Fatalf("expected 0 records for non-Skill tool, got %d", len(records))
	}
}

func TestCheckSkillExecution_MissingMetadata(t *testing.T) {
	dir := t.TempDir()
	paths := config.PathsFromRoot(dir)
	paths.EnsureDirs()

	// Create skill directory without metadata.json
	skillDir := filepath.Join(paths.Skills, "bare-skill")
	os.MkdirAll(skillDir, 0755)

	CheckSkillExecution("Skill", "post_tool_use", "",
		map[string]interface{}{"skill": "bare-skill"}, paths)

	records, _ := ReadAllRecords(paths.Stats)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	var data SkillExecutionData
	json.Unmarshal(records[0].Data, &data)
	if data.EstimatedTokensSaved != 0 {
		t.Errorf("expected 0 tokens saved (no metadata), got %d", data.EstimatedTokensSaved)
	}
}
