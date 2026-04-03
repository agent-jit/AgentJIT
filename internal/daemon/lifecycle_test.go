package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestWriteAndReadPID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")

	if err := WritePID(pidPath); err != nil {
		t.Fatalf("WritePID: %v", err)
	}

	pid, err := ReadPID(pidPath)
	if err != nil {
		t.Fatalf("ReadPID: %v", err)
	}

	if pid != os.Getpid() {
		t.Errorf("PID = %d, want %d", pid, os.Getpid())
	}
}

func TestReadPIDNonExistent(t *testing.T) {
	_, err := ReadPID("/nonexistent/daemon.pid")
	if err == nil {
		t.Error("expected error for nonexistent PID file")
	}
}

func TestIsRunning(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")

	// No PID file — not running
	if IsRunning(pidPath) {
		t.Error("should not be running without PID file")
	}

	// Write current PID — should be running
	if err := WritePID(pidPath); err != nil {
		t.Fatalf("WritePID: %v", err)
	}
	if !IsRunning(pidPath) {
		t.Error("should be running with valid PID")
	}

	// Write bogus PID — should not be running
	os.WriteFile(pidPath, []byte("999999999"), 0644)
	if IsRunning(pidPath) {
		t.Error("should not be running with invalid PID")
	}
}

func TestCleanupStalePID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")

	// Write a stale PID
	os.WriteFile(pidPath, []byte("999999999"), 0644)

	CleanupStalePID(pidPath)

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("stale PID file should have been removed")
	}
}

func TestRemovePID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")

	if err := WritePID(pidPath); err != nil {
		t.Fatalf("WritePID: %v", err)
	}
	RemovePID(pidPath)

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file should have been removed")
	}
}

func TestWritePIDContent(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "daemon.pid")

	if err := WritePID(pidPath); err != nil {
		t.Fatalf("WritePID: %v", err)
	}

	data, _ := os.ReadFile(pidPath)
	pidStr := string(data)
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		t.Fatalf("PID file content is not a number: %q", pidStr)
	}
	if pid != os.Getpid() {
		t.Errorf("PID = %d, want %d", pid, os.Getpid())
	}
}
