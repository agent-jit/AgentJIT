package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/agentjit/internal/config"
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
