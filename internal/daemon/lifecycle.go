package daemon

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

// WritePID writes the current process PID to the given path.
func WritePID(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// ReadPID reads a PID from the given file.
func ReadPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

// IsRunning checks if a daemon is running by reading the PID file
// and verifying the process exists.
func IsRunning(pidPath string) bool {
	pid, err := ReadPID(pidPath)
	if err != nil {
		return false
	}
	return processExists(pid)
}

// CleanupStalePID removes the PID file if the referenced process is not running.
func CleanupStalePID(pidPath string) {
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		return
	}
	if !IsRunning(pidPath) {
		os.Remove(pidPath)
	}
}

// RemovePID removes the PID file.
func RemovePID(path string) {
	os.Remove(path)
}

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
	proc.Release()
	return nil
}
