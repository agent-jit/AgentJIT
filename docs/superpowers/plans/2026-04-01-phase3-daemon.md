# Phase 3: Daemon — Unix Socket Server, Lifecycle, Dream Triggers

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the persistent daemon that receives events over a Unix socket, manages its own lifecycle (PID, idle timeout, auto-start), and monitors dream trigger conditions.

**Architecture:** Single Go process with three goroutines: socket server, dream trigger monitor, and idle timeout watcher. Communicates with the CLI via Unix domain socket using a simple line-delimited JSON protocol.

**Tech Stack:** Go, standard library (net, os, os/signal, sync, time)

**Depends on:** Phase 1 (config, paths), Phase 2 (ingest schema, writer)

---

### Task 1: Implement PID and Socket Lifecycle

**Files:**
- Create: `internal/daemon/lifecycle.go`
- Create: `internal/daemon/lifecycle_test.go`

- [ ] **Step 1: Write failing test for PID management**

Create `internal/daemon/lifecycle_test.go`:

```go
package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestWriteAndReadPID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")

	if err := WritePID(pidPath); err != nil {
		t.Fatalf("WritePID: %v", err)
	}

	pid, err := ReadPID(pidPath)
	if err != nil {
		t.Fatalf("ReadPID: %v", err)
	}

	if pid != os.Getpid() {
		t.Errorf("PID = %d, want %d", pid, os.Getpid())
	}
}

func TestReadPIDNonExistent(t *testing.T) {
	_, err := ReadPID("/nonexistent/daemon.pid")
	if err == nil {
		t.Error("expected error for nonexistent PID file")
	}
}

func TestIsRunning(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")

	// No PID file — not running
	if IsRunning(pidPath) {
		t.Error("should not be running without PID file")
	}

	// Write current PID — should be running
	WritePID(pidPath)
	if !IsRunning(pidPath) {
		t.Error("should be running with valid PID")
	}

	// Write bogus PID — should not be running
	os.WriteFile(pidPath, []byte("999999999"), 0644)
	if IsRunning(pidPath) {
		t.Error("should not be running with invalid PID")
	}
}

func TestCleanupStalePID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")

	// Write a stale PID
	os.WriteFile(pidPath, []byte("999999999"), 0644)

	CleanupStalePID(pidPath)

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("stale PID file should have been removed")
	}
}

func TestRemovePID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")

	WritePID(pidPath)
	RemovePID(pidPath)

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file should have been removed")
	}
}

func TestWritePIDContent(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")

	WritePID(pidPath)

	data, _ := os.ReadFile(pidPath)
	pidStr := string(data)
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		t.Fatalf("PID file content is not a number: %q", pidStr)
	}
	if pid != os.Getpid() {
		t.Errorf("PID = %d, want %d", pid, os.Getpid())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/daemon/ -run TestWrite -v`
Expected: FAIL — package not found

- [ ] **Step 3: Implement PID lifecycle**

Create `internal/daemon/lifecycle.go`:

```go
package daemon

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

// WritePID writes the current process PID to the given path.
func WritePID(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// ReadPID reads a PID from the given file.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

// IsRunning checks if a daemon is running by reading the PID file
// and verifying the process exists.
func IsRunning(pidPath string) bool {
	pid, err := ReadPID(pidPath)
	if err != nil {
		return false
	}
	return processExists(pid)
}

// CleanupStalePID removes the PID file if the referenced process is not running.
func CleanupStalePID(pidPath string) {
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		return
	}
	if !IsRunning(pidPath) {
		os.Remove(pidPath)
	}
}

// RemovePID removes the PID file.
func RemovePID(path string) {
	os.Remove(path)
}

// processExists checks whether a process with the given PID exists.
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check existence.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// StartDaemonProcess starts the daemon as a background process by re-executing
// the current binary with "daemon start" arguments.
func StartDaemonProcess() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	attr := &os.ProcAttr{
		Dir:   "/",
		Env:   os.Environ(),
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	}

	proc, err := os.StartProcess(exe, []string{exe, "daemon", "start", "--foreground"}, attr)
	if err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}

	// Detach from child
	proc.Release()
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/daemon/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/lifecycle.go internal/daemon/lifecycle_test.go
git commit -m "feat: add daemon PID lifecycle management"
```

---

### Task 2: Implement Unix Socket Server

**Files:**
- Create: `internal/daemon/server.go`
- Create: `internal/daemon/server_test.go`

- [ ] **Step 1: Write failing test for socket server**

Create `internal/daemon/server_test.go`:

```go
package daemon

import (
	"encoding/json"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
)

func TestServerStartAndAcceptEvent(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()
	cfg := config.DefaultConfig()

	socketPath := filepath.Join(root, "test.sock")
	srv := NewServer(socketPath, paths, cfg)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.Start()
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Send an event
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	payload := map[string]interface{}{
		"session_id":      "test_srv",
		"hook_event_name": "PostToolUse",
		"cwd":             "/dev",
		"tool_name":       "Bash",
		"tool_input":      map[string]interface{}{"command": "ls"},
	}
	data, _ := json.Marshal(payload)
	data = append(data, '\n')
	conn.Write(data)
	conn.Close()

	// Give server time to process
	time.Sleep(100 * time.Millisecond)

	// Verify event count
	if srv.EventCount() < 1 {
		t.Errorf("EventCount = %d, want >= 1", srv.EventCount())
	}

	// Shutdown
	srv.Stop()
	wg.Wait()
}

func TestServerShutdownCleansSocket(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()
	cfg := config.DefaultConfig()

	socketPath := filepath.Join(root, "test.sock")
	srv := NewServer(socketPath, paths, cfg)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.Start()
	}()

	time.Sleep(100 * time.Millisecond)
	srv.Stop()
	wg.Wait()

	// Socket file should be removed
	if _, err := net.Dial("unix", socketPath); err == nil {
		t.Error("socket should not be connectable after stop")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/daemon/ -run TestServer -v`
Expected: FAIL — `NewServer` undefined

- [ ] **Step 3: Implement Unix socket server**

Create `internal/daemon/server.go`:

```go
package daemon

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/ingest"
)

// Server is the daemon's Unix socket server that receives and writes events.
type Server struct {
	socketPath string
	paths      config.Paths
	cfg        config.Config
	listener   net.Listener
	writer     *ingest.Writer
	eventCount atomic.Int64
	lastEvent  atomic.Int64 // unix timestamp of last event
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewServer creates a new daemon server.
func NewServer(socketPath string, paths config.Paths, cfg config.Config) *Server {
	return &Server{
		socketPath: socketPath,
		paths:      paths,
		cfg:        cfg,
		writer:     ingest.NewWriter(paths),
		stopCh:     make(chan struct{}),
	}
}

// Start begins listening on the Unix socket. Blocks until Stop is called.
func (s *Server) Start() error {
	// Remove stale socket
	os.Remove(s.socketPath)

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.socketPath, err)
	}
	s.listener = listener

	// Accept connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-s.stopCh:
					return
				default:
					log.Printf("[AJ] accept error: %v", err)
					continue
				}
			}
			s.wg.Add(1)
			go s.handleConn(conn)
		}
	}()

	<-s.stopCh
	return nil
}

// Stop shuts down the server gracefully.
func (s *Server) Stop() {
	close(s.stopCh)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	os.Remove(s.socketPath)
}

// EventCount returns the total number of events received.
func (s *Server) EventCount() int64 {
	return s.eventCount.Load()
}

// LastEventTime returns the time of the last received event.
func (s *Server) LastEventTime() time.Time {
	ts := s.lastEvent.Load()
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0)
}

// EventsSinceCompile returns the number of events since the counter was last reset.
// The dream trigger calls ResetEventCounter after firing.
func (s *Server) EventsSinceCompile() int64 {
	return s.eventCount.Load()
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB max
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}

		event, err := ingest.NormalizeEvent(raw, s.cfg.Ingestion.MaxResponseBytes)
		if err != nil {
			log.Printf("[AJ] normalize error: %v", err)
			continue
		}

		if err := s.writer.Write(event); err != nil {
			log.Printf("[AJ] write error: %v", err)
			continue
		}

		s.eventCount.Add(1)
		s.lastEvent.Store(time.Now().Unix())
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/daemon/ -run TestServer -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/server.go internal/daemon/server_test.go
git commit -m "feat: add Unix socket server for event ingestion"
```

---

### Task 3: Implement Dream Trigger Monitor

**Files:**
- Create: `internal/compile/trigger.go`
- Create: `internal/compile/trigger_test.go`

- [ ] **Step 1: Write failing test for trigger evaluation**

Create `internal/compile/trigger_test.go`:

```go
package dream

import (
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
)

func TestManualTrigger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dream.TriggerMode = "manual"

	trigger := NewTrigger(cfg)

	if trigger.ShouldFire(100, time.Now()) {
		t.Error("manual trigger should never fire automatically")
	}
}

func TestIntervalTrigger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dream.TriggerMode = "interval"
	cfg.Dream.TriggerIntervalMinutes = 1

	trigger := NewTrigger(cfg)

	// Not enough time elapsed
	if trigger.ShouldFire(100, time.Now()) {
		t.Error("should not fire before interval")
	}

	// Enough time elapsed
	trigger.lastDreamTime = time.Now().Add(-2 * time.Minute)
	if !trigger.ShouldFire(100, time.Now()) {
		t.Error("should fire after interval")
	}
}

func TestEventCountTrigger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dream.TriggerMode = "event_count"
	cfg.Dream.TriggerEventThreshold = 50

	trigger := NewTrigger(cfg)

	// Below threshold
	if trigger.ShouldFire(49, time.Now()) {
		t.Error("should not fire below threshold")
	}

	// At threshold
	if !trigger.ShouldFire(50, time.Now()) {
		t.Error("should fire at threshold")
	}
}

func TestMarkFired(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dream.TriggerMode = "interval"
	cfg.Dream.TriggerIntervalMinutes = 1

	trigger := NewTrigger(cfg)
	trigger.lastDreamTime = time.Now().Add(-2 * time.Minute)

	if !trigger.ShouldFire(0, time.Now()) {
		t.Fatal("should fire")
	}

	trigger.MarkFired()

	if trigger.ShouldFire(0, time.Now()) {
		t.Error("should not fire right after marking fired")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/compile/ -run TestManual -v`
Expected: FAIL — package not found

- [ ] **Step 3: Implement trigger logic**

Create `internal/compile/trigger.go`:

```go
package dream

import (
	"sync"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
)

// Trigger evaluates whether the compilation should fire.
type Trigger struct {
	cfg           config.CompileConfig
	lastDreamTime time.Time
	running       bool
	mu            sync.Mutex
}

// NewTrigger creates a Trigger with the given config.
func NewTrigger(cfg config.Config) *Trigger {
	return &Trigger{
		cfg:           cfg.Dream,
		lastDreamTime: time.Now(),
	}
}

// ShouldFire returns true if the compile sequence should be triggered
// based on the current mode, event count since last dream, and current time.
func (tr *Trigger) ShouldFire(eventsSinceDream int64, now time.Time) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.running {
		return false
	}

	switch tr.cfg.TriggerMode {
	case "manual":
		return false
	case "interval":
		elapsed := now.Sub(tr.lastDreamTime)
		return elapsed >= time.Duration(tr.cfg.TriggerIntervalMinutes)*time.Minute
	case "event_count":
		return eventsSinceDream >= int64(tr.cfg.TriggerEventThreshold)
	default:
		return false
	}
}

// MarkFired records that a dream was just triggered.
func (tr *Trigger) MarkFired() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.lastDreamTime = time.Now()
}

// SetRunning marks the dream as currently running (prevents concurrent runs).
func (tr *Trigger) SetRunning(running bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.running = running
}

// IsRunning returns whether a dream is currently running.
func (tr *Trigger) IsRunning() bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.running
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/compile/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/compile/trigger.go internal/compile/trigger_test.go
git commit -m "feat: add dream trigger with manual/interval/event_count modes"
```

---

### Task 4: Implement Idle Timeout Watcher

**Files:**
- Modify: `internal/daemon/server.go`
- Create: `internal/daemon/idle_test.go`

- [ ] **Step 1: Write failing test for idle shutdown**

Create `internal/daemon/idle_test.go`:

```go
package daemon

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
)

func TestIdleTimeoutShutdown(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()
	cfg := config.DefaultConfig()
	cfg.Daemon.IdleTimeoutMinutes = 0 // Use custom duration for testing

	socketPath := filepath.Join(root, "test.sock")
	srv := NewServer(socketPath, paths, cfg)
	srv.SetIdleTimeout(500 * time.Millisecond) // Short timeout for test

	var wg sync.WaitGroup
	wg.Add(1)

	stopped := make(chan struct{})
	go func() {
		defer wg.Done()
		srv.Start()
		close(stopped)
	}()

	// Wait for idle timeout to trigger shutdown
	select {
	case <-stopped:
		// Good — server stopped due to idle
	case <-time.After(3 * time.Second):
		srv.Stop()
		wg.Wait()
		t.Fatal("server did not stop after idle timeout")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/daemon/ -run TestIdleTimeout -v`
Expected: FAIL — `SetIdleTimeout` undefined

- [ ] **Step 3: Add idle timeout to server**

Add to `internal/daemon/server.go` — add a field and method:

Add field to `Server` struct:
```go
	idleTimeout time.Duration
```

Add method:
```go
// SetIdleTimeout overrides the idle timeout duration. For testing.
func (s *Server) SetIdleTimeout(d time.Duration) {
	s.idleTimeout = d
}

func (s *Server) effectiveIdleTimeout() time.Duration {
	if s.idleTimeout > 0 {
		return s.idleTimeout
	}
	if s.cfg.Daemon.IdleTimeoutMinutes > 0 {
		return time.Duration(s.cfg.Daemon.IdleTimeoutMinutes) * time.Minute
	}
	return 30 * time.Minute
}
```

Update `Start()` to launch an idle checker goroutine:

```go
func (s *Server) Start() error {
	os.Remove(s.socketPath)

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.socketPath, err)
	}
	s.listener = listener
	s.lastEvent.Store(time.Now().Unix())

	// Accept connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-s.stopCh:
					return
				default:
					log.Printf("[AJ] accept error: %v", err)
					continue
				}
			}
			s.wg.Add(1)
			go s.handleConn(conn)
		}
	}()

	// Idle timeout checker
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				last := s.LastEventTime()
				if !last.IsZero() && time.Since(last) > s.effectiveIdleTimeout() {
					log.Println("[AJ] Idle timeout reached, shutting down")
					s.Stop()
					return
				}
			}
		}
	}()

	<-s.stopCh
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/daemon/ -run TestIdleTimeout -v -timeout 10s`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/daemon/server.go internal/daemon/idle_test.go
git commit -m "feat: add idle timeout auto-shutdown to daemon"
```

---

### Task 5: Wire Daemon CLI Commands

**Files:**
- Modify: `cmd/agentjit/daemon.go`

- [ ] **Step 1: Implement daemon start/stop/status commands**

Update `cmd/agentjit/daemon.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the AJ daemon",
}

var ifNotRunning bool
var foreground bool

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the AJ daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		daemon.CleanupStalePID(paths.PID)

		if ifNotRunning && daemon.IsRunning(paths.PID) {
			pid, _ := daemon.ReadPID(paths.PID)
			// Output context for SessionStart hook
			ctx := map[string]string{
				"additionalContext": fmt.Sprintf("[AJ] Ingestion active. Daemon PID %d.", pid),
			}
			data, _ := json.Marshal(ctx)
			fmt.Println(string(data))
			return nil
		}

		if daemon.IsRunning(paths.PID) {
			return fmt.Errorf("daemon already running")
		}

		paths.EnsureDirs()

		cfg, err := config.Load(paths.Config)
		if err != nil {
			cfg = config.DefaultConfig()
		}

		if !foreground {
			// Start as background process
			if err := daemon.StartDaemonProcess(); err != nil {
				return err
			}
			fmt.Println("[AJ] Daemon started in background")
			return nil
		}

		// Foreground mode — run the server directly
		if err := daemon.WritePID(paths.PID); err != nil {
			return fmt.Errorf("writing PID: %w", err)
		}
		defer daemon.RemovePID(paths.PID)

		socketPath := paths.Socket
		if cfg.Daemon.SocketPath != "" {
			socketPath = cfg.Daemon.SocketPath
		}

		srv := daemon.NewServer(socketPath, paths, cfg)
		fmt.Printf("[AJ] Daemon started (PID %d)\n", os.Getpid())
		return srv.Start()
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the AJ daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		if !daemon.IsRunning(paths.PID) {
			fmt.Println("[AJ] Daemon is not running")
			return nil
		}

		// Send shutdown signal via socket
		conn, err := net.DialTimeout("unix", paths.Socket, 2*time.Second)
		if err != nil {
			// Can't connect — kill the process
			pid, _ := daemon.ReadPID(paths.PID)
			proc, _ := os.FindProcess(pid)
			if proc != nil {
				proc.Signal(os.Interrupt)
			}
			daemon.RemovePID(paths.PID)
			fmt.Println("[AJ] Daemon stopped (via signal)")
			return nil
		}
		conn.Write([]byte("SHUTDOWN\n"))
		conn.Close()

		fmt.Println("[AJ] Daemon stopped")
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		if !daemon.IsRunning(paths.PID) {
			fmt.Println("[AJ] Daemon is not running")
			return nil
		}

		pid, _ := daemon.ReadPID(paths.PID)
		fmt.Printf("[AJ] Daemon running (PID %d)\n", pid)
		return nil
	},
}

func init() {
	daemonStartCmd.Flags().BoolVar(&ifNotRunning, "if-not-running", false, "Start only if not already running")
	daemonStartCmd.Flags().BoolVar(&foreground, "foreground", false, "Run in foreground")
	daemonCmd.AddCommand(daemonStartCmd, daemonStopCmd, daemonStatusCmd)
	rootCmd.AddCommand(daemonCmd)
}
```

- [ ] **Step 2: Build and verify**

Run:
```bash
go build -o agentjit ./cmd/agentjit/
./aj daemon status
./aj daemon --help
```
Expected: "Daemon is not running", help shows start/stop/status

- [ ] **Step 3: Commit**

```bash
git add cmd/agentjit/daemon.go
git commit -m "feat: wire daemon start/stop/status CLI commands"
```

---

### Task 6: Add Log Retention Cleanup

**Files:**
- Create: `internal/ingest/retention.go`
- Create: `internal/ingest/retention_test.go`

- [ ] **Step 1: Write failing test for retention cleanup**

Create `internal/ingest/retention_test.go`:

```go
package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-jit/agentjit/internal/config"
)

func TestCleanupOldLogs(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	paths.EnsureDirs()

	// Create fake date directories
	dirs := []string{"2026-01-01", "2026-02-01", "2026-03-30", "2026-03-31", "2026-04-01"}
	for _, d := range dirs {
		dir := filepath.Join(paths.Logs, d)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "test.jsonl"), []byte("{}"), 0644)
	}

	// Cleanup with 2-day retention, reference date 2026-04-01
	removed, err := CleanupOldLogs(paths.Logs, 2, "2026-04-01")
	if err != nil {
		t.Fatalf("CleanupOldLogs: %v", err)
	}

	if removed != 3 {
		t.Errorf("removed = %d, want 3", removed)
	}

	// Verify old dirs are gone
	for _, d := range []string{"2026-01-01", "2026-02-01", "2026-03-30"} {
		if _, err := os.Stat(filepath.Join(paths.Logs, d)); !os.IsNotExist(err) {
			t.Errorf("directory %s should have been removed", d)
		}
	}

	// Verify recent dirs are kept
	for _, d := range []string{"2026-03-31", "2026-04-01"} {
		if _, err := os.Stat(filepath.Join(paths.Logs, d)); err != nil {
			t.Errorf("directory %s should have been kept", d)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ingest/ -run TestCleanup -v`
Expected: FAIL — `CleanupOldLogs` undefined

- [ ] **Step 3: Implement retention cleanup**

Create `internal/ingest/retention.go`:

```go
package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CleanupOldLogs removes date directories older than retentionDays from logsDir.
// referenceDate is in YYYY-MM-DD format. Returns count of removed directories.
func CleanupOldLogs(logsDir string, retentionDays int, referenceDate string) (int, error) {
	refTime, err := time.Parse("2006-01-02", referenceDate)
	if err != nil {
		return 0, fmt.Errorf("parsing reference date: %w", err)
	}

	cutoff := refTime.AddDate(0, 0, -retentionDays)

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirDate, err := time.Parse("2006-01-02", entry.Name())
		if err != nil {
			// Skip directories that don't match date format
			continue
		}

		if dirDate.Before(cutoff) {
			dirPath := filepath.Join(logsDir, entry.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				return removed, fmt.Errorf("removing %s: %w", dirPath, err)
			}
			removed++
		}
	}

	return removed, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ingest/ -run TestCleanup -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ingest/retention.go internal/ingest/retention_test.go
git commit -m "feat: add log retention cleanup for date-partitioned logs"
```
