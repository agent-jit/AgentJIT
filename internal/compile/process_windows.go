//go:build windows

package compile

import "os/exec"

// setProcGroup is a no-op on Windows — process groups work differently.
func setProcGroup(cmd *exec.Cmd) {}

// killProcGroup kills the process on Windows.
func killProcGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
