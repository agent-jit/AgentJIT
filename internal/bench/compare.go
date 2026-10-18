package bench

import (
	"sort"
)

// Distribution summarizes the tokens-to-success of verified rollouts. Reporting
// the spread (not just the mean) guards against a low mean hiding high variance —
// a headline break-even built on a noisy mean would mislead.
type Distribution struct {
	N      int     `json:"n"`      // number of verified rollouts
	Mean   float64 `json:"mean"`   // mean tokens-to-success
	Median float64 `json:"median"` // median tokens-to-success
	Min    int     `json:"min"`
	Max    int     `json:"max"`
}

// T2SDistribution summarizes tokens-to-success over the verified rollouts of a
// task. The bool is false when no rollout verified (no distribution to report).
func (tr TaskResult) T2SDistribution() (Distribution, bool) {
	verified := tr.Verified()
	if len(verified) == 0 {
		return Distribution{}, false
	}
	vals := make([]int, len(verified))
	sum := 0
	for i, r := range verified {
		vals[i] = r.Usage.TotalTokens()
		sum += vals[i]
	}
	sort.Ints(vals)

	return Distribution{
		N:      len(vals),
		Mean:   float64(sum) / float64(len(vals)),
		Median: median(vals),
		Min:    vals[0],
		Max:    vals[len(vals)-1],
	}, true
}

func median(sorted []int) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	mid := n / 2
	if n%2 == 1 {
		return float64(sorted[mid])
	}
	return float64(sorted[mid-1]+sorted[mid]) / 2
}

// Comparison is a baseline-vs-JIT result for a single task (or shape). Savings
// are computed at iso-accuracy: if either arm never verified, there is no valid
// per-invocation saving to report.
type Comparison struct {
	Task            string  `json:"task"`
	BaselineMetrics Metrics `json:"baseline_metrics"`
	JITMetrics      Metrics `json:"jit_metrics"`
	// Preserve per-attempt data, setup cost and failures for independent analysis.
	BaselineResult TaskResult `json:"baseline_result"`
	JITResult      TaskResult `json:"jit_result"`
	// Descriptive only: equal observed accuracy is not statistical equivalence.
	LatencyComparable bool     `json:"latency_comparable"`
	WallMeanSavingMS  *float64 `json:"wall_mean_saving_ms,omitempty"`
	Shape             string   `json:"shape,omitempty"`

	BaselineSuccess float64 `json:"baseline_success"`
	JITSuccess      float64 `json:"jit_success"`

	Baseline             Distribution `json:"baseline_t2s"`
	JIT                  Distribution `json:"jit_t2s"`
	BaselineTotalTokens  int          `json:"baseline_total_tokens"`
	JITTotalTokens       int          `json:"jit_total_tokens"`
	BaselineTotalCostUSD float64      `json:"baseline_total_cost_usd"`
	JITTotalCostUSD      float64      `json:"jit_total_cost_usd"`
	UsageComplete        bool         `json:"usage_complete"`

	// PerInvocationSaving = baseline mean attempt tokens − JIT mean attempt tokens (positive => JIT is
	// cheaper). Valid only when Comparable is true.
	PerInvocationSaving float64 `json:"per_invocation_saving"`
	// Comparable requires successes, equal success rates and complete usage.
	Comparable bool `json:"comparable"`
	// IsoAccuracy flags whether the two arms reached the same success rate. When
	// false, the token comparison is not apples-to-apples and must be read with care.
	IsoAccuracy bool `json:"iso_accuracy"`
}

// Compare builds a Comparison from a baseline and JIT TaskResult for one task.
func Compare(baseline, jit TaskResult) Comparison {
	c := Comparison{
		BaselineMetrics: baseline.Metrics(), JITMetrics: jit.Metrics(),
		BaselineResult: baseline, JITResult: jit,
		Task:            baseline.Task,
		Shape:           baseline.Shape,
		BaselineSuccess: baseline.SuccessRate(),
		JITSuccess:      jit.SuccessRate(),
		IsoAccuracy:     baseline.SuccessRate() == jit.SuccessRate(),
		UsageComplete:   true,
	}
	for _, r := range baseline.Rollouts {
		c.BaselineTotalTokens += r.Usage.TotalTokens()
		c.BaselineTotalCostUSD += r.Usage.TotalCostUSD
		if r.Usage.Unavailable || (r.Err != "" && r.Usage.TotalTokens() == 0) {
			c.UsageComplete = false
		}
	}
	for _, r := range jit.Rollouts {
		c.JITTotalTokens += r.Usage.TotalTokens()
		c.JITTotalCostUSD += r.Usage.TotalCostUSD
		if r.Usage.Unavailable || (r.Err != "" && r.Usage.TotalTokens() == 0) {
			c.UsageComplete = false
		}
	}
	bDist, bOK := baseline.T2SDistribution()
	jDist, jOK := jit.T2SDistribution()
	c.Baseline = bDist
	c.JIT = jDist
	if bOK && jOK && c.IsoAccuracy && c.UsageComplete {
		c.Comparable = true
		c.PerInvocationSaving = float64(c.BaselineTotalTokens)/float64(len(baseline.Rollouts)) - float64(c.JITTotalTokens)/float64(len(jit.Rollouts))
	}
	c.LatencyComparable = bOK && jOK && c.IsoAccuracy && len(baseline.Rollouts) == len(jit.Rollouts) && c.BaselineMetrics.Wall.N == len(baseline.Rollouts) && c.JITMetrics.Wall.N == len(jit.Rollouts)
	if c.LatencyComparable {
		delta := c.BaselineMetrics.Wall.Mean - c.JITMetrics.Wall.Mean
		c.WallMeanSavingMS = &delta
	}
	return c
}

// BreakEven reports how many invocations recoup a skill's compile cost.
//
// breakEvenReps = compileCost / perInvocationSaving. It is only meaningful when
// the comparison is valid and the per-invocation saving is positive; a
// non-positive saving means the skill never pays off (returns ok=false).
func (c Comparison) BreakEven(compileCost int) (float64, bool) {
	if !c.Comparable || !c.IsoAccuracy || !c.UsageComplete || c.PerInvocationSaving <= 0 || compileCost < 0 {
		return 0, false
	}
	return float64(compileCost) / c.PerInvocationSaving, true
}
