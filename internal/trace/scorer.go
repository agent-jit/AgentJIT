package trace

// ScorePattern evaluates confidence that a pattern can be compiled deterministically.
// Returns a float64 in [0, 1]. Scores >= threshold route to deterministic backend.
func ScorePattern(p Pattern) float64 {
	if len(p.Steps) == 0 {
		return 0
	}

	score := 1.0

	// Penalize non-Bash tools proportionally: more non-Bash steps = lower confidence.
	if len(p.Steps) > 0 {
		nonBash := 0
		for _, step := range p.Steps {
			if step.ToolName != "Bash" {
				nonBash++
			}
		}
		if nonBash > 0 {
			ratio := float64(nonBash) / float64(len(p.Steps))
			score -= 0.3 + 0.5*ratio
		}
	}

	// Average parameters per step
	if len(p.Steps) > 0 {
		totalParams := 0
		for _, step := range p.Steps {
			totalParams += len(step.Parameters)
		}
		avgParams := float64(totalParams) / float64(len(p.Steps))
		if avgParams > 3 {
			score -= 0.2
		}
	}

	// Path length penalty
	if len(p.Steps) > 12 {
		score -= 0.1
	}

	if score < 0 {
		score = 0
	}
	return score
}

// RoutePatterns splits patterns into deterministic and LLM batches
// based on confidence scoring.
func RoutePatterns(patterns []Pattern, threshold float64) (deterministic, llm []Pattern) {
	for _, p := range patterns {
		if ScorePattern(p) >= threshold {
			deterministic = append(deterministic, p)
		} else {
			llm = append(llm, p)
		}
	}
	return
}
