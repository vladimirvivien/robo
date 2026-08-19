//go:build !windows

package ui

import (
	"os"

	"golang.org/x/sys/unix"
)

// RestoreCookedMode restores standard terminal mode on POSIX systems.
func RestoreCookedMode() {
	if termios, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS); err == nil {
		termios.Lflag |= unix.ECHO | unix.ICANON | unix.ISIG
		_ = unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETS, termios)
	}
}
