// internal/compile/tiered.go
package compile

import (
	"context"
	"log"

	"github.com/agent-jit/agentjit/internal/trace"
)

// TieredCompiler routes patterns to deterministic or LLM backends
// based on confidence scoring.
type TieredCompiler struct {
	deterministic CompilerBackend
	llm           CompilerBackend
	threshold     float64
}

// NewTieredCompiler creates a compiler that routes patterns by confidence score.
func NewTieredCompiler(deterministic, llm CompilerBackend, threshold float64) *TieredCompiler {
	return &TieredCompiler{
		deterministic: deterministic,
		llm:           llm,
		threshold:     threshold,
	}
}

// Compile routes patterns to the appropriate backend and merges results.
func (tc *TieredCompiler) Compile(ctx context.Context, patterns []trace.Pattern) ([]SkillResult, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	detBatch, llmBatch := trace.RoutePatterns(patterns, tc.threshold)

	var allResults []SkillResult

	if len(detBatch) > 0 {
		log.Printf("[AJ] Compiling %d pattern(s) deterministically", len(detBatch))
		results, err := tc.deterministic.Compile(ctx, detBatch)
		if err != nil {
			log.Printf("[AJ] Deterministic compilation failed: %v", err)
		} else {
			allResults = append(allResults, results...)
		}
	}

	if len(llmBatch) > 0 {
		log.Printf("[AJ] Routing %d pattern(s) to LLM compiler", len(llmBatch))
		results, err := tc.llm.Compile(ctx, llmBatch)
		if err != nil {
			log.Printf("[AJ] LLM compilation failed: %v", err)
		} else {
			allResults = append(allResults, results...)
		}
	}

	return allResults, nil
}
