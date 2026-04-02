package dream

import (
	"testing"
	"time"

	"github.com/anthropics/agentjit/internal/config"
)

func TestManualTrigger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dream.TriggerMode = "manual"

	trigger := NewTrigger(cfg)

	if trigger.ShouldFire(100, time.Now()) {
		t.Error("manual trigger should never fire automatically")
	}
}

func TestIntervalTrigger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dream.TriggerMode = "interval"
	cfg.Dream.TriggerIntervalMinutes = 1

	trigger := NewTrigger(cfg)

	// Not enough time elapsed
	if trigger.ShouldFire(100, time.Now()) {
		t.Error("should not fire before interval")
	}

	// Enough time elapsed
	trigger.lastDreamTime = time.Now().Add(-2 * time.Minute)
	if !trigger.ShouldFire(100, time.Now()) {
		t.Error("should fire after interval")
	}
}

func TestEventCountTrigger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dream.TriggerMode = "event_count"
	cfg.Dream.TriggerEventThreshold = 50

	trigger := NewTrigger(cfg)

	// Below threshold
	if trigger.ShouldFire(49, time.Now()) {
		t.Error("should not fire below threshold")
	}

	// At threshold
	if !trigger.ShouldFire(50, time.Now()) {
		t.Error("should fire at threshold")
	}
}

func TestMarkFired(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Dream.TriggerMode = "interval"
	cfg.Dream.TriggerIntervalMinutes = 1

	trigger := NewTrigger(cfg)
	trigger.lastDreamTime = time.Now().Add(-2 * time.Minute)

	if !trigger.ShouldFire(0, time.Now()) {
		t.Fatal("should fire")
	}

	trigger.MarkFired()

	if trigger.ShouldFire(0, time.Now()) {
		t.Error("should not fire right after marking fired")
	}
}
