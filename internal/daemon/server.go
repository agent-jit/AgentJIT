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

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/ingest"
)

// Server is the daemon's Unix socket server that receives and writes events.
type Server struct {
	socketPath  string
	paths       config.Paths
	cfg         config.Config
	listener    net.Listener
	writer      *ingest.Writer
	eventCount  atomic.Int64
	lastEvent   atomic.Int64 // unix timestamp of last event
	stopCh      chan struct{}
	skillWatcher *Watcher
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
	os.Remove(s.socketPath)

	listener, err := net.Listen("unix", s.socketPath)
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
	os.Remove(s.socketPath)
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
	return s.eventCount.Load()
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

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

		s.eventCount.Add(1)
		s.lastEvent.Store(time.Now().Unix())
	}
}
