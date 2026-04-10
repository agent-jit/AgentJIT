package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/ingest"
)

// ProcessedFiles tracks which transcript files have been bootstrapped.
type ProcessedFiles struct {
	Files map[string]time.Time `json:"files"`
}

// BootstrapOptions configures the bootstrap run.
type BootstrapOptions struct {
	Since   string // YYYY-MM-DD filter
	Project string // Project path filter
	DryRun  bool
}

// BootstrapResult reports what was bootstrapped.
type BootstrapResult struct {
	SessionsProcessed int
	EventsImported    int
}

// RunBootstrap scans Claude Code transcripts and imports tool use events into
// AgentJIT's log format. Tracks processed files to avoid re-importing.
func RunBootstrap(paths config.Paths, cfg config.Config, claudeProjectsDir string, opts BootstrapOptions) (BootstrapResult, error) {
	var result BootstrapResult

	// Load processed tracker
	processed := loadProcessed(paths.BootstrapProcessed)

	// Find all transcript JSONL files
	transcripts, err := findTranscripts(claudeProjectsDir, opts)
	if err != nil {
		return result, fmt.Errorf("finding transcripts: %w", err)
	}

	writer := ingest.NewWriter(paths)

	for _, path := range transcripts {
		// Skip already processed
		if _, ok := processed.Files[path]; ok {
			continue
		}

		events, err := ParseTranscript(path, cfg.Ingestion.MaxResponseBytes)
		if err != nil {
			continue
		}

		if len(events) == 0 {
			continue
		}

		result.SessionsProcessed++
		result.EventsImported += len(events)

		if !opts.DryRun {
			for _, event := range events {
				if err := writer.Write(event); err != nil {
					return result, fmt.Errorf("writing event: %w", err)
				}
			}
			processed.Files[path] = time.Now()
		}
	}

	// Save processed tracker
	if !opts.DryRun && result.SessionsProcessed > 0 {
		if err := saveProcessed(paths.BootstrapProcessed, processed); err != nil {
			return result, fmt.Errorf("saving processed tracker: %w", err)
		}
	}

	return result, nil
}

func findTranscripts(claudeProjectsDir string, opts BootstrapOptions) ([]string, error) {
	var transcripts []string

	projectDirs, err := os.ReadDir(claudeProjectsDir)
	if err != nil {
		return nil, err
	}

	for _, pd := range projectDirs {
		if !pd.IsDir() {
			continue
		}

		// Filter by project if specified
		if opts.Project != "" {
			// Claude encodes paths by replacing / with -
			encoded := strings.ReplaceAll(opts.Project, "/", "-")
			if !strings.Contains(pd.Name(), encoded) {
				continue
			}
		}

		dirPath := filepath.Join(claudeProjectsDir, pd.Name())
		files, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".jsonl" {
				continue
			}

			// Filter by date if specified
			if opts.Since != "" {
				sinceTime, err := time.Parse("2006-01-02", opts.Since)
				if err == nil {
					info, err := f.Info()
					if err == nil && info.ModTime().Before(sinceTime) {
						continue
					}
				}
			}

			transcripts = append(transcripts, filepath.Join(dirPath, f.Name()))
		}
	}

	return transcripts, nil
}

func loadProcessed(path string) ProcessedFiles {
	processed := ProcessedFiles{Files: make(map[string]time.Time)}
	data, err := os.ReadFile(path)
	if err != nil {
		return processed
	}
	if err := json.Unmarshal(data, &processed); err != nil {
		return processed
	}
	if processed.Files == nil {
		processed.Files = make(map[string]time.Time)
	}
	return processed
}

func saveProcessed(path string, processed ProcessedFiles) error {
	data, err := json.MarshalIndent(processed, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
