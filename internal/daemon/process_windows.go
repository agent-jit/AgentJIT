//go:build windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// processExists checks whether a process with the given PID exists.
func processExists(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == 259 // STILL_ACTIVE
}

// StartDaemonProcess starts the daemon as a background process by re-executing
// the current binary with "daemon start" arguments.
func StartDaemonProcess() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	cmd := exec.Command(exe, "daemon", "start", "--foreground")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008, // DETACHED_PROCESS
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}
	return nil
}
