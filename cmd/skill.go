package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"github.com/vladimirvivien/robo/internal/skill"
	"github.com/vladimirvivien/robo/internal/ui"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage and inspect domain skills (SKILL.md)",
	Long: `skill manages domain playbooks and operational instructions discovered across
workspace (.robo/skills/), user (~/.robo/skills/), and built-in scopes.`,
}

var skillListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all discovered skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg := skill.NewRegistry("", "")
		if err := reg.Discover(); err != nil {
			return fmt.Errorf("discover skills: %w", err)
		}

		skills := reg.List()
		if len(skills) == 0 {
			fmt.Println("No skills found.")
			return nil
		}

		if flagOutput == "json" {
			data, err := json.MarshalIndent(skills, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		if !ui.IsStdoutTerminal() || flagOutput == "plain" {
			for _, s := range skills {
				ver := s.Version
				if ver == "" {
					ver = "-"
				}
				fmt.Printf("%-24s %-10s %-9s %s\n", s.Name, s.Scope, ver, s.Description)
			}
			return nil
		}

		// Interactive Card / Table UI with exact column widths
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Width(24)
		scopeProjectStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Width(10)  // Green
		scopeGlobalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Width(10)   // Cyan
		scopeBuiltinStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(10) // Gray
		verStyle := dimStyle.Width(9)

		var sb strings.Builder
		sb.WriteString(headerStyle.Width(24).Render("NAME"))
		sb.WriteString(headerStyle.Width(10).Render("SCOPE"))
		sb.WriteString(headerStyle.Width(9).Render("VERSION"))
		sb.WriteString(headerStyle.Render("DESCRIPTION"))
		sb.WriteString("\n")
		sb.WriteString(dimStyle.Render(strings.Repeat("─", 80)))
		sb.WriteString("\n")

		for _, s := range skills {
			scopeRendered := scopeBuiltinStyle.Render(string(s.Scope))
			switch s.Scope {
			case skill.ScopeProject:
				scopeRendered = scopeProjectStyle.Render(string(s.Scope))
			case skill.ScopeGlobal:
				scopeRendered = scopeGlobalStyle.Render(string(s.Scope))
			}

			ver := s.Version
			if ver == "" {
				ver = "-"
			}

			sb.WriteString(nameStyle.Render(s.Name))
			sb.WriteString(scopeRendered)
			sb.WriteString(verStyle.Render(ver))
			sb.WriteString(s.Description)
			sb.WriteString("\n")
		}

		fmt.Print(sb.String())
		return nil
	},
}

var skillShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Display metadata and instructions for a specific skill",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		skillName := args[0]
		reg := skill.NewRegistry("", "")
		if err := reg.Discover(); err != nil {
			return fmt.Errorf("discover skills: %w", err)
		}

		s, ok := reg.Get(skillName)
		if !ok {
			return fmt.Errorf("skill '%s' not found (use 'robo skill list' to view available skills)", skillName)
		}

		if flagOutput == "json" {
			data, err := json.MarshalIndent(s, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		if !ui.IsStdoutTerminal() || flagOutput == "plain" {
			fmt.Printf("Name: %s\nScope: %s\nVersion: %s\nPath: %s\nDescription: %s\n\n",
				s.Name, s.Scope, s.Version, s.Path, s.Description)
			if len(s.Triggers.Keywords) > 0 {
				fmt.Printf("Keywords: %s\n", strings.Join(s.Triggers.Keywords, ", "))
			}
			if len(s.Triggers.Files) > 0 {
				fmt.Printf("Files: %s\n", strings.Join(s.Triggers.Files, ", "))
			}
			fmt.Printf("\n--- Instructions ---\n\n%s\n", s.Body)
			return nil
		}

		// Rich Card Display
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250"))
		valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))

		fmt.Println(headerStyle.Render(fmt.Sprintf("Skill: %s", s.Name)))
		fmt.Printf("  %s %s\n", labelStyle.Render("Scope:"), valStyle.Render(string(s.Scope)))
		if s.Version != "" {
			fmt.Printf("  %s %s\n", labelStyle.Render("Version:"), valStyle.Render(s.Version))
		}
		if s.Path != "" {
			fmt.Printf("  %s %s\n", labelStyle.Render("Path:"), valStyle.Render(s.Path))
		}
		fmt.Printf("  %s %s\n", labelStyle.Render("Description:"), valStyle.Render(s.Description))

		if len(s.Triggers.Keywords) > 0 {
			fmt.Printf("  %s %s\n", labelStyle.Render("Keyword Triggers:"), valStyle.Render(strings.Join(s.Triggers.Keywords, ", ")))
		}
		if len(s.Triggers.Files) > 0 {
			fmt.Printf("  %s %s\n", labelStyle.Render("File Triggers:"), valStyle.Render(strings.Join(s.Triggers.Files, ", ")))
		}

		fmt.Println("\n" + labelStyle.Render("Operating Instructions:") + "\n")
		rendered, err := ui.RenderMarkdown(s.Body, ui.TerminalWidth())
		if err == nil {
			fmt.Println(rendered)
		} else {
			fmt.Println(s.Body)
		}

		return nil
	},
}

func init() {
	skillCmd.AddCommand(skillListCmd)
	skillCmd.AddCommand(skillShowCmd)
	RootCmd.AddCommand(skillCmd)
}
