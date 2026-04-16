package config

import (
	"os"
	"path/filepath"
)

// Paths holds all filesystem paths used by AJ.
type Paths struct {
	Root       string // ~/.aj
	Config     string // ~/.aj/config.json
	Logs       string // ~/.aj/logs
	Skills     string // ~/.aj/skills
	PID        string // ~/.aj/daemon.pid
	Socket     string // ~/.aj/daemon.sock
	CompileLog   string // ~/.aj/compile-log.jsonl
	CompileMarker string // ~/.aj/last_compile_marker
	BootstrapProcessed string // ~/.aj/bootstrap_processed.json
	Stats          string // ~/.aj/stats.jsonl
	IRCatalog      string // path to IR catalog YAML
}

// DefaultPaths returns Paths rooted at ~/.aj.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	root := filepath.Join(home, ".aj")
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
		CompileLog:         filepath.Join(root, "compile-log.jsonl"),
		CompileMarker:      filepath.Join(root, "last_compile_marker"),
		BootstrapProcessed: filepath.Join(root, "bootstrap_processed.json"),
		Stats:              filepath.Join(root, "stats.jsonl"),
		IRCatalog:          filepath.Join(root, "ir_catalog.yaml"),
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

// ClaudeSkillsGlobal returns the path to Claude Code's global skills directory.
func ClaudeSkillsGlobal() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "skills"), nil
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
