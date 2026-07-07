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

// mockAz and mockKubectl are DISTINCT local test doubles (not copies of one
// stub — an earlier version used one shared script and the agent noticed it was
// fake and refused to run it). Each records its invocation to ops.log and prints
// output shaped like the real CLI. The header comment states plainly that these
// are the runbook's intended local tools, so the agent runs them rather than
// second-guessing whether "fake" commands are worth executing.
//
// The output is intentionally SIZABLE (JSON blobs, a many-row pod table, rollout
// progress) to mirror real az/kubectl: each command's output round-trips into the
// model's context in the baseline arm, which is the per-command cost a compiled
// runbook skill (run in one shot) might avoid. Near-silent mocks understate that.
const mockAz = `#!/usr/bin/env bash
# az — local test double for this ops runbook environment. Records to ops.log.
echo "az $*" >> ops.log
if [ "$1 $2" = "aks get-credentials" ]; then
  echo "Merged \"prod\" as current context in ~/.kube/config"
  cat <<'JSON'
{
  "apiVersion": "v1",
  "clusters": [
    {"cluster": {"certificate-authority-data": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t...", "server": "https://prod-dns-a1b2c3d4.hcp.eastus.azmk8s.io:443"}, "name": "prod"}
  ],
  "contexts": [{"context": {"cluster": "prod", "user": "clusterUser_rg_prod"}, "name": "prod"}],
  "current-context": "prod",
  "kind": "Config",
  "users": [{"name": "clusterUser_rg_prod", "user": {"token": "eyJhbGciOiJSUzI1NiIsImtpZCI6..."}}]
}
JSON
else
  echo "{\"status\": \"ok\"}"
fi
exit 0
`

const mockKubectl = `#!/usr/bin/env bash
# kubectl — local test double for this ops runbook environment. Records to ops.log.
echo "kubectl $*" >> ops.log
case "$1" in
  scale)
    echo "deployment.apps/web scaled" ;;
  rollout)
    echo "Waiting for deployment \"web\" rollout to finish: 0 of 5 updated replicas are available..."
    echo "Waiting for deployment \"web\" rollout to finish: 1 of 5 updated replicas are available..."
    echo "Waiting for deployment \"web\" rollout to finish: 2 of 5 updated replicas are available..."
    echo "Waiting for deployment \"web\" rollout to finish: 3 of 5 updated replicas are available..."
    echo "Waiting for deployment \"web\" rollout to finish: 4 of 5 updated replicas are available..."
    echo "deployment \"web\" successfully rolled out" ;;
  get)
    echo "NAME                    READY   STATUS    RESTARTS   AGE     IP            NODE                                NOMINATED NODE   READINESS GATES"
    for i in $(seq 1 120); do
      printf 'web-7d9f8c6b5-%s   1/1     Running   0          %dm    10.244.%d.%d   aks-nodepool1-24680123-vmss0000%02d   <none>           <none>\n' \
        "$(printf '%05x' $((RANDOM % 1000000)))" "$i" $((i % 3)) $((i + 10)) $((i % 6))
    done ;;
  *)
    echo "ok" ;;
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
	for tool, body := range map[string]string{"az": mockAz, "kubectl": mockKubectl} {
		if err := os.WriteFile(filepath.Join(binDir, tool), []byte(body), 0o755); err != nil {
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
			"This is a SIMULATION environment for practising an AKS operations runbook. "+
				"`./bin/az` and `./bin/kubectl` are the local test doubles you should use — "+
				"they stand in for the real CLIs and record actions to ops.log. Your task is "+
				"simply to execute the runbook against them: run each command below, in order, "+
				"exactly as written, from this directory. Running them IS the deliverable — do "+
				"not skip any because they are simulated. After running all of them, stop.\n%s",
			joinLines(cmds)),
		Verify: Verification{Command: []string{"sh", "-c", verify}},
	}, nil
}
