package bench

import "testing"

// taskResult builds a TaskResult with one rollout per (tokens, verified) pair.
func taskResult(task string, arm Arm, shape string, rollouts []struct {
	tokens   int
	verified bool
}) TaskResult {
	tr := TaskResult{Task: task, Arm: arm, Shape: shape}
	for i, r := range rollouts {
		tr.Rollouts = append(tr.Rollouts, RolloutResult{
			Task:     task,
			Arm:      arm,
			Index:    i,
			Usage:    Usage{InputTokens: r.tokens},
			Verified: r.verified,
		})
	}
	return tr
}

func TestT2SDistribution(t *testing.T) {
	tr := taskResult("t", ArmBaseline, "s", []struct {
		tokens   int
		verified bool
	}{
		{tokens: 100, verified: true},
		{tokens: 300, verified: true},
		{tokens: 200, verified: true},
		{tokens: 999, verified: false}, // excluded
	})

	d, ok := tr.T2SDistribution()
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if d.N != 3 {
		t.Errorf("N = %d, want 3 (verified only)", d.N)
	}
	if d.Mean != 200 {
		t.Errorf("Mean = %v, want 200", d.Mean)
	}
	if d.Median != 200 {
		t.Errorf("Median = %v, want 200", d.Median)
	}
	if d.Min != 100 || d.Max != 300 {
		t.Errorf("Min/Max = %d/%d, want 100/300", d.Min, d.Max)
	}
}

func TestMedianEven(t *testing.T) {
	if got := median([]int{10, 20, 30, 40}); got != 25 {
		t.Errorf("median = %v, want 25", got)
	}
}

func TestCompareSavingAndBreakEven(t *testing.T) {
	mk := func(arm Arm, tokens int) TaskResult {
		return taskResult("rename", arm, "shapeA", []struct {
			tokens   int
			verified bool
		}{
			{tokens: tokens, verified: true},
			{tokens: tokens, verified: true},
		})
	}
	baseline := mk(ArmBaseline, 1000)
	jit := mk(ArmJIT, 400)

	c := Compare(baseline, jit)
	if !c.Comparable {
		t.Fatal("Comparable = false, want true")
	}
	if !c.IsoAccuracy {
		t.Error("IsoAccuracy = false, want true (both 100%)")
	}
	if c.PerInvocationSaving != 600 {
		t.Errorf("PerInvocationSaving = %v, want 600", c.PerInvocationSaving)
	}

	// compile cost 3000, saving 600/use => break-even at 5 invocations.
	be, ok := c.BreakEven(3000)
	if !ok {
		t.Fatal("BreakEven ok = false, want true")
	}
	if be != 5 {
		t.Errorf("BreakEven = %v, want 5", be)
	}
}

// If either arm never verifies, there is no valid comparison.
func TestCompareNotComparableWhenArmUnverified(t *testing.T) {
	baseline := taskResult("t", ArmBaseline, "s", []struct {
		tokens   int
		verified bool
	}{{tokens: 1000, verified: true}})
	jit := taskResult("t", ArmJIT, "s", []struct {
		tokens   int
		verified bool
	}{{tokens: 400, verified: false}}) // JIT never solved

	c := Compare(baseline, jit)
	if c.Comparable {
		t.Error("Comparable = true, want false (JIT never verified)")
	}
	if _, ok := c.BreakEven(1000); ok {
		t.Error("BreakEven ok = true, want false when not comparable")
	}
}

// A skill that doesn't actually save (or costs more) never breaks even.
func TestBreakEvenNonPositiveSaving(t *testing.T) {
	mk := func(arm Arm, tokens int) TaskResult {
		return taskResult("t", arm, "s", []struct {
			tokens   int
			verified bool
		}{{tokens: tokens, verified: true}})
	}
	c := Compare(mk(ArmBaseline, 400), mk(ArmJIT, 500)) // JIT costs more
	if c.PerInvocationSaving >= 0 {
		t.Errorf("PerInvocationSaving = %v, want negative", c.PerInvocationSaving)
	}
	if _, ok := c.BreakEven(1000); ok {
		t.Error("BreakEven ok = true, want false for non-positive saving")
	}
}
