package bench

import (
	"fmt"
	"os"
	"path/filepath"
)

// Fixture materializes a self-contained, reproducible workspace for a
// repetition-parameterized task (same shape repeated N times), and reports the
// Task (prompt + verifier) that runs against it. Generating the workspace makes
// the benchmark corpus deterministic and free of external repo dependencies.
type Fixture interface {
	// Shape is the structural family (used to stratify break-even/compile cost).
	Shape() string
	// Generate writes the target files under dir for repetition count n and
	// returns the Task to run against them.
	Generate(dir string, n int) (Task, error)
}

// NullCheckFixture generates N Go files, each with a function that dereferences
// a pointer parameter without a nil guard. The workflow under test is "add a
// nil-check to every such function" — a same-shape task that repeats N times,
// so a compiled skill can be reused across all N (the reuse axis JIT needs).
//
// Verification is a dual gate (per the Phase 3 methodology): the package must
// build AND exactly N guards must be present — catching "compiled but wrong".
type NullCheckFixture struct{}

func (NullCheckFixture) Shape() string { return "nullcheck" }

func (NullCheckFixture) Generate(dir string, n int) (Task, error) {
	if n < 1 {
		return Task{}, fmt.Errorf("nullcheck fixture needs n >= 1, got %d", n)
	}
	target := filepath.Join(dir, "nullcheck")
	if err := os.MkdirAll(target, 0755); err != nil {
		return Task{}, err
	}
	if err := os.WriteFile(filepath.Join(target, "go.mod"), []byte("module nullcheckbench\n\ngo 1.22\n"), 0644); err != nil {
		return Task{}, err
	}
	for i := 0; i < n; i++ {
		src := fmt.Sprintf(`package nullcheckbench

// Length%d returns the length of s. It dereferences s without a nil check.
func Length%d(s *string) int {
	return len(*s)
}
`, i, i)
		if err := os.WriteFile(filepath.Join(target, fmt.Sprintf("field%d.go", i)), []byte(src), 0644); err != nil {
			return Task{}, err
		}
	}

	// Dual-gate verifier: build must pass, and exactly n guard lines must exist.
	// `grep -rc` over the package, summed, must equal n. We check via a small
	// shell pipeline so the fixture stays declarative.
	guardCheck := fmt.Sprintf(
		`test "$(grep -rho 'if s == nil' . | wc -l | tr -d '[:space:]')" = "%d"`, n)
	verifyScript := fmt.Sprintf("go build ./... && %s", guardCheck)

	return Task{
		ID:      fmt.Sprintf("nullcheck-%d", n),
		Shape:   "nullcheck",
		RepoDir: target,
		Prompt: fmt.Sprintf(
			"In this Go package, every LengthN function dereferences its *string "+
				"parameter s without checking for nil. Add a guard `if s == nil { return 0 }` "+
				"at the start of each of the %d functions. Keep the package building.", n),
		Verify: Verification{Command: []string{"sh", "-c", verifyScript}},
	}, nil
}
