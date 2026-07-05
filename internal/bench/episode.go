package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// Usage mirrors the token accounting reported by `claude --print --output-format
// json` — the same envelope internal/compile parses. Tokens are read from the
// API's reported usage, never estimated.
type Usage struct {
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens"`
	TotalCostUSD             float64 `json:"total_cost_usd"`
	NumTurns                 int     `json:"num_turns"`
	DurationMs               int64   `json:"duration_ms"`
}

// TotalTokens is the context-token count used for tokens-to-success: input +
// output + cache tokens (everything the agent paid to reach the result).
func (u Usage) TotalTokens() int {
	return u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// AgentRunner executes a single agent episode for a task and returns its token
// usage. The production implementation shells out to the `claude` CLI; tests
// inject a fake so the harness logic is verifiable without a live agent or API.
type AgentRunner interface {
	Run(ctx context.Context, task Task) (Usage, error)
}

// claudeEnvelope is the JSON shape emitted by `claude --print --output-format json`.
type claudeEnvelope struct {
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	NumTurns   int    `json:"num_turns"`
	DurationMs int64  `json:"duration_ms"`
	SessionID  string `json:"session_id"`
}

// ClaudeRunner drives an episode via the `claude` CLI, matching how
// internal/compile invokes it. AllowedTools restricts the tool surface;
// ExtraArgs are appended verbatim (e.g. --model, --add-dir).
type ClaudeRunner struct {
	AllowedTools string
	ExtraArgs    []string
}

// Run invokes `claude --print --output-format json -p <prompt>` in the task's
// RepoDir and parses the reported token usage.
func (r ClaudeRunner) Run(ctx context.Context, task Task) (Usage, error) {
	allowed := r.AllowedTools
	if allowed == "" {
		allowed = "Read,Write,Edit,Bash,Glob,Grep"
	}
	args := []string{"--print", "--output-format", "json", "--allowedTools", allowed}
	args = append(args, r.ExtraArgs...)
	args = append(args, "-p", task.Prompt)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = task.RepoDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return Usage{}, fmt.Errorf("running claude for task %s: %w", task.ID, err)
	}

	var env claudeEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		return Usage{}, fmt.Errorf("parsing claude output for task %s: %w", task.ID, err)
	}
	return Usage{
		InputTokens:              env.Usage.InputTokens,
		OutputTokens:             env.Usage.OutputTokens,
		CacheCreationInputTokens: env.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     env.Usage.CacheReadInputTokens,
		TotalCostUSD:             env.TotalCostUSD,
		NumTurns:                 env.NumTurns,
		DurationMs:               env.DurationMs,
	}, nil
}
