package bench

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"
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
	Err           string `json:"err,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
	// Nil timings mean execution never started, not zero latency.
	AgentMS      *float64 `json:"agent_ms,omitempty"`
	VerifyMS     *float64 `json:"verify_ms,omitempty"`
	WallMS       *float64 `json:"wall_ms,omitempty"`
	ResultDigest string   `json:"result_digest,omitempty"`
	DigestError  string   `json:"digest_error,omitempty"`
}

// TaskResult aggregates N rollouts of one task under one arm.
type TaskResult struct {
	Task     string          `json:"task"`
	Arm      Arm             `json:"arm"`
	Shape    string          `json:"shape,omitempty"`
	Rollouts []RolloutResult `json:"rollouts"`
	SetupMS  float64         `json:"setup_ms"`
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
	// Timeout bounds agent plus verifier per attempt; zero uses the caller deadline.
	Timeout time.Duration
}

// RunTask snapshots the task, runs Setup once on that snapshot, and gives each
// rollout a fresh copy. Agent and verifier share the episode's working tree.
// A rollout whose episode or verifier errors is recorded with Err set and
// Verified=false; it never counts toward tokens-to-success.
func (rn Runner) RunTask(ctx context.Context, task Task, arm Arm, n int) TaskResult {
	prepared, cleanup, setupMS, err := rn.prepare(task, arm)
	defer cleanup()
	res := TaskResult{Task: task.ID, Arm: arm, Shape: task.Shape, SetupMS: setupMS}
	for i := 0; i < n; i++ {
		res.Rollouts = append(res.Rollouts, rn.runOne(ctx, prepared, arm, i, err))
	}
	return res
}

// RunPaired prepares each arm once, then alternates AB / BA to reduce order drift.
// Both arms must use identical prompts, verification and result projections.
func (rn Runner) RunPaired(ctx context.Context, baseline, jit Task, n int) (TaskResult, TaskResult, error) {
	if baseline.ID != jit.ID || baseline.Prompt != jit.Prompt || !reflect.DeepEqual(baseline.Verify, jit.Verify) || !reflect.DeepEqual(baseline.ResultFiles, jit.ResultFiles) {
		return TaskResult{}, TaskResult{}, fmt.Errorf("paired tasks require identical IDs, prompts, verification and result files")
	}
	b, bc, bms, be := rn.prepare(baseline, ArmBaseline)
	defer bc()
	j, jc, jms, je := rn.prepare(jit, ArmJIT)
	defer jc()
	br := TaskResult{Task: baseline.ID, Shape: baseline.Shape, Arm: ArmBaseline, SetupMS: bms}
	jr := TaskResult{Task: jit.ID, Shape: jit.Shape, Arm: ArmJIT, SetupMS: jms}
	for i := 0; i < n; i++ {
		rb := func() { br.Rollouts = append(br.Rollouts, rn.runOne(ctx, b, ArmBaseline, i, be)) }
		rj := func() { jr.Rollouts = append(jr.Rollouts, rn.runOne(ctx, j, ArmJIT, i, je)) }
		if i%2 == 0 {
			rb()
			rj()
		} else {
			rj()
			rb()
		}
	}
	return br, jr, nil
}

func (rn Runner) prepare(task Task, arm Arm) (Task, func(), float64, error) {
	start := time.Now()
	root, err := os.MkdirTemp("", "aj-bench-")
	if err != nil {
		return task, func() {}, elapsedMS(start), err
	}
	cleanup := func() { cleanupTempDir(root) }
	src := task.RepoDir
	if src == "" {
		src, err = os.Getwd()
	}
	task.RepoDir = filepath.Join(root, "seed")
	if err == nil {
		err = copyTree(src, task.RepoDir)
	}
	if err == nil && rn.Setup != nil {
		err = rn.Setup(task, arm)
	}
	return task, cleanup, elapsedMS(start), err
}

func elapsedMS(start time.Time) float64 {
	return float64(time.Since(start)) / float64(time.Millisecond)
}

func failureReason(ctx context.Context, err error, fallback string) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return "canceled"
	}
	return fallback
}

func (rn Runner) runOne(ctx context.Context, task Task, arm Arm, index int, setupErr error) RolloutResult {
	r := RolloutResult{Task: task.ID, Arm: arm, Index: index}
	if setupErr != nil {
		r.Err = setupErr.Error()
		r.FailureReason = "setup_error"
		r.Usage.Unavailable = true
		return r
	}
	episode := task
	dir, err := os.MkdirTemp(filepath.Dir(task.RepoDir), "episode-")
	if err == nil {
		defer cleanupTempDir(dir)
		err = copyTree(task.RepoDir, dir)
	}
	if err != nil {
		r.Err = err.Error()
		r.FailureReason = "isolation_error"
		r.Usage.Unavailable = true
		return r
	}
	episode.RepoDir = dir
	if rn.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rn.Timeout)
		defer cancel()
	}
	start := time.Now()
	usage, agentErr := rn.Agent.Run(ctx, episode)
	agentMS := elapsedMS(start)
	r.Usage, r.AgentMS = usage, &agentMS
	if agentErr != nil || ctx.Err() != nil {
		if agentErr == nil {
			agentErr = ctx.Err()
		}
		r.Err = agentErr.Error()
		r.FailureReason = failureReason(ctx, agentErr, "agent_error")
	} else {
		verifyStart := time.Now()
		ok, verifyErr := rn.Verifier.Verify(ctx, episode)
		verifyMS := elapsedMS(verifyStart)
		r.VerifyMS = &verifyMS
		switch {
		case verifyErr != nil || ctx.Err() != nil:
			if verifyErr == nil {
				verifyErr = ctx.Err()
			}
			r.Err = verifyErr.Error()
			r.FailureReason = failureReason(ctx, verifyErr, "verifier_error")
		case !ok:
			r.FailureReason = "verification_failed"
		default:
			r.Verified = true
		}
	}
	wallMS := elapsedMS(start)
	r.WallMS = &wallMS
	// Digest collection is outside task latency and cannot change the verifier verdict.
	if r.Verified && len(task.ResultFiles) > 0 {
		r.ResultDigest, err = resultDigest(episode)
		if err != nil {
			r.DigestError = err.Error()
		}
	}
	return r
}
