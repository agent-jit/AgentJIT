package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Daemon.IdleTimeoutMinutes != 30 {
		t.Errorf("IdleTimeoutMinutes = %d, want 30", cfg.Daemon.IdleTimeoutMinutes)
	}
	if cfg.Dream.TriggerMode != "manual" {
		t.Errorf("TriggerMode = %q, want manual", cfg.Dream.TriggerMode)
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
	if cfg.Dream.TriggerMode != "manual" {
		t.Errorf("expected default config, got TriggerMode = %q", cfg.Dream.TriggerMode)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	cfg.Dream.TriggerMode = "interval"
	cfg.Dream.TriggerIntervalMinutes = 15

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Dream.TriggerMode != "interval" {
		t.Errorf("TriggerMode = %q, want interval", loaded.Dream.TriggerMode)
	}
	if loaded.Dream.TriggerIntervalMinutes != 15 {
		t.Errorf("TriggerIntervalMinutes = %d, want 15", loaded.Dream.TriggerIntervalMinutes)
	}
	// Verify defaults are preserved for unmodified fields
	if loaded.Daemon.IdleTimeoutMinutes != 30 {
		t.Errorf("IdleTimeoutMinutes = %d, want 30", loaded.Daemon.IdleTimeoutMinutes)
	}
}

func TestLoadPartialJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write a partial config — only dream section
	partial := []byte(`{"dream": {"trigger_mode": "event_count"}}`)
	if err := os.WriteFile(path, partial, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Dream.TriggerMode != "event_count" {
		t.Errorf("TriggerMode = %q, want event_count", cfg.Dream.TriggerMode)
	}
	// Since Load() starts from DefaultConfig() and json.Unmarshal only
	// overwrites fields present in the JSON, unspecified fields within
	// the dream section retain their defaults, and other sections are
	// completely untouched.
	if cfg.Daemon.IdleTimeoutMinutes != 30 {
		t.Errorf("IdleTimeoutMinutes = %d, want 30 (default preserved)", cfg.Daemon.IdleTimeoutMinutes)
	}
}
