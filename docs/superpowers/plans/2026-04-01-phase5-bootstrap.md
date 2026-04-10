# Phase 5: Bootstrap — Historical Transcript Import

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `aj bootstrap` command that imports historical Claude Code session transcripts into AJ's JSONL log format, giving the compiler data to work with before any hooks have fired.

**Architecture:** The bootstrap command locates Claude Code transcript JSONL files under `~/.claude/projects/`, parses them to extract tool use events (the transcripts are JSONL with structured message objects), normalizes those events into our schema, and writes them to `~/.aj/logs/`. A processed-files tracker prevents re-processing.

**Tech Stack:** Go, standard library (encoding/json, os, path/filepath, bufio, time)

**Depends on:** Phase 1 (config, paths), Phase 2 (ingest schema/writer)

---

### Task 1: Implement Transcript Parser

**Files:**
- Create: `internal/bootstrap/parser.go`
- Create: `internal/bootstrap/parser_test.go`

- [ ] **Step 1: Write failing test for transcript parsing**

Create `internal/bootstrap/parser_test.go`:

```go
package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTranscript(t *testing.T) {
	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "test-session.jsonl")

	// Write a realistic Claude Code transcript JSONL
	lines := []string{
		`{"type":"user","sessionId":"abc123","timestamp":"2026-03-30T10:00:00.000Z","cwd":"/Users/dev/project","message":{"role":"user","content":"list files"}}`,
		`{"type":"assistant","sessionId":"abc123","timestamp":"2026-03-30T10:00:01.000Z","cwd":"/Users/dev/project","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"Bash","input":{"command":"ls -la","description":"List files"}}]}}`,
		`{"type":"progress","sessionId":"abc123","timestamp":"2026-03-30T10:00:02.000Z","cwd":"/Users/dev/project","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"total 8\ndrwxr-xr-x  3 dev staff  96 Mar 30 10:00 .\n"}]}}`,
		`{"type":"assistant","sessionId":"abc123","timestamp":"2026-03-30T10:00:03.000Z","cwd":"/Users/dev/project","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_2","name":"Read","input":{"file_path":"/Users/dev/project/main.go"}}]}}`,
		`{"type":"progress","sessionId":"abc123","timestamp":"2026-03-30T10:00:04.000Z","cwd":"/Users/dev/project","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_2","content":"package main\nfunc main() {}"}]}}`,
	}

	f, _ := os.Create(transcriptPath)
	for _, line := range lines {
		f.WriteString(line + "\n")
	}
	f.Close()

	events, err := ParseTranscript(transcriptPath, 512)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 tool uses", len(events))
	}

	// First event: Bash ls -la
	if events[0].ToolName != "Bash" {
		t.Errorf("event[0].ToolName = %q, want Bash", events[0].ToolName)
	}
	cmd, _ := events[0].ToolInput["command"].(string)
	if cmd != "ls -la" {
		t.Errorf("event[0] command = %q, want 'ls -la'", cmd)
	}
	if events[0].SessionID != "abc123" {
		t.Errorf("event[0].SessionID = %q, want abc123", events[0].SessionID)
	}
	if events[0].BootstrapSource != "bootstrap" {
		t.Errorf("event[0].BootstrapSource = %q, want bootstrap", events[0].BootstrapSource)
	}

	// Second event: Read
	if events[1].ToolName != "Read" {
		t.Errorf("event[1].ToolName = %q, want Read", events[1].ToolName)
	}
}

func TestParseTranscriptEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(path, []byte(""), 0644)

	events, err := ParseTranscript(path, 512)
	if err != nil {
		t.Fatalf("ParseTranscript: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
	}
}

func TestParseTranscriptNonExistent(t *testing.T) {
	_, err := ParseTranscript("/nonexistent/file.jsonl", 512)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/bootstrap/ -v`
Expected: FAIL — package not found

- [ ] **Step 3: Implement transcript parser**

Create `internal/bootstrap/parser.go`:

```go
package bootstrap

import (
	"bufio"
	"encoding/json"
	"os"
	"time"

	"github.com/agent-jit/agentjit/internal/ingest"
)

// transcriptLine represents a single line in a Claude Code transcript JSONL.
type transcriptLine struct {
	Type      string          `json:"type"`
	SessionID string          `json:"sessionId"`
	Timestamp string          `json:"timestamp"`
	CWD       string          `json:"cwd"`
	Message   json.RawMessage `json:"message"`
}

// messageContent represents the message field in a transcript line.
type messageContent struct {
	Role    string            `json:"role"`
	Content json.RawMessage   `json:"content"`
}

// toolUseBlock represents a tool_use block in assistant messages.
type toolUseBlock struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// toolResultBlock represents a tool_result block in progress messages.
type toolResultBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   interface{} `json:"content"`
}

// ParseTranscript reads a Claude Code transcript JSONL file and extracts
// tool use events into the normalized AJ event schema.
func ParseTranscript(path string, maxResponseBytes int) ([]ingest.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []ingest.Event
	// Map tool_use_id to event index for pairing results
	pendingTools := make(map[string]int)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024) // 4MB buffer for large lines

	for scanner.Scan() {
		var line transcriptLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}

		ts, _ := time.Parse(time.RFC3339Nano, line.Timestamp)
		if ts.IsZero() {
			ts, _ = time.Parse("2006-01-02T15:04:05.000Z", line.Timestamp)
		}

		var msg messageContent
		if err := json.Unmarshal(line.Message, &msg); err != nil {
			continue
		}

		// Assistant messages contain tool_use blocks
		if line.Type == "assistant" && msg.Role == "assistant" {
			var blocks []json.RawMessage
			if err := json.Unmarshal(msg.Content, &blocks); err != nil {
				continue
			}

			for _, block := range blocks {
				var tu toolUseBlock
				if err := json.Unmarshal(block, &tu); err != nil {
					continue
				}
				if tu.Type != "tool_use" {
					continue
				}

				event := ingest.Event{
					Timestamp:        ts,
					SessionID:        line.SessionID,
					Harness:          "claude-code",
					EventType:        "post_tool_use",
					ToolName:         tu.Name,
					ToolInput:        tu.Input,
					WorkingDirectory: line.CWD,
					BootstrapSource:  "bootstrap",
				}

				events = append(events, event)
				pendingTools[tu.ID] = len(events) - 1
			}
		}

		// Progress messages contain tool_result blocks
		if line.Type == "progress" && msg.Role == "user" {
			var blocks []json.RawMessage
			if err := json.Unmarshal(msg.Content, &blocks); err != nil {
				continue
			}

			for _, block := range blocks {
				var tr toolResultBlock
				if err := json.Unmarshal(block, &tr); err != nil {
					continue
				}
				if tr.Type != "tool_result" {
					continue
				}

				idx, ok := pendingTools[tr.ToolUseID]
				if !ok {
					continue
				}

				// Attach truncated response
				var summary string
				switch v := tr.Content.(type) {
				case string:
					summary = v
				default:
					data, _ := json.Marshal(v)
					summary = string(data)
				}
				if len(summary) > maxResponseBytes {
					summary = summary[:maxResponseBytes]
				}
				events[idx].ToolResponseSummary = summary

				delete(pendingTools, tr.ToolUseID)
			}
		}
	}

	return events, scanner.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/bootstrap/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bootstrap/parser.go internal/bootstrap/parser_test.go
git commit -m "feat: add transcript parser for bootstrap import"
```

---

### Task 2: Implement Bootstrap Orchestrator

**Files:**
- Create: `internal/bootstrap/bootstrap.go`
- Create: `internal/bootstrap/bootstrap_test.go`

- [ ] **Step 1: Write failing test for bootstrap orchestrator**

Create `internal/bootstrap/bootstrap_test.go`:

```go
package bootstrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-jit/agentjit/internal/config"
)

func createFakeTranscript(t *testing.T, dir, sessionID string) string {
	t.Helper()
	path := filepath.Join(dir, sessionID+".jsonl")
	lines := []string{
		`{"type":"assistant","sessionId":"` + sessionID + `","timestamp":"2026-03-30T10:00:01.000Z","cwd":"/dev","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"Bash","input":{"command":"echo hi"}}]}}`,
		`{"type":"progress","sessionId":"` + sessionID + `","timestamp":"2026-03-30T10:00:02.000Z","cwd":"/dev","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"hi"}]}}`,
	}
	f, _ := os.Create(path)
	for _, l := range lines {
		f.WriteString(l + "\n")
	}
	f.Close()
	return path
}

func TestRunBootstrap(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()
	cfg := config.DefaultConfig()

	// Create fake Claude projects directory
	claudeProjectsDir := filepath.Join(root, "claude-projects")
	projectDir := filepath.Join(claudeProjectsDir, "-Users-dev-project")
	os.MkdirAll(projectDir, 0755)

	createFakeTranscript(t, projectDir, "session-001")
	createFakeTranscript(t, projectDir, "session-002")

	result, err := RunBootstrap(paths, cfg, claudeProjectsDir, BootstrapOptions{})
	if err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}

	if result.SessionsProcessed != 2 {
		t.Errorf("SessionsProcessed = %d, want 2", result.SessionsProcessed)
	}
	if result.EventsImported < 2 {
		t.Errorf("EventsImported = %d, want >= 2", result.EventsImported)
	}

	// Verify log files were created
	entries, _ := os.ReadDir(paths.Logs)
	if len(entries) == 0 {
		t.Error("no log date directories created")
	}

	// Verify processed tracker was updated
	data, err := os.ReadFile(paths.BootstrapProcessed)
	if err != nil {
		t.Fatalf("reading bootstrap_processed: %v", err)
	}
	var processed ProcessedFiles
	json.Unmarshal(data, &processed)
	if len(processed.Files) != 2 {
		t.Errorf("processed files = %d, want 2", len(processed.Files))
	}
}

func TestRunBootstrapIdempotent(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()
	cfg := config.DefaultConfig()

	claudeProjectsDir := filepath.Join(root, "claude-projects")
	projectDir := filepath.Join(claudeProjectsDir, "-Users-dev-project")
	os.MkdirAll(projectDir, 0755)

	createFakeTranscript(t, projectDir, "session-001")

	// Run twice
	RunBootstrap(paths, cfg, claudeProjectsDir, BootstrapOptions{})
	result, _ := RunBootstrap(paths, cfg, claudeProjectsDir, BootstrapOptions{})

	if result.SessionsProcessed != 0 {
		t.Errorf("second run should process 0 sessions, got %d", result.SessionsProcessed)
	}
}

func TestRunBootstrapDryRun(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()
	cfg := config.DefaultConfig()

	claudeProjectsDir := filepath.Join(root, "claude-projects")
	projectDir := filepath.Join(claudeProjectsDir, "-Users-dev-project")
	os.MkdirAll(projectDir, 0755)

	createFakeTranscript(t, projectDir, "session-001")

	result, err := RunBootstrap(paths, cfg, claudeProjectsDir, BootstrapOptions{DryRun: true})
	if err != nil {
		t.Fatalf("RunBootstrap: %v", err)
	}

	if result.SessionsProcessed != 1 {
		t.Errorf("SessionsProcessed = %d, want 1", result.SessionsProcessed)
	}

	// Verify nothing was written
	entries, _ := os.ReadDir(paths.Logs)
	if len(entries) != 0 {
		t.Error("dry run should not create log files")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/bootstrap/ -run TestRunBootstrap -v`
Expected: FAIL — `RunBootstrap` undefined

- [ ] **Step 3: Implement bootstrap orchestrator**

Create `internal/bootstrap/bootstrap.go`:

```go
package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/ingest"
)

// ProcessedFiles tracks which transcript files have been bootstrapped.
type ProcessedFiles struct {
	Files map[string]time.Time `json:"files"`
}

// BootstrapOptions configures the bootstrap run.
type BootstrapOptions struct {
	Since   string // YYYY-MM-DD filter
	Project string // Project path filter
	DryRun  bool
}

// BootstrapResult reports what was bootstrapped.
type BootstrapResult struct {
	SessionsProcessed int
	EventsImported    int
}

// RunBootstrap scans Claude Code transcripts and imports tool use events into
// AJ's log format. Tracks processed files to avoid re-importing.
func RunBootstrap(paths config.Paths, cfg config.Config, claudeProjectsDir string, opts BootstrapOptions) (BootstrapResult, error) {
	var result BootstrapResult

	// Load processed tracker
	processed := loadProcessed(paths.BootstrapProcessed)

	// Find all transcript JSONL files
	transcripts, err := findTranscripts(claudeProjectsDir, opts)
	if err != nil {
		return result, fmt.Errorf("finding transcripts: %w", err)
	}

	writer := ingest.NewWriter(paths)

	for _, path := range transcripts {
		// Skip already processed
		if _, ok := processed.Files[path]; ok {
			continue
		}

		events, err := ParseTranscript(path, cfg.Ingestion.MaxResponseBytes)
		if err != nil {
			continue
		}

		if len(events) == 0 {
			continue
		}

		result.SessionsProcessed++
		result.EventsImported += len(events)

		if !opts.DryRun {
			for _, event := range events {
				if err := writer.Write(event); err != nil {
					return result, fmt.Errorf("writing event: %w", err)
				}
			}
			processed.Files[path] = time.Now()
		}
	}

	// Save processed tracker
	if !opts.DryRun && result.SessionsProcessed > 0 {
		if err := saveProcessed(paths.BootstrapProcessed, processed); err != nil {
			return result, fmt.Errorf("saving processed tracker: %w", err)
		}
	}

	return result, nil
}

func findTranscripts(claudeProjectsDir string, opts BootstrapOptions) ([]string, error) {
	var transcripts []string

	projectDirs, err := os.ReadDir(claudeProjectsDir)
	if err != nil {
		return nil, err
	}

	for _, pd := range projectDirs {
		if !pd.IsDir() {
			continue
		}

		// Filter by project if specified
		if opts.Project != "" {
			// Claude encodes paths by replacing / with -
			encoded := strings.ReplaceAll(opts.Project, "/", "-")
			if !strings.Contains(pd.Name(), encoded) {
				continue
			}
		}

		dirPath := filepath.Join(claudeProjectsDir, pd.Name())
		files, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".jsonl" {
				continue
			}

			// Filter by date if specified
			if opts.Since != "" {
				sinceTime, err := time.Parse("2006-01-02", opts.Since)
				if err == nil {
					info, err := f.Info()
					if err == nil && info.ModTime().Before(sinceTime) {
						continue
					}
				}
			}

			transcripts = append(transcripts, filepath.Join(dirPath, f.Name()))
		}
	}

	return transcripts, nil
}

func loadProcessed(path string) ProcessedFiles {
	processed := ProcessedFiles{Files: make(map[string]time.Time)}
	data, err := os.ReadFile(path)
	if err != nil {
		return processed
	}
	json.Unmarshal(data, &processed)
	if processed.Files == nil {
		processed.Files = make(map[string]time.Time)
	}
	return processed
}

func saveProcessed(path string, processed ProcessedFiles) error {
	data, err := json.MarshalIndent(processed, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/bootstrap/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/bootstrap/bootstrap.go internal/bootstrap/bootstrap_test.go
git commit -m "feat: add bootstrap orchestrator for historical transcript import"
```

---

### Task 3: Wire Bootstrap Command

**Files:**
- Modify: `cmd/agentjit/bootstrap_cmd.go`

- [ ] **Step 1: Implement bootstrap command**

Update `cmd/agentjit/bootstrap_cmd.go`:

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/agent-jit/agentjit/internal/bootstrap"
	"github.com/agent-jit/agentjit/internal/config"
	"github.com/spf13/cobra"
)

var bootstrapSince string
var bootstrapProject string
var bootstrapDryRun bool

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Import historical Claude Code transcripts into logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}
		paths.EnsureDirs()

		cfg, err := config.Load(paths.Config)
		if err != nil {
			cfg = config.DefaultConfig()
		}

		claudeProjectsDir, err := config.ClaudeProjectsDir()
		if err != nil {
			return fmt.Errorf("finding Claude projects dir: %w", err)
		}

		opts := bootstrap.BootstrapOptions{
			Since:   bootstrapSince,
			Project: bootstrapProject,
			DryRun:  bootstrapDryRun,
		}

		if bootstrapDryRun {
			fmt.Println("[AJ] Dry run — no files will be written")
		}

		result, err := bootstrap.RunBootstrap(paths, cfg, claudeProjectsDir, opts)
		if err != nil {
			return err
		}

		if result.SessionsProcessed == 0 {
			fmt.Println("[AJ] No new sessions to import")
			return nil
		}

		fmt.Printf("[AJ] Bootstrapped %d sessions (%d events)\n",
			result.SessionsProcessed, result.EventsImported)

		if !bootstrapDryRun {
			fmt.Print("[AJ] Run compilation now? [Y/n] ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))

			if answer == "" || answer == "y" || answer == "yes" {
				// Trigger dream via the dream command's RunE
				return dreamCmd.RunE(dreamCmd, nil)
			}
		}

		return nil
	},
}

func init() {
	bootstrapCmd.Flags().StringVar(&bootstrapSince, "since", "", "Only transcripts after this date (YYYY-MM-DD)")
	bootstrapCmd.Flags().StringVar(&bootstrapProject, "project", "", "Only transcripts for this project path")
	bootstrapCmd.Flags().BoolVar(&bootstrapDryRun, "dry-run", false, "Show what would be processed without writing")
	rootCmd.AddCommand(bootstrapCmd)
}
```

- [ ] **Step 2: Build and verify**

Run:
```bash
go build -o agentjit ./cmd/agentjit/
./aj bootstrap --dry-run
```
Expected: Lists sessions found or "No new sessions to import"

- [ ] **Step 3: Commit**

```bash
git add cmd/agentjit/bootstrap_cmd.go
git commit -m "feat: wire bootstrap command with dry-run and dream prompt"
```
