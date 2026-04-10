# Phase 2: Event Ingestion — Schema, JSONL Writer, Ingest Command, Hook Installation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the event ingestion pipeline — from Claude Code hook payloads arriving on stdin, through normalization, to JSONL files on disk. Then implement hook installation into Claude Code settings.

**Architecture:** The `aj ingest` command reads JSON from stdin, normalizes it into our schema, and writes to date/session-partitioned JSONL files. Hook installation reads/merges JSON into Claude Code's settings.json without clobbering existing hooks.

**Tech Stack:** Go, standard library (encoding/json, os, bufio, time)

**Depends on:** Phase 1 (config, paths, CLI skeleton)

---

### Task 1: Define Event Schema

**Files:**
- Create: `internal/ingest/schema.go`
- Create: `internal/ingest/schema_test.go`

- [ ] **Step 1: Write test for event normalization**

Create `internal/ingest/schema_test.go`:

```go
package ingest

import (
	"encoding/json"
	"testing"
)

func TestNormalizePostToolUse(t *testing.T) {
	raw := `{
		"session_id": "abc123",
		"hook_event_name": "PostToolUse",
		"cwd": "/Users/dev/project",
		"tool_name": "Bash",
		"tool_input": {"command": "kubectl logs -n ns pod/x", "description": "Get logs"},
		"tool_response": "some very long output that should be truncated..."
	}`

	event, err := NormalizeEvent([]byte(raw), 20)
	if err != nil {
		t.Fatalf("NormalizeEvent: %v", err)
	}

	if event.SessionID != "abc123" {
		t.Errorf("SessionID = %q, want abc123", event.SessionID)
	}
	if event.EventType != "post_tool_use" {
		t.Errorf("EventType = %q, want post_tool_use", event.EventType)
	}
	if event.Harness != "claude-code" {
		t.Errorf("Harness = %q, want claude-code", event.Harness)
	}
	if event.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", event.ToolName)
	}
	if event.WorkingDirectory != "/Users/dev/project" {
		t.Errorf("WorkingDirectory = %q", event.WorkingDirectory)
	}
	if len(event.ToolResponseSummary) > 20 {
		t.Errorf("ToolResponseSummary not truncated: len=%d", len(event.ToolResponseSummary))
	}
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestNormalizeSessionStart(t *testing.T) {
	raw := `{
		"session_id": "abc123",
		"hook_event_name": "SessionStart",
		"cwd": "/Users/dev/project",
		"source": "startup"
	}`

	event, err := NormalizeEvent([]byte(raw), 512)
	if err != nil {
		t.Fatalf("NormalizeEvent: %v", err)
	}

	if event.EventType != "session_start" {
		t.Errorf("EventType = %q, want session_start", event.EventType)
	}
	if event.Source != "startup" {
		t.Errorf("Source = %q, want startup", event.Source)
	}
}

func TestNormalizeSessionEnd(t *testing.T) {
	raw := `{
		"session_id": "abc123",
		"hook_event_name": "SessionEnd",
		"cwd": "/Users/dev/project",
		"reason": "prompt_input_exit",
		"session_duration_ms": 45000,
		"num_turns": 12
	}`

	event, err := NormalizeEvent([]byte(raw), 512)
	if err != nil {
		t.Fatalf("NormalizeEvent: %v", err)
	}

	if event.EventType != "session_end" {
		t.Errorf("EventType = %q, want session_end", event.EventType)
	}
	if event.Reason != "prompt_input_exit" {
		t.Errorf("Reason = %q, want prompt_input_exit", event.Reason)
	}
	if event.SessionDurationMs != 45000 {
		t.Errorf("SessionDurationMs = %d, want 45000", event.SessionDurationMs)
	}
	if event.NumTurns != 12 {
		t.Errorf("NumTurns = %d, want 12", event.NumTurns)
	}
}

func TestNormalizePostToolUseFailure(t *testing.T) {
	raw := `{
		"session_id": "abc123",
		"hook_event_name": "PostToolUseFailure",
		"cwd": "/Users/dev/project",
		"tool_name": "Bash",
		"tool_input": {"command": "bad-command"},
		"error": "command not found"
	}`

	event, err := NormalizeEvent([]byte(raw), 512)
	if err != nil {
		t.Fatalf("NormalizeEvent: %v", err)
	}

	if event.EventType != "post_tool_use_failure" {
		t.Errorf("EventType = %q, want post_tool_use_failure", event.EventType)
	}
	if event.Error != "command not found" {
		t.Errorf("Error = %q, want 'command not found'", event.Error)
	}
}

func TestEventToJSON(t *testing.T) {
	raw := `{
		"session_id": "abc123",
		"hook_event_name": "PostToolUse",
		"cwd": "/Users/dev",
		"tool_name": "Bash",
		"tool_input": {"command": "ls"}
	}`

	event, err := NormalizeEvent([]byte(raw), 512)
	if err != nil {
		t.Fatalf("NormalizeEvent: %v", err)
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var roundtrip Event
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if roundtrip.SessionID != "abc123" {
		t.Errorf("roundtrip SessionID = %q", roundtrip.SessionID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ingest/ -run TestNormalize -v`
Expected: FAIL — package not found

- [ ] **Step 3: Implement event schema and normalization**

Create `internal/ingest/schema.go`:

```go
package ingest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Event is the normalized AJ event schema.
type Event struct {
	Timestamp           time.Time              `json:"timestamp"`
	SessionID           string                 `json:"session_id"`
	Harness             string                 `json:"harness"`
	EventType           string                 `json:"event_type"`
	Source              string                 `json:"source,omitempty"`
	ToolName            string                 `json:"tool_name,omitempty"`
	ToolInput           map[string]interface{} `json:"tool_input,omitempty"`
	ToolResponseSummary string                 `json:"tool_response_summary,omitempty"`
	ExitCode            *int                   `json:"exit_code,omitempty"`
	WorkingDirectory    string                 `json:"working_directory"`
	Error               string                 `json:"error,omitempty"`
	Reason              string                 `json:"reason,omitempty"`
	SessionDurationMs   int64                  `json:"session_duration_ms,omitempty"`
	NumTurns            int                    `json:"num_turns,omitempty"`
	Metrics             *EventMetrics          `json:"metrics,omitempty"`
	BootstrapSource     string                 `json:"source_type,omitempty"`
}

type EventMetrics struct {
	ExecutionDurationMs int64 `json:"execution_duration_ms,omitempty"`
}

// hookPayload is the raw JSON structure from Claude Code hooks.
type hookPayload struct {
	SessionID        string                 `json:"session_id"`
	HookEventName    string                 `json:"hook_event_name"`
	CWD              string                 `json:"cwd"`
	ToolName         string                 `json:"tool_name"`
	ToolInput        map[string]interface{} `json:"tool_input"`
	ToolResponse     interface{}            `json:"tool_response"`
	ToolUseID        string                 `json:"tool_use_id"`
	Error            string                 `json:"error"`
	Source           string                 `json:"source"`
	Reason           string                 `json:"reason"`
	SessionDurationMs int64                 `json:"session_duration_ms"`
	NumTurns         int                    `json:"num_turns"`
}

// eventNameMap maps Claude Code hook event names to our normalized event types.
var eventNameMap = map[string]string{
	"PostToolUse":        "post_tool_use",
	"PostToolUseFailure": "post_tool_use_failure",
	"SessionStart":       "session_start",
	"SessionEnd":         "session_end",
}

// NormalizeEvent converts a raw Claude Code hook JSON payload into a normalized Event.
// maxResponseBytes controls truncation of tool_response.
func NormalizeEvent(raw []byte, maxResponseBytes int) (Event, error) {
	var payload hookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Event{}, fmt.Errorf("parsing hook payload: %w", err)
	}

	eventType, ok := eventNameMap[payload.HookEventName]
	if !ok {
		eventType = strings.ToLower(payload.HookEventName)
	}

	event := Event{
		Timestamp:        time.Now().UTC(),
		SessionID:        payload.SessionID,
		Harness:          "claude-code",
		EventType:        eventType,
		ToolName:         payload.ToolName,
		ToolInput:        payload.ToolInput,
		WorkingDirectory: payload.CWD,
		Error:            payload.Error,
		Source:           payload.Source,
		Reason:           payload.Reason,
		SessionDurationMs: payload.SessionDurationMs,
		NumTurns:         payload.NumTurns,
	}

	// Truncate tool response
	if payload.ToolResponse != nil {
		var summary string
		switch v := payload.ToolResponse.(type) {
		case string:
			summary = v
		default:
			data, _ := json.Marshal(v)
			summary = string(data)
		}
		if len(summary) > maxResponseBytes {
			summary = summary[:maxResponseBytes]
		}
		event.ToolResponseSummary = summary
	}

	return event, nil
}

// DateKey returns the date string (YYYY-MM-DD) for partitioning logs.
func (e Event) DateKey() string {
	return e.Timestamp.Format("2006-01-02")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ingest/ -run TestNormalize -v && go test ./internal/ingest/ -run TestEventToJSON -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ingest/schema.go internal/ingest/schema_test.go
git commit -m "feat: add event schema with normalization from hook payloads"
```

---

### Task 2: Implement JSONL Writer

**Files:**
- Create: `internal/ingest/writer.go`
- Create: `internal/ingest/writer_test.go`

- [ ] **Step 1: Write failing test for JSONL writer**

Create `internal/ingest/writer_test.go`:

```go
package ingest

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
)

func TestWriteEvent(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	w := NewWriter(paths)

	event := Event{
		Timestamp:        time.Date(2026, 4, 1, 5, 28, 45, 0, time.UTC),
		SessionID:        "cld_abc123",
		Harness:          "claude-code",
		EventType:        "post_tool_use",
		ToolName:         "Bash",
		ToolInput:        map[string]interface{}{"command": "ls"},
		WorkingDirectory: "/Users/dev",
	}

	if err := w.Write(event); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify file was created in correct location
	expectedPath := filepath.Join(root, "logs", "2026-04-01", "cld_abc123.jsonl")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected file at %s: %v", expectedPath, err)
	}

	// Read and verify content
	f, err := os.Open(expectedPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatal("expected one line in JSONL")
	}

	var read Event
	if err := json.Unmarshal(scanner.Bytes(), &read); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if read.SessionID != "cld_abc123" {
		t.Errorf("SessionID = %q, want cld_abc123", read.SessionID)
	}
	if read.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", read.ToolName)
	}
}

func TestWriteMultipleEvents(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()

	w := NewWriter(paths)

	for i := 0; i < 3; i++ {
		event := Event{
			Timestamp:        time.Date(2026, 4, 1, 5, 28, 45, 0, time.UTC),
			SessionID:        "cld_abc123",
			Harness:          "claude-code",
			EventType:        "post_tool_use",
			ToolName:         "Bash",
			WorkingDirectory: "/Users/dev",
		}
		if err := w.Write(event); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	expectedPath := filepath.Join(root, "logs", "2026-04-01", "cld_abc123.jsonl")
	f, _ := os.Open(expectedPath)
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	if count != 3 {
		t.Errorf("line count = %d, want 3", count)
	}
}

func TestWriteDifferentSessions(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()

	w := NewWriter(paths)

	e1 := Event{
		Timestamp: time.Date(2026, 4, 1, 5, 0, 0, 0, time.UTC),
		SessionID: "session_a", Harness: "claude-code", EventType: "post_tool_use",
		WorkingDirectory: "/dev",
	}
	e2 := Event{
		Timestamp: time.Date(2026, 4, 1, 6, 0, 0, 0, time.UTC),
		SessionID: "session_b", Harness: "claude-code", EventType: "post_tool_use",
		WorkingDirectory: "/dev",
	}

	w.Write(e1)
	w.Write(e2)

	fileA := filepath.Join(root, "logs", "2026-04-01", "session_a.jsonl")
	fileB := filepath.Join(root, "logs", "2026-04-01", "session_b.jsonl")

	if _, err := os.Stat(fileA); err != nil {
		t.Errorf("session_a file missing: %v", err)
	}
	if _, err := os.Stat(fileB); err != nil {
		t.Errorf("session_b file missing: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ingest/ -run TestWrite -v`
Expected: FAIL — `NewWriter` undefined

- [ ] **Step 3: Implement JSONL writer**

Create `internal/ingest/writer.go`:

```go
package ingest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agent-jit/agentjit/internal/config"
)

// Writer appends normalized events to date/session-partitioned JSONL files.
type Writer struct {
	paths config.Paths
}

// NewWriter creates a Writer that stores logs under the given paths.
func NewWriter(paths config.Paths) *Writer {
	return &Writer{paths: paths}
}

// Write appends a single event to the appropriate JSONL file.
func (w *Writer) Write(event Event) error {
	dateDir := w.paths.SessionLogDir(event.DateKey())
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return fmt.Errorf("creating date dir: %w", err)
	}

	filePath := w.paths.SessionLogFile(event.DateKey(), event.SessionID)

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing event: %w", err)
	}

	return nil
}

// WriteToPath appends a single event to a specific file path. Used for fallback
// when the daemon is unreachable.
func WriteToPath(filePath string, event Event) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating dir: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ingest/ -run TestWrite -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ingest/writer.go internal/ingest/writer_test.go
git commit -m "feat: add JSONL writer with date/session partitioning"
```

---

### Task 3: Implement the Ingest Command

**Files:**
- Modify: `cmd/agentjit/ingest.go`
- Create: `internal/ingest/ingest.go`
- Create: `internal/ingest/ingest_test.go`

- [ ] **Step 1: Write test for ingest pipeline**

Create `internal/ingest/ingest_test.go`:

```go
package ingest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-jit/agentjit/internal/config"
)

func TestIngestFromStdin(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()

	input := `{
		"session_id": "test_session",
		"hook_event_name": "PostToolUse",
		"cwd": "/Users/dev/project",
		"tool_name": "Bash",
		"tool_input": {"command": "echo hello"},
		"tool_response": "hello"
	}`

	cfg := config.DefaultConfig()
	reader := bytes.NewReader([]byte(input))

	err := IngestFromReader(reader, paths, cfg)
	if err != nil {
		t.Fatalf("IngestFromReader: %v", err)
	}

	// Check that a log file was created
	entries, err := os.ReadDir(paths.Logs)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no date directories created")
	}

	// Find the JSONL file
	dateDir := filepath.Join(paths.Logs, entries[0].Name())
	files, _ := os.ReadDir(dateDir)
	if len(files) == 0 {
		t.Fatal("no JSONL files created")
	}
	if files[0].Name() != "test_session.jsonl" {
		t.Errorf("unexpected filename: %s", files[0].Name())
	}
}

func TestIngestEmptyInput(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()

	cfg := config.DefaultConfig()
	reader := bytes.NewReader([]byte(""))

	err := IngestFromReader(reader, paths, cfg)
	if err == nil {
		t.Error("expected error for empty input")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ingest/ -run TestIngest -v`
Expected: FAIL — `IngestFromReader` undefined

- [ ] **Step 3: Implement ingest pipeline**

Create `internal/ingest/ingest.go`:

```go
package ingest

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
)

// IngestFromReader reads a single JSON hook payload from r, normalizes it,
// and writes it to the log. It first tries forwarding to the daemon socket;
// on failure, falls back to direct file write.
func IngestFromReader(r io.Reader, paths config.Paths, cfg config.Config) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("empty input")
	}

	event, err := NormalizeEvent(data, cfg.Ingestion.MaxResponseBytes)
	if err != nil {
		return fmt.Errorf("normalizing event: %w", err)
	}

	// Try forwarding to daemon socket
	if err := forwardToDaemon(paths.Socket, data); err == nil {
		return nil
	}

	// Fallback: write directly to JSONL file
	writer := NewWriter(paths)
	return writer.Write(event)
}

// forwardToDaemon sends the raw payload to the daemon via Unix socket.
func forwardToDaemon(socketPath string, data []byte) error {
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write(data)
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ingest/ -run TestIngest -v`
Expected: All PASS

- [ ] **Step 5: Wire ingest command to pipeline**

Update `cmd/agentjit/ingest.go`:

```go
package main

import (
	"os"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/ingest"
	"github.com/spf13/cobra"
)

var ingestCmd = &cobra.Command{
	Use:    "ingest",
	Short:  "Receive hook JSON on stdin and forward to daemon",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		cfg, err := config.Load(paths.Config)
		if err != nil {
			cfg = config.DefaultConfig()
		}

		return ingest.IngestFromReader(os.Stdin, paths, cfg)
	},
}

func init() {
	rootCmd.AddCommand(ingestCmd)
}
```

- [ ] **Step 6: Build and test manually**

Run:
```bash
go build -o agentjit ./cmd/agentjit/
echo '{"session_id":"manual_test","hook_event_name":"PostToolUse","cwd":"/tmp","tool_name":"Bash","tool_input":{"command":"echo hi"}}' | ./aj ingest
ls ~/.aj/logs/
```
Expected: A date directory with `manual_test.jsonl` inside

- [ ] **Step 7: Commit**

```bash
git add internal/ingest/ingest.go internal/ingest/ingest_test.go cmd/agentjit/ingest.go
git commit -m "feat: implement ingest command with daemon forwarding and JSONL fallback"
```

---

### Task 4: Implement Hook Installation

**Files:**
- Create: `internal/hooks/install.go`
- Create: `internal/hooks/install_test.go`
- Create: `internal/hooks/templates.go`
- Modify: `cmd/agentjit/init_cmd.go`

- [ ] **Step 1: Write test for hook template generation**

Create `internal/hooks/install_test.go`:

```go
package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHookTemplates(t *testing.T) {
	hooks := AJHooks()

	if len(hooks) != 4 {
		t.Fatalf("expected 4 hook events, got %d", len(hooks))
	}

	// Verify PostToolUse is async
	ptHooks, ok := hooks["PostToolUse"]
	if !ok {
		t.Fatal("missing PostToolUse")
	}
	group := ptHooks[0]
	handler := group.Hooks[0]
	if !handler.Async {
		t.Error("PostToolUse should be async")
	}

	// Verify SessionStart is synchronous
	ssHooks := hooks["SessionStart"]
	ssHandler := ssHooks[0].Hooks[0]
	if ssHandler.Async {
		t.Error("SessionStart should be synchronous")
	}
}

func TestInstallHooksIntoEmptySettings(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Write empty settings
	os.WriteFile(settingsPath, []byte("{}"), 0644)

	if err := InstallHooks(settingsPath); err != nil {
		t.Fatalf("InstallHooks: %v", err)
	}

	// Read and verify
	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	hooks, ok := settings["hooks"]
	if !ok {
		t.Fatal("hooks key missing from settings")
	}

	hooksMap, ok := hooks.(map[string]interface{})
	if !ok {
		t.Fatal("hooks is not an object")
	}

	for _, event := range []string{"PostToolUse", "PostToolUseFailure", "SessionStart", "SessionEnd"} {
		if _, ok := hooksMap[event]; !ok {
			t.Errorf("missing hook event: %s", event)
		}
	}
}

func TestInstallHooksPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Write settings with an existing hook
	existing := `{
		"hooks": {
			"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo existing"}]}]
		},
		"model": "opus"
	}`
	os.WriteFile(settingsPath, []byte(existing), 0644)

	if err := InstallHooks(settingsPath); err != nil {
		t.Fatalf("InstallHooks: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	// Verify existing hook is preserved
	hooksMap := settings["hooks"].(map[string]interface{})
	if _, ok := hooksMap["PreToolUse"]; !ok {
		t.Error("existing PreToolUse hook was clobbered")
	}

	// Verify new hooks were added
	if _, ok := hooksMap["PostToolUse"]; !ok {
		t.Error("PostToolUse hook not added")
	}

	// Verify non-hook settings preserved
	if settings["model"] != "opus" {
		t.Error("model setting was clobbered")
	}
}

func TestInstallHooksIdempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsPath, []byte("{}"), 0644)

	// Install twice
	InstallHooks(settingsPath)
	InstallHooks(settingsPath)

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	// PostToolUse should have exactly 1 matcher group, not 2
	hooksMap := settings["hooks"].(map[string]interface{})
	ptHooks := hooksMap["PostToolUse"].([]interface{})
	if len(ptHooks) != 1 {
		t.Errorf("PostToolUse has %d groups after double install, want 1", len(ptHooks))
	}
}

func TestUninstallHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	existing := `{
		"hooks": {
			"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo existing"}]}],
			"PostToolUse": [{"hooks": [{"type": "command", "command": "aj ingest", "async": true}]}]
		},
		"model": "opus"
	}`
	os.WriteFile(settingsPath, []byte(existing), 0644)

	if err := UninstallHooks(settingsPath); err != nil {
		t.Fatalf("UninstallHooks: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	hooksMap := settings["hooks"].(map[string]interface{})

	// AJ hooks should be removed
	if _, ok := hooksMap["PostToolUse"]; ok {
		t.Error("PostToolUse should have been removed")
	}

	// Existing non-AJ hooks preserved
	if _, ok := hooksMap["PreToolUse"]; !ok {
		t.Error("PreToolUse should have been preserved")
	}

	// Non-hook settings preserved
	if settings["model"] != "opus" {
		t.Error("model was clobbered")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/hooks/ -v`
Expected: FAIL — package not found

- [ ] **Step 3: Implement hook templates**

Create `internal/hooks/templates.go`:

```go
package hooks

// HookHandler represents a single hook handler in Claude Code settings.
type HookHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Async   bool   `json:"async,omitempty"`
}

// MatcherGroup represents a matcher group containing handlers.
type MatcherGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []HookHandler `json:"hooks"`
}

// AJHooks returns the hook configuration for all AJ hook events.
func AJHooks() map[string][]MatcherGroup {
	return map[string][]MatcherGroup{
		"PostToolUse": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "aj ingest", Async: true},
				},
			},
		},
		"PostToolUseFailure": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "aj ingest", Async: true},
				},
			},
		},
		"SessionStart": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "aj daemon start --if-not-running && aj ingest"},
				},
			},
		},
		"SessionEnd": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "aj ingest", Async: true},
				},
			},
		},
	}
}

// isAJHook checks if a hook handler belongs to AJ.
func isAJHook(command string) bool {
	return len(command) >= 14 && (command[:14] == "agentjit inges" || command[:14] == "agentjit daemo")
}
```

- [ ] **Step 4: Implement hook install/uninstall**

Create `internal/hooks/install.go`:

```go
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InstallHooks merges AJ hooks into the given Claude Code settings file.
// Creates the file if it doesn't exist. Preserves existing hooks and settings.
// Idempotent — skips events where AJ hooks are already present.
func InstallHooks(settingsPath string) error {
	settings, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	hooksObj, _ := settings["hooks"].(map[string]interface{})
	if hooksObj == nil {
		hooksObj = make(map[string]interface{})
	}

	agentjitHooks := AJHooks()

	for event, groups := range agentjitHooks {
		existing, _ := hooksObj[event].([]interface{})

		// Check if AJ hooks already present
		if hasAJHooks(existing) {
			continue
		}

		// Marshal our groups and append
		for _, group := range groups {
			groupJSON, _ := json.Marshal(group)
			var groupMap interface{}
			json.Unmarshal(groupJSON, &groupMap)
			existing = append(existing, groupMap)
		}
		hooksObj[event] = existing
	}

	settings["hooks"] = hooksObj
	return writeSettings(settingsPath, settings)
}

// UninstallHooks removes AJ hooks from the given Claude Code settings file.
// Preserves all non-AJ hooks and settings.
func UninstallHooks(settingsPath string) error {
	settings, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	hooksObj, _ := settings["hooks"].(map[string]interface{})
	if hooksObj == nil {
		return nil
	}

	for event, val := range hooksObj {
		groups, ok := val.([]interface{})
		if !ok {
			continue
		}

		var kept []interface{}
		for _, g := range groups {
			groupMap, ok := g.(map[string]interface{})
			if !ok {
				kept = append(kept, g)
				continue
			}
			handlers, ok := groupMap["hooks"].([]interface{})
			if !ok {
				kept = append(kept, g)
				continue
			}

			hasAJ := false
			for _, h := range handlers {
				hm, ok := h.(map[string]interface{})
				if !ok {
					continue
				}
				cmd, _ := hm["command"].(string)
				if isAJHook(cmd) {
					hasAJ = true
					break
				}
			}
			if !hasAJ {
				kept = append(kept, g)
			}
		}

		if len(kept) == 0 {
			delete(hooksObj, event)
		} else {
			hooksObj[event] = kept
		}
	}

	if len(hooksObj) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooksObj
	}

	return writeSettings(settingsPath, settings)
}

func hasAJHooks(groups []interface{}) bool {
	for _, g := range groups {
		groupMap, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		handlers, ok := groupMap["hooks"].([]interface{})
		if !ok {
			continue
		}
		for _, h := range handlers {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			cmd, _ := hm["command"].(string)
			if isAJHook(cmd) {
				return true
			}
		}
	}
	return false
}

func readSettings(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("reading settings: %w", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing settings: %w", err)
	}
	return settings, nil
}

func writeSettings(path string, settings map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating settings dir: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/hooks/ -v`
Expected: All PASS

- [ ] **Step 6: Wire init command**

Update `cmd/agentjit/init_cmd.go`:

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/hooks"
	"github.com/spf13/cobra"
)

var initLocal bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize AJ and install Claude Code hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		// 1. Create directories
		fmt.Println("[AJ] Initializing...")
		fmt.Println()
		fmt.Println("1. Creating directories")
		if err := paths.EnsureDirs(); err != nil {
			return fmt.Errorf("creating directories: %w", err)
		}
		fmt.Printf("   ✓ %s\n", paths.Root)
		fmt.Printf("   ✓ %s\n", paths.Logs)
		fmt.Printf("   ✓ %s\n", paths.Skills)

		// 2. Write default config (if not exists)
		fmt.Println()
		fmt.Println("2. Writing default config")
		if _, err := os.Stat(paths.Config); os.IsNotExist(err) {
			if err := config.Save(paths.Config, config.DefaultConfig()); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}
			fmt.Printf("   ✓ %s\n", paths.Config)
		} else {
			fmt.Printf("   ✓ %s (already exists)\n", paths.Config)
		}

		// 3. Install hooks
		fmt.Println()
		fmt.Println("3. Installing Claude Code hooks")
		var settingsPath string
		if initLocal {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			settingsPath = config.ClaudeSettingsLocal(cwd)
			// Also create local skills directory
			localSkills := filepath.Join(cwd, ".claude", "skills")
			os.MkdirAll(localSkills, 0755)
		} else {
			settingsPath, err = config.ClaudeSettingsGlobal()
			if err != nil {
				return err
			}
		}

		if err := hooks.InstallHooks(settingsPath); err != nil {
			return fmt.Errorf("installing hooks: %w", err)
		}
		fmt.Printf("   ✓ Hooks installed in %s\n", settingsPath)

		// 4. Verify on PATH
		fmt.Println()
		fmt.Println("4. Verifying agentjit is on PATH")
		if path, err := exec.LookPath("agentjit"); err == nil {
			fmt.Printf("   ✓ Found at %s\n", path)
		} else {
			fmt.Println("   ⚠ agentjit not found on PATH. Add it to use hooks.")
		}

		fmt.Println()
		fmt.Println("[AJ] Ready. Hooks will activate on your next Claude Code session.")
		fmt.Println("[AJ] Run 'aj bootstrap' to import historical sessions.")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove AJ hooks and optionally delete data",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Uninstall global hooks
		settingsPath, err := config.ClaudeSettingsGlobal()
		if err != nil {
			return err
		}
		if err := hooks.UninstallHooks(settingsPath); err != nil {
			return fmt.Errorf("removing hooks: %w", err)
		}
		fmt.Printf("[AJ] Hooks removed from %s\n", settingsPath)

		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		fmt.Printf("[AJ] Data directory at %s was not removed. Delete manually if desired.\n", paths.Root)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initLocal, "local", false, "Install hooks into project-local .claude/settings.json")
	initCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(initCmd)
}
```

- [ ] **Step 7: Build and verify**

Run:
```bash
go build -o agentjit ./cmd/agentjit/
./aj init --help
```
Expected: Help text showing --local flag

- [ ] **Step 8: Commit**

```bash
git add internal/hooks/ cmd/agentjit/init_cmd.go
git commit -m "feat: implement hook installation with non-destructive merge into settings.json"
```
