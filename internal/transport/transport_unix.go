//go:build !windows

package transport

import (
	"net"
	"os"
	"time"
)

// Listen creates a Unix domain socket listener at the given address.
func Listen(address string) (net.Listener, error) {
	return net.Listen("unix", address)
}

// Dial connects to a Unix domain socket at the given address.
func Dial(address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", address, timeout)
}

// Cleanup removes a stale Unix domain socket file.
func Cleanup(address string) {
	os.Remove(address)
}
