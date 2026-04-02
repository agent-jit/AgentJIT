package daemon

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/agentjit/internal/config"
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
