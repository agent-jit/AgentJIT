package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-jit/agentjit/internal/stats"
)

func TestCompileCostFromStats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")

	// Two compile sessions: (1000+200) + (2000+400) = 3600 tokens of compile cost.
	// A skill-execution record must NOT contribute to compile cost.
	if err := stats.AppendCompileSession(path, stats.CompileSessionData{InputTokens: 1000, OutputTokens: 200}); err != nil {
		t.Fatal(err)
	}
	if err := stats.AppendCompileSession(path, stats.CompileSessionData{InputTokens: 2000, OutputTokens: 400}); err != nil {
		t.Fatal(err)
	}
	if err := stats.AppendSkillExecution(path, stats.SkillExecutionData{Success: true, EstimatedTokensSaved: 5000}); err != nil {
		t.Fatal(err)
	}

	cost, err := CompileCostFromStats(path)
	if err != nil {
		t.Fatalf("CompileCostFromStats: %v", err)
	}
	if cost != 3600 {
		t.Errorf("compile cost = %d, want 3600 (input+output, skills excluded)", cost)
	}
}

func TestCompileCostFromStatsMissingFile(t *testing.T) {
	cost, err := CompileCostFromStats(filepath.Join(t.TempDir(), "nope.jsonl"))
	if err != nil {
		t.Fatalf("missing stats file should not error: %v", err)
	}
	if cost != 0 {
		t.Errorf("compile cost = %d, want 0 for missing file", cost)
	}
}

// Guard against the record schema drifting away from what CompileCostFromStats
// reads: a hand-written compile record must still be summed.
func TestCompileCostRecordShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.jsonl")
	data, _ := json.Marshal(stats.CompileSessionData{InputTokens: 7, OutputTokens: 3})
	rec := stats.Record{Type: stats.RecordCompileSession, Data: data}
	if err := stats.AppendRecord(path, rec); err != nil {
		t.Fatal(err)
	}
	cost, err := CompileCostFromStats(path)
	if err != nil {
		t.Fatal(err)
	}
	if cost != 10 {
		t.Errorf("compile cost = %d, want 10", cost)
	}
	// sanity: the file really exists and is non-empty
	if fi, err := os.Stat(path); err != nil || fi.Size() == 0 {
		t.Fatalf("stats file missing/empty: %v", err)
	}
}
