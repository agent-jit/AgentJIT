//go:build !windows

package compile

import (
	"os/exec"
	"syscall"
)

// setProcGroup sets the subprocess to run in its own process group
// so it can be killed cleanly on interrupt.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcGroup kills the process group of the given process.
func killProcGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}
