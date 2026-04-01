package config

import (
	"testing"
)

func TestGetField(t *testing.T) {
	cfg := DefaultConfig()

	val, err := GetField(cfg, "dream.trigger_mode")
	if err != nil {
		t.Fatalf("GetField: %v", err)
	}
	if val != "manual" {
		t.Errorf("GetField = %q, want manual", val)
	}
}

func TestGetFieldNested(t *testing.T) {
	cfg := DefaultConfig()

	val, err := GetField(cfg, "daemon.idle_timeout_minutes")
	if err != nil {
		t.Fatalf("GetField: %v", err)
	}
	if val != float64(30) {
		t.Errorf("GetField = %v, want 30", val)
	}
}

func TestGetFieldInvalid(t *testing.T) {
	cfg := DefaultConfig()

	_, err := GetField(cfg, "nonexistent.key")
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestSetField(t *testing.T) {
	cfg := DefaultConfig()

	updated, err := SetField(cfg, "dream.trigger_mode", "interval")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}

	val, _ := GetField(updated, "dream.trigger_mode")
	if val != "interval" {
		t.Errorf("after SetField, got %q, want interval", val)
	}
}

func TestSetFieldNumeric(t *testing.T) {
	cfg := DefaultConfig()

	updated, err := SetField(cfg, "dream.trigger_interval_minutes", "15")
	if err != nil {
		t.Fatalf("SetField: %v", err)
	}

	val, _ := GetField(updated, "dream.trigger_interval_minutes")
	if val != float64(15) {
		t.Errorf("after SetField, got %v, want 15", val)
	}
}
