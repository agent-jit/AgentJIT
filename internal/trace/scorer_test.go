package trace

import "testing"

func TestScorePattern_PureBash(t *testing.T) {
	p := Pattern{
		Steps: []PatternStep{
			{ToolName: "Bash", Template: "git status"},
			{ToolName: "Bash", Template: "git diff"},
		},
	}
	score := ScorePattern(p)
	if score != 1.0 {
		t.Errorf("pure bash score = %f, want 1.0", score)
	}
}

func TestScorePattern_NonBashPenalty(t *testing.T) {
	p := Pattern{
		Steps: []PatternStep{
			{ToolName: "Bash", Template: "git status"},
			{ToolName: "Read", Template: "Read call"},
		},
	}
	score := ScorePattern(p)
	if score >= 1.0 {
		t.Errorf("mixed tools should be penalized, got %f", score)
	}
}

func TestScorePattern_DataFlowBonus(t *testing.T) {
	// Use a mixed-tool pattern so base score is < 1.0 and bonus is observable.
	p := Pattern{
		Steps: []PatternStep{
			{ToolName: "Bash", Template: "kubectl get pods"},
			{ToolName: "Read", Template: "read file"},
		},
	}
	baseScore := ScorePattern(p)
	if baseScore >= 1.0 {
		t.Fatalf("test fixture should score below 1.0, got %f", baseScore)
	}

	boostedScore := ScorePatternWithDataFlow(p, 2)
	if boostedScore <= baseScore {
		t.Errorf("data-flow bonus should increase score: base=%f, boosted=%f", baseScore, boostedScore)
	}
}

func TestScorePattern_DataFlowBonusCapped(t *testing.T) {
	p := Pattern{
		Steps: []PatternStep{
			{ToolName: "Bash", Template: "a"},
			{ToolName: "Bash", Template: "b"},
			{ToolName: "Bash", Template: "c"},
			{ToolName: "Bash", Template: "d"},
			{ToolName: "Bash", Template: "e"},
		},
	}
	score := ScorePatternWithDataFlow(p, 10)
	if score > 1.0 {
		t.Errorf("score should not exceed 1.0, got %f", score)
	}
}
