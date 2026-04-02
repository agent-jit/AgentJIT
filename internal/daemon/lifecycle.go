package daemon

import (
	"os"
	"strconv"
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
