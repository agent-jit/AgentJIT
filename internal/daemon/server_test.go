package daemon

import (
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/transport"
)

func TestServerStartAndAcceptEvent(t *testing.T) {
	root := t.TempDir()
	paths := config.PathsFromRoot(root)
	_ = paths.EnsureDirs()
	cfg := config.DefaultConfig()

	socketPath := filepath.Join(root, "test.sock")
	srv := NewServer(socketPath, paths, cfg)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Start()
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Send an event
	conn, err := transport.Dial(socketPath, 2*time.Second)
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
	_, _ = conn.Write(data)
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
	_ = paths.EnsureDirs()
	cfg := config.DefaultConfig()

	socketPath := filepath.Join(root, "test.sock")
	srv := NewServer(socketPath, paths, cfg)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Start()
	}()

	time.Sleep(100 * time.Millisecond)
	srv.Stop()
	wg.Wait()

	// Socket should not be connectable after stop
	if _, err := transport.Dial(socketPath, 2*time.Second); err == nil {
		t.Error("socket should not be connectable after stop")
	}
}
