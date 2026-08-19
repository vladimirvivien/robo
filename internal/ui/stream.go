package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// StreamCard formats and streams LLM output in real-time inside a styled Lipgloss Card.
type StreamCard struct {
	w           io.Writer
	provider    string
	model       string
	usedLocal   bool
	width       int
	borderStyle lipgloss.Style
	gutterStyle lipgloss.Style
	started     bool
	lineBuf     strings.Builder
	fullBuf     strings.Builder
	mu          sync.Mutex
	renderWidth int
}

// NewStreamCard creates a new real-time card streamer.
func NewStreamCard(w io.Writer, provider, model string, usedLocal bool, termWidth int) *StreamCard {
	cardWidth := CappedWidth(termWidth)

	borderColor := colorCharple
	if !usedLocal {
		borderColor = colorMalibu
	}

	return &StreamCard{
		w:           w,
		provider:    provider,
		model:       model,
		usedLocal:   usedLocal,
		width:       cardWidth,
		renderWidth: max(cardWidth-8, 30),
		borderStyle: lipgloss.NewStyle().Foreground(borderColor),
		gutterStyle: lipgloss.NewStyle().Foreground(borderColor),
	}
}

// Start prints the top rounded card border and header badge.
func (s *StreamCard) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return
	}
	s.started = true

	// Build clean top border: ╭────────────────╮
	fillLen := max(s.width-2, 2)
	fill := strings.Repeat("─", fillLen)

	topBorder := s.borderStyle.Render("╭" + fill + "╮")
	s.writeStr(topBorder + "\n")
}

// WriteToken processes an incoming text chunk and streams wrapped lines with the card gutter.
func (s *StreamCard) WriteToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		s.Start()
	}

	// Filter internal routing tokens
	if strings.Contains(token, "[ESCALATE_TO_CLOUD]") {
		token = strings.ReplaceAll(token, "[ESCALATE_TO_CLOUD]", "")
	}
	if token == "" {
		return
	}

	s.fullBuf.WriteString(token)

	for _, r := range token {
		if r == '\n' {
			line := s.lineBuf.String()
			s.lineBuf.Reset()
			s.printCardLine(line)
			continue
		}

		s.lineBuf.WriteRune(r)

		// Word wrap check
		if ansi.StringWidth(s.lineBuf.String()) >= s.renderWidth {
			current := s.lineBuf.String()
			lastSpace := strings.LastIndexFunc(current, unicode.IsSpace)
			if lastSpace > 0 && lastSpace < len(current)-1 {
				line := current[:lastSpace]
				remainder := strings.TrimLeftFunc(current[lastSpace:], unicode.IsSpace)
				s.lineBuf.Reset()
				s.lineBuf.WriteString(remainder)
				s.printCardLine(line)
			}
		}
	}
}

// printCardLine prints a single content line formatted with the left card gutter.
func (s *StreamCard) printCardLine(line string) {
	gutter := s.gutterStyle.Render("│")
	if strings.TrimSpace(line) == "" {
		s.writeStr(fmt.Sprintf("%s\n", gutter))
		return
	}
	s.writeStr(fmt.Sprintf("%s  %s\n", gutter, line))
}

// Finish flushes any remaining buffered text and prints the bottom card border.
func (s *StreamCard) Finish() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return ""
	}

	// Flush remaining line buffer
	if s.lineBuf.Len() > 0 {
		line := s.lineBuf.String()
		s.lineBuf.Reset()
		s.printCardLine(line)
	}

	s.writeStr(s.gutterStyle.Render("│") + "\n")

	// Build bottom border: ╰────────────╯
	fillLen := max(s.width-2, 2)
	bottomBorder := s.borderStyle.Render("╰" + strings.Repeat("─", fillLen) + "╯")
	s.writeStr(bottomBorder + "\n")

	return CleanResponseText(s.fullBuf.String())
}

// FullText returns the accumulated response text.
func (s *StreamCard) FullText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return CleanResponseText(s.fullBuf.String())
}

func (s *StreamCard) writeStr(str string) {
	if s.w != nil {
		if _, err := s.w.Write([]byte(str)); err != nil {
			return
		}
	}
}
