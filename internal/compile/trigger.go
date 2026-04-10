package compile

import (
	"sync"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
)

// Trigger evaluates whether the compilation should fire.
type Trigger struct {
	cfg             config.CompileConfig
	lastCompileTime time.Time
	running         bool
	mu              sync.Mutex
}

// NewTrigger creates a Trigger with the given config.
func NewTrigger(cfg config.Config) *Trigger {
	return &Trigger{
		cfg:             cfg.Compile,
		lastCompileTime: time.Now(),
	}
}

// ShouldFire returns true if the compile sequence should be triggered
// based on the current mode, event count since last compile, and current time.
func (tr *Trigger) ShouldFire(eventsSinceCompile int64, now time.Time) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.running {
		return false
	}

	switch tr.cfg.TriggerMode {
	case "manual":
		return false
	case "interval":
		elapsed := now.Sub(tr.lastCompileTime)
		return elapsed >= time.Duration(tr.cfg.TriggerIntervalMinutes)*time.Minute
	case "event_count":
		return eventsSinceCompile >= int64(tr.cfg.TriggerEventThreshold)
	default:
		return false
	}
}

// MarkFired records that a compile was just triggered.
func (tr *Trigger) MarkFired() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.lastCompileTime = time.Now()
}

// SetRunning marks the compilation as currently running (prevents concurrent runs).
func (tr *Trigger) SetRunning(running bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.running = running
}

// IsRunning returns whether a compilation is currently running.
func (tr *Trigger) IsRunning() bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.running
}
