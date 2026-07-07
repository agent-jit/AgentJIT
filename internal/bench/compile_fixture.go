package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agent-jit/agentjit/internal/ingest"
)

// CompilableFixture is a Fixture whose workflow is a repetitive shell sequence
// that AgentJIT's deterministic compiler can turn into a real skill. Unlike
// SkilledFixture (which installs a hand-written SKILL.md), the JIT arm here runs
// the real `aj compile` over seeded session logs, so the benchmark measures
// AgentJIT's own compiled skill — not a stand-in.
type CompilableFixture interface {
	Fixture
	// SeedSessions writes session logs of the workflow under logsDir/<date>/,
	// enough repetitions that the trace analyzer detects a hot path
	// (MinPatternFrequency, default 3).
	SeedSessions(logsDir string) error
	// SkillName is the deterministic skill name aj compile will produce for this
	// workflow (derived from the first Bash step), so the JIT arm can locate it.
	SkillName() string
}

// ShellSeqFixture has a workflow that is a fixed sequence of shell commands
// creating N marker files. AgentJIT compiles the repeated sequence into a
// shell-script skill; the JIT arm can then invoke that script instead of issuing
// each command.
type ShellSeqFixture struct{}

func (ShellSeqFixture) Shape() string { return "shellseq" }

// SkillName matches inferSkillName's output for this workflow's first Bash step
// ("mkdir -p out" -> tokens "mkdir","out" joined -> "mkdir-out").
func (ShellSeqFixture) SkillName() string { return "mkdir-out" }

// commands returns the workflow's shell steps for repeat count n: make an output
// dir, then create n numbered marker files.
func (ShellSeqFixture) commands(n int) []string {
	cmds := []string{"mkdir -p out"}
	for i := 0; i < n; i++ {
		cmds = append(cmds, fmt.Sprintf("touch out/marker%d.txt", i))
	}
	return cmds
}

func (f ShellSeqFixture) SeedSessions(logsDir string) error {
	// 4 sessions (> MinPatternFrequency=3) all repeating the same command shape,
	// so the deterministic compiler detects and compiles the hot path.
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for s := 0; s < 4; s++ {
		sid := fmt.Sprintf("cld_shellseq%d", s)
		dateDir := filepath.Join(logsDir, base.Format("2006-01-02"))
		if err := os.MkdirAll(dateDir, 0o755); err != nil {
			return err
		}
		var lines []string
		for i, cmd := range f.commands(2) { // seed with a fixed shape (n=2)
			ev := ingest.Event{
				Timestamp:        base.Add(time.Duration(s) * time.Minute).Add(time.Duration(i) * time.Second),
				SessionID:        sid,
				Harness:          "claude-code",
				EventType:        "post_tool_use",
				ToolName:         "Bash",
				ToolInput:        map[string]interface{}{"command": cmd},
				WorkingDirectory: "/tmp/shellseq",
			}
			b, err := json.Marshal(ev)
			if err != nil {
				return err
			}
			lines = append(lines, string(b))
		}
		content := ""
		for _, l := range lines {
			content += l + "\n"
		}
		if err := os.WriteFile(filepath.Join(dateDir, sid+".jsonl"), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (f ShellSeqFixture) Generate(dir string, n int) (Task, error) {
	if n < 1 {
		return Task{}, fmt.Errorf("shellseq fixture needs n >= 1, got %d", n)
	}
	target := filepath.Join(dir, "shellseq")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return Task{}, err
	}

	// Verifier: exactly n marker files exist under out/.
	verifyScript := fmt.Sprintf(
		`test "$(ls out/marker*.txt 2>/dev/null | wc -l | tr -d '[:space:]')" = "%d"`, n)

	cmds := f.commands(n)
	return Task{
		ID:      fmt.Sprintf("shellseq-%d", n),
		Shape:   "shellseq",
		RepoDir: target,
		Prompt: fmt.Sprintf(
			"In this directory, create an `out/` directory and %d empty marker files "+
				"named out/marker0.txt .. out/marker%d.txt. The exact command sequence is:\n%s",
			n, n-1, joinLines(cmds)),
		Verify: Verification{Command: []string{"sh", "-c", verifyScript}},
	}, nil
}

func joinLines(xs []string) string {
	out := ""
	for _, x := range xs {
		out += "  " + x + "\n"
	}
	return out
}
