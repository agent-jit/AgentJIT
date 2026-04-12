package trace

import "testing"

func TestScorePattern_AllBashHighConfidence(t *testing.T) {
	p := Pattern{
		Steps: []PatternStep{
			{ToolName: "Bash", Template: "git status", Parameters: nil},
			{ToolName: "Bash", Template: "git diff", Parameters: nil},
		},
		Frequency: 5,
	}
	score := ScorePattern(p)
	if score < 0.6 {
		t.Errorf("all-Bash pattern should be high confidence, got %.2f", score)
	}
}

func TestScorePattern_MixedToolsLowConfidence(t *testing.T) {
	p := Pattern{
		Steps: []PatternStep{
			{ToolName: "Bash", Template: "ls", Parameters: nil},
			{ToolName: "Read", Template: "Read call", Parameters: nil},
			{ToolName: "Edit", Template: "Edit call", Parameters: nil},
			{ToolName: "Bash", Template: "make test", Parameters: nil},
		},
		Frequency: 3,
	}
	score := ScorePattern(p)
	if score >= 0.6 {
		t.Errorf("mixed-tool pattern should be low confidence, got %.2f", score)
	}
}

func TestScorePattern_TooManyParams(t *testing.T) {
	p := Pattern{
		Steps: []PatternStep{
			{ToolName: "Bash", Template: "cmd $A $B $C $D", Parameters: make([]Parameter, 4)},
			{ToolName: "Bash", Template: "cmd $E $F $G $H", Parameters: make([]Parameter, 4)},
		},
		Frequency: 3,
	}
	score := ScorePattern(p)
	// avgParamsPerStep = 4 > 3, should lose 0.2
	if score >= 1.0 {
		t.Errorf("many-param pattern should have reduced score, got %.2f", score)
	}
}

func TestScorePattern_LongPathPenalty(t *testing.T) {
	steps := make([]PatternStep, 15)
	for i := range steps {
		steps[i] = PatternStep{ToolName: "Bash", Template: "cmd"}
	}
	p := Pattern{Steps: steps, Frequency: 3}
	score := ScorePattern(p)
	// pathLength > 12, should lose 0.1
	if score >= 1.0 {
		t.Errorf("long pattern should have reduced score, got %.2f", score)
	}
}

func TestRoutePatterns(t *testing.T) {
	high := Pattern{
		Steps:     []PatternStep{{ToolName: "Bash", Template: "git status"}},
		Frequency: 5,
	}
	low := Pattern{
		Steps: []PatternStep{
			{ToolName: "Bash"}, {ToolName: "Read"}, {ToolName: "Edit"},
			{ToolName: "Bash"}, {ToolName: "Read"}, {ToolName: "Edit"},
			{ToolName: "Bash"}, {ToolName: "Read"}, {ToolName: "Edit"},
			{ToolName: "Bash"}, {ToolName: "Read"}, {ToolName: "Edit"},
			{ToolName: "Bash"},
		},
		Frequency: 3,
	}

	det, llm := RoutePatterns([]Pattern{high, low}, 0.6)

	if len(det) != 1 {
		t.Errorf("expected 1 deterministic pattern, got %d", len(det))
	}
	if len(llm) != 1 {
		t.Errorf("expected 1 LLM pattern, got %d", len(llm))
	}
}
