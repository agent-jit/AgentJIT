package bench

import (
	"context"
	"os/exec"
)

// Verification declares how to check that a rollout solved a task. Exactly one
// mode should be set; Command is the primary mode (run tests / a checker that
// exits 0 on success).
type Verification struct {
	// Command runs in the task's RepoDir; exit code 0 means the task is solved.
	// e.g. ["go", "test", "./..."] or ["pytest", "-q"].
	Command []string `json:"command,omitempty"`
}

// Verifier decides whether a completed episode reached a verified-correct
// result. Only verified rollouts count toward tokens-to-success — a skill that
// lowers success rate must not bank "savings" (iso-accuracy gate).
type Verifier interface {
	Verify(ctx context.Context, task Task) (bool, error)
}

// CommandVerifier runs Verification.Command in the task's RepoDir and treats a
// zero exit code as success.
type CommandVerifier struct{}

// Verify runs the task's verification command. A task with no command is
// treated as unverifiable and reported as not-passed, so it can never silently
// count as a success.
func (CommandVerifier) Verify(ctx context.Context, task Task) (bool, error) {
	if len(task.Verify.Command) == 0 {
		return false, nil
	}
	cmd := exec.CommandContext(ctx, task.Verify.Command[0], task.Verify.Command[1:]...)
	cmd.Dir = task.RepoDir
	if err := cmd.Run(); err != nil {
		// A non-zero exit is a legitimate "not solved", not a harness error.
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
