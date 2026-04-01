# Phase 1: Foundation — Project Scaffold, Config, CLI Skeleton

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Set up the Go project with module, directory structure, configuration system, path helpers, and a working CLI skeleton with all subcommands stubbed.

**Architecture:** Single Go binary using cobra for CLI. Config is a JSON file at `~/.agentjit/config.json` with typed Go structs and sensible defaults. Path helpers centralize all filesystem path logic.

**Tech Stack:** Go 1.22+, cobra (CLI), standard library (encoding/json, os, path/filepath)

---

### Task 1: Initialize Go Module and Directory Structure

**Files:**
- Create: `go.mod`
- Create: `cmd/agentjit/main.go`
- Create: `internal/config/paths.go`
- Create: `internal/config/config.go`
- Create: `Makefile`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd /Users/pc/web3/agentjit
go mod init github.com/anthropics/agentjit
```
Expected: `go.mod` created with module path

- [ ] **Step 2: Create directory structure**

Run:
```bash
mkdir -p cmd/agentjit
mkdir -p internal/{config,daemon,ingest,dream,hooks,skills,bootstrap}
mkdir -p prompts
```

- [ ] **Step 3: Create path helpers**

Create `internal/config/paths.go`:

```go
package config

import (
	"os"
	"path/filepath"
)

// Paths holds all filesystem paths used by AgentJIT.
type Paths struct {
	Root       string // ~/.agentjit
	Config     string // ~/.agentjit/config.json
	Logs       string // ~/.agentjit/logs
	Skills     string // ~/.agentjit/skills
	PID        string // ~/.agentjit/daemon.pid
	Socket     string // ~/.agentjit/daemon.sock
	DreamLog   string // ~/.agentjit/dream-log.jsonl
	DreamMarker string // ~/.agentjit/last_dream_marker
	BootstrapProcessed string // ~/.agentjit/bootstrap_processed.json
}

// DefaultPaths returns Paths rooted at ~/.agentjit.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	root := filepath.Join(home, ".agentjit")
	return PathsFromRoot(root), nil
}

// PathsFromRoot returns Paths rooted at the given directory.
// Useful for testing with a temp directory.
func PathsFromRoot(root string) Paths {
	return Paths{
		Root:               root,
		Config:             filepath.Join(root, "config.json"),
		Logs:               filepath.Join(root, "logs"),
		Skills:             filepath.Join(root, "skills"),
		PID:                filepath.Join(root, "daemon.pid"),
		Socket:             filepath.Join(root, "daemon.sock"),
		DreamLog:           filepath.Join(root, "dream-log.jsonl"),
		DreamMarker:        filepath.Join(root, "last_dream_marker"),
		BootstrapProcessed: filepath.Join(root, "bootstrap_processed.json"),
	}
}

// EnsureDirs creates the root, logs, and skills directories if they don't exist.
func (p Paths) EnsureDirs() error {
	for _, dir := range []string{p.Root, p.Logs, p.Skills} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// SessionLogDir returns the log directory for a given date string (YYYY-MM-DD).
func (p Paths) SessionLogDir(date string) string {
	return filepath.Join(p.Logs, date)
}

// SessionLogFile returns the JSONL file path for a given date and session ID.
func (p Paths) SessionLogFile(date, sessionID string) string {
	return filepath.Join(p.Logs, date, sessionID+".jsonl")
}

// ClaudeSettingsGlobal returns the path to Claude Code's global settings.
func ClaudeSettingsGlobal() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// ClaudeSettingsLocal returns the path to Claude Code's local project settings.
func ClaudeSettingsLocal(projectDir string) string {
	return filepath.Join(projectDir, ".claude", "settings.json")
}

// ClaudeProjectsDir returns the path to Claude Code's projects directory.
func ClaudeProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}
```

- [ ] **Step 4: Create config struct with defaults**

Create `internal/config/config.go`:

```go
package config

import (
	"encoding/json"
	"os"
)

type DaemonConfig struct {
	IdleTimeoutMinutes int    `json:"idle_timeout_minutes"`
	SocketPath         string `json:"socket_path,omitempty"`
}

type IngestionConfig struct {
	MaxResponseBytes int `json:"max_response_bytes"`
	LogRetentionDays int `json:"log_retention_days"`
}

type DreamConfig struct {
	TriggerMode           string `json:"trigger_mode"`
	TriggerIntervalMinutes int   `json:"trigger_interval_minutes"`
	TriggerEventThreshold  int   `json:"trigger_event_threshold"`
	MaxContextLines        int   `json:"max_context_lines"`
	MinPatternFrequency    int   `json:"min_pattern_frequency"`
	MinTokenSavings        int   `json:"min_token_savings"`
	DeprecateAfterSessions int   `json:"deprecate_after_sessions"`
}

type ScopeConfig struct {
	GlobalCLITools        []string `json:"global_cli_tools"`
	CrossProjectThreshold int      `json:"cross_project_threshold"`
}

type Config struct {
	Daemon    DaemonConfig    `json:"daemon"`
	Ingestion IngestionConfig `json:"ingestion"`
	Dream     DreamConfig     `json:"dream"`
	Scope     ScopeConfig     `json:"scope"`
}

func DefaultConfig() Config {
	return Config{
		Daemon: DaemonConfig{
			IdleTimeoutMinutes: 30,
		},
		Ingestion: IngestionConfig{
			MaxResponseBytes: 512,
			LogRetentionDays: 30,
		},
		Dream: DreamConfig{
			TriggerMode:            "manual",
			TriggerIntervalMinutes: 30,
			TriggerEventThreshold:  100,
			MaxContextLines:        50000,
			MinPatternFrequency:    3,
			MinTokenSavings:        500,
			DeprecateAfterSessions: 20,
		},
		Scope: ScopeConfig{
			GlobalCLITools: []string{
				"kubectl", "az", "gh", "docker", "aws",
				"gcloud", "terraform", "helm", "ssh", "scp",
			},
			CrossProjectThreshold: 2,
		},
	}
}

// Load reads config from the given path. Returns default config if file doesn't exist.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Save writes config to the given path.
func Save(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

- [ ] **Step 5: Commit**

```bash
git add go.mod cmd/ internal/config/ Makefile
git commit -m "feat: initialize Go module with config and path helpers"
```

---

### Task 2: Write Tests for Config and Paths

**Files:**
- Create: `internal/config/paths_test.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write path helper tests**

Create `internal/config/paths_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsFromRoot(t *testing.T) {
	p := PathsFromRoot("/tmp/agentjit-test")

	if p.Root != "/tmp/agentjit-test" {
		t.Errorf("Root = %q, want /tmp/agentjit-test", p.Root)
	}
	if p.Config != "/tmp/agentjit-test/config.json" {
		t.Errorf("Config = %q, want config.json", p.Config)
	}
	if p.PID != "/tmp/agentjit-test/daemon.pid" {
		t.Errorf("PID = %q, want daemon.pid", p.PID)
	}
	if p.Socket != "/tmp/agentjit-test/daemon.sock" {
		t.Errorf("Socket = %q, want daemon.sock", p.Socket)
	}
}

func TestSessionLogFile(t *testing.T) {
	p := PathsFromRoot("/tmp/agentjit-test")
	got := p.SessionLogFile("2026-04-01", "cld_abc123")
	want := "/tmp/agentjit-test/logs/2026-04-01/cld_abc123.jsonl"
	if got != want {
		t.Errorf("SessionLogFile = %q, want %q", got, want)
	}
}

func TestEnsureDirs(t *testing.T) {
	root := t.TempDir()
	p := PathsFromRoot(root)

	if err := p.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	for _, dir := range []string{p.Root, p.Logs, p.Skills} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %q not created: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", dir)
		}
	}
}

func TestClaudeSettingsLocal(t *testing.T) {
	got := ClaudeSettingsLocal("/Users/dev/project")
	want := filepath.Join("/Users/dev/project", ".claude", "settings.json")
	if got != want {
		t.Errorf("ClaudeSettingsLocal = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run path tests to verify they pass**

Run: `go test ./internal/config/ -run TestPaths -v && go test ./internal/config/ -run TestSessionLogFile -v && go test ./internal/config/ -run TestEnsureDirs -v && go test ./internal/config/ -run TestClaudeSettingsLocal -v`
Expected: All PASS

- [ ] **Step 3: Write config load/save tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Daemon.IdleTimeoutMinutes != 30 {
		t.Errorf("IdleTimeoutMinutes = %d, want 30", cfg.Daemon.IdleTimeoutMinutes)
	}
	if cfg.Dream.TriggerMode != "manual" {
		t.Errorf("TriggerMode = %q, want manual", cfg.Dream.TriggerMode)
	}
	if cfg.Ingestion.MaxResponseBytes != 512 {
		t.Errorf("MaxResponseBytes = %d, want 512", cfg.Ingestion.MaxResponseBytes)
	}
	if len(cfg.Scope.GlobalCLITools) != 10 {
		t.Errorf("GlobalCLITools len = %d, want 10", len(cfg.Scope.GlobalCLITools))
	}
}

func TestLoadNonExistent(t *testing.T) {
	cfg, err := Load("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("Load nonexistent: %v", err)
	}
	if cfg.Dream.TriggerMode != "manual" {
		t.Errorf("expected default config, got TriggerMode = %q", cfg.Dream.TriggerMode)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	cfg.Dream.TriggerMode = "interval"
	cfg.Dream.TriggerIntervalMinutes = 15

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Dream.TriggerMode != "interval" {
		t.Errorf("TriggerMode = %q, want interval", loaded.Dream.TriggerMode)
	}
	if loaded.Dream.TriggerIntervalMinutes != 15 {
		t.Errorf("TriggerIntervalMinutes = %d, want 15", loaded.Dream.TriggerIntervalMinutes)
	}
	// Verify defaults are preserved for unmodified fields
	if loaded.Daemon.IdleTimeoutMinutes != 30 {
		t.Errorf("IdleTimeoutMinutes = %d, want 30", loaded.Daemon.IdleTimeoutMinutes)
	}
}

func TestLoadPartialJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write a partial config — only dream section
	partial := []byte(`{"dream": {"trigger_mode": "event_count"}}`)
	if err := os.WriteFile(path, partial, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Dream.TriggerMode != "event_count" {
		t.Errorf("TriggerMode = %q, want event_count", cfg.Dream.TriggerMode)
	}
	// Default values for unspecified fields should be zero values, not defaults
	// because json.Unmarshal overwrites the struct
	// This is intentional — partial config overrides defaults
}
```

- [ ] **Step 4: Run config tests to verify they pass**

Run: `go test ./internal/config/ -run TestDefault -v && go test ./internal/config/ -run TestLoad -v && go test ./internal/config/ -run TestSave -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/paths_test.go internal/config/config_test.go
git commit -m "test: add config and paths unit tests"
```

---

### Task 3: CLI Skeleton with Cobra

**Files:**
- Create: `cmd/agentjit/main.go`
- Create: `cmd/agentjit/root.go`
- Create: `cmd/agentjit/init_cmd.go`
- Create: `cmd/agentjit/daemon.go`
- Create: `cmd/agentjit/dream.go`
- Create: `cmd/agentjit/bootstrap_cmd.go`
- Create: `cmd/agentjit/config_cmd.go`
- Create: `cmd/agentjit/skills_cmd.go`
- Create: `cmd/agentjit/ingest.go`

- [ ] **Step 1: Add cobra dependency**

Run:
```bash
cd /Users/pc/web3/agentjit
go get github.com/spf13/cobra@latest
```

- [ ] **Step 2: Create root command**

Create `cmd/agentjit/root.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "agentjit",
	Short: "Background JIT compiler for autonomous coding agents",
	Long:  "AgentJIT silently ingests agent execution telemetry, identifies recurring patterns, and compiles them into parameterized skills.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Create main.go**

Create `cmd/agentjit/main.go`:

```go
package main

func main() {
	Execute()
}
```

- [ ] **Step 4: Create init command stub**

Create `cmd/agentjit/init_cmd.go`:

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initLocal bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize AgentJIT and install Claude Code hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] init not yet implemented")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove AgentJIT hooks and optionally delete data",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] uninstall not yet implemented")
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initLocal, "local", false, "Install hooks into project-local .claude/settings.json")
	initCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(initCmd)
}
```

- [ ] **Step 5: Create daemon command stub**

Create `cmd/agentjit/daemon.go`:

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the AgentJIT daemon",
}

var ifNotRunning bool

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the AgentJIT daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] daemon start not yet implemented")
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the AgentJIT daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] daemon stop not yet implemented")
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] daemon status not yet implemented")
		return nil
	},
}

func init() {
	daemonStartCmd.Flags().BoolVar(&ifNotRunning, "if-not-running", false, "Start only if not already running")
	daemonCmd.AddCommand(daemonStartCmd, daemonStopCmd, daemonStatusCmd)
	rootCmd.AddCommand(daemonCmd)
}
```

- [ ] **Step 6: Create dream command stub**

Create `cmd/agentjit/dream.go`:

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var dreamCmd = &cobra.Command{
	Use:   "dream",
	Short: "Trigger the JIT compilation/reflection phase",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] dream not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dreamCmd)
}
```

- [ ] **Step 7: Create bootstrap command stub**

Create `cmd/agentjit/bootstrap_cmd.go`:

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var bootstrapSince string
var bootstrapProject string
var bootstrapDryRun bool

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Import historical Claude Code transcripts into logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] bootstrap not yet implemented")
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

- [ ] **Step 8: Create config command stub**

Create `cmd/agentjit/config_cmd.go`:

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configAll bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or modify AgentJIT configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a config value",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] config get not yet implemented")
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] config set not yet implemented")
		return nil
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] config reset not yet implemented")
		return nil
	},
}

func init() {
	configGetCmd.Flags().BoolVar(&configAll, "all", false, "Dump full config")
	configCmd.AddCommand(configGetCmd, configSetCmd, configResetCmd)
	rootCmd.AddCommand(configCmd)
}
```

- [ ] **Step 9: Create skills command stub**

Create `cmd/agentjit/skills_cmd.go`:

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage generated skills",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List generated skills with ROI metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] skills list not yet implemented")
		return nil
	},
}

var skillsRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a generated skill and deregister it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] skills remove not yet implemented")
		return nil
	},
}

func init() {
	skillsCmd.AddCommand(skillsListCmd, skillsRemoveCmd)
	rootCmd.AddCommand(skillsCmd)
}
```

- [ ] **Step 10: Create ingest command stub**

Create `cmd/agentjit/ingest.go`:

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var ingestCmd = &cobra.Command{
	Use:    "ingest",
	Short:  "Receive hook JSON on stdin and forward to daemon",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] ingest not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ingestCmd)
}
```

- [ ] **Step 11: Build and verify CLI**

Run:
```bash
cd /Users/pc/web3/agentjit
go build -o agentjit ./cmd/agentjit/
./agentjit --help
./agentjit daemon --help
./agentjit config --help
./agentjit skills --help
```
Expected: Help text for all commands with subcommands listed

- [ ] **Step 12: Create Makefile**

Create `Makefile`:

```makefile
.PHONY: build test clean install

BINARY=agentjit
BUILD_DIR=./cmd/agentjit

build:
	go build -o $(BINARY) $(BUILD_DIR)

test:
	go test ./...

clean:
	rm -f $(BINARY)

install: build
	mv $(BINARY) /usr/local/bin/$(BINARY)
```

- [ ] **Step 13: Commit**

```bash
git add cmd/ Makefile go.mod go.sum
git commit -m "feat: add CLI skeleton with all subcommands stubbed"
```

---

### Task 4: Implement Config Get/Set/Reset Commands

**Files:**
- Modify: `cmd/agentjit/config_cmd.go`
- Create: `internal/config/accessor.go`
- Create: `internal/config/accessor_test.go`

- [ ] **Step 1: Write failing test for config accessor**

Create `internal/config/accessor_test.go`:

```go
package config

import (
	"testing"
)

func TestGetField(t *testing.T) {
	cfg := DefaultConfig()

	val, err := GetField(cfg, "dream.trigger_mode")
	if err != nil {
		t.Fatalf("GetField: %v", err)
	}
	if val != "manual" {
		t.Errorf("GetField = %q, want manual", val)
	}
}

func TestGetFieldNested(t *testing.T) {
	cfg := DefaultConfig()

	val, err := GetField(cfg, "daemon.idle_timeout_minutes")
	if err != nil {
		t.Fatalf("GetField: %v", err)
	}
	if val != float64(30) {
		t.Errorf("GetField = %v, want 30", val)
	}
}

func TestGetFieldInvalid(t *testing.T) {
	cfg := DefaultConfig()

	_, err := GetField(cfg, "nonexistent.key")
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestSetField(t *testing.T) {
	cfg := DefaultConfig()

	updated, err := SetField(cfg, "dream.trigger_mode", "interval")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}

	val, _ := GetField(updated, "dream.trigger_mode")
	if val != "interval" {
		t.Errorf("after SetField, got %q, want interval", val)
	}
}

func TestSetFieldNumeric(t *testing.T) {
	cfg := DefaultConfig()

	updated, err := SetField(cfg, "dream.trigger_interval_minutes", "15")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}

	val, _ := GetField(updated, "dream.trigger_interval_minutes")
	if val != float64(15) {
		t.Errorf("after SetField, got %v, want 15", val)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run TestGetField -v`
Expected: FAIL — `GetField` undefined

- [ ] **Step 3: Implement config accessor**

Create `internal/config/accessor.go`:

```go
package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// GetField retrieves a value from Config by dot-notation key (e.g. "dream.trigger_mode").
func GetField(cfg Config, key string) (interface{}, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid key %q: must be section.field (e.g. dream.trigger_mode)", key)
	}

	section, ok := m[parts[0]]
	if !ok {
		return nil, fmt.Errorf("unknown section %q", parts[0])
	}

	sectionMap, ok := section.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("section %q is not an object", parts[0])
	}

	val, ok := sectionMap[parts[1]]
	if !ok {
		return nil, fmt.Errorf("unknown key %q in section %q", parts[1], parts[0])
	}

	return val, nil
}

// SetField updates a value in Config by dot-notation key. Values are auto-typed
// (numbers become float64, "true"/"false" become bool, everything else stays string).
func SetField(cfg Config, key, value string) (Config, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return cfg, err
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return cfg, err
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return cfg, fmt.Errorf("invalid key %q: must be section.field", key)
	}

	section, ok := m[parts[0]]
	if !ok {
		return cfg, fmt.Errorf("unknown section %q", parts[0])
	}

	sectionMap, ok := section.(map[string]interface{})
	if !ok {
		return cfg, fmt.Errorf("section %q is not an object", parts[0])
	}

	if _, ok := sectionMap[parts[1]]; !ok {
		return cfg, fmt.Errorf("unknown key %q in section %q", parts[1], parts[0])
	}

	// Auto-type the value
	var typed interface{}
	if n, err := strconv.ParseFloat(value, 64); err == nil {
		typed = n
	} else if b, err := strconv.ParseBool(value); err == nil {
		typed = b
	} else {
		typed = value
	}

	sectionMap[parts[1]] = typed
	m[parts[0]] = sectionMap

	updated, err := json.Marshal(m)
	if err != nil {
		return cfg, err
	}

	var result Config
	if err := json.Unmarshal(updated, &result); err != nil {
		return cfg, err
	}

	return result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -run "TestGetField|TestSetField" -v`
Expected: All PASS

- [ ] **Step 5: Wire config commands to accessor**

Update `cmd/agentjit/config_cmd.go`:

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/spf13/cobra"
)

var configAll bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or modify AgentJIT configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a config value",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.Config)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if configAll || len(args) == 0 {
			data, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		val, err := config.GetField(cfg, args[0])
		if err != nil {
			return err
		}
		fmt.Println(val)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.Config)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		updated, err := config.SetField(cfg, args[0], args[1])
		if err != nil {
			return err
		}

		if err := config.Save(paths.Config, updated); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Printf("[AgentJIT] Set %s = %s\n", args[0], args[1])
		return nil
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}
		if err := config.Save(paths.Config, config.DefaultConfig()); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Println("[AgentJIT] Config reset to defaults")
		return nil
	},
}

func init() {
	configGetCmd.Flags().BoolVar(&configAll, "all", false, "Dump full config")
	configCmd.AddCommand(configGetCmd, configSetCmd, configResetCmd)
	rootCmd.AddCommand(configCmd)
}
```

- [ ] **Step 6: Build and verify**

Run:
```bash
go build -o agentjit ./cmd/agentjit/
./agentjit config get --all
```
Expected: Prints default config JSON

- [ ] **Step 7: Commit**

```bash
git add internal/config/accessor.go internal/config/accessor_test.go cmd/agentjit/config_cmd.go
git commit -m "feat: implement config get/set/reset commands"
```
