package ui

import (
	"os"

	"github.com/charmbracelet/x/term"
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

// TerminalWidth returns the detected terminal window width, falling back to 80 columns.
func TerminalWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w <= 0 {
		w, _, err = term.GetSize(os.Stdin.Fd())
		if err != nil || w <= 0 {
			return 80
		}
	}
	return w
}

// CappedWidth returns the optimal responsive width for UI cards (between 40 and 100 columns).
func CappedWidth(termWidth int) int {
	if termWidth <= 0 {
		termWidth = TerminalWidth()
	}
	if termWidth < 40 {
		return 40
	}
	if termWidth > 100 {
		return 100
	}
	return termWidth
}
