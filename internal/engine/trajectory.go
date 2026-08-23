package engine

import (
	"fmt"
	"strings"
)

// Default truncation limits for token protection
const (
	DefaultHeadLines = 10
	DefaultTailLines = 10
)

// TrajectoryManager tracks and formats multi-turn session step history with sliding-window compression.
type TrajectoryManager struct {
	steps []StepRecord
}

// NewTrajectoryManager creates an empty trajectory manager.
func NewTrajectoryManager() *TrajectoryManager {
	return &TrajectoryManager{
		steps: make([]StepRecord, 0, 8),
	}
}

// AddStep appends a completed step to the trajectory.
func (tm *TrajectoryManager) AddStep(step StepRecord) {
	tm.steps = append(tm.steps, step)
}

// Steps returns all recorded steps.
func (tm *TrajectoryManager) Steps() []StepRecord {
	return tm.steps
}

// LastStep returns the most recent step, or nil if empty.
func (tm *TrajectoryManager) LastStep() *StepRecord {
	if len(tm.steps) == 0 {
		return nil
	}
	return &tm.steps[len(tm.steps)-1]
}

// Count returns the number of recorded steps.
func (tm *TrajectoryManager) Count() int {
	return len(tm.steps)
}

// FormatPromptContext produces a token-efficient prompt representation of the trajectory.
// It keeps the immediate prior step in full truncated detail and folds older steps into 1-line receipts.
func (tm *TrajectoryManager) FormatPromptContext(goal string) string {
	if len(tm.steps) == 0 {
		return goal
	}

	var sb strings.Builder
	sb.WriteString("User Goal: " + goal + "\n\n")
	sb.WriteString("Session Execution History:\n")

	totalSteps := len(tm.steps)

	for i, s := range tm.steps {
		isImmediatePrior := (i == totalSteps-1)

		if !isImmediatePrior {
			// Fold older steps into a compact 1-line execution receipt
			statusStr := "Succeeded (Exit 0)"
			if s.ExitCode != 0 {
				statusStr = fmt.Sprintf("Failed (Exit %d)", s.ExitCode)
			}
			summaryOut := summarizeSingleLine(s.Output)
			if summaryOut != "" {
				fmt.Fprintf(&sb, "• Step %d: `%s` ──> %s [Output: %s]\n", s.Step, s.Command, statusStr, summaryOut)
			} else if s.ExitCode == 0 {
				fmt.Fprintf(&sb, "• Step %d: `%s` ──> %s [Output: (empty - 0 matches)]\n", s.Step, s.Command, statusStr)
			} else {
				fmt.Fprintf(&sb, "• Step %d: `%s` ──> %s\n", s.Step, s.Command, statusStr)
			}
		} else {
			// Immediate previous step retains full truncated detail
			if i > 0 {
				sb.WriteString("\n[Most Recent Step Result]\n")
			}
			fmt.Fprintf(&sb, "Step %d:\n", s.Step)
			fmt.Fprintf(&sb, "  Command: %s\n", s.Command)
			if s.Description != "" {
				fmt.Fprintf(&sb, "  Intent: %s\n", s.Description)
			}
			if s.ExitCode == 0 {
				fmt.Fprintf(&sb, "  Status: Succeeded (Exit Code 0)\n")
			} else {
				fmt.Fprintf(&sb, "  Status: Failed (Exit Code %d)\n", s.ExitCode)
			}
			if strings.TrimSpace(s.Output) != "" {
				fmt.Fprintf(&sb, "  Output:\n%s\n", TruncateOutput(s.Output, DefaultHeadLines, DefaultTailLines))
			} else if s.ExitCode == 0 {
				sb.WriteString("  Output: (empty output - 0 matching records or lines produced)\n")
			}
			if strings.TrimSpace(s.Error) != "" {
				fmt.Fprintf(&sb, "  Error:\n%s\n", TruncateOutput(s.Error, 5, 5))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("Evaluate progress toward the goal:\n")
	sb.WriteString("- If the previous query returned empty output (0 matching lines/services/processes), conclude that no matches exist.\n")
	sb.WriteString("- Do NOT re-execute the same query command.\n")
	sb.WriteString("- If the goal is satisfied or a conclusion is reached, provide the final answer directly in markdown without calling \"execute_shell\".\n")

	return sb.String()
}

// TruncateOutput compresses verbose terminal output by preserving the first headLines and last tailLines.
func TruncateOutput(text string, headLines, tailLines int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	lines := strings.Split(trimmed, "\n")
	maxLines := headLines + tailLines
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}

	head := lines[:headLines]
	tail := lines[len(lines)-tailLines:]
	omitted := len(lines) - maxLines

	return fmt.Sprintf("%s\n... [%d lines omitted] ...\n%s",
		strings.Join(head, "\n"),
		omitted,
		strings.Join(tail, "\n"),
	)
}

func summarizeSingleLine(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	first := strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		if len(first) > 40 {
			first = first[:40] + "..."
		}
		return fmt.Sprintf("%s (%d lines)", first, len(lines))
	}
	if len(first) > 60 {
		return first[:60] + "..."
	}
	return first
}
