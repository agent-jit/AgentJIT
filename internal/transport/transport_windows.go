//go:build windows

package transport

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// pipeName derives a deterministic Windows named pipe path from a socket address.
func pipeName(address string) string {
	h := sha256.Sum256([]byte(address))
	return `\\.\pipe\agentjit-` + hex.EncodeToString(h[:6])
}

// Listen creates a Windows named pipe listener.
// The address parameter is used to derive a deterministic pipe name.
func Listen(address string) (net.Listener, error) {
	return winio.ListenPipe(pipeName(address), &winio.PipeConfig{})
}

// Dial connects to a Windows named pipe derived from the given address.
func Dial(address string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(pipeName(address), &timeout)
}

// Cleanup is a no-op on Windows. Named pipes are kernel objects
// that are automatically cleaned up when the owning process exits.
func Cleanup(address string) {}
