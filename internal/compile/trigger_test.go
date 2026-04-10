package compile

import (
	"testing"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
)

func TestManualTrigger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Compile.TriggerMode = "manual"

	trigger := NewTrigger(cfg)

	if trigger.ShouldFire(100, time.Now()) {
		t.Error("manual trigger should never fire automatically")
	}
}

func TestIntervalTrigger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Compile.TriggerMode = "interval"
	cfg.Compile.TriggerIntervalMinutes = 1

	trigger := NewTrigger(cfg)

	// Not enough time elapsed
	if trigger.ShouldFire(100, time.Now()) {
		t.Error("should not fire before interval")
	}

	// Enough time elapsed
	trigger.lastCompileTime = time.Now().Add(-2 * time.Minute)
	if !trigger.ShouldFire(100, time.Now()) {
		t.Error("should fire after interval")
	}
}

func TestEventCountTrigger(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Compile.TriggerMode = "event_count"
	cfg.Compile.TriggerEventThreshold = 50

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
	cfg.Compile.TriggerMode = "interval"
	cfg.Compile.TriggerIntervalMinutes = 1

	trigger := NewTrigger(cfg)
	trigger.lastCompileTime = time.Now().Add(-2 * time.Minute)

	if !trigger.ShouldFire(0, time.Now()) {
		t.Fatal("should fire")
	}

	trigger.MarkFired()

	if trigger.ShouldFire(0, time.Now()) {
		t.Error("should not fire right after marking fired")
	}
}
