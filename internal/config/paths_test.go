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

func TestDefaultPathsHonorsAJHome(t *testing.T) {
	root := filepath.Join(os.TempDir(), "aj-sandbox")
	t.Setenv("AJ_HOME", root)

	p, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths: %v", err)
	}
	if p.Root != root {
		t.Errorf("Root = %q, want %q", p.Root, root)
	}
	if p.Config != filepath.Join(root, "config.json") {
		t.Errorf("Config = %q, want %q", p.Config, filepath.Join(root, "config.json"))
	}
}

func TestDefaultPathsFallsBackToHome(t *testing.T) {
	t.Setenv("AJ_HOME", "")

	p, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	want := filepath.Join(home, ".aj")
	if p.Root != want {
		t.Errorf("Root = %q, want %q", p.Root, want)
	}
}

// Two different AJ_HOME roots must produce fully disjoint paths so a benchmark
// sandbox never collides with real data.
func TestDefaultPathsIsolatesRoots(t *testing.T) {
	rootA := filepath.Join(os.TempDir(), "aj-a")
	t.Setenv("AJ_HOME", rootA)
	a, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths A: %v", err)
	}

	rootB := filepath.Join(os.TempDir(), "aj-b")
	t.Setenv("AJ_HOME", rootB)
	b, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths B: %v", err)
	}

	if a.Root == b.Root {
		t.Fatalf("roots collide: both %q", a.Root)
	}
	if a.Logs == b.Logs || a.Skills == b.Skills || a.Stats == b.Stats {
		t.Errorf("sub-paths collide between roots %q and %q", a.Root, b.Root)
	}
}
