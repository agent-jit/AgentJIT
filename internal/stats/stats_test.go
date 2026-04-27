package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendAndReadRecords(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.jsonl")

	// Append compile session
	err := AppendCompileSession(statsPath, CompileSessionData{
		SessionID:   "sess-1",
		InputTokens: 1000,
		OutputTokens: 200,
		TotalCostUSD: 0.15,
	})
	if err != nil {
		t.Fatalf("AppendCompileSession: %v", err)
	}

	// Append skill execution (success)
	err = AppendSkillExecution(statsPath, SkillExecutionData{
		SkillName:            "deploy-staging",
		Success:              true,
		EstimatedTokensSaved: 500,
	})
	if err != nil {
		t.Fatalf("AppendSkillExecution: %v", err)
	}

	// Append skill execution (failure)
	err = AppendSkillExecution(statsPath, SkillExecutionData{
		SkillName: "deploy-staging",
		Success:   false,
	})
	if err != nil {
		t.Fatalf("AppendSkillExecution: %v", err)
	}

	records, err := ReadAllRecords(statsPath)
	if err != nil {
		t.Fatalf("ReadAllRecords: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	if records[0].Type != RecordCompileSession {
		t.Errorf("record 0: expected type %s, got %s", RecordCompileSession, records[0].Type)
	}
	if records[1].Type != RecordSkillExecution {
		t.Errorf("record 1: expected type %s, got %s", RecordSkillExecution, records[1].Type)
	}
}

func TestReadNonExistentFile(t *testing.T) {
	records, err := ReadAllRecords("/nonexistent/stats.jsonl")
	if err != nil {
		t.Fatalf("expected nil error for nonexistent file, got %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestReadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.jsonl")
	if err := os.WriteFile(statsPath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	records, err := ReadAllRecords(statsPath)
	if err != nil {
		t.Fatalf("ReadAllRecords: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestReadSkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.jsonl")

	content := `{"type":"compile_session","timestamp":"2026-04-01T00:00:00Z","data":{}}
this is not json
{"type":"skill_execution","timestamp":"2026-04-01T00:00:00Z","data":{}}
`
	if err := os.WriteFile(statsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	records, err := ReadAllRecords(statsPath)
	if err != nil {
		t.Fatalf("ReadAllRecords: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records (skipping malformed), got %d", len(records))
	}
}

func TestAggregate(t *testing.T) {
	compile1, _ := json.Marshal(CompileSessionData{InputTokens: 1000, OutputTokens: 200, TotalCostUSD: 0.15})
	compile2, _ := json.Marshal(CompileSessionData{InputTokens: 2000, OutputTokens: 400, TotalCostUSD: 0.30})
	skill1, _ := json.Marshal(SkillExecutionData{Success: true, EstimatedTokensSaved: 500})
	skill2, _ := json.Marshal(SkillExecutionData{Success: true, EstimatedTokensSaved: 300})
	skill3, _ := json.Marshal(SkillExecutionData{Success: false, FailureCategory: "script_error", FailureReason: "file not found"})
	skill4, _ := json.Marshal(SkillExecutionData{Success: false, FailureCategory: "target_failure", FailureReason: "exit code 1"})

	records := []Record{
		{Type: RecordCompileSession, Data: compile1},
		{Type: RecordCompileSession, Data: compile2},
		{Type: RecordSkillExecution, Data: skill1},
		{Type: RecordSkillExecution, Data: skill2},
		{Type: RecordSkillExecution, Data: skill3},
		{Type: RecordSkillExecution, Data: skill4},
	}

	agg := Aggregate(records)

	if agg.CompileSessions != 2 {
		t.Errorf("CompileSessions: expected 2, got %d", agg.CompileSessions)
	}
	if agg.CompileInputTokens != 3000 {
		t.Errorf("CompileInputTokens: expected 3000, got %d", agg.CompileInputTokens)
	}
	if agg.CompileOutputTokens != 600 {
		t.Errorf("CompileOutputTokens: expected 600, got %d", agg.CompileOutputTokens)
	}
	if agg.SkillExecutions != 4 {
		t.Errorf("SkillExecutions: expected 4, got %d", agg.SkillExecutions)
	}
	if agg.SkillSuccesses != 2 {
		t.Errorf("SkillSuccesses: expected 2, got %d", agg.SkillSuccesses)
	}
	if agg.SkillFailures != 2 {
		t.Errorf("SkillFailures: expected 2, got %d", agg.SkillFailures)
	}
	if agg.SkillScriptErrors != 1 {
		t.Errorf("SkillScriptErrors: expected 1, got %d", agg.SkillScriptErrors)
	}
	if agg.SkillTargetFailures != 1 {
		t.Errorf("SkillTargetFailures: expected 1, got %d", agg.SkillTargetFailures)
	}
	if agg.EstTokensSaved != 800 {
		t.Errorf("EstTokensSaved: expected 800, got %d", agg.EstTokensSaved)
	}
}

func TestAppendCompileSession_DeterministicFields(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "stats.jsonl")

	data := CompileSessionData{
		SessionID:             "det-1",
		SkillsCreated:        2,
		DeterministicPatterns: 2,
		LLMPatterns:          0,
	}

	if err := AppendCompileSession(statsPath, data); err != nil {
		t.Fatalf("AppendCompileSession: %v", err)
	}

	records, err := ReadAllRecords(statsPath)
	if err != nil {
		t.Fatalf("ReadAllRecords: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	var parsed CompileSessionData
	if err := json.Unmarshal(records[0].Data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.DeterministicPatterns != 2 {
		t.Errorf("DeterministicPatterns = %d, want 2", parsed.DeterministicPatterns)
	}
	if parsed.LLMPatterns != 0 {
		t.Errorf("LLMPatterns = %d, want 0", parsed.LLMPatterns)
	}
}

func TestAppendToNonExistentDir(t *testing.T) {
	dir := t.TempDir()
	statsPath := filepath.Join(dir, "subdir", "stats.jsonl")

	err := AppendCompileSession(statsPath, CompileSessionData{
		SessionID:   "sess-1",
		InputTokens: 100,
	})
	if err != nil {
		t.Fatalf("AppendCompileSession to nested dir: %v", err)
	}

	records, err := ReadAllRecords(statsPath)
	if err != nil {
		t.Fatalf("ReadAllRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
}
