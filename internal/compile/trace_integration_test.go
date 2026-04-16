package compile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/ir"
)

func TestRunTraceAnalysis_WithIRCatalog(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	cat, err := ir.DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if err := ir.SaveCatalog(paths.IRCatalog, cat); err != nil {
		t.Fatal(err)
	}

	for _, sid := range []string{"s1", "s2", "s3"} {
		dateDir := filepath.Join(paths.Logs, "2026-01-01")
		os.MkdirAll(dateDir, 0755)
		t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
		events := []string{
			`{"timestamp":"` + t0.Format(time.RFC3339) + `","session_id":"` + sid + `","event_type":"post_tool_use","tool_name":"Bash","tool_input":{"command":"git status"},"working_directory":"/repo"}`,
			`{"timestamp":"` + t0.Add(time.Minute).Format(time.RFC3339) + `","session_id":"` + sid + `","event_type":"post_tool_use","tool_name":"Bash","tool_input":{"command":"git diff"},"working_directory":"/repo"}`,
		}
		content := ""
		for _, e := range events {
			content += e + "\n"
		}
		os.WriteFile(filepath.Join(dateDir, sid+".jsonl"), []byte(content), 0644)
	}

	cfg := config.DefaultConfig()
	cfg.Compile.MinPatternFrequency = 3

	result, err := RunTraceAnalysis(paths, cfg)
	if err != nil {
		t.Fatal(err)
	}

	if result.PatternsFound == 0 {
		t.Error("expected at least 1 pattern")
	}
}
