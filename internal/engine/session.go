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
		Local:  r.Config == nil || r.Config.Robo.InferenceMode != "llm",
	}
	if r.Config != nil {
		if r.Config.Robo.InferenceMode == "llm" {
			result.Provider = r.Config.LLM.Provider
			result.Model = r.Config.LLM.Model
		} else {
			result.Provider = "litertlm"
			result.Model = r.Config.SLM.Model
		}
	}

	shellType := shell.DetectShell()
	targetOS := runtime.GOOS
	targetArch := runtime.GOARCH

	var shellCtx *shell.Context
	if r.Config == nil || r.Config.Robo.CaptureHistory {
		maxLines := 10
		if r.Config != nil && r.Config.Robo.MaxHistoryLines > 0 {
			maxLines = r.Config.Robo.MaxHistoryLines
		}
		collector := shell.NewCollector(nil)
		shellCtx, _ = collector.Collect(ctx, maxLines)
	}

	customInstructions := r.Settings.CustomInstructions

	tm := NewTrajectoryManager()

	defer ui.StopActiveSpinner()

	for step := 1; step <= r.Settings.MaxSteps; step++ {
		// 1. Build prompt for Turn N with sliding-window compressed trajectory
		sysPrompt := shell.BuildSystemPrompt(targetOS, targetArch, shellType, customInstructions, shellCtx)
		stepPrompt := tm.FormatPromptContext(goal)

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

		// 2. Stream generation with animated visual spinner
		isInteractive := (ui.IsStderrTerminal() || ui.IsStdoutTerminal()) &&
			(r.Settings.OutputFormat == "markdown" || r.Settings.OutputFormat == "md" || r.Settings.OutputFormat == "")
		if isInteractive {
			if step == 1 {
				ui.StartSpinner("Working...")
			} else {
				ui.StartSpinner(fmt.Sprintf("Evaluating step %d...", step))
			}
		}

		var fullText strings.Builder
		var proposedToolCalls []ToolCall

		stream, err := r.Engine.GenerateStream(ctx, req)
		if err != nil {
			ui.StopActiveSpinner()
			result.Status = "error"
			return result, fmt.Errorf("session step %d generation failed: %w", step, err)
		}

		for chunk := range stream {
			if chunk.Error != nil {
				ui.StopActiveSpinner()
				result.Status = "error"
				return result, fmt.Errorf("session step %d stream error: %w", step, chunk.Error)
			}
			fullText.WriteString(chunk.Text)
			if len(chunk.ToolCalls) > 0 {
				proposedToolCalls = append(proposedToolCalls, chunk.ToolCalls...)
			}
		}

		ui.StopActiveSpinner()

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
			result.TotalSteps = tm.Count()
			result.Steps = tm.Steps()
			return result, nil
		}

		// 3b. Loop Detection: Check if the exact same command was already executed in the prior step
		if lastStep := tm.LastStep(); lastStep != nil && lastStep.Executed {
			if strings.TrimSpace(strings.ToLower(cmdStr)) == strings.TrimSpace(strings.ToLower(lastStep.Command)) {
				if strings.TrimSpace(lastStep.Output) == "" {
					result.FinalResponse = fmt.Sprintf("The query `%s` was executed and produced no matching output (0 matches found).", lastStep.Command)
				} else {
					result.FinalResponse = fmt.Sprintf("The command `%s` was executed with output:\n\n%s", lastStep.Command, lastStep.Output)
				}
				result.Status = "completed"
				result.TotalSteps = tm.Count()
				result.Steps = tm.Steps()
				return result, nil
			}
		}

		// 4. Handle Step Execution
		var modelRisk string
		var isDestructive bool
		if len(proposedToolCalls) > 0 {
			modelRisk = proposedToolCalls[0].Risk
			isDestructive = proposedToolCalls[0].IsDestructive
		}
		assess := shell.EvaluateCombinedRisk(cmdStr, modelRisk, isDestructive)

		stepRec := StepRecord{
			Step:        step,
			Command:     cmdStr,
			Description: explanation,
			Executed:    !r.Settings.DryRun,
			RiskTier:    string(assess.Tier),
			RiskScore:   assess.Score,
		}

		if r.Settings.DryRun {
			stepRec.Executed = false
			stepRec.Output = "[Dry-Run simulated success]"
			stepRec.ExitCode = 0
			tm.AddStep(stepRec)
			if r.Settings.OneShot {
				break
			}
			continue
		}

		// Interactive or Unattended Execution
		toolHandler := shell.NewToolHandler(r.Config)
		toolOut, err := toolHandler.Handle(ctx, shell.ShellInput{
			Prompt:        goal,
			Command:       cmdStr,
			Description:   explanation,
			Risk:          modelRisk,
			IsDestructive: isDestructive,
		})
		if err != nil {
			stepRec.Error = err.Error()
			stepRec.ExitCode = 1
			tm.AddStep(stepRec)
			if r.Settings.OneShot {
				result.Status = "error"
				break
			}
			continue
		}

		if toolOut.Tier != "" {
			stepRec.RiskTier = string(toolOut.Tier)
			stepRec.RiskScore = toolOut.RiskScore
		}

		if toolOut.Cancelled {
			result.Status = "cancelled"
			stepRec.Error = "user cancelled command execution"
			tm.AddStep(stepRec)
			break
		}

		stepRec.Output = toolOut.Output
		stepRec.Error = toolOut.Error
		stepRec.ExitCode = toolOut.ExitCode
		tm.AddStep(stepRec)

		// If OneShot mode, stop after first command execution
		if r.Settings.OneShot {
			break
		}
	}

	result.TotalSteps = tm.Count()
	result.Steps = tm.Steps()
	if result.Status == "completed" && tm.Count() >= r.Settings.MaxSteps && result.FinalResponse == "" {
		result.Status = "max_steps_reached"
	}

	return result, nil
}
