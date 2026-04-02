//go:build windows

package daemon

import "os"

// GracefulStop terminates the process with the given PID.
// Windows does not support os.Interrupt on arbitrary processes,
// so we fall back to Kill. The primary shutdown path is the
// SHUTDOWN message sent via the IPC socket.
func GracefulStop(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
