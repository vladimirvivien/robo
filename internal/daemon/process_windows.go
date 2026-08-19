//go:build windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

// DetachCmd configures the command to run as a completely detached background process
// without inheriting console handles or stealing stdin from the parent terminal.
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

	const (
		detachedProcess       = 0x00000008
		createNoWindow        = 0x08000000
		createNewProcessGroup = 0x00000200
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: detachedProcess | createNoWindow | createNewProcessGroup,
		HideWindow:    true,
	}
}
