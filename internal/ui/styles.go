package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	// Modern Vibrant Palette
	colorCyan        = lipgloss.Color("#00F0FF")
	colorSky         = lipgloss.Color("#38BDF8")
	colorMagenta     = lipgloss.Color("#F43F5E")
	colorPurple      = lipgloss.Color("#A855F7")
	colorLocalGreen  = lipgloss.Color("#10B981")
	colorCloudBlue   = lipgloss.Color("#2563EB")
	colorWarningGold = lipgloss.Color("#F59E0B")
	colorErrorRed    = lipgloss.Color("#EF4444")
	colorTextMuted   = lipgloss.Color("#94A3B8")
	colorDarkBg      = lipgloss.Color("#0F172A")

	// Badge Styles
	styleLocalBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorLocalGreen).
			Padding(0, 1)

	styleCloudBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorCloudBlue).
			Padding(0, 1)

	styleWarningBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#000000")).
				Background(colorWarningGold).
				Padding(0, 1)

	styleErrorBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorErrorRed).
			Padding(0, 1)

	// Card Styles
	styleCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCyan).
			Padding(0, 1).
			MarginTop(0).
			MarginBottom(1)

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSky)

	styleCommand = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan).
			Background(colorDarkBg).
			Padding(0, 1)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorTextMuted)
)

// BadgeLocal renders a styled badge for on-device local engine executions.
func BadgeLocal(text string) string {
	if text == "" {
		text = "Local: on-device"
	}
	return styleLocalBadge.Render(text)
}

// BadgeCloud renders a styled badge for cloud model executions.
func BadgeCloud(text string) string {
	if text == "" {
		text = "Cloud: frontier"
	}
	return styleCloudBadge.Render(text)
}

// BadgeWarning renders a warning badge.
func BadgeWarning(text string) string {
	return styleWarningBadge.Render(text)
}

// BadgeError renders an error badge.
func BadgeError(text string) string {
	return styleErrorBadge.Render(text)
}

// Card wraps content in a styled rounded border box.
func Card(title, content, footer string) string {
	var sb strings.Builder
	if title != "" {
		sb.WriteString(styleTitle.Render(title))
		sb.WriteString("\n\n")
	}
	sb.WriteString(content)
	if footer != "" {
		sb.WriteString("\n\n")
		sb.WriteString(styleMuted.Render(footer))
	}
	return styleCard.Render(sb.String())
}

// CommandCard formats a proposed shell command with review framing.
func CommandCard(title, command string) string {
	var sb strings.Builder
	if title == "" {
		title = "Proposed Shell Command"
	}
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorPurple).Render(title))
	sb.WriteString("\n\n  ")
	sb.WriteString(styleCommand.Render(command))
	return styleCard.Render(sb.String())
}

// PromptIndicator renders the stylized interactive REPL prompt string.
func PromptIndicator(sessionName string) string {
	bracketStyle := lipgloss.NewStyle().Bold(true).Foreground(colorSky)
	arrowStyle := lipgloss.NewStyle().Bold(true).Foreground(colorMagenta)
	return fmt.Sprintf("%s %s ", bracketStyle.Render("["+sessionName+"]"), arrowStyle.Render("❯"))
}

// HeaderBanner formats the engine execution provenance header.
func HeaderBanner(provider, model string, usedLocal bool) string {
	if usedLocal {
		badgeText := fmt.Sprintf("Local: %s", model)
		if model == "" {
			badgeText = "Local: LiteRT-LM"
		}
		return BadgeLocal(badgeText)
	}

	badgeText := fmt.Sprintf("Cloud: %s", model)
	if model == "" {
		badgeText = fmt.Sprintf("Cloud: %s", provider)
	}
	return BadgeCloud(badgeText)
}

// ErrorCard formats an error message in a styled warning box.
func ErrorCard(errText string) string {
	return Card(BadgeError("Error"), errText, "")
}
