package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/ui"
)

func TestStreamCard_BasicStreaming(t *testing.T) {
	var buf bytes.Buffer
	sc := ui.NewStreamCard(&buf, "litertlm", "gemma-4-E4B-it", true, 60)

	sc.Start()
	sc.WriteToken("Hello ")
	sc.WriteToken("world! This is a real-time streaming test.")
	sc.WriteToken("\n\n```powershell\nGet-Process\n```")
	full := sc.Finish()

	out := buf.String()

	// Validate card framing elements
	if !strings.Contains(out, "╭") {
		t.Error("expected top border '╭'")
	}
	if !strings.Contains(out, "│") {
		t.Error("expected vertical gutter '│'")
	}
	if !strings.Contains(out, "╰") {
		t.Error("expected bottom border '╰'")
	}
	if !strings.Contains(full, "Hello world!") {
		t.Errorf("expected full text to contain 'Hello world!', got %q", full)
	}
}

func TestStreamCard_WordWrapping(t *testing.T) {
	var buf bytes.Buffer
	sc := ui.NewStreamCard(&buf, "googleai", "gemini-2.5-flash", false, 40)

	sc.Start()
	tokens := []string{
		"The ", "quick ", "brown ", "fox ", "jumps ", "over ", "the ", "lazy ", "dog. ",
		"This ", "sentence ", "should ", "wrap ", "across ", "multiple ", "guttered ", "lines.",
	}
	for _, tok := range tokens {
		sc.WriteToken(tok)
	}
	sc.Finish()

	out := buf.String()
	lines := strings.Split(out, "\n")
	if len(lines) < 4 {
		t.Errorf("expected wrapped output to have at least 4 lines, got %d", len(lines))
	}
}
