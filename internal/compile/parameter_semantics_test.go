package compile

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/ingest"
	"github.com/agent-jit/agentjit/internal/trace"
)

// Exercise trace mining through generated-script execution, using held-out
// filenames and decoy files that expose silent glob expansion.
func TestCompiledPathSemantics(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil { t.Skip("bash unavailable") }
	var events []ingest.Event
	for i := 0; i < 4; i++ {
		for j, command := range []string{fmt.Sprintf("cat -- \"./seed%d.txt\"", i), "echo DONE"} {
			events = append(events, ingest.Event{
				Timestamp: time.Date(2026, 10, 11, 0, i, j, 0, time.UTC),
				SessionID: fmt.Sprintf("seed-%d", i), Harness: "claude-code",
				EventType: "post_tool_use", ToolName: "Bash",
				ToolInput: map[string]interface{}{"command": command},
			})
		}
	}
	g := trace.BuildGraph(events)
	patterns := trace.Parameterize(trace.FindHotPaths(g, 3, 2, 20), events, g)
	if len(patterns) != 1 || len(patterns[0].Steps) != 2 { t.Fatalf("unexpected patterns: %+v", patterns) }
	paths := config.PathsFromRoot(t.TempDir())
	if err := paths.EnsureDirs(); err != nil { t.Fatal(err) }
	skills, err := NewDeterministicBackend(paths, "linux").Compile(context.Background(), patterns)
	if err != nil { t.Fatal(err) }
	script := filepath.Join(skills[0].Path, "scripts", skills[0].Name+".sh")
	for _, name := range []string{"plain.txt", "-dash.txt", "with space.txt", "glob*.txt", "question?.txt", "bracket[1].txt", "中文.txt", "quote'\".txt", "$(echo injected).txt", "line\nbreak.txt"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			payload := "TARGET:" + name + "\n"
			if err := os.WriteFile(filepath.Join(dir, name), []byte(payload), 0600); err != nil { t.Fatal(err) }
			for _, decoy := range []string{"globA.txt", "questionX.txt", "bracket1.txt"} {
				if err := os.WriteFile(filepath.Join(dir, decoy), []byte("DECOY\n"), 0600); err != nil { t.Fatal(err) }
			}
			cmd := exec.Command("bash", script, "./"+name)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil || string(out) != payload+"DONE\n" { t.Fatalf("compiled output=%q err=%v", out, err) }
		})
	}
}
