package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Daemon.IdleTimeoutMinutes != 30 {
		t.Errorf("IdleTimeoutMinutes = %d, want 30", cfg.Daemon.IdleTimeoutMinutes)
	}
	if cfg.Compile.TriggerMode != "manual" {
		t.Errorf("TriggerMode = %q, want manual", cfg.Compile.TriggerMode)
	}
	if cfg.Ingestion.MaxResponseBytes != 512 {
		t.Errorf("MaxResponseBytes = %d, want 512", cfg.Ingestion.MaxResponseBytes)
	}
	if len(cfg.Scope.GlobalCLITools) != 10 {
		t.Errorf("GlobalCLITools len = %d, want 10", len(cfg.Scope.GlobalCLITools))
	}
}

func TestLoadNonExistent(t *testing.T) {
	cfg, err := Load("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("Load nonexistent: %v", err)
	}
	if cfg.Compile.TriggerMode != "manual" {
		t.Errorf("expected default config, got TriggerMode = %q", cfg.Compile.TriggerMode)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	cfg.Compile.TriggerMode = "interval"
	cfg.Compile.TriggerIntervalMinutes = 15

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Compile.TriggerMode != "interval" {
		t.Errorf("TriggerMode = %q, want interval", loaded.Compile.TriggerMode)
	}
	if loaded.Compile.TriggerIntervalMinutes != 15 {
		t.Errorf("TriggerIntervalMinutes = %d, want 15", loaded.Compile.TriggerIntervalMinutes)
	}
	// Verify defaults are preserved for unmodified fields
	if loaded.Daemon.IdleTimeoutMinutes != 30 {
		t.Errorf("IdleTimeoutMinutes = %d, want 30", loaded.Daemon.IdleTimeoutMinutes)
	}
}

func TestLoadPartialJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write a partial config — only compile section
	partial := []byte(`{"compile": {"trigger_mode": "event_count"}}`)
	if err := os.WriteFile(path, partial, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Compile.TriggerMode != "event_count" {
		t.Errorf("TriggerMode = %q, want event_count", cfg.Compile.TriggerMode)
	}
	// Since Load() starts from DefaultConfig() and json.Unmarshal only
	// overwrites fields present in the JSON, unspecified fields within
	// the compile section retain their defaults, and other sections are
	// completely untouched.
	if cfg.Daemon.IdleTimeoutMinutes != 30 {
		t.Errorf("IdleTimeoutMinutes = %d, want 30 (default preserved)", cfg.Daemon.IdleTimeoutMinutes)
	}
}

func TestResolvePlatform_DefaultUsesRuntime(t *testing.T) {
	cfg := DefaultConfig()
	got := cfg.Compile.ResolvePlatform()
	if got != runtime.GOOS {
		t.Errorf("ResolvePlatform() = %q, want %q", got, runtime.GOOS)
	}
}

func TestResolvePlatform_ExplicitOverride(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Compile.Platform = "windows"
	got := cfg.Compile.ResolvePlatform()
	if got != "windows" {
		t.Errorf("ResolvePlatform() = %q, want %q", got, "windows")
	}

	cfg.Compile.Platform = "linux"
	got = cfg.Compile.ResolvePlatform()
	if got != "linux" {
		t.Errorf("ResolvePlatform() = %q, want %q", got, "linux")
	}
}
