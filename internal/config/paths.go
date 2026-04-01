package config

import (
	"os"
	"path/filepath"
)

// Paths holds all filesystem paths used by AgentJIT.
type Paths struct {
	Root       string // ~/.agentjit
	Config     string // ~/.agentjit/config.json
	Logs       string // ~/.agentjit/logs
	Skills     string // ~/.agentjit/skills
	PID        string // ~/.agentjit/daemon.pid
	Socket     string // ~/.agentjit/daemon.sock
	DreamLog   string // ~/.agentjit/dream-log.jsonl
	DreamMarker string // ~/.agentjit/last_dream_marker
	BootstrapProcessed string // ~/.agentjit/bootstrap_processed.json
}

// DefaultPaths returns Paths rooted at ~/.agentjit.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	root := filepath.Join(home, ".agentjit")
	return PathsFromRoot(root), nil
}

// PathsFromRoot returns Paths rooted at the given directory.
// Useful for testing with a temp directory.
func PathsFromRoot(root string) Paths {
	return Paths{
		Root:               root,
		Config:             filepath.Join(root, "config.json"),
		Logs:               filepath.Join(root, "logs"),
		Skills:             filepath.Join(root, "skills"),
		PID:                filepath.Join(root, "daemon.pid"),
		Socket:             filepath.Join(root, "daemon.sock"),
		DreamLog:           filepath.Join(root, "dream-log.jsonl"),
		DreamMarker:        filepath.Join(root, "last_dream_marker"),
		BootstrapProcessed: filepath.Join(root, "bootstrap_processed.json"),
	}
}

// EnsureDirs creates the root, logs, and skills directories if they don't exist.
func (p Paths) EnsureDirs() error {
	for _, dir := range []string{p.Root, p.Logs, p.Skills} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// SessionLogDir returns the log directory for a given date string (YYYY-MM-DD).
func (p Paths) SessionLogDir(date string) string {
	return filepath.Join(p.Logs, date)
}

// SessionLogFile returns the JSONL file path for a given date and session ID.
func (p Paths) SessionLogFile(date, sessionID string) string {
	return filepath.Join(p.Logs, date, sessionID+".jsonl")
}

// ClaudeSettingsGlobal returns the path to Claude Code's global settings.
func ClaudeSettingsGlobal() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// ClaudeSettingsLocal returns the path to Claude Code's local project settings.
func ClaudeSettingsLocal(projectDir string) string {
	return filepath.Join(projectDir, ".claude", "settings.json")
}

// ClaudeProjectsDir returns the path to Claude Code's projects directory.
func ClaudeProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}
