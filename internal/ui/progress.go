package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
)

var (
	styleProgressFilled = lipgloss.NewStyle().Foreground(colorCharple)
	styleProgressEmpty  = lipgloss.NewStyle().Foreground(colorIron)
	styleProgressPct    = lipgloss.NewStyle().Bold(true).Foreground(colorSalt)
	styleProgressBytes  = lipgloss.NewStyle().Foreground(colorSmoke)
	styleProgressTitle  = lipgloss.NewStyle().Bold(true).Foreground(colorDolly)
)

// ProgressBar manages an interactive, real-time terminal download progress bar.
type ProgressBar struct {
	title       string
	out         io.Writer
	mu          sync.Mutex
	lastRender  time.Time
	interactive bool
	finished    bool
}

// NewProgressBar creates a new visual progress bar.
func NewProgressBar(title string) *ProgressBar {
	return &ProgressBar{
		title:       title,
		out:         os.Stdout,
		interactive: IsStdoutTerminal(),
	}
}

// Update renders the current progress state.
func (p *ProgressBar) Update(downloaded, total int64, pct float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.finished {
		return
	}

	// Throttle console output to ~20 FPS (every 50ms) to prevent terminal flicker
	if time.Since(p.lastRender) < 50*time.Millisecond && pct < 100.0 && pct > 0.0 {
		return
	}
	p.lastRender = time.Now()

	if !p.interactive {
		return
	}

	barWidth := 25
	termWidth := TerminalWidth()
	if termWidth > 80 {
		barWidth = 35
	}

	var filledLen int
	if pct >= 100.0 {
		filledLen = barWidth
	} else if pct > 0.0 {
		filledLen = min(int(float64(barWidth)*(pct/100.0)), barWidth)
	}

	emptyLen := max(barWidth-filledLen, 0)

	filledStr := styleProgressFilled.Render(strings.Repeat("█", filledLen))
	emptyStr := styleProgressEmpty.Render(strings.Repeat("░", emptyLen))
	bar := fmt.Sprintf("[%s%s]", filledStr, emptyStr)

	pctStr := styleProgressPct.Render(fmt.Sprintf("%5.1f%%", pct))
	bytesStr := styleProgressBytes.Render(fmt.Sprintf("(%s / %s)", FormatBytes(downloaded), FormatBytes(total)))
	titleStr := styleProgressTitle.Render(p.title)

	line := fmt.Sprintf("\r%s %s %s %s", titleStr, bar, pctStr, bytesStr)
	if _, err := fmt.Fprint(p.out, line); err != nil {
		return
	}
}

// Finish marks the download complete and cleans up the terminal line.
func (p *ProgressBar) Finish(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.finished {
		return
	}
	p.finished = true

	if p.interactive {
		if _, err := fmt.Fprint(p.out, "\r\033[K"); err != nil {
			return
		}
		if message != "" {
			if _, err := fmt.Fprintln(p.out, BadgeSuccess(message)); err != nil {
				return
			}
		}
	}
}

// FormatBytes formats byte counts into human-readable strings (e.g. 2.58 GB).
func FormatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), units[exp])
}
