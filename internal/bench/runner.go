package bench

import (
	"context"
)

// RolloutResult is the outcome of a single agent episode on a task.
type RolloutResult struct {
	Task     string `json:"task"`
	Arm      Arm    `json:"arm"`
	Index    int    `json:"index"`
	Usage    Usage  `json:"usage"`
	Verified bool   `json:"verified"`
	// Err is non-empty if the episode or verifier errored (distinct from an
	// unverified-but-clean rollout).
	Err string `json:"err,omitempty"`
}

// TaskResult aggregates N rollouts of one task under one arm.
type TaskResult struct {
	Task     string          `json:"task"`
	Arm      Arm             `json:"arm"`
	Shape    string          `json:"shape,omitempty"`
	Rollouts []RolloutResult `json:"rollouts"`
}

// Verified returns the rollouts that reached a verified-correct result.
func (tr TaskResult) Verified() []RolloutResult {
	var out []RolloutResult
	for _, r := range tr.Rollouts {
		if r.Verified {
			out = append(out, r)
		}
	}
	return out
}

// SuccessRate is the fraction of rollouts that verified. Reported alongside
// tokens so arms are only ever compared at iso-accuracy.
func (tr TaskResult) SuccessRate() float64 {
	if len(tr.Rollouts) == 0 {
		return 0
	}
	return float64(len(tr.Verified())) / float64(len(tr.Rollouts))
}

// MeanTokensToSuccess is the average total tokens over verified rollouts only —
// an unverified rollout is a failure, not a cheap success, so it does not count.
// Returns 0 and false when no rollout verified.
func (tr TaskResult) MeanTokensToSuccess() (float64, bool) {
	verified := tr.Verified()
	if len(verified) == 0 {
		return 0, false
	}
	var sum int
	for _, r := range verified {
		sum += r.Usage.TotalTokens()
	}
	return float64(sum) / float64(len(verified)), true
}

// Runner executes tasks through an AgentRunner and a Verifier.
type Runner struct {
	Agent    AgentRunner
	Verifier Verifier
	// Setup, if set, runs once before a task's rollouts begin — used to prepare
	// arm-specific state (e.g. install the JIT arm's skill). A Setup error fails
	// every rollout of that task (recorded, not counted).
	Setup func(task Task, arm Arm) error
}

// RunTask runs a task under one arm for n rollouts, gating each on the verifier.
// A rollout whose episode or verifier errors is recorded with Err set and
// Verified=false; it never counts toward tokens-to-success.
func (rn Runner) RunTask(ctx context.Context, task Task, arm Arm, n int) TaskResult {
	res := TaskResult{Task: task.ID, Arm: arm, Shape: task.Shape}
	if rn.Setup != nil {
		if err := rn.Setup(task, arm); err != nil {
			for i := 0; i < n; i++ {
				res.Rollouts = append(res.Rollouts, RolloutResult{
					Task: task.ID, Arm: arm, Index: i, Err: "setup: " + err.Error(),
				})
			}
			return res
		}
	}
	for i := 0; i < n; i++ {
		rollout := RolloutResult{Task: task.ID, Arm: arm, Index: i}

		usage, err := rn.Agent.Run(ctx, task)
		if err != nil {
			rollout.Err = err.Error()
			res.Rollouts = append(res.Rollouts, rollout)
			continue
		}
		rollout.Usage = usage

		ok, err := rn.Verifier.Verify(ctx, task)
		if err != nil {
			rollout.Err = err.Error()
			res.Rollouts = append(res.Rollouts, rollout)
			continue
		}
		rollout.Verified = ok
		res.Rollouts = append(res.Rollouts, rollout)
	}
	return res
}
