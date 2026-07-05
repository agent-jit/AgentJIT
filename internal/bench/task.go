// Package bench implements AgentJIT's benchmark sandbox: it runs agent tasks
// under a BASELINE arm (no skills) and a JIT arm (skills pre-compiled) and
// compares tokens-to-success at iso-accuracy. See
// docs/superpowers/specs/2026-07-05-benchmark-sandbox-design.md.
package bench

// Arm identifies which benchmark condition an episode runs under.
type Arm string

const (
	// ArmBaseline runs with no compiled skills present — the agent solves cold.
	ArmBaseline Arm = "baseline"
	// ArmJIT runs with skills for the workflow pre-compiled and available.
	ArmJIT Arm = "jit"
)

// Task is a single benchmark case loaded from a JSONL fixture file.
type Task struct {
	// ID uniquely identifies the task (stable across runs, used in reports).
	ID string `json:"id"`
	// Prompt is the instruction handed to the agent.
	Prompt string `json:"prompt"`
	// RepoDir is the working directory the agent operates in (optional).
	RepoDir string `json:"repo_dir,omitempty"`
	// GitSHA pins the repo state for reproducibility (optional; informational
	// unless the harness checks it out).
	GitSHA string `json:"git_sha,omitempty"`
	// Verify declares how to check that a rollout produced a correct result.
	Verify Verification `json:"verify"`
	// Shape groups tasks of the same structural form, so break-even and compile
	// cost can be stratified by shape rather than averaged globally.
	Shape string `json:"shape,omitempty"`
}
