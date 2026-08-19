package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
)

var (
	spinnerFrames    = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	styleSpinner     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00F0FF"))
	styleSpinnerText = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
)

// Spinner represents an interactive terminal busy indicator.
type Spinner struct {
	message string
	stopCh  chan struct{}
	doneCh  chan struct{}
	mu      sync.Mutex
	stopped bool
	out     io.Writer
}

// StartSpinner creates and starts an animated spinner on stdout if in a terminal.
func StartSpinner(message string) *Spinner {
	s := &Spinner{
		message: message,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
		out:     os.Stdout,
	}

	if !IsStdoutTerminal() {
		close(s.doneCh)
		return s
	}

	go s.run()
	return s
}

func (s *Spinner) run() {
	defer close(s.doneCh)

	// Render initial frame immediately so there is zero delay before the user sees visual feedback
	s.mu.Lock()
	initialMsg := s.message
	s.mu.Unlock()
	if _, err := fmt.Fprintf(s.out, "\r%s %s", styleSpinner.Render(spinnerFrames[0]), styleSpinnerText.Render(initialMsg)); err != nil {
		return
	}

	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	frameIdx := 1
	for {
		select {
		case <-s.stopCh:
			// Clear spinner line
			if _, err := fmt.Fprint(s.out, "\r\033[K"); err != nil {
				return
			}
			return
		case <-ticker.C:
			s.mu.Lock()
			msg := s.message
			s.mu.Unlock()

			frame := spinnerFrames[frameIdx%len(spinnerFrames)]
			frameIdx++

			if _, err := fmt.Fprintf(s.out, "\r%s %s", styleSpinner.Render(frame), styleSpinnerText.Render(msg)); err != nil {
				return
			}
		}
	}
}

// UpdateMessage changes the status text displayed next to the spinner.
func (s *Spinner) UpdateMessage(msg string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = msg
}

// Stop halts the spinner and clears its terminal line.
func (s *Spinner) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()

	close(s.stopCh)
	<-s.doneCh
}
