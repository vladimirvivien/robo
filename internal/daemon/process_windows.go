//go:build windows

package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// DetachCmd configures the command to run as a completely detached background process
// without inheriting console handles and redirecting logs to ~/.robo/robod.log.
func DetachCmd(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	home, err := os.UserHomeDir()
	if err == nil {
		logDir := filepath.Join(home, ".robo")
		_ = os.MkdirAll(logDir, 0750)
		logFile, err := os.OpenFile(filepath.Join(logDir, "robod.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err == nil {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}
	}

	const (
		createNoWindow        = 0x08000000
		createNewProcessGroup = 0x00000200
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNoWindow | createNewProcessGroup,
		HideWindow:    true,
	}
}
