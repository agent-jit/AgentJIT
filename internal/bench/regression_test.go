package bench

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type agentFunc func(context.Context, Task) (Usage, error)
func (f agentFunc) Run(ctx context.Context, task Task) (Usage, error) { return f(ctx, task) }

func TestRolloutsIsolateWritesAndSetup(t *testing.T) {
	source := t.TempDir()
	setups := 0
	rn := Runner{
		Setup: func(task Task, arm Arm) error {
			setups++
			return os.WriteFile(filepath.Join(task.RepoDir, "skill"), []byte(arm), 0600)
		},
		Agent: agentFunc(func(_ context.Context, task Task) (Usage, error) {
			if task.RepoDir == source { t.Fatal("agent received source directory") }
			if _, err := os.Stat(filepath.Join(task.RepoDir, "mutation")); !os.IsNotExist(err) { t.Fatal("previous attempt leaked") }
			if _, err := os.Stat(filepath.Join(task.RepoDir, "skill")); err != nil { t.Fatal(err) }
			if err := os.WriteFile(filepath.Join(task.RepoDir, "mutation"), nil, 0600); err != nil { t.Fatal(err) }
			return Usage{InputTokens: 23, TotalCostUSD: 0.01}, errors.New("failed after spending tokens")
		}),
		Verifier: &fakeVerifier{verdicts: []bool{true}},
	}
	for _, arm := range []Arm{ArmBaseline, ArmJIT} {
		res := rn.RunTask(context.Background(), Task{ID: "isolation", RepoDir: source}, arm, 3)
		if len(res.Rollouts) != 3 { t.Fatal(res) }
		for _, r := range res.Rollouts {
			if r.Usage.InputTokens != 23 || r.Usage.TotalCostUSD != 0.01 || r.Err == "" || r.Verified { t.Fatalf("lost failed usage: %+v", r) }
		}
	}
	if setups != 2 { t.Fatalf("setup ran %d times, want once per arm", setups) }
	entries, err := os.ReadDir(source)
	if err != nil || len(entries) != 0 { t.Fatalf("source mutated: %v %v", entries, err) }
}

func TestSavingsIncludeFailuresAndGateAccuracy(t *testing.T) {
	b := TaskResult{Rollouts: []RolloutResult{{Usage: Usage{InputTokens: 1000}, Verified: true}, {Usage: Usage{InputTokens: 100}}}}
	j := TaskResult{Rollouts: []RolloutResult{{Usage: Usage{InputTokens: 100}, Verified: true}, {Usage: Usage{InputTokens: 2000}, Err: "failed"}}}
	c := Compare(b, j)
	if !c.Comparable || c.PerInvocationSaving != -500 || c.JITTotalTokens != 2100 { t.Fatalf("failure spend excluded: %+v", c) }
	if _, ok := c.BreakEven(100); ok { t.Fatal("expensive failures incorrectly pay off") }
	j.Rollouts[1].Usage.InputTokens = 10
	j.Rollouts[1].Verified = true
	j.Rollouts[1].Err = ""
	c = Compare(b, j)
	if c.Comparable { t.Fatal("unequal accuracy comparable") }
	if _, ok := c.BreakEven(100); ok { t.Fatal("unequal accuracy pays off") }
	j.Rollouts[1].Verified = false
	j.Rollouts[1].Usage.Unavailable = true
	if Compare(b, j).Comparable { t.Fatal("unknown usage counted as free") }
}

func TestClaudeRunnerRetainsErrorEnvelopeUsage(t *testing.T) {
	if runtime.GOOS == "windows" { t.Skip("test CLI uses a POSIX shell") }
	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, tc := range []struct { name, output, exit string; want int; unknown bool }{
		{"nonzero", `{"usage":{"input_tokens":12},"total_cost_usd":0.01}`, "1", 12, false},
		{"error envelope", `{"is_error":true,"usage":{"input_tokens":12}}`, "0", 12, false},
		{"invalid JSON", `broken`, "1", 0, true},
		{"missing usage", `{}`, "0", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nprintf '%s\\n' '"+tc.output+"'\nexit "+tc.exit+"\n"), 0700); err != nil { t.Fatal(err) }
			u, err := (ClaudeRunner{}).Run(context.Background(), Task{RepoDir: t.TempDir()})
			if err == nil || u.InputTokens != tc.want || u.Unavailable != tc.unknown { t.Fatalf("usage=%+v err=%v", u, err) }
		})
	}
}

type verifierFunc func(context.Context, Task) (bool, error)
func (f verifierFunc) Verify(ctx context.Context, task Task) (bool, error) { return f(ctx, task) }

func TestVerifierSeesOnlyCurrentEpisode(t *testing.T) {
	var active string
	seen := make(map[string]bool)
	rn := Runner{
		Agent: agentFunc(func(_ context.Context, task Task) (Usage, error) {
			if seen[task.RepoDir] { t.Fatal("working directory reused") }
			seen[task.RepoDir] = true
			active = task.RepoDir
			return Usage{InputTokens: 1}, os.WriteFile(filepath.Join(active, "result"), []byte("done"), 0600)
		}),
		Verifier: verifierFunc(func(_ context.Context, task Task) (bool, error) {
			if task.RepoDir != active { t.Fatal("verifier received a different tree") }
			b, err := os.ReadFile(filepath.Join(task.RepoDir, "result"))
			return string(b) == "done", err
		}),
	}
	r := rn.RunTask(context.Background(), Task{ID: "verify", RepoDir: t.TempDir()}, ArmJIT, 2)
	if r.SuccessRate() != 1 { t.Fatalf("verification failed: %+v", r) }
	for dir := range seen {
		if _, err := os.Stat(dir); !os.IsNotExist(err) { t.Fatalf("episode not cleaned up: %s", dir) }
	}
}

func TestCopyTreeSymlinkIsolation(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file"), []byte("seed"), 0600); err != nil { t.Fatal(err) }
	if err := os.Symlink(filepath.Join(src, "file"), filepath.Join(src, "link")); err != nil { t.Skip(err) }
	dst := filepath.Join(t.TempDir(), "copy")
	if err := copyTree(src, dst); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dst, "link"), []byte("changed"), 0600); err != nil { t.Fatal(err) }
	b, err := os.ReadFile(filepath.Join(src, "file"))
	if err != nil || string(b) != "seed" { t.Fatal("symlink escaped snapshot") }
	if err := os.Symlink(t.TempDir(), filepath.Join(src, "external")); err != nil { t.Fatal(err) }
	if err := copyTree(src, filepath.Join(t.TempDir(), "copy")); err == nil { t.Fatal("accepted external symlink") }
}
