package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsFromRoot(t *testing.T) {
	p := PathsFromRoot("/tmp/agentjit-test")

	if p.Root != "/tmp/agentjit-test" {
		t.Errorf("Root = %q, want /tmp/agentjit-test", p.Root)
	}
	if p.Config != "/tmp/agentjit-test/config.json" {
		t.Errorf("Config = %q, want config.json", p.Config)
	}
	if p.PID != "/tmp/agentjit-test/daemon.pid" {
		t.Errorf("PID = %q, want daemon.pid", p.PID)
	}
	if p.Socket != "/tmp/agentjit-test/daemon.sock" {
		t.Errorf("Socket = %q, want daemon.sock", p.Socket)
	}
}

func TestSessionLogFile(t *testing.T) {
	p := PathsFromRoot("/tmp/agentjit-test")
	got := p.SessionLogFile("2026-04-01", "cld_abc123")
	want := "/tmp/agentjit-test/logs/2026-04-01/cld_abc123.jsonl"
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
	got := ClaudeSettingsLocal("/Users/dev/project")
	want := filepath.Join("/Users/dev/project", ".claude", "settings.json")
	if got != want {
		t.Errorf("ClaudeSettingsLocal = %q, want %q", got, want)
	}
}
