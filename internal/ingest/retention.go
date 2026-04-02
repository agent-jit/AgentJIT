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

		if !dirDate.After(cutoff) {
			dirPath := filepath.Join(logsDir, entry.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				return removed, fmt.Errorf("removing %s: %w", dirPath, err)
			}
			removed++
		}
	}

	return removed, nil
}
