package dream

import (
	"sync"
	"time"

	"github.com/anthropics/agentjit/internal/config"
)

// Trigger evaluates whether the dream compilation should fire.
type Trigger struct {
	cfg           config.DreamConfig
	lastDreamTime time.Time
	running       bool
	mu            sync.Mutex
}

// NewTrigger creates a Trigger with the given config.
func NewTrigger(cfg config.Config) *Trigger {
	return &Trigger{
		cfg:           cfg.Dream,
		lastDreamTime: time.Now(),
	}
}

// ShouldFire returns true if the dream sequence should be triggered
// based on the current mode, event count since last dream, and current time.
func (tr *Trigger) ShouldFire(eventsSinceDream int64, now time.Time) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.running {
		return false
	}

	switch tr.cfg.TriggerMode {
	case "manual":
		return false
	case "interval":
		elapsed := now.Sub(tr.lastDreamTime)
		return elapsed >= time.Duration(tr.cfg.TriggerIntervalMinutes)*time.Minute
	case "event_count":
		return eventsSinceDream >= int64(tr.cfg.TriggerEventThreshold)
	default:
		return false
	}
}

// MarkFired records that a dream was just triggered.
func (tr *Trigger) MarkFired() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.lastDreamTime = time.Now()
}

// SetRunning marks the dream as currently running (prevents concurrent runs).
func (tr *Trigger) SetRunning(running bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.running = running
}

// IsRunning returns whether a dream is currently running.
func (tr *Trigger) IsRunning() bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.running
}
