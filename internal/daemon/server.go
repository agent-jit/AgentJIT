package daemon

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/ingest"
)

// Server is the daemon's Unix socket server that receives and writes events.
type Server struct {
	socketPath string
	paths      config.Paths
	cfg        config.Config
	listener   net.Listener
	writer     *ingest.Writer
	eventCount atomic.Int64
	lastEvent  atomic.Int64 // unix timestamp of last event
	stopCh     chan struct{}
	wg         sync.WaitGroup
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

	// Accept connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-s.stopCh:
					return
				default:
					log.Printf("[AgentJIT] accept error: %v", err)
					continue
				}
			}
			s.wg.Add(1)
			go s.handleConn(conn)
		}
	}()

	<-s.stopCh
	return nil
}

// Stop shuts down the server gracefully.
func (s *Server) Stop() {
	close(s.stopCh)
	if s.listener != nil {
		s.listener.Close()
	}
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

// EventsSinceDream returns the number of events since the counter was last reset.
func (s *Server) EventsSinceDream() int64 {
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
			log.Printf("[AgentJIT] normalize error: %v", err)
			continue
		}

		if err := s.writer.Write(event); err != nil {
			log.Printf("[AgentJIT] write error: %v", err)
			continue
		}

		s.eventCount.Add(1)
		s.lastEvent.Store(time.Now().Unix())
	}
}
