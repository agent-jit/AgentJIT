//go:build !windows

package daemon

import "os"

// GracefulStop sends an interrupt signal to the process with the given PID.
func GracefulStop(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(os.Interrupt)
}
