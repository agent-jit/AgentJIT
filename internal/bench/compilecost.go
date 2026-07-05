package bench

import (
	"github.com/agent-jit/agentjit/internal/stats"
)

// CompileCostFromStats returns the total compile-cost tokens (input + output)
// recorded in an AJ stats.jsonl file — the tokens AgentJIT spent building skills.
// This is the denominator for break-even: it should be read from the same
// isolated sandbox (AJ_HOME) whose JIT arm was compiled, so the cost matches the
// skills under test.
//
// A missing stats file means no compilation happened yet and reads as zero cost
// (ReadAllRecords already treats a nonexistent file as empty).
func CompileCostFromStats(statsPath string) (int, error) {
	records, err := stats.ReadAllRecords(statsPath)
	if err != nil {
		return 0, err
	}
	agg := stats.Aggregate(records)
	return agg.CompileInputTokens + agg.CompileOutputTokens, nil
}
