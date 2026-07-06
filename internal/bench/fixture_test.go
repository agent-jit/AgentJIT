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

func TestNullCheckFixtureGenerate(t *testing.T) {
	dir := t.TempDir()
	task, err := NullCheckFixture{}.Generate(dir, 3)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if task.ID != "nullcheck-3" || task.Shape != "nullcheck" {
		t.Errorf("task = %+v", task)
	}

	// 3 field files + go.mod exist.
	for i := 0; i < 3; i++ {
		if _, err := os.Stat(filepath.Join(task.RepoDir, fmt.Sprintf("field%d.go", i))); err != nil {
			t.Errorf("field%d.go missing: %v", i, err)
		}
	}
	if _, err := os.Stat(filepath.Join(task.RepoDir, "go.mod")); err != nil {
		t.Errorf("go.mod missing: %v", err)
	}
}

// The dual gate must FAIL on the un-modified fixture (no guards yet): the
// package builds but zero of N guards are present.
func TestNullCheckVerifierFailsBeforeFix(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	dir := t.TempDir()
	task, err := NullCheckFixture{}.Generate(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := CommandVerifier{}.Verify(context.Background(), task)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Error("verifier passed on unmodified fixture; expected fail (0 guards != 2)")
	}
}

// After adding the guards to all N functions, the dual gate must PASS.
func TestNullCheckVerifierPassesAfterFix(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	dir := t.TempDir()
	task, err := NullCheckFixture{}.Generate(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the agent's edit: insert the guard into each field file.
	entries, _ := os.ReadDir(task.RepoDir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "field") {
			continue
		}
		p := filepath.Join(task.RepoDir, e.Name())
		b, _ := os.ReadFile(p)
		fixed := strings.Replace(string(b),
			"return len(*s)",
			"if s == nil {\n\t\treturn 0\n\t}\n\treturn len(*s)", 1)
		if err := os.WriteFile(p, []byte(fixed), 0644); err != nil {
			t.Fatal(err)
		}
	}
	ok, err := CommandVerifier{}.Verify(context.Background(), task)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Error("verifier failed after adding all guards; expected pass")
	}
}

func TestFixtureRegistry(t *testing.T) {
	f, ok := FixtureByShape("nullcheck")
	if !ok {
		t.Fatal("nullcheck fixture not registered")
	}
	if f.Shape() != "nullcheck" {
		t.Errorf("Shape() = %q, want nullcheck", f.Shape())
	}
	if _, ok := FixtureByShape("does-not-exist"); ok {
		t.Error("unknown shape reported as registered")
	}
	shapes := FixtureShapes()
	if len(shapes) == 0 {
		t.Error("FixtureShapes() is empty")
	}
}
