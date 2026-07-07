package bench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildAJ builds the aj binary into a temp dir and returns its path, or skips
// the test if the toolchain isn't available.
func buildAJ(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "aj")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/agent-jit/agentjit/cmd/aj")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building aj: %v\n%s", err, out)
	}
	return bin
}

func TestShellSeqSeedSessions(t *testing.T) {
	dir := t.TempDir()
	if err := (ShellSeqFixture{}).SeedSessions(dir); err != nil {
		t.Fatalf("SeedSessions: %v", err)
	}
	// 4 session files should exist under the date dir.
	matches, _ := filepath.Glob(filepath.Join(dir, "*", "cld_shellseq*.jsonl"))
	if len(matches) != 4 {
		t.Errorf("seeded %d session files, want 4", len(matches))
	}
}

// End-to-end: seed -> real `aj compile` (deterministic, zero tokens) -> the
// generated skill is installed into the task tree, cost is 0.
func TestCompileSkillProducesRealSkill(t *testing.T) {
	ajBin := buildAJ(t)
	fx := ShellSeqFixture{}
	task, err := fx.Generate(t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}

	cost, err := CompileSkill(context.Background(), ajBin, fx, task)
	if err != nil {
		t.Fatalf("CompileSkill: %v", err)
	}
	// Deterministic compile is zero-token.
	if cost != 0 {
		t.Errorf("compile cost = %d, want 0 (deterministic path)", cost)
	}
	// The compiled skill (with its generated shell script) landed in the task.
	script := filepath.Join(task.RepoDir, ".claude", "skills", fx.SkillName(), "scripts", fx.SkillName()+".sh")
	if _, err := os.Stat(script); err != nil {
		t.Errorf("compiled skill script not installed at %s: %v", script, err)
	}
}
