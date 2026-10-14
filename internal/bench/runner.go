package bench

import (
	"context"
	"os"
	"path/filepath"
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

// RunTask snapshots the task, runs Setup once on that snapshot, and gives each
// rollout a fresh copy. Agent and verifier share the episode's working tree.
// A rollout whose episode or verifier errors is recorded with Err set and
// Verified=false; it never counts toward tokens-to-success.
func (rn Runner) RunTask(ctx context.Context, task Task, arm Arm, n int) TaskResult {
	res := TaskResult{Task: task.ID, Arm: arm, Shape: task.Shape}
	fail := func(err error) TaskResult {
		for i := 0; i < n; i++ {
			res.Rollouts = append(res.Rollouts, RolloutResult{Task: task.ID, Arm: arm, Index: i, Err: "setup: " + err.Error()})
		}
		return res
	}
	root, err := os.MkdirTemp("", "aj-bench-")
	if err != nil {
		return fail(err)
	}
	defer cleanupTempDir(root)
	src := task.RepoDir
	if src == "" {
		src, err = os.Getwd()
		if err != nil {
			return fail(err)
		}
	}
	task.RepoDir = filepath.Join(root, "seed")
	if err := copyTree(src, task.RepoDir); err != nil {
		return fail(err)
	}
	if rn.Setup != nil {
		if err := rn.Setup(task, arm); err != nil {
			return fail(err)
		}
	}
	for i := 0; i < n; i++ {
		rollout := RolloutResult{Task: task.ID, Arm: arm, Index: i}
		episode := task
		episode.RepoDir, err = os.MkdirTemp(root, "episode-")
		if err != nil {
			rollout.Err = "create episode: " + err.Error()
			res.Rollouts = append(res.Rollouts, rollout)
			continue
		}
		if err := copyTree(task.RepoDir, episode.RepoDir); err != nil {
			rollout.Err = "copy episode: " + err.Error()
			res.Rollouts = append(res.Rollouts, rollout)
			continue
		}
		// Cleanup before the next attempt, including failed attempts.
		run := func() {
			defer cleanupTempDir(episode.RepoDir)

			usage, err := rn.Agent.Run(ctx, episode)
			rollout.Usage = usage
			if err != nil {
				rollout.Err = err.Error()
				return
			}

			ok, err := rn.Verifier.Verify(ctx, episode)
			if err != nil {
				rollout.Err = err.Error()
				return
			}
			rollout.Verified = ok
		}
		run()
		res.Rollouts = append(res.Rollouts, rollout)
	}
	return res
}
