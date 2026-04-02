package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsFromRoot(t *testing.T) {
	root := filepath.Join(os.TempDir(), "aj-test")
	p := PathsFromRoot(root)

	if p.Root != root {
		t.Errorf("Root = %q, want %q", p.Root, root)
	}
	if p.Config != filepath.Join(root, "config.json") {
		t.Errorf("Config = %q, want config.json", p.Config)
	}
	if p.PID != filepath.Join(root, "daemon.pid") {
		t.Errorf("PID = %q, want daemon.pid", p.PID)
	}
	if p.Socket != filepath.Join(root, "daemon.sock") {
		t.Errorf("Socket = %q, want daemon.sock", p.Socket)
	}
}

func TestSessionLogFile(t *testing.T) {
	root := filepath.Join(os.TempDir(), "aj-test")
	p := PathsFromRoot(root)
	got := p.SessionLogFile("2026-04-01", "cld_abc123")
	want := filepath.Join(root, "logs", "2026-04-01", "cld_abc123.jsonl")
	if got != want {
		t.Errorf("SessionLogFile = %q, want %q", got, want)
	}
}

func TestEnsureDirs(t *testing.T) {
	root := t.TempDir()
	p := PathsFromRoot(root)

	if err := p.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	for _, dir := range []string{p.Root, p.Logs, p.Skills} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %q not created: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", dir)
		}
	}
}

func TestClaudeSettingsLocal(t *testing.T) {
	root := filepath.Join(os.TempDir(), "project")
	got := ClaudeSettingsLocal(root)
	want := filepath.Join(root, ".claude", "settings.json")
	if got != want {
		t.Errorf("ClaudeSettingsLocal = %q, want %q", got, want)
	}
}
