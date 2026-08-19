//go:build !windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

// DetachCmd configures the command to run as a completely detached background daemon
// without inheriting standard I/O handles from the parent terminal.
func DetachCmd(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}
