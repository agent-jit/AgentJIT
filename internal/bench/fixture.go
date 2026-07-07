package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// fixtures is the registry of built-in repetition fixtures, keyed by shape name.
var fixtures = map[string]Fixture{
	"nullcheck": NullCheckFixture{},
	"shellseq":  ShellSeqFixture{},
	"migrate":   MigrateFixture{},
	"aksops":    AKSOpsFixture{},
}

// FixtureByShape returns the built-in fixture for a shape name.
func FixtureByShape(shape string) (Fixture, bool) {
	f, ok := fixtures[shape]
	return f, ok
}

// FixtureShapes returns the sorted list of registered fixture shape names.
func FixtureShapes() []string {
	names := make([]string, 0, len(fixtures))
	for k := range fixtures {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

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

// SkilledFixture is a Fixture that can install the compiled skill for its
// workflow, so the JIT arm runs with the skill available (as if AgentJIT had
// compiled it from prior runs). The skill is installed into the task's own
// working tree (.claude/skills/), so `claude` picks it up in that cwd without
// touching any global config — keeping arms isolated.
type SkilledFixture interface {
	Fixture
	// InstallSkill writes the workflow's skill under task.RepoDir/.claude/skills.
	InstallSkill(task Task) error
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
	// Scope the grep to Go sources (--include='*.go') so an installed skill's
	// SKILL.md example (which contains the guard text) can't inflate the count.
	guardCheck := fmt.Sprintf(
		`test "$(grep -rho --include='*.go' 'if s == nil' . | wc -l | tr -d '[:space:]')" = "%d"`, n)
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

// InstallSkill writes a SKILL.md into task.RepoDir/.claude/skills/nullcheck-guard
// that gives the exact, deterministic transformation for this workflow, so the
// JIT arm can apply it mechanically instead of reasoning it out each time.
func (NullCheckFixture) InstallSkill(task Task) error {
	skillDir := filepath.Join(task.RepoDir, ".claude", "skills", "nullcheck-guard")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}
	skill := "---\n" +
		"name: nullcheck-guard\n" +
		"description: Add a nil guard to Go functions that dereference a *string parameter s.\n" +
		"---\n\n" +
		"# nullcheck-guard\n\n" +
		"For every function of the form `func LengthN(s *string) int { return len(*s) }`,\n" +
		"insert a guard as the first statement so a nil pointer is handled:\n\n" +
		"```go\n" +
		"func LengthN(s *string) int {\n" +
		"\tif s == nil {\n" +
		"\t\treturn 0\n" +
		"\t}\n" +
		"\treturn len(*s)\n" +
		"}\n" +
		"```\n\n" +
		"Apply this edit to every matching function in the package, then ensure it still builds.\n"
	return os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0o644)
}
