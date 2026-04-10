package compile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/ingest"
)

// WriteMarker writes a timestamp to the compile marker file.
func WriteMarker(path string, t time.Time) error {
	return os.WriteFile(path, []byte(t.UTC().Format(time.RFC3339)), 0644)
}

// ReadMarker reads the timestamp from the compile marker file.
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
// the last compile marker. Returns events sorted by timestamp, capped at maxLines.
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
				allEvents = allEvents[:maxLines]
				break
			}
		}
	}

	// Sort by timestamp
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.Before(allEvents[j].Timestamp)
	})

	if len(allEvents) > maxLines {
		allEvents = allEvents[:maxLines]
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
