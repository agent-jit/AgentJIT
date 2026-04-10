package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherDetectsNewSkill(t *testing.T) {
	dir := t.TempDir()
	notifications := make(chan string, 10)

	w, err := NewWatcher([]string{dir}, func(path string) {
		notifications <- path
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Stop()

	go w.Start()

	// Wait for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Create a new skill directory, then wait for the watcher to register
	// the subdirectory before writing the skill file into it.
	skillDir := filepath.Join(dir, "new-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wait for notification
	select {
	case path := <-notifications:
		if !filepath.IsAbs(path) {
			t.Errorf("expected absolute path, got %q", path)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for notification")
	}
}

func TestWatcherStopClean(t *testing.T) {
	dir := t.TempDir()

	w, err := NewWatcher([]string{dir}, func(path string) {})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	go w.Start()
	time.Sleep(100 * time.Millisecond)

	// Should not panic or hang
	w.Stop()
}
