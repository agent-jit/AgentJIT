package bench

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateFixtureGenerate(t *testing.T) {
	dir := t.TempDir()
	task, err := MigrateFixture{}.Generate(dir, 3)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if task.ID != "migrate-3" || task.Shape != "migrate" {
		t.Errorf("task = %+v", task)
	}
	for i := 0; i < 3; i++ {
		if _, err := os.Stat(filepath.Join(task.RepoDir, fmt.Sprintf("mod%d.go", i))); err != nil {
			t.Errorf("mod%d.go missing: %v", i, err)
		}
	}
	if _, err := os.Stat(filepath.Join(task.RepoDir, "api.go")); err != nil {
		t.Errorf("api.go missing: %v", err)
	}
}

// The dual gate must FAIL on the unmodified fixture: OldName calls still present.
func TestMigrateVerifierFailsBeforeFix(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	dir := t.TempDir()
	task, err := MigrateFixture{}.Generate(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := CommandVerifier{}.Verify(context.Background(), task)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Error("verifier passed on unmodified fixture; OldName calls still present")
	}
}

// After migrating every OldName call to NewName, the dual gate must PASS.
func TestMigrateVerifierPassesAfterFix(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	dir := t.TempDir()
	task, err := MigrateFixture{}.Generate(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the migration in the mod*.go files (not api.go).
	entries, _ := os.ReadDir(task.RepoDir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "mod") {
			continue
		}
		p := filepath.Join(task.RepoDir, e.Name())
		b, _ := os.ReadFile(p)
		fixed := strings.Replace(string(b), "OldName(t)", "NewName(t)", 1)
		if err := os.WriteFile(p, []byte(fixed), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ok, err := CommandVerifier{}.Verify(context.Background(), task)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Error("verifier failed after migrating all call sites; expected pass")
	}
}

// The hand-written skill must not fool the verifier: its SKILL.md contains
// NewName( in an example, but the grep is scoped to *.go.
func TestMigrateInstallSkillDoesNotContaminate(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	dir := t.TempDir()
	task, err := MigrateFixture{}.Generate(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := (MigrateFixture{}).InstallSkill(task); err != nil {
		t.Fatalf("InstallSkill: %v", err)
	}
	// Still fails: code is unmodified regardless of the skill's example text.
	ok, err := CommandVerifier{}.Verify(context.Background(), task)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Error("verifier passed due to SKILL.md contamination; grep must exclude non-.go files")
	}
}
