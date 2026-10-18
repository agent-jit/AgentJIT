package bench

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func ms(v float64) *float64 { return &v }

func TestMetricsIncludeFailuresAndMissingTiming(t *testing.T) {
	tr := TaskResult{Rollouts: []RolloutResult{
		{Verified: true, WallMS: ms(100), ResultDigest: "a"},
		{Verified: true, WallMS: ms(200), ResultDigest: "a"},
		{Verified: true, WallMS: ms(300), ResultDigest: "b"},
		{WallMS: ms(1000), FailureReason: "timeout"},
		{FailureReason: "setup_error"},
	}}
	m := tr.Metrics()
	if m.Wall.N != 4 || m.Wall.P50 != 200 || m.Wall.P95 != 1000 || m.Wall.Mean != 400 || m.SuccessfulWall.N != 3 {
		t.Fatalf("bad latency: %+v", m)
	}
	r := m.Reliability
	if r.SuccessRate != .6 || !r.MixedOutcomes || r.Failures["timeout"] != 1 || r.Failures["setup_error"] != 1 || r.ResultAgreement == nil || math.Abs(*r.ResultAgreement-1.0/3) > 1e-9 {
		t.Fatalf("bad reliability: %+v", r)
	}
	if math.Abs(r.SuccessLow-.23072428) > 1e-6 || math.Abs(r.SuccessHigh-.88237923) > 1e-6 {
		t.Fatalf("bad Wilson interval: %+v", r)
	}
	if (TaskResult{}).Metrics().Reliability.ResultAgreement != nil {
		t.Fatal("empty result claims stability")
	}
}

func TestLatencyComparisonIndependentOfTokens(t *testing.T) {
	b := TaskResult{Rollouts: []RolloutResult{{Verified: true, WallMS: ms(100), Usage: Usage{Unavailable: true}}}}
	j := TaskResult{Rollouts: []RolloutResult{{Verified: true, WallMS: ms(50)}}}
	c := Compare(b, j)
	if c.Comparable || !c.LatencyComparable || *c.WallMeanSavingMS != 50 {
		t.Fatalf("bad gate: %+v", c)
	}
	j.Rollouts[0].Verified = false
	if Compare(b, j).LatencyComparable {
		t.Fatal("different accuracy comparable")
	}
	j.Rollouts[0].Verified = true
	j.Rollouts[0].WallMS = nil
	if Compare(b, j).WallMeanSavingMS != nil {
		t.Fatal("missing latency treated as zero")
	}
}

func TestRunnerFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name                string
		agentErr, verifyErr error
		pass                bool
		reason              string
	}{
		{"agent", errors.New("agent failed"), nil, false, "agent_error"},
		{"timeout", context.DeadlineExceeded, nil, false, "timeout"},
		{"cancel", context.Canceled, nil, false, "canceled"},
		{"verifier", nil, errors.New("cannot start verifier"), false, "verifier_error"},
		{"wrong result", nil, nil, false, "verification_failed"},
		{"success", nil, nil, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rn := Runner{Agent: fakeAgent{err: tc.agentErr}, Verifier: verifierFunc(func(context.Context, Task) (bool, error) { return tc.pass, tc.verifyErr })}
			r := rn.RunTask(context.Background(), Task{RepoDir: t.TempDir()}, ArmBaseline, 1).Rollouts[0]
			if r.FailureReason != tc.reason || r.AgentMS == nil || r.WallMS == nil || *r.WallMS < *r.AgentMS {
				t.Fatalf("bad outcome: %+v", r)
			}
			if (r.VerifyMS == nil) != (tc.agentErr != nil) {
				t.Fatalf("bad verifier timing: %+v", r)
			}
		})
	}
	rn := Runner{Setup: func(Task, Arm) error { return errors.New("setup") }}
	r := rn.RunTask(context.Background(), Task{RepoDir: t.TempDir()}, ArmJIT, 1).Rollouts[0]
	if r.WallMS != nil || r.FailureReason != "setup_error" || !r.Usage.Unavailable {
		t.Fatalf("setup: %+v", r)
	}
}

func TestRunnerDeadline(t *testing.T) {
	rn := Runner{Timeout: time.Millisecond, Agent: agentFunc(func(ctx context.Context, _ Task) (Usage, error) { <-ctx.Done(); return Usage{}, ctx.Err() })}
	r := rn.RunTask(context.Background(), Task{RepoDir: t.TempDir()}, ArmBaseline, 1).Rollouts[0]
	if r.FailureReason != "timeout" || r.WallMS == nil {
		t.Fatalf("deadline: %+v", r)
	}
}

func TestPairedOrderAndContract(t *testing.T) {
	var order []string
	setups := 0
	rn := Runner{
		Setup: func(task Task, arm Arm) error {
			setups++
			return os.WriteFile(filepath.Join(task.RepoDir, "arm"), []byte(arm), 0600)
		},
		Agent: agentFunc(func(_ context.Context, task Task) (Usage, error) {
			b, err := os.ReadFile(filepath.Join(task.RepoDir, "arm"))
			order = append(order, string(b))
			return Usage{}, err
		}),
		Verifier: &fakeVerifier{verdicts: []bool{true}},
	}
	task := Task{ID: "same", RepoDir: t.TempDir()}
	b, j, err := rn.RunPaired(context.Background(), task, task, 3)
	if err != nil || setups != 2 || len(b.Rollouts) != 3 || len(j.Rollouts) != 3 || !reflect.DeepEqual(order, []string{"baseline", "jit", "jit", "baseline", "baseline", "jit"}) {
		t.Fatalf("pairing: %v %v setups=%d", err, order, setups)
	}
	other := task
	other.Verify.Command = []string{"true"}
	if _, _, err := rn.RunPaired(context.Background(), task, other, 3); err == nil {
		t.Fatal("accepted different verifiers")
	}
}

func TestResultDigest(t *testing.T) {
	task := Task{RepoDir: t.TempDir(), ResultFiles: []string{"result"}}
	if err := os.WriteFile(filepath.Join(task.RepoDir, "result"), []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := resultDigest(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(task.RepoDir, "result"), []byte("b"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := resultDigest(task)
	if err != nil || a == b {
		t.Fatal("digest did not change", err)
	}
	task.ResultFiles = []string{"../escape"}
	if _, err := resultDigest(task); err == nil {
		t.Fatal("accepted escape")
	}
	task.ResultFiles = []string{"missing"}
	if _, err := resultDigest(task); err == nil {
		t.Fatal("accepted missing output")
	}
}

// Scripted integration tests exercise real compilation and 20 isolated repeats
// per arm. They test the harness, NOT model latency or a JIT performance gain.
func TestRepeatedCompiledFixtures(t *testing.T) {
	aj := buildAJ(t)
	for _, tc := range []struct {
		fx           CompilableFixture
		n            int
		jitSuccesses int
	}{
		{ShellSeqFixture{}, 1, 20},
		// The current compiler selects the mkdir+first-touch prefix. A script-only
		// agent must fail n=2 rather than receiving credit for an incomplete result.
		{ShellSeqFixture{}, 2, 0},
		{AKSOpsFixture{}, 2, 20},
	} {
		fx := tc.fx
		t.Run(fmt.Sprintf("%s-%d", fx.Shape(), tc.n), func(t *testing.T) {
			task, err := fx.Generate(t.TempDir(), tc.n)
			if err != nil {
				t.Fatal(err)
			}
			rn := Runner{Verifier: CommandVerifier{}, Setup: func(task Task, arm Arm) error {
				if arm == ArmJIT {
					_, err := CompileSkill(context.Background(), aj, fx, task)
					return err
				}
				return nil
			}, Agent: agentFunc(func(ctx context.Context, task Task) (Usage, error) {
				script := filepath.Join(".claude", "skills", fx.SkillName(), "scripts", fx.SkillName()+".sh")
				var commands []string
				if _, err := os.Stat(filepath.Join(task.RepoDir, script)); err == nil {
					commands = []string{"bash " + script}
				} else {
					switch f := fx.(type) {
					case ShellSeqFixture:
						commands = f.commands(tc.n)
					case AKSOpsFixture:
						commands = f.commands(tc.n)
					}
				}
				for _, command := range commands {
					cmd := exec.CommandContext(ctx, "sh", "-c", command)
					cmd.Dir = task.RepoDir
					if out, err := cmd.CombinedOutput(); err != nil {
						return Usage{}, errors.New(string(out) + err.Error())
					}
				}
				return Usage{}, nil
			})}
			b, j, err := rn.RunPaired(context.Background(), task, task, 20)
			if err != nil {
				t.Fatal(err)
			}
			for _, res := range []TaskResult{b, j} {
				m := res.Metrics()
				want := 20
				if res.Arm == ArmJIT {
					want = tc.jitSuccesses
				}
				if m.Reliability.Successes != want || m.Wall.N != 20 {
					t.Fatalf("repeat failed: %+v %+v", m, res.Rollouts)
				}
			}
			if tc.jitSuccesses == 0 {
				if j.Metrics().Reliability.Failures["verification_failed"] != 20 || j.Metrics().Reliability.ResultAgreement != nil || Compare(b, j).LatencyComparable {
					t.Fatal("incomplete script received credit")
				}
			} else if b.Rollouts[0].ResultDigest != j.Rollouts[0].ResultDigest || j.Metrics().Reliability.ResultAgreement == nil || *j.Metrics().Reliability.ResultAgreement != 1 {
				t.Fatal("arms produced different results")
			}
		})
	}
}

func TestShellSeqRejectsWrongNamesAndContents(t *testing.T) {
	task, err := (ShellSeqFixture{}).Generate(t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(task.RepoDir, "out")
	if err := os.Mkdir(out, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"marker5.txt", "marker6.txt"} {
		if err := os.WriteFile(filepath.Join(out, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if ok, err := (CommandVerifier{}).Verify(context.Background(), task); err != nil || ok {
		t.Fatalf("wrong names accepted: %v", err)
	}
	if err := os.RemoveAll(out); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(out, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"marker0.txt", "marker1.txt"} {
		if err := os.WriteFile(filepath.Join(out, name), []byte("not empty"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if ok, err := (CommandVerifier{}).Verify(context.Background(), task); err != nil || ok {
		t.Fatalf("wrong contents accepted: %v", err)
	}
}
