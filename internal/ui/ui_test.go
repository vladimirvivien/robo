package ui_test

import (
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/ui"
)

func TestBadges(t *testing.T) {
	localBadge := ui.BadgeLocal("Local: Gemma 3 1B")
	if !strings.Contains(localBadge, "Local: Gemma 3 1B") {
		t.Errorf("BadgeLocal missing text: %s", localBadge)
	}

	cloudBadge := ui.BadgeCloud("Cloud: Gemini 2.5 Flash")
	if !strings.Contains(cloudBadge, "Cloud: Gemini 2.5 Flash") {
		t.Errorf("BadgeCloud missing text: %s", cloudBadge)
	}

	warnBadge := ui.BadgeWarning("Warning")
	if !strings.Contains(warnBadge, "Warning") {
		t.Errorf("BadgeWarning missing text: %s", warnBadge)
	}

	errBadge := ui.BadgeError("Error")
	if !strings.Contains(errBadge, "Error") {
		t.Errorf("BadgeError missing text: %s", errBadge)
	}
}

func TestCard(t *testing.T) {
	card := ui.Card("Header Title", "Main body content", "Footer metadata")
	if !strings.Contains(card, "Header Title") {
		t.Errorf("Card missing title: %s", card)
	}
	if !strings.Contains(card, "Main body content") {
		t.Errorf("Card missing content: %s", card)
	}
	if !strings.Contains(card, "Footer metadata") {
		t.Errorf("Card missing footer: %s", card)
	}
}

func TestCommandCard(t *testing.T) {
	cmdCard := ui.CommandCard("Shell Task", "find . -type f -name '*.go'")
	if !strings.Contains(cmdCard, "Shell Task") {
		t.Errorf("CommandCard missing title: %s", cmdCard)
	}
	if !strings.Contains(cmdCard, "find . -type f -name '*.go'") {
		t.Errorf("CommandCard missing command: %s", cmdCard)
	}
}

func TestHeaderBanner(t *testing.T) {
	localBanner := ui.HeaderBanner("litertlm", "gemma3-1b-it-int4", true)
	if !strings.Contains(localBanner, "Local:") {
		t.Errorf("HeaderBanner local missing prefix: %s", localBanner)
	}

	cloudBanner := ui.HeaderBanner("googleai", "gemini-2.5-flash", false)
	if !strings.Contains(cloudBanner, "Cloud:") {
		t.Errorf("HeaderBanner cloud missing prefix: %s", cloudBanner)
	}
}

func TestRenderMarkdown(t *testing.T) {
	mdInput := "# Title\n\nThis is a **bold** paragraph with `inline code`."
	rendered, err := ui.RenderMarkdown(mdInput, 80)
	if err != nil {
		t.Fatalf("RenderMarkdown failed: %v", err)
	}

	if !strings.Contains(rendered, "Title") || !strings.Contains(rendered, "bold") {
		t.Errorf("unexpected rendered markdown: %s", rendered)
	}
}

func TestTerminalDetection(t *testing.T) {
	// Should not panic or crash
	_ = ui.IsStdoutTerminal()
	_ = ui.IsStdinTerminal()
	width := ui.TerminalWidth()
	if width <= 0 {
		t.Errorf("invalid terminal width: %d", width)
	}
}

func TestPromptIndicator(t *testing.T) {
	indicator := ui.PromptIndicator("daily-2026-08-18")
	if !strings.Contains(indicator, "daily-2026-08-18") || !strings.Contains(indicator, "❯") {
		t.Errorf("unexpected prompt indicator: %s", indicator)
	}
}

func TestSpinner(t *testing.T) {
	sp := ui.StartSpinner("Processing test...")
	sp.UpdateMessage("Still processing...")
	sp.Stop()
	// Calling stop twice should be safe
	sp.Stop()
}

func TestProgressBar(t *testing.T) {
	pb := ui.NewProgressBar("Downloading model")
	pb.Update(500, 1000, 50.0)
	pb.Finish("Downloaded model")
	// Second finish call should be safe
	pb.Finish("")
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{2580000000, "2.40 GB"},
	}

	for _, tc := range tests {
		got := ui.FormatBytes(tc.bytes)
		if !strings.Contains(got, strings.Split(tc.expected, " ")[1]) {
			t.Errorf("FormatBytes(%d) = %q, expected unit %q", tc.bytes, got, tc.expected)
		}
	}
}

func TestCleanResponseText(t *testing.T) {
	raw := "[ESCALATE_TO_CLOUD]\n\nI cannot architect a full system.\n```powershell\nGet-Date\n```"
	cleaned := ui.CleanResponseText(raw)
	if strings.Contains(cleaned, "[ESCALATE_TO_CLOUD]") {
		t.Errorf("CleanResponseText failed to strip signal: %s", cleaned)
	}
	if !strings.Contains(cleaned, "I cannot architect a full system.") {
		t.Errorf("CleanResponseText corrupted text: %s", cleaned)
	}
}
