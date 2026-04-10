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
