package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// TrajectoryStep defines step-level serialization for multi-stage session execution.
type TrajectoryStep struct {
	Step        int     `json:"step"`
	Command     string  `json:"command,omitempty"`
	Description string  `json:"description,omitempty"`
	Output      string  `json:"output,omitempty"`
	Error       string  `json:"error,omitempty"`
	ExitCode    int     `json:"exit_code"`
	Executed    bool    `json:"executed"`
	RiskTier    string  `json:"risk_tier,omitempty"`
	RiskScore   float64 `json:"risk_score,omitempty"`
}

// SessionOutputData contains full multi-step trajectory and metadata for rendering.
type SessionOutputData struct {
	Goal          string           `json:"goal"`
	Status        string           `json:"status"` // "completed", "cancelled", "max_steps_reached", "error"
	TotalSteps    int              `json:"total_steps"`
	Steps         []TrajectoryStep `json:"steps,omitempty"`
	FinalResponse string           `json:"final_response,omitempty"`
	Provider      string           `json:"provider,omitempty"`
	Model         string           `json:"model,omitempty"`
	Local         bool             `json:"local"`
}

// TrajectoryFormatter handles multi-step session serialization across formats.
type TrajectoryFormatter struct {
	Mode        string
	Interactive bool
	Width       int
}

// NewTrajectoryFormatter creates a new TrajectoryFormatter.
func NewTrajectoryFormatter(mode string, interactive bool, width int) *TrajectoryFormatter {
	return &TrajectoryFormatter{
		Mode:        strings.ToLower(strings.TrimSpace(mode)),
		Interactive: interactive,
		Width:       width,
	}
}

// FormatSession renders SessionOutputData to the given writer according to the configured mode.
func (tf *TrajectoryFormatter) FormatSession(w io.Writer, data SessionOutputData) error {
	switch tf.Mode {
	case "json":
		return tf.formatJSON(w, data)
	case "code", "cmd", "command":
		return tf.formatCode(w, data)
	case "plain", "text", "raw":
		return tf.formatPlain(w, data)
	case "markdown", "md", "":
		return tf.formatMarkdown(w, data)
	default:
		return fmt.Errorf("unsupported output format: %q (expected markdown, plain, json, code)", tf.Mode)
	}
}

func (tf *TrajectoryFormatter) formatJSON(w io.Writer, data SessionOutputData) error {
	cleanData := SessionOutputData{
		Goal:          ansi.Strip(strings.TrimSpace(data.Goal)),
		Status:        data.Status,
		TotalSteps:    data.TotalSteps,
		Steps:         make([]TrajectoryStep, len(data.Steps)),
		FinalResponse: ansi.Strip(strings.TrimSpace(data.FinalResponse)),
		Provider:      data.Provider,
		Model:         data.Model,
		Local:         data.Local,
	}

	for i, s := range data.Steps {
		cleanData.Steps[i] = TrajectoryStep{
			Step:        s.Step,
			Command:     ansi.Strip(strings.TrimSpace(s.Command)),
			Description: ansi.Strip(strings.TrimSpace(s.Description)),
			Output:      ansi.Strip(strings.TrimSpace(s.Output)),
			Error:       ansi.Strip(strings.TrimSpace(s.Error)),
			ExitCode:    s.ExitCode,
			Executed:    s.Executed,
			RiskTier:    s.RiskTier,
			RiskScore:   s.RiskScore,
		}
	}

	encoded, err := json.MarshalIndent(cleanData, "", "  ")
	if err != nil {
		return fmt.Errorf("format session json: %w", err)
	}
	_, err = fmt.Fprintln(w, string(encoded))
	return err
}

func (tf *TrajectoryFormatter) formatCode(w io.Writer, data SessionOutputData) error {
	var sb strings.Builder
	isWindows := (runtime.GOOS == "windows")

	if isWindows {
		sb.WriteString("# robo replay script\n")
		if data.Goal != "" {
			fmt.Fprintf(&sb, "# Goal: %s\n", data.Goal)
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("#!/usr/bin/env bash\n")
		sb.WriteString("set -euo pipefail\n")
		if data.Goal != "" {
			fmt.Fprintf(&sb, "# Goal: %s\n", data.Goal)
		}
		sb.WriteString("\n")
	}

	for _, s := range data.Steps {
		cmd := ansi.Strip(strings.TrimSpace(s.Command))
		if cmd == "" {
			continue
		}
		if s.Description != "" {
			fmt.Fprintf(&sb, "# Step %d: %s\n", s.Step, s.Description)
		} else {
			fmt.Fprintf(&sb, "# Step %d\n", s.Step)
		}
		sb.WriteString(cmd + "\n\n")
	}

	res := strings.TrimSpace(sb.String())
	if res == "" && data.FinalResponse != "" {
		res = ansi.Strip(strings.TrimSpace(data.FinalResponse))
	}
	_, err := fmt.Fprintln(w, res)
	return err
}

func (tf *TrajectoryFormatter) formatPlain(w io.Writer, data SessionOutputData) error {
	if data.FinalResponse != "" {
		_, err := fmt.Fprintln(w, ansi.Strip(strings.TrimSpace(data.FinalResponse)))
		return err
	}

	if len(data.Steps) > 0 {
		last := data.Steps[len(data.Steps)-1]
		if last.Output != "" {
			_, err := fmt.Fprintln(w, ansi.Strip(strings.TrimSpace(last.Output)))
			return err
		}
		if last.Command != "" {
			_, err := fmt.Fprintln(w, ansi.Strip(strings.TrimSpace(last.Command)))
			return err
		}
	}

	return nil
}

func (tf *TrajectoryFormatter) formatMarkdown(w io.Writer, data SessionOutputData) error {
	trimmed := strings.TrimSpace(data.FinalResponse)
	if trimmed == "" {
		return nil
	}

	if !tf.Interactive {
		_, err := fmt.Fprintln(w, trimmed)
		return err
	}

	cardWidth := CappedWidth(tf.Width)
	renderWidth := max(cardWidth-6, 30)

	rendered, _ := RenderMarkdown(trimmed, renderWidth)
	renderedTrimmed := strings.TrimSpace(rendered)
	if renderedTrimmed == "" {
		return nil
	}

	card := CardWithWidth("", renderedTrimmed, "", cardWidth)
	_, err := fmt.Fprintln(w, card)
	return err
}
