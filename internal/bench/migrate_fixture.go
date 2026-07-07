package bench

import (
	"fmt"
	"os"
	"path/filepath"
)

// MigrateFixture generates N Go files that each call a deprecated function
// OldName, buried among unrelated helper code so the call sites are not obvious.
// The workflow is "migrate every OldName call to NewName" — an EXPLORATION-heavy
// task: unlike nullcheck/shellseq, the agent must first FIND all the call sites
// (grep/read across files) before editing. This is the shape where a JIT skill
// might actually save tokens, by skipping that exploration.
//
// It is Edit-based, so (like nullcheck) it is not deterministically compilable;
// the JIT arm uses a hand-written skill that names the exact rename.
type MigrateFixture struct{}

func (MigrateFixture) Shape() string { return "migrate" }

func (MigrateFixture) Generate(dir string, n int) (Task, error) {
	if n < 1 {
		return Task{}, fmt.Errorf("migrate fixture needs n >= 1, got %d", n)
	}
	target := filepath.Join(dir, "migrate")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return Task{}, err
	}
	if err := os.WriteFile(filepath.Join(target, "go.mod"), []byte("module migratebench\n\ngo 1.22\n"), 0o644); err != nil {
		return Task{}, err
	}
	// The deprecated + replacement API live in api.go so both names resolve.
	api := `package migratebench

// OldName is the deprecated API. NewName replaces it.
func OldName(x int) int { return x + 1 }

// NewName is the replacement.
func NewName(x int) int { return x + 1 }
`
	if err := os.WriteFile(filepath.Join(target, "api.go"), []byte(api), 0o644); err != nil {
		return Task{}, err
	}

	// N files, each with the OldName call buried after some distractor helpers so
	// it can't be found without reading/searching.
	for i := 0; i < n; i++ {
		src := fmt.Sprintf(`package migratebench

// helperA%d is unrelated padding so the call site isn't the first line.
func helperA%d(a, b int) int { return a*b - a }

// helperB%d is more unrelated padding.
func helperB%d(s string) int { return len(s) * 2 }

// Compute%d uses the deprecated API somewhere in the middle of the file.
func Compute%d(v int) int {
	t := helperA%d(v, 3)
	u := OldName(t)
	return u + helperB%d("padding")
}
`, i, i, i, i, i, i, i, i)
		if err := os.WriteFile(filepath.Join(target, fmt.Sprintf("mod%d.go", i)), []byte(src), 0o644); err != nil {
			return Task{}, err
		}
	}

	// Dual gate: build passes, no OldName( calls remain, and exactly n NewName(
	// call sites exist. Scope to *.go so an installed SKILL.md can't skew counts.
	// api.go defines both funcs (func OldName/func NewName), so count *calls*:
	// "OldName(" excluding the definition line, and "NewName(" call sites == n+1
	// (n calls + the definition). Simpler: require zero "= OldName(" style calls.
	verify := fmt.Sprintf(
		`go build ./... `+
			`&& test "$(grep -rho --include='*.go' 'OldName(' . | wc -l | tr -d '[:space:]')" = "1" `+
			`&& test "$(grep -rho --include='*.go' 'NewName(' . | wc -l | tr -d '[:space:]')" = "%d"`,
		n+1)

	return Task{
		ID:      fmt.Sprintf("migrate-%d", n),
		Shape:   "migrate",
		RepoDir: target,
		Prompt: fmt.Sprintf(
			"This Go package has %d files that call the deprecated function OldName. "+
				"Find every call to OldName across the package and change it to NewName "+
				"(same signature). Do not remove the OldName/NewName definitions in api.go. "+
				"Keep the package building.", n),
		Verify: Verification{Command: []string{"sh", "-c", verify}},
	}, nil
}

// InstallSkill writes a hand-written skill naming the exact rename, so the JIT
// arm can skip the exploration (finding call sites) that dominates the baseline.
func (MigrateFixture) InstallSkill(task Task) error {
	skillDir := filepath.Join(task.RepoDir, ".claude", "skills", "migrate-oldname")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}
	skill := "---\n" +
		"name: migrate-oldname\n" +
		"description: Migrate deprecated OldName calls to NewName across the package.\n" +
		"---\n\n" +
		"# migrate-oldname\n\n" +
		"The call sites are in the `ComputeK` functions in files `mod0.go .. modK.go`. In each, the line\n\n" +
		"```go\n" +
		"\tu := OldName(t)\n" +
		"```\n\n" +
		"becomes\n\n" +
		"```go\n" +
		"\tu := NewName(t)\n" +
		"```\n\n" +
		"Do not touch api.go (it defines both functions). Then ensure the package builds.\n"
	return os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skill), 0o644)
}
