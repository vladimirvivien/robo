package engine

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/shell"
	"github.com/vladimirvivien/robo/internal/ui"
)

// SessionConfig defines execution settings for the agent completion loop.
type SessionConfig struct {
	MaxSteps           int    `json:"max_steps"`     // Maximum steps before halting (default: 5)
	OneShot            bool   `json:"one_shot"`      // Force single-turn execution ($N=1$)
	Yolo               bool   `json:"yolo"`          // Auto-execute non-destructive steps
	DryRun             bool   `json:"dry_run"`       // Simulate execution without host mutation
	OutputFormat       string `json:"output_format"` // "markdown", "json", "code", "plain"
	ForceBackend       string `json:"force_backend"` // "local", "cloud", or "" (auto)
	CustomInstructions string `json:"custom_instructions,omitempty"`
	StdinContent       string `json:"stdin_content,omitempty"`
}

// StepRecord records the trajectory of an individual step within a session.
type StepRecord struct {
	Step        int    `json:"step"`
	Command     string `json:"command,omitempty"`
	Description string `json:"description,omitempty"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	ExitCode    int    `json:"exit_code"`
	Executed    bool   `json:"executed"`
}

// SessionResult contains the complete summary and trajectory of an agent completion session.
type SessionResult struct {
	Goal          string       `json:"goal"`
	Status        string       `json:"status"` // "completed", "cancelled", "max_steps_exceeded", "error"
	TotalSteps    int          `json:"total_steps"`
	Steps         []StepRecord `json:"steps,omitempty"`
	FinalResponse string       `json:"final_response,omitempty"`
	Provider      string       `json:"provider,omitempty"`
	Model         string       `json:"model,omitempty"`
	Local         bool         `json:"local"`
}

// SessionRunner executes the 1..N step autonomous completion loop.
type SessionRunner struct {
	Engine   Engine
	Config   *config.Config
	Settings SessionConfig
}

// NewSessionRunner creates a new SessionRunner instance.
func NewSessionRunner(eng Engine, cfg *config.Config, settings SessionConfig) *SessionRunner {
	if settings.MaxSteps <= 0 {
		settings.MaxSteps = 5
	}
	if settings.OneShot {
		settings.MaxSteps = 1
	}
	return &SessionRunner{
		Engine:   eng,
		Config:   cfg,
		Settings: settings,
	}
}

// Run executes the agent loop until the goal is satisfied, cancelled, or max steps reached.
func (r *SessionRunner) Run(ctx context.Context, goal string) (*SessionResult, error) {
	result := &SessionResult{
		Goal:   goal,
		Status: "completed",
		Local:  r.Config != nil && r.Config.LLM.Local.Enabled,
	}
	if r.Config != nil {
		if r.Config.LLM.Local.Enabled {
			result.Provider = r.Config.LLM.Local.Provider
			result.Model = r.Config.LLM.Local.Model
		} else {
			result.Provider = r.Config.LLM.Cloud.Provider
			result.Model = r.Config.LLM.Cloud.Model
		}
	}

	shellType := shell.DetectShell()
	targetOS := runtime.GOOS
	targetArch := runtime.GOARCH

	var shellCtx *shell.Context
	if r.Config == nil || r.Config.Shell.CaptureHistory {
		maxLines := 10
		if r.Config != nil && r.Config.Shell.MaxHistoryLines > 0 {
			maxLines = r.Config.Shell.MaxHistoryLines
		}
		collector := shell.NewCollector(nil)
		shellCtx, _ = collector.Collect(ctx, maxLines)
	}

	customInstructions := r.Settings.CustomInstructions

	var trajectory []StepRecord

	for step := 1; step <= r.Settings.MaxSteps; step++ {
		// 1. Build prompt for Turn N
		sysPrompt := shell.BuildSystemPrompt(targetOS, targetArch, shellType, customInstructions, shellCtx)
		stepPrompt := r.buildStepPrompt(goal, trajectory)

		req := Request{
			Prompt:       stepPrompt,
			SystemPrompt: sysPrompt,
			ForceBackend: r.Settings.ForceBackend,
		}

		if r.Settings.StdinContent != "" {
			req.ContextFiles = append(req.ContextFiles, FileContext{
				Path:    "stdin",
				Content: r.Settings.StdinContent,
			})
		}

		// 2. Stream generation
		var fullText strings.Builder
		var proposedToolCalls []ToolCall

		stream, err := r.Engine.GenerateStream(ctx, req)
		if err != nil {
			result.Status = "error"
			return result, fmt.Errorf("session step %d generation failed: %w", step, err)
		}

		for chunk := range stream {
			if chunk.Error != nil {
				result.Status = "error"
				return result, fmt.Errorf("session step %d stream error: %w", step, chunk.Error)
			}
			fullText.WriteString(chunk.Text)
			if len(chunk.ToolCalls) > 0 {
				proposedToolCalls = append(proposedToolCalls, chunk.ToolCalls...)
			}
		}

		rawResponse := fullText.String()
		cleaned := ui.CleanResponseText(rawResponse)
		cmdStr := ""
		explanation := ""

		if len(proposedToolCalls) > 0 {
			cmdStr = proposedToolCalls[0].Command
			explanation = proposedToolCalls[0].Description
		} else {
			cmdStr = shell.ExtractProposedCommand(cleaned)
			if cmdStr == "" {
				explanation = cleaned
			}
		}

		// 3. Check for task completion (no command proposed = final text response)
		if strings.TrimSpace(cmdStr) == "" {
			result.FinalResponse = cleaned
			result.Status = "completed"
			result.TotalSteps = len(trajectory)
			result.Steps = trajectory
			return result, nil
		}

		// 4. Handle Step Execution
		stepRec := StepRecord{
			Step:        step,
			Command:     cmdStr,
			Description: explanation,
			Executed:    !r.Settings.DryRun,
		}

		if r.Settings.DryRun {
			stepRec.Executed = false
			trajectory = append(trajectory, stepRec)
			if r.Settings.OneShot {
				break
			}
			continue
		}

		// Interactive or Unattended Execution
		toolHandler := shell.NewToolHandler(r.Config)
		toolOut, err := toolHandler.Handle(ctx, shell.ShellInput{
			Prompt:      goal,
			Command:     cmdStr,
			Description: explanation,
		})
		if err != nil {
			stepRec.Error = err.Error()
			stepRec.ExitCode = 1
			trajectory = append(trajectory, stepRec)
			if r.Settings.OneShot {
				result.Status = "error"
				break
			}
			continue
		}

		if toolOut.Cancelled {
			result.Status = "cancelled"
			stepRec.Error = "user cancelled command execution"
			trajectory = append(trajectory, stepRec)
			break
		}

		stepRec.Output = toolOut.Output
		stepRec.Error = toolOut.Error
		stepRec.ExitCode = toolOut.ExitCode
		trajectory = append(trajectory, stepRec)

		// If OneShot mode, stop after first command execution
		if r.Settings.OneShot {
			break
		}
	}

	result.TotalSteps = len(trajectory)
	result.Steps = trajectory
	if result.Status == "completed" && len(trajectory) >= r.Settings.MaxSteps && result.FinalResponse == "" {
		result.Status = "max_steps_reached"
	}

	return result, nil
}

func (r *SessionRunner) buildStepPrompt(goal string, trajectory []StepRecord) string {
	if len(trajectory) == 0 {
		return goal
	}

	var sb strings.Builder
	sb.WriteString("User Goal: " + goal + "\n\n")
	sb.WriteString("Session Execution History:\n")

	for _, s := range trajectory {
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
			fmt.Fprintf(&sb, "  Output:\n%s\n", truncateText(s.Output, 20))
		}
		if strings.TrimSpace(s.Error) != "" {
			fmt.Fprintf(&sb, "  Error:\n%s\n", truncateText(s.Error, 10))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Evaluate the progress toward the goal:\n")
	sb.WriteString("- If another command is required, invoke \"execute_shell\" with the next specific command.\n")
	sb.WriteString("- If the goal is satisfied, provide the final concise answer/summary directly in markdown without calling \"execute_shell\".\n")

	return sb.String()
}

func truncateText(text string, maxLines int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	half := maxLines / 2
	head := lines[:half]
	tail := lines[len(lines)-half:]
	return fmt.Sprintf("%s\n... [%d lines omitted] ...\n%s", strings.Join(head, "\n"), len(lines)-maxLines, strings.Join(tail, "\n"))
}
