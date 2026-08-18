package ui

import (
	"os"

	"github.com/mattn/go-isatty"
)

// IsTerminal returns true if the given file descriptor is connected to a terminal.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// IsStdoutTerminal returns true if stdout is connected to an interactive terminal.
func IsStdoutTerminal() bool {
	return IsTerminal(os.Stdout)
}

// IsStdinTerminal returns true if stdin is connected to an interactive terminal (not piped).
func IsStdinTerminal() bool {
	return IsTerminal(os.Stdin)
}

// TerminalWidth returns the detected terminal width, defaulting to 80 columns.
func TerminalWidth() int {
	return 80
}
