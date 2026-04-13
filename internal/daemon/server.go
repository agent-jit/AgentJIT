package daemon

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agent-jit/agentjit/internal/compile"
	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/ingest"
	"github.com/agent-jit/agentjit/internal/skills"
	"github.com/agent-jit/agentjit/internal/stats"
	"github.com/agent-jit/agentjit/internal/transport"
	"github.com/agent-jit/agentjit/prompts"
)

// Server is the daemon's Unix socket server that receives and writes events.
type Server struct {
	socketPath  string
	paths       config.Paths
	cfg         config.Config
	listener    net.Listener
	writer            *ingest.Writer
	eventCount        atomic.Int64
	compileEventCount atomic.Int64 // events since last compile (reset after auto-compile)
	lastEvent         atomic.Int64 // unix timestamp of last event
	stopCh            chan struct{}
	trigger           *compile.Trigger
	skillWatcher      *Watcher
	stopOnce     sync.Once
	wg           sync.WaitGroup
	idleTimeout  time.Duration
}

// NewServer creates a new daemon server.
func NewServer(socketPath string, paths config.Paths, cfg config.Config) *Server {
	return &Server{
		socketPath: socketPath,
		paths:      paths,
		cfg:        cfg,
		writer:     ingest.NewWriter(paths),
		stopCh:     make(chan struct{}),
	}
}

// Start begins listening on the Unix socket. Blocks until Stop is called.
func (s *Server) Start() error {
	// Remove stale socket
	transport.Cleanup(s.socketPath)

	listener, err := transport.Listen(s.socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.socketPath, err)
	}
	s.listener = listener
	s.lastEvent.Store(time.Now().Unix())

	// Skill file watcher
	watchDirs := []string{s.paths.Skills}
	watcher, err := NewWatcher(watchDirs, func(path string) {
		skillDir := filepath.Dir(path)
		skillName := filepath.Base(skillDir)

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			log.Printf("[AJ] Compiled skill: '%s'\n", skillName)
			return
		}
		content := string(data)

		savings := "unknown"
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "savings_per_invocation:") {
				savings = strings.TrimSpace(strings.TrimPrefix(trimmed, "savings_per_invocation:"))
				break
			}
		}

		log.Printf("[AJ] Compiled skill: '%s'. Estimated savings: %s tokens/invocation.\n",
			skillName, savings)

		// Symlink new skill into Claude Code skills directory
		claudeSkillsDir, csErr := config.ClaudeSkillsGlobal()
		if csErr == nil {
			if linkErr := skills.LinkSkill(s.paths.Skills, claudeSkillsDir, skillName); linkErr != nil {
				log.Printf("[AJ] Could not link skill %s: %v", skillName, linkErr)
			}
		}
	})
	if err != nil {
		log.Printf("[AJ] Could not start skill watcher: %v", err)
	} else {
		s.skillWatcher = watcher
		go watcher.Start()
	}

	// Accept connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-s.stopCh:
					return
				default:
					log.Printf("[AJ] accept error: %v", err)
					continue
				}
			}
			s.wg.Add(1)
			go s.handleConn(conn)
		}
	}()

	// Idle timeout checker
	go func() {
		checkInterval := s.effectiveIdleTimeout() / 5
		if checkInterval < 100*time.Millisecond {
			checkInterval = 100 * time.Millisecond
		}
		if checkInterval > 10*time.Second {
			checkInterval = 10 * time.Second
		}
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				last := s.LastEventTime()
				if !last.IsZero() && time.Since(last) > s.effectiveIdleTimeout() {
					log.Println("[AJ] Idle timeout reached, shutting down")
					s.Stop()
					return
				}
			}
		}
	}()

	// Auto-compile trigger checker
	if s.cfg.Compile.TriggerMode != "manual" {
		s.trigger = compile.NewTrigger(s.cfg)
		// Seed in-memory counter from disk so events from prior daemon
		// lifetimes are counted toward the trigger threshold.
		if diskCount, _, err := compile.CountEventsSinceMarker(s.paths); err == nil && diskCount > 0 {
			s.compileEventCount.Store(int64(diskCount))
		}
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-s.stopCh:
					return
				case <-ticker.C:
					events := s.compileEventCount.Load()
					if !s.trigger.ShouldFire(events, time.Now()) {
						continue
					}
					s.trigger.SetRunning(true)
					log.Printf("[AJ] Auto-compile triggered (mode: %s, events: %d)", s.cfg.Compile.TriggerMode, events)
					if err := compile.RunCompile(s.paths, s.cfg, prompts.Compiler); err != nil {
						log.Printf("[AJ] Auto-compile failed: %v", err)
					} else {
						log.Println("[AJ] Auto-compile completed successfully")
						s.trigger.MarkFired()
						s.compileEventCount.Add(-events)
					}
					s.trigger.SetRunning(false)
				}
			}
		}()
	}

	<-s.stopCh
	return nil
}

// Stop shuts down the server gracefully.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		if s.listener != nil {
			s.listener.Close()
		}
		if s.skillWatcher != nil {
			s.skillWatcher.Stop()
		}
	})
	s.wg.Wait()
	transport.Cleanup(s.socketPath)
}

// EventCount returns the total number of events received.
func (s *Server) EventCount() int64 {
	return s.eventCount.Load()
}

// LastEventTime returns the time of the last received event.
func (s *Server) LastEventTime() time.Time {
	ts := s.lastEvent.Load()
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0)
}

// SetIdleTimeout overrides the idle timeout duration. For testing.
func (s *Server) SetIdleTimeout(d time.Duration) {
	s.idleTimeout = d
}

func (s *Server) effectiveIdleTimeout() time.Duration {
	if s.idleTimeout > 0 {
		return s.idleTimeout
	}
	if s.cfg.Daemon.IdleTimeoutMinutes > 0 {
		return time.Duration(s.cfg.Daemon.IdleTimeoutMinutes) * time.Minute
	}
	return 30 * time.Minute
}

// EventsSinceCompile returns the number of events since the counter was last reset.
func (s *Server) EventsSinceCompile() int64 {
	return s.compileEventCount.Load()
}

// ResetCompileCounter resets the events-since-compile counter to zero.
func (s *Server) ResetCompileCounter() {
	s.compileEventCount.Store(0)
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB max
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}

		event, err := ingest.NormalizeEvent(raw, s.cfg.Ingestion.MaxResponseBytes)
		if err != nil {
			log.Printf("[AJ] normalize error: %v", err)
			continue
		}

		if err := s.writer.Write(event); err != nil {
			log.Printf("[AJ] write error: %v", err)
			continue
		}

		// Track AJ skill executions for stats
		stats.CheckSkillExecution(event.ToolName, event.EventType, event.SessionID, event.ToolInput, s.paths)

		s.eventCount.Add(1)
		s.compileEventCount.Add(1)
		s.lastEvent.Store(time.Now().Unix())
	}
}
