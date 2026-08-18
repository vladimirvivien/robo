package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	// Colors
	colorLocalGreen  = lipgloss.Color("#10B981")
	colorCloudBlue   = lipgloss.Color("#3B82F6")
	colorWarningGold = lipgloss.Color("#F59E0B")
	colorErrorRed    = lipgloss.Color("#EF4444")
	colorDimGray     = lipgloss.Color("#6B7280")

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
			BorderForeground(lipgloss.Color("#4B5563")).
			Padding(0, 1).
			MarginTop(0).
			MarginBottom(1)

	styleCommand = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F3F4F6")).
			Background(lipgloss.Color("#111827")).
			Padding(0, 1)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorDimGray)
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
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9CA3AF")).Render(title))
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
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorCloudBlue).Render(title))
	sb.WriteString("\n\n  ")
	sb.WriteString(styleCommand.Render(command))
	return styleCard.Render(sb.String())
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
