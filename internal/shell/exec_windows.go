//go:build windows

package shell

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr always hides console windows when spawning child processes on Windows.
func setSysProcAttr(cmd *exec.Cmd, _ bool) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
