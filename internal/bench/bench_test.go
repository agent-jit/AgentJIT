package bench

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fakeAgent returns a fixed usage (and optional error) without running claude.
type fakeAgent struct {
	usage Usage
	err   error
}

func (f fakeAgent) Run(context.Context, Task) (Usage, error) { return f.usage, f.err }

// fakeVerifier returns a scripted pass/fail per rollout, cycling through verdicts.
type fakeVerifier struct {
	verdicts []bool
	i        int
}

func (v *fakeVerifier) Verify(context.Context, Task) (bool, error) {
	if len(v.verdicts) == 0 {
		return false, nil
	}
	ok := v.verdicts[v.i%len(v.verdicts)]
	v.i++
	return ok, nil
}

func TestUsageTotalTokens(t *testing.T) {
	u := Usage{InputTokens: 100, OutputTokens: 20, CacheCreationInputTokens: 5, CacheReadInputTokens: 3}
	if got := u.TotalTokens(); got != 128 {
		t.Errorf("TotalTokens = %d, want 128", got)
	}
}

// The iso-accuracy gate: only verified rollouts count toward tokens-to-success.
func TestRunTaskGatesOnVerifier(t *testing.T) {
	agent := fakeAgent{usage: Usage{InputTokens: 900, OutputTokens: 100}} // 1000 tokens each
	// 3 rollouts: pass, fail, pass -> 2 verified.
	verifier := &fakeVerifier{verdicts: []bool{true, false, true}}
	rn := Runner{Agent: agent, Verifier: verifier}

	res := rn.RunTask(context.Background(), Task{ID: "t1", Shape: "s"}, ArmBaseline, 3)

	if len(res.Rollouts) != 3 {
		t.Fatalf("rollouts = %d, want 3", len(res.Rollouts))
	}
	if len(res.Verified()) != 2 {
		t.Errorf("verified = %d, want 2", len(res.Verified()))
	}
	if got := res.SuccessRate(); got != 2.0/3.0 {
		t.Errorf("SuccessRate = %v, want 2/3", got)
	}
	mean, ok := res.MeanTokensToSuccess()
	if !ok {
		t.Fatal("MeanTokensToSuccess ok = false, want true")
	}
	if mean != 1000 {
		t.Errorf("MeanTokensToSuccess = %v, want 1000 (verified rollouts only)", mean)
	}
}

// A task that never verifies yields no tokens-to-success — savings can't be
// banked on failures.
func TestMeanTokensToSuccessNoneVerified(t *testing.T) {
	agent := fakeAgent{usage: Usage{InputTokens: 500}}
	verifier := &fakeVerifier{verdicts: []bool{false}}
	rn := Runner{Agent: agent, Verifier: verifier}

	res := rn.RunTask(context.Background(), Task{ID: "t2"}, ArmJIT, 2)

	if _, ok := res.MeanTokensToSuccess(); ok {
		t.Error("MeanTokensToSuccess ok = true, want false when nothing verified")
	}
	if res.SuccessRate() != 0 {
		t.Errorf("SuccessRate = %v, want 0", res.SuccessRate())
	}
}

// An episode error is recorded and never counts as a verified success.
func TestRunTaskRecordsAgentError(t *testing.T) {
	agent := fakeAgent{err: context.DeadlineExceeded}
	rn := Runner{Agent: agent, Verifier: &fakeVerifier{verdicts: []bool{true}}}

	res := rn.RunTask(context.Background(), Task{ID: "t3"}, ArmBaseline, 1)

	if len(res.Rollouts) != 1 || res.Rollouts[0].Err == "" {
		t.Fatalf("expected 1 rollout with Err set, got %+v", res.Rollouts)
	}
	if res.Rollouts[0].Verified {
		t.Error("errored rollout must not be verified")
	}
}

func TestLoadTasks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.jsonl")
	content := `# a comment line
{"id":"add-nullcheck","prompt":"add a null check","shape":"nullcheck","verify":{"command":["true"]}}

{"id":"migrate","prompt":"apply migration","shape":"migrate","verify":{"command":["go","test","./..."]}}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tasks, err := LoadTasks(path)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("loaded %d tasks, want 2 (comment + blank ignored)", len(tasks))
	}
	if tasks[0].ID != "add-nullcheck" || tasks[0].Shape != "nullcheck" {
		t.Errorf("task[0] = %+v", tasks[0])
	}
	if len(tasks[1].Verify.Command) != 3 {
		t.Errorf("task[1].Verify.Command = %v, want 3 elems", tasks[1].Verify.Command)
	}
}

func TestLoadTasksMissingID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(path, []byte(`{"prompt":"no id"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTasks(path); err == nil {
		t.Error("expected error for task missing id, got nil")
	}
}
