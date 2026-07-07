package bench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAKSOpsFixtureGenerate(t *testing.T) {
	dir := t.TempDir()
	task, err := AKSOpsFixture{}.Generate(dir, 2)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if task.ID != "aksops-2" || task.Shape != "aksops" {
		t.Errorf("task = %+v", task)
	}
	// Mock tools exist and are executable.
	for _, tool := range []string{"az", "kubectl"} {
		info, err := os.Stat(filepath.Join(task.RepoDir, "bin", tool))
		if err != nil {
			t.Errorf("mock %s missing: %v", tool, err)
			continue
		}
		if info.Mode()&0o100 == 0 {
			t.Errorf("mock %s not executable", tool)
		}
	}
}

// The verifier must FAIL before the runbook runs (no ops.log) and PASS after
// the mock commands have appended their audit lines.
func TestAKSOpsVerifier(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	dir := t.TempDir()
	task, err := AKSOpsFixture{}.Generate(dir, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Before: no ops.log -> fail.
	ok, err := CommandVerifier{}.Verify(context.Background(), task)
	if err != nil {
		t.Fatalf("Verify(before): %v", err)
	}
	if ok {
		t.Error("verifier passed before the runbook ran")
	}

	// Run the runbook by executing each command in the task's RepoDir.
	for _, c := range (AKSOpsFixture{}).commands(3) {
		cmd := exec.Command("sh", "-c", c)
		cmd.Dir = task.RepoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("running %q: %v\n%s", c, err, out)
		}
	}

	// After: ops.log has all steps -> pass.
	ok, err = CommandVerifier{}.Verify(context.Background(), task)
	if err != nil {
		t.Fatalf("Verify(after): %v", err)
	}
	if !ok {
		t.Error("verifier failed after the runbook ran; expected pass")
	}
}

// End-to-end: seed -> real `aj compile` -> the generated runbook skill is
// installed (zero-token deterministic path).
func TestAKSOpsCompileSkill(t *testing.T) {
	ajBin := buildAJ(t)
	fx := AKSOpsFixture{}
	task, err := fx.Generate(t.TempDir(), 3)
	if err != nil {
		t.Fatal(err)
	}
	cost, err := CompileSkill(context.Background(), ajBin, fx, task)
	if err != nil {
		t.Fatalf("CompileSkill: %v", err)
	}
	if cost != 0 {
		t.Errorf("compile cost = %d, want 0 (deterministic)", cost)
	}
	script := filepath.Join(task.RepoDir, ".claude", "skills", fx.SkillName(), "scripts", fx.SkillName()+".sh")
	if _, err := os.Stat(script); err != nil {
		t.Errorf("compiled runbook skill not installed at %s: %v", script, err)
	}
}
