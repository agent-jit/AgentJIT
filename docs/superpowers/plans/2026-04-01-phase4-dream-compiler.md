# Phase 4: Dream Compiler — Log Gathering, Context Building, Claude Invocation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `aj compile` command that gathers unprocessed logs and existing skills, constructs a context payload, invokes Claude CLI with the compiler prompt, and records results.

**Architecture:** The dream command is an orchestrator — it prepares data and shells out to `claude`. The compiler prompt is a markdown file that instructs Claude to perform pattern detection, parameterization, scope inference, ROI calculation, and skill generation.

**Tech Stack:** Go, standard library (os/exec, encoding/json, time), Claude CLI

**Depends on:** Phase 1 (config, paths), Phase 2 (ingest schema), Phase 3 (daemon trigger)

---

### Task 1: Implement Log Gatherer

**Files:**
- Create: `internal/compile/gatherer.go`
- Create: `internal/compile/gatherer_test.go`

- [ ] **Step 1: Write failing test for log gathering**

Create `internal/compile/gatherer_test.go`:

```go
package dream

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/ingest"
)

func writeTestEvent(t *testing.T, dir, sessionID string, ts time.Time) {
	t.Helper()
	dateKey := ts.Format("2006-01-02")
	dateDir := filepath.Join(dir, dateKey)
	os.MkdirAll(dateDir, 0755)

	event := ingest.Event{
		Timestamp:        ts,
		SessionID:        sessionID,
		Harness:          "claude-code",
		EventType:        "post_tool_use",
		ToolName:         "Bash",
		ToolInput:        map[string]interface{}{"command": "echo test"},
		WorkingDirectory: "/dev",
	}
	data, _ := json.Marshal(event)
	data = append(data, '\n')

	path := filepath.Join(dateDir, sessionID+".jsonl")
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	f.Write(data)
	f.Close()
}

func TestGatherUnprocessedLogs(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()

	// Write events across dates
	t1 := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	writeTestEvent(t, paths.Logs, "old_session", t1)
	writeTestEvent(t, paths.Logs, "new_session", t2)

	// Set marker to before t2 but after t1
	marker := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	WriteMarker(paths.CompileMarker, marker)

	events, err := GatherUnprocessedLogs(paths, 50000)
	if err != nil {
		t.Fatalf("GatherUnprocessedLogs: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("got %d events, want 1 (only new_session)", len(events))
	}
}

func TestGatherAllLogsNoMarker(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()

	t1 := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	writeTestEvent(t, paths.Logs, "session_a", t1)
	writeTestEvent(t, paths.Logs, "session_b", t2)

	events, err := GatherUnprocessedLogs(paths, 50000)
	if err != nil {
		t.Fatalf("GatherUnprocessedLogs: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("got %d events, want 2", len(events))
	}
}

func TestGatherRespectsMaxLines(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()

	ts := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		writeTestEvent(t, paths.Logs, "big_session", ts)
	}

	events, err := GatherUnprocessedLogs(paths, 5)
	if err != nil {
		t.Fatalf("GatherUnprocessedLogs: %v", err)
	}

	if len(events) > 5 {
		t.Errorf("got %d events, want <= 5", len(events))
	}
}

func TestWriteAndReadMarker(t *testing.T) {
	dir := t.TempDir()
	markerPath := filepath.Join(dir, "marker")

	ts := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	if err := WriteMarker(markerPath, ts); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	read, err := ReadMarker(markerPath)
	if err != nil {
		t.Fatalf("ReadMarker: %v", err)
	}

	if !read.Equal(ts) {
		t.Errorf("ReadMarker = %v, want %v", read, ts)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/compile/ -run TestGather -v`
Expected: FAIL — `GatherUnprocessedLogs` undefined

- [ ] **Step 3: Implement log gatherer**

Create `internal/compile/gatherer.go`:

```go
package dream

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/ingest"
)

// WriteMarker writes a timestamp to the dream marker file.
func WriteMarker(path string, t time.Time) error {
	return os.WriteFile(path, []byte(t.UTC().Format(time.RFC3339)), 0644)
}

// ReadMarker reads the timestamp from the dream marker file.
// Returns zero time if file doesn't exist.
func ReadMarker(path string) (time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, string(data))
}

// GatherUnprocessedLogs reads all JSONL events from log files newer than
// the last dream marker. Returns events sorted by timestamp, capped at maxLines.
func GatherUnprocessedLogs(paths config.Paths, maxLines int) ([]ingest.Event, error) {
	marker, _ := ReadMarker(paths.CompileMarker)

	dateDirs, err := os.ReadDir(paths.Logs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading logs dir: %w", err)
	}

	// Sort date dirs chronologically
	sort.Slice(dateDirs, func(i, j int) bool {
		return dateDirs[i].Name() < dateDirs[j].Name()
	})

	var allEvents []ingest.Event

	for _, dateDir := range dateDirs {
		if !dateDir.IsDir() {
			continue
		}

		// Check if this date dir is after marker
		if !marker.IsZero() {
			dirDate, err := time.Parse("2006-01-02", dateDir.Name())
			if err != nil {
				continue
			}
			// Skip dirs entirely before marker date
			if dirDate.Before(marker.Truncate(24 * time.Hour)) {
				continue
			}
		}

		dirPath := filepath.Join(paths.Logs, dateDir.Name())
		sessionFiles, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, sf := range sessionFiles {
			if filepath.Ext(sf.Name()) != ".jsonl" {
				continue
			}

			events, err := readJSONLFile(filepath.Join(dirPath, sf.Name()), marker)
			if err != nil {
				continue
			}

			allEvents = append(allEvents, events...)

			if len(allEvents) >= maxLines {
				allEvents = allEvents[len(allEvents)-maxLines:]
				break
			}
		}
	}

	// Sort by timestamp
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.Before(allEvents[j].Timestamp)
	})

	if len(allEvents) > maxLines {
		allEvents = allEvents[len(allEvents)-maxLines:]
	}

	return allEvents, nil
}

func readJSONLFile(path string, after time.Time) ([]ingest.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []ingest.Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var event ingest.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if !after.IsZero() && !event.Timestamp.After(after) {
			continue
		}
		events = append(events, event)
	}

	return events, scanner.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/compile/ -run "TestGather|TestWrite" -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/compile/gatherer.go internal/compile/gatherer_test.go
git commit -m "feat: add log gatherer with marker-based incremental processing"
```

---

### Task 2: Implement Existing Skills Inventory

**Files:**
- Create: `internal/skills/inventory.go`
- Create: `internal/skills/inventory_test.go`

- [ ] **Step 1: Write failing test for skill inventory**

Create `internal/skills/inventory_test.go`:

```go
package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSkillsDir(t *testing.T) {
	dir := t.TempDir()

	// Create a skill directory with skill.md
	skillDir := filepath.Join(dir, "get-logs")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "skill.md"), []byte(`---
name: get-logs
description: Fetch logs from pods
generated_by: agentjit
version: 1
created: 2026-04-01T06:00:00Z
scope: global
roi:
  savings_per_invocation: 18300
  observed_frequency: 7
---

## Usage
Fetch logs.
`), 0644)

	skills, err := ScanSkillsDir(dir)
	if err != nil {
		t.Fatalf("ScanSkillsDir: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}

	s := skills[0]
	if s.Name != "get-logs" {
		t.Errorf("Name = %q, want get-logs", s.Name)
	}
	if s.GeneratedBy != "agentjit" {
		t.Errorf("GeneratedBy = %q, want agentjit", s.GeneratedBy)
	}
}

func TestScanSkillsDirEmpty(t *testing.T) {
	dir := t.TempDir()

	skills, err := ScanSkillsDir(dir)
	if err != nil {
		t.Fatalf("ScanSkillsDir: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}

func TestScanSkillsDirNonExistent(t *testing.T) {
	skills, err := ScanSkillsDir("/nonexistent/dir")
	if err != nil {
		t.Fatalf("ScanSkillsDir: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/skills/ -v`
Expected: FAIL — package not found

- [ ] **Step 3: Implement skill inventory scanner**

Create `internal/skills/inventory.go`:

```go
package skills

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// SkillMeta holds metadata parsed from a skill.md frontmatter.
type SkillMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	GeneratedBy string `json:"generated_by"`
	Version     int    `json:"version"`
	Scope       string `json:"scope"`
	Path        string `json:"path"`
	RawContent  string `json:"raw_content"`
}

// ScanSkillsDir reads all skill directories under the given root and returns metadata.
func ScanSkillsDir(root string) ([]SkillMeta, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []SkillMeta

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(root, entry.Name(), "skill.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		meta := parseSkillFrontmatter(string(data))
		meta.Path = filepath.Join(root, entry.Name())
		meta.RawContent = string(data)

		if meta.Name == "" {
			meta.Name = entry.Name()
		}

		skills = append(skills, meta)
	}

	return skills, nil
}

func parseSkillFrontmatter(content string) SkillMeta {
	var meta SkillMeta

	scanner := bufio.NewScanner(strings.NewReader(content))
	inFrontmatter := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break // end of frontmatter
		}

		if !inFrontmatter {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			meta.Name = val
		case "description":
			meta.Description = val
		case "generated_by":
			meta.GeneratedBy = val
		case "scope":
			meta.Scope = val
		}
	}

	return meta
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/skills/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/skills/inventory.go internal/skills/inventory_test.go
git commit -m "feat: add skill inventory scanner for compiler"
```

---

### Task 3: Write the Compiler Prompt

**Files:**
- Create: `prompts/compiler.md`

- [ ] **Step 1: Write the compiler prompt**

Create `prompts/compiler.md`:

```markdown
# AJ Dream Compiler

You are a JIT compiler for autonomous coding agents. You analyze execution logs from Claude Code sessions to identify recurring multi-step patterns and compile them into deterministic, parameterized skills.

## Input

You will receive two sections of data:

### 1. Execution Logs (JSONL)
Each line is a JSON event with this schema:
- `timestamp` — when the event occurred
- `session_id` — which session this belongs to
- `event_type` — `post_tool_use`, `post_tool_use_failure`, `session_start`, `session_end`
- `tool_name` — the tool called (Bash, Read, Write, Edit, etc.)
- `tool_input` — the tool's input (e.g. `{"command": "kubectl logs ..."}`)
- `tool_response_summary` — truncated output
- `exit_code` — for shell commands
- `working_directory` — where the command ran

### 2. Existing Skills Inventory
A list of previously generated skills with their metadata, so you can update or deprecate them.

## Your Job

### Step 1: Pattern Detection
Scan the logs for sequences of 2+ consecutive tool calls that appear with the same logical structure across multiple sessions. A "pattern" means:
- Same sequence of tool names in the same order
- Same or similar commands/operations
- Potentially different parameter values (these become script arguments)

### Step 2: Filter by Thresholds
Only consider patterns that meet BOTH criteria:
- **Minimum frequency:** Appeared in at least {{MIN_PATTERN_FREQUENCY}} distinct sessions
- **Minimum token savings:** Estimated savings per invocation >= {{MIN_TOKEN_SAVINGS}} tokens

### Step 3: ROI Calculation
For each candidate pattern, calculate:
- `stochastic_cost`: Estimate input + output tokens by counting characters in tool_input and tool_response_summary across observed instances, dividing by 4 (rough token estimate)
- `deterministic_cost`: 200 tokens (skill invocation overhead)
- `savings_per_invocation`: stochastic_cost - deterministic_cost
- `total_projected_savings`: savings_per_invocation * observed_frequency

### Step 4: Scope Inference
Determine where each skill should be registered:
1. If the pattern appears in logs from 2+ distinct `working_directory` project roots → **global** (write to `{{GLOBAL_SKILLS_DIR}}`)
2. If it only appears in one project → **local** (write to `<project>/.claude/skills/`)
3. Fallback: if commands primarily use global CLIs ({{GLOBAL_CLI_TOOLS}}) → **global**

### Step 5: Manage Existing Skills
Before creating new skills, check the existing inventory:
- **Optimize**: If new data suggests an existing skill could have more parameters or better error handling, update it
- **Merge**: If two existing skills are frequently called in sequence, combine them
- **Deprecate**: If an existing skill hasn't appeared in logs for {{DEPRECATE_AFTER_SESSIONS}} sessions, mark it deprecated
- **Version**: When updating a skill, rename the old file to `skill.v<N>.md` as backup

### Step 6: Output Action Plan
Before writing any files, output your proposed changes:
```
## Proposed Changes
- NEW: <skill-name> (savings: X tokens/invocation, frequency: Y)
- UPDATE: <skill-name> v1→v2 (reason)
- DEPRECATE: <skill-name> (reason)
- MERGE: <skill-a> + <skill-b> → <merged-name>
```

### Step 7: Generate Skills
For each approved pattern, create a skill directory with:

**skill.md** — with YAML frontmatter containing:
- name, description, generated_by (always "agentjit"), version, created, updated
- source_pattern_hash (hash of the pattern's tool sequence)
- scope (global/local)
- roi (stochastic_tokens_avg, deterministic_tokens_avg, savings_per_invocation, observed_frequency, total_projected_savings)

Then a body with: Usage, Parameters, and Execution sections.

**companion script (.sh)** — a bash script that:
- Uses `set -euo pipefail`
- Takes parameters as positional arguments with usage messages
- Includes the actual commands from the observed pattern
- Handles errors with exit code 2 for auth/permission failures (triggers self-healing — Claude Code will receive the stderr and attempt to resolve)
- Exits 1 for other errors

### Step 8: Write Dream Log Entry
After generating all skills, output a JSON summary on a single line starting with `DREAM_LOG:`:
```
DREAM_LOG:{"timestamp":"...","skills_created":1,"skills_updated":0,"skills_deprecated":0,"details":[{"action":"create","name":"...","savings":12400}]}
```

## Constraints
- Do NOT generate skills for patterns below the configured thresholds
- Do NOT overwrite existing skills unless the new version has strictly higher ROI
- Do NOT generate skills for trivial single-command patterns unless they save significant tokens
- Always parameterize dynamic values (pod names, namespaces, file paths, branch names)
- Keep companion scripts simple and auditable
```

- [ ] **Step 2: Commit**

```bash
git add prompts/compiler.md
git commit -m "feat: add compiler prompt for Claude-driven pattern compilation"
```

---

### Task 4: Implement Dream Compiler Orchestrator

**Files:**
- Create: `internal/compile/compiler.go`
- Create: `internal/compile/compiler_test.go`

- [ ] **Step 1: Write test for context building**

Create `internal/compile/compiler_test.go`:

```go
package dream

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/ingest"
	"github.com/anthropics/agentjit/internal/skills"
)

func TestBuildContext(t *testing.T) {
	cfg := config.DefaultConfig()

	events := []ingest.Event{
		{
			Timestamp: time.Now(), SessionID: "s1", Harness: "claude-code",
			EventType: "post_tool_use", ToolName: "Bash",
			ToolInput: map[string]interface{}{"command": "ls"},
			WorkingDirectory: "/dev",
		},
	}

	existingSkills := []skills.SkillMeta{
		{Name: "get-logs", Description: "Fetch logs", GeneratedBy: "agentjit"},
	}

	context, err := BuildContext(events, existingSkills, cfg)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	if !strings.Contains(context, "s1") {
		t.Error("context should contain session ID")
	}
	if !strings.Contains(context, "get-logs") {
		t.Error("context should contain existing skill name")
	}
	if !strings.Contains(context, "EXECUTION LOGS") {
		t.Error("context should have EXECUTION LOGS section header")
	}
	if !strings.Contains(context, "EXISTING SKILLS") {
		t.Error("context should have EXISTING SKILLS section header")
	}
}

func TestBuildPrompt(t *testing.T) {
	cfg := config.DefaultConfig()

	// Create a temp prompt file
	dir := t.TempDir()
	promptPath := dir + "/compiler.md"
	os.WriteFile(promptPath, []byte("Template with {{MIN_PATTERN_FREQUENCY}} and {{GLOBAL_SKILLS_DIR}}"), 0644)

	prompt, err := BuildPrompt(promptPath, cfg, "/home/user/.agentjit/skills")
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if strings.Contains(prompt, "{{MIN_PATTERN_FREQUENCY}}") {
		t.Error("template variable not replaced")
	}
	if !strings.Contains(prompt, "3") {
		t.Error("expected min_pattern_frequency default of 3")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/compile/ -run TestBuild -v`
Expected: FAIL — `BuildContext` undefined

- [ ] **Step 3: Implement context builder and prompt templater**

Create `internal/compile/compiler.go`:

```go
package dream

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/ingest"
	"github.com/anthropics/agentjit/internal/skills"
)

// BuildContext creates the context payload string for the compiler.
func BuildContext(events []ingest.Event, existingSkills []skills.SkillMeta, cfg config.Config) (string, error) {
	var sb strings.Builder

	sb.WriteString("## EXECUTION LOGS\n\n")
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		sb.Write(data)
		sb.WriteByte('\n')
	}

	sb.WriteString("\n## EXISTING SKILLS INVENTORY\n\n")
	if len(existingSkills) == 0 {
		sb.WriteString("No existing skills.\n")
	} else {
		for _, skill := range existingSkills {
			sb.WriteString(fmt.Sprintf("### %s\n", skill.Name))
			sb.WriteString(fmt.Sprintf("- Description: %s\n", skill.Description))
			sb.WriteString(fmt.Sprintf("- Scope: %s\n", skill.Scope))
			sb.WriteString(fmt.Sprintf("- Path: %s\n", skill.Path))
			sb.WriteString(fmt.Sprintf("- Generated by: %s\n\n", skill.GeneratedBy))
			if skill.RawContent != "" {
				sb.WriteString("Full content:\n```\n")
				sb.WriteString(skill.RawContent)
				sb.WriteString("\n```\n\n")
			}
		}
	}

	return sb.String(), nil
}

// BuildPrompt reads the compiler prompt template and replaces config variables.
func BuildPrompt(promptPath string, cfg config.Config, globalSkillsDir string) (string, error) {
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("reading prompt: %w", err)
	}

	prompt := string(data)

	replacements := map[string]string{
		"{{MIN_PATTERN_FREQUENCY}}":  strconv.Itoa(cfg.Dream.MinPatternFrequency),
		"{{MIN_TOKEN_SAVINGS}}":      strconv.Itoa(cfg.Dream.MinTokenSavings),
		"{{DEPRECATE_AFTER_SESSIONS}}": strconv.Itoa(cfg.Dream.DeprecateAfterSessions),
		"{{GLOBAL_SKILLS_DIR}}":      globalSkillsDir,
		"{{GLOBAL_CLI_TOOLS}}":       strings.Join(cfg.Scope.GlobalCLITools, ", "),
	}

	for key, val := range replacements {
		prompt = strings.ReplaceAll(prompt, key, val)
	}

	return prompt, nil
}

// RunCompile executes the full compilation sequence.
func RunCompile(paths config.Paths, cfg config.Config, promptPath string) error {
	// 1. Gather unprocessed logs
	events, err := GatherUnprocessedLogs(paths, cfg.Dream.MaxContextLines)
	if err != nil {
		return fmt.Errorf("gathering logs: %w", err)
	}

	if len(events) == 0 {
		fmt.Println("[AJ] No new events to process")
		return nil
	}

	// 2. Gather existing skills
	existingSkills, _ := skills.ScanSkillsDir(paths.Skills)

	// 3. Build context
	context, err := BuildContext(events, existingSkills, cfg)
	if err != nil {
		return fmt.Errorf("building context: %w", err)
	}

	// 4. Build prompt
	prompt, err := BuildPrompt(promptPath, cfg, paths.Skills)
	if err != nil {
		return fmt.Errorf("building prompt: %w", err)
	}

	// 5. Write context to temp file
	tmpFile, err := os.CreateTemp("", "agentjit-dream-*.jsonl")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(context)
	tmpFile.Close()

	// 6. Invoke Claude
	fmt.Printf("[AJ] Starting compilation (%d events, %d existing skills)\n",
		len(events), len(existingSkills))

	cmd := exec.Command("claude",
		"--print",
		"-p", prompt,
		"--allowedTools", "Read,Write,Bash,Glob,Grep",
	)

	contextData, _ := os.ReadFile(tmpFile.Name())
	cmd.Stdin = strings.NewReader(string(contextData))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running claude: %w", err)
	}

	// 7. Update marker
	if err := WriteMarker(paths.CompileMarker, time.Now().UTC()); err != nil {
		return fmt.Errorf("writing marker: %w", err)
	}

	fmt.Println("[AJ] Dream compilation complete")
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/compile/ -run TestBuild -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/compile/compiler.go internal/compile/compiler_test.go
git commit -m "feat: add compiler orchestrator with context building"
```

---

### Task 5: Wire Dream Command

**Files:**
- Modify: `cmd/agentjit/dream.go`

- [ ] **Step 1: Implement dream command**

Update `cmd/agentjit/dream.go`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/dream"
	"github.com/spf13/cobra"
)

var dreamCmd = &cobra.Command{
	Use:   "dream",
	Short: "Trigger the JIT compilation/reflection phase",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		cfg, err := config.Load(paths.Config)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Find the compiler prompt
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("finding executable: %w", err)
		}
		promptPath := filepath.Join(filepath.Dir(exe), "prompts", "compiler.md")

		// Fallback: check relative to working directory
		if _, err := os.Stat(promptPath); os.IsNotExist(err) {
			cwd, _ := os.Getwd()
			promptPath = filepath.Join(cwd, "prompts", "compiler.md")
		}

		if _, err := os.Stat(promptPath); os.IsNotExist(err) {
			return fmt.Errorf("compiler prompt not found at %s — run from the agentjit project directory or install properly", promptPath)
		}

		return dream.RunCompile(paths, cfg, promptPath)
	},
}

func init() {
	rootCmd.AddCommand(dreamCmd)
}
```

- [ ] **Step 2: Build and verify**

Run:
```bash
go build -o agentjit ./cmd/agentjit/
./aj compile
```
Expected: "[AJ] No new events to process" (since no logs yet)

- [ ] **Step 3: Commit**

```bash
git add cmd/agentjit/dream.go
git commit -m "feat: wire dream command to compiler orchestrator"
```
