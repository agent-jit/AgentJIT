package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agent-jit/agentjit/internal/ingest"
)

// AKSOpsFixture models a repetitive SRE/operations workflow: a fixed sequence of
// az/kubectl commands (get-credentials, scale, rollout status, get pods). This is
// AgentJIT's intended sweet spot — repetitive multi-step *tool use*, Bash-shaped,
// so the deterministic compiler turns it into a runnable shell-script skill.
//
// Unlike the code-edit fixtures, the compiled skill here can *do the work* (run
// the command sequence) rather than merely annotate an edit — so the JIT arm can
// invoke it instead of reasoning through each command.
//
// az/kubectl are MOCKED (fake scripts under bin/ that append to ops.log), so the
// benchmark is hermetic and reproducible — no real cluster or cloud.
type AKSOpsFixture struct{}

func (AKSOpsFixture) Shape() string { return "aksops" }

// SkillName is the deterministic skill name aj compile produces for this
// workflow (verified empirically): inferSkillName takes the first Bash step's
// first non-flag tokens ("./bin/az","aks","get-credentials") and sanitizes them.
func (AKSOpsFixture) SkillName() string { return "bin-az-aks-get-credentials" }

// commands is the fixed ops sequence (same every run — the reuse axis JIT needs).
// N is accepted for interface symmetry but the sequence is fixed (an ops runbook
// is not parameterized by a count); N only scales the replicas value for variety.
func (AKSOpsFixture) commands(n int) []string {
	replicas := 2 + n
	return []string{
		"./bin/az aks get-credentials --name prod --resource-group rg",
		fmt.Sprintf("./bin/kubectl scale deployment/web --replicas=%d", replicas),
		"./bin/kubectl rollout status deployment/web",
		"./bin/kubectl get pods -o wide",
	}
}

func (f AKSOpsFixture) SeedSessions(logsDir string) error {
	base := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	for s := 0; s < 4; s++ {
		sid := fmt.Sprintf("cld_aksops%d", s)
		dateDir := filepath.Join(logsDir, base.Format("2006-01-02"))
		if err := os.MkdirAll(dateDir, 0o755); err != nil {
			return err
		}
		var content string
		for i, cmd := range f.commands(2) { // fixed shape for the seed
			ev := ingest.Event{
				Timestamp:        base.Add(time.Duration(s) * time.Minute).Add(time.Duration(i) * time.Second),
				SessionID:        sid,
				Harness:          "claude-code",
				EventType:        "post_tool_use",
				ToolName:         "Bash",
				ToolInput:        map[string]interface{}{"command": cmd},
				WorkingDirectory: "/tmp/aksops",
			}
			b, err := json.Marshal(ev)
			if err != nil {
				return err
			}
			content += string(b) + "\n"
		}
		if err := os.WriteFile(filepath.Join(dateDir, sid+".jsonl"), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// mockTool is a fake az/kubectl that prints plausible CLI output and records the
// invocation in ops.log. It emits realistic-looking output (not an obvious no-op)
// so the agent treats it as a normal tool and runs the runbook rather than
// second-guessing whether stub commands are worth executing.
const mockTool = `#!/usr/bin/env bash
echo "$(basename "$0") $*" >> ops.log
case "$(basename "$0") $1" in
  "az aks")        echo "Merged \"prod\" as current context in ~/.kube/config" ;;
  "kubectl scale") echo "deployment.apps/web scaled" ;;
  "kubectl rollout") echo "deployment \"web\" successfully rolled out" ;;
  "kubectl get")   echo "NAME                   READY   STATUS    RESTARTS   AGE"; echo "web-7d9f8c6b5-abcde    1/1     Running   0          5m" ;;
  *)               echo "ok" ;;
esac
exit 0
`

func (f AKSOpsFixture) Generate(dir string, n int) (Task, error) {
	if n < 1 {
		return Task{}, fmt.Errorf("aksops fixture needs n >= 1, got %d", n)
	}
	target := filepath.Join(dir, "aksops")
	binDir := filepath.Join(target, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return Task{}, err
	}
	for _, tool := range []string{"az", "kubectl"} {
		if err := os.WriteFile(filepath.Join(binDir, tool), []byte(mockTool), 0o755); err != nil {
			return Task{}, err
		}
	}

	cmds := f.commands(n)
	// Verifier: ops.log recorded all 4 steps, in order, with the right replicas.
	replicas := 2 + n
	verify := fmt.Sprintf(
		`test -f ops.log `+
			`&& grep -q 'az aks get-credentials --name prod' ops.log `+
			`&& grep -q 'kubectl scale deployment/web --replicas=%d' ops.log `+
			`&& grep -q 'kubectl rollout status deployment/web' ops.log `+
			`&& grep -q 'kubectl get pods -o wide' ops.log`,
		replicas)

	return Task{
		ID:      fmt.Sprintf("aksops-%d", n),
		Shape:   "aksops",
		RepoDir: target,
		Prompt: fmt.Sprintf(
			"Execute the following AKS operations runbook exactly as written, running each "+
				"command in order from this directory. Use the local ./bin/az and ./bin/kubectl "+
				"(that is the intended environment for this runbook). Run all of them, then stop.\n%s",
			joinLines(cmds)),
		Verify: Verification{Command: []string{"sh", "-c", verify}},
	}, nil
}
