//go:build !windows

package shell

import "os/exec"

func setSysProcAttr(cmd *exec.Cmd, isInteractive bool) {}
