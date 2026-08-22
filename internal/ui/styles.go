package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	// Charmtone Pantera Palette (Charm-Crush aesthetic)
	colorCharple  = lipgloss.Color("#7D56F4") // Signature Charm Purple (Primary brand, borders)
	colorDolly    = lipgloss.Color("#E865AE") // Charm Magenta / Pink (Secondary, prompt arrow)
	colorJulep    = lipgloss.Color("#00D787") // Vibrant Mint Green (Local SLM, success badges)
	colorMalibu   = lipgloss.Color("#5299E0") // Sky Azure Blue (Cloud LLM, info badges)
	colorMustard  = lipgloss.Color("#FFB300") // Warm Gold (Warnings, attention)
	colorSriracha = lipgloss.Color("#E83B46") // Bright Error Red
	colorSalt     = lipgloss.Color("#FAFAFA") // Pure Crisp White
	colorSash     = lipgloss.Color("#EDEDED") // Bright Text
	colorSmoke    = lipgloss.Color("#A3A3A3") // Medium Muted Text
	colorIron     = lipgloss.Color("#3D3D3D") // Border / Gutter Gray
	colorBBQ      = lipgloss.Color("#1F1F1F") // Code Block Dark Background
	colorPepper   = lipgloss.Color("#171717") // Deep Background

	// Badge Styles
	styleLocalBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSalt).
			Background(colorJulep).
			Padding(0, 1)

	styleCloudBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSalt).
			Background(colorCharple).
			Padding(0, 1)

	styleWarningBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPepper).
				Background(colorMustard).
				Padding(0, 1)

	styleSuccessBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSalt).
				Background(colorJulep).
				Padding(0, 1)

	styleErrorBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSalt).
			Background(colorSriracha).
			Padding(0, 1)

	// Card Styles
	styleCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorCharple).
			Padding(0, 1).
			MarginTop(0).
			MarginBottom(1)

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCharple)

	styleCommand = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSash).
			Background(colorBBQ).
			Padding(0, 1)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorSmoke)
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

// BadgeSuccess renders a success badge.
func BadgeSuccess(text string) string {
	return styleSuccessBadge.Render(text)
}

// BadgeError renders an error badge.
func BadgeError(text string) string {
	return styleErrorBadge.Render(text)
}

// BadgeReadOnly renders a styled badge for safe read-only queries.
func BadgeReadOnly(text string) string {
	if text == "" {
		text = "Tier 1: Read-Only"
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(colorSalt).
		Background(colorMalibu).
		Padding(0, 1).
		Render(text)
}

// BadgeMutating renders a styled badge for state-modifying actions.
func BadgeMutating(text string) string {
	if text == "" {
		text = "Tier 2: Mutating"
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPepper).
		Background(colorMustard).
		Padding(0, 1).
		Render(text)
}

// BadgeDestructive renders a styled badge for high-risk destructive actions.
func BadgeDestructive(text string) string {
	if text == "" {
		text = "Tier 3: Destructive"
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(colorSalt).
		Background(colorSriracha).
		Padding(0, 1).
		Render(text)
}

// Card wraps content in a styled rounded border box with automatic width bounding.
func Card(title, content, footer string) string {
	return CardWithWidth(title, content, footer, CappedWidth(0))
}

// CardWithWidth wraps content in a styled rounded border box with a specified width constraint.
func CardWithWidth(title, content, footer string, width int) string {
	if width <= 0 {
		width = CappedWidth(0)
	}

	contentWidth := max(width-4, 30)

	var sb strings.Builder
	if title != "" {
		if strings.Contains(title, "\x1b[") {
			sb.WriteString(title)
		} else {
			sb.WriteString(styleTitle.Render(title))
		}
		sb.WriteString("\n\n")
	}
	sb.WriteString(content)
	if footer != "" {
		sb.WriteString("\n\n")
		sb.WriteString(styleMuted.Render(footer))
	}

	cardStyle := styleCard.Width(contentWidth)
	return cardStyle.Render(sb.String())
}

// CleanResponseText strips internal routing signals (such as [ESCALATE_TO_CLOUD]) and cleans whitespace.
func CleanResponseText(text string) string {
	cleaned := strings.ReplaceAll(text, "[ESCALATE_TO_CLOUD]", "")
	return strings.TrimSpace(cleaned)
}

// CommandCard formats a proposed shell command with review framing.
func CommandCard(title, command string) string {
	return CommandCardWithWidth(title, command, CappedWidth(0))
}

// CommandCardWithWidth formats a proposed shell command with review framing and explicit width bounding.
func CommandCardWithWidth(title, command string, width int) string {
	return RiskCommandCardWithWidth(title, command, "read-only", "", width)
}

// RiskCommandCard formats a proposed command with border and badges matching its risk tier.
func RiskCommandCard(title, command string, tier string, warning string) string {
	return RiskCommandCardWithWidth(title, command, tier, warning, CappedWidth(0))
}

// RiskCommandCardWithWidth formats a proposed command with explicit width and risk styling.
func RiskCommandCardWithWidth(title, command string, tier string, warning string, width int) string {
	if width <= 0 {
		width = CappedWidth(0)
	}
	contentWidth := max(width-4, 30)

	borderColor := colorCharple
	titleColor := colorDolly
	badge := BadgeReadOnly("")

	switch strings.ToLower(tier) {
	case "destructive", "tier-3":
		borderColor = colorSriracha
		titleColor = colorSriracha
		badge = BadgeDestructive("")
	case "mutating", "tier-2":
		borderColor = colorMustard
		titleColor = colorMustard
		badge = BadgeMutating("")
	}

	var sb strings.Builder
	if title == "" {
		title = "🤖 Proposed Shell Command"
	}

	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(title))
	if badge != "" {
		sb.WriteString("  " + badge)
	}
	if warning != "" {
		sb.WriteString("\n\n  " + lipgloss.NewStyle().Bold(true).Foreground(colorSriracha).Render("⚠️  "+warning))
	}
	sb.WriteString("\n\n  ")
	sb.WriteString(styleCommand.Render(command))

	cardStyle := styleCard.Width(contentWidth).BorderForeground(borderColor)
	return cardStyle.Render(sb.String())
}

// PromptIndicator renders the stylized interactive REPL prompt string.
func PromptIndicator(prefix string) string {
	if strings.TrimSpace(prefix) == "" {
		prefix = "🤖 robo>"
	}
	prefixStyle := lipgloss.NewStyle().Bold(true).Foreground(colorCharple)
	return fmt.Sprintf("%s ", prefixStyle.Render(strings.TrimSpace(prefix)))
}

// HeaderBanner formats the engine execution provenance header.
func HeaderBanner(provider, model string, usedLocal bool) string {
	if usedLocal {
		badgeText := fmt.Sprintf("🤖 Local: %s", model)
		if model == "" {
			badgeText = "🤖 Local: LiteRT-LM"
		}
		return BadgeLocal(badgeText)
	}

	badgeText := fmt.Sprintf("🤖 Cloud: %s", model)
	if model == "" {
		badgeText = fmt.Sprintf("🤖 Cloud: %s", provider)
	}
	return BadgeCloud(badgeText)
}

// ErrorCard formats an error message in a styled warning box.
func ErrorCard(errText string) string {
	return Card(BadgeError("🤖 Error"), errText, "")
}
