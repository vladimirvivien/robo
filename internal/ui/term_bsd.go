//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package ui

import (
	"os"

	"golang.org/x/sys/unix"
)

// RestoreCookedMode restores standard terminal mode on Darwin and BSD systems.
func RestoreCookedMode() {
	if termios, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TIOCGETA); err == nil {
		termios.Lflag |= unix.ECHO | unix.ICANON | unix.ISIG
		_ = unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETA, termios)
	}
}
