//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"syscall"
)

// processExists checks whether a process with the given PID exists.
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check existence.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// StartDaemonProcess starts the daemon as a background process by re-executing
// the current binary with "daemon start" arguments.
func StartDaemonProcess() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	attr := &os.ProcAttr{
		Dir:   "/",
		Env:   os.Environ(),
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	}

	proc, err := os.StartProcess(exe, []string{exe, "daemon", "start", "--foreground"}, attr)
	if err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}

	// Detach from child
	_ = proc.Release()
	return nil
}
