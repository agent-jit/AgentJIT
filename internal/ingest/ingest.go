package ingest

import (
	"fmt"
	"io"
	"time"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/transport"
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

// forwardToDaemon sends the raw payload to the daemon via IPC.
func forwardToDaemon(socketPath string, data []byte) error {
	conn, err := transport.Dial(socketPath, 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write(data)
	return err
}
