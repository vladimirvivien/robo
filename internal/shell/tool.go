package shell

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/ui"
)

// ShellInput represents the structured payload produced by the model.
type ShellInput struct {
	Prompt        string `json:"prompt,omitempty" description:"The user's original natural-language prompt."`
	Command       string `json:"command" description:"The exact shell command or script to execute on the host operating system."`
	Description   string `json:"description,omitempty" description:"A brief explanation of what this command accomplishes."`
	Risk          string `json:"risk,omitempty"`
	Warning       string `json:"warning,omitempty"`
	IsDestructive bool   `json:"is_destructive,omitempty"`
}

// ShellOutput represents the outcome of command execution.
type ShellOutput struct {
	Output    string   `json:"output"`
	ExitCode  int      `json:"exit_code"`
	Error     string   `json:"error,omitempty"`
	Cancelled bool     `json:"cancelled,omitempty"`
	Tier      RiskTier `json:"tier,omitempty"`
	RiskScore float64  `json:"risk_score,omitempty"`
}

// ToolHandler handles interactive review and execution of shell commands proposed by models.
type ToolHandler struct {
	cfg *config.Config
}

// NewToolHandler creates a new interactive shell tool handler.
func NewToolHandler(cfg *config.Config) *ToolHandler {
	return &ToolHandler{cfg: cfg}
}

func recordExecution(prompt, cmd, desc, out string, exitCode int, err error) {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	cwd, _ := os.Getwd()
	rec := ExecutionRecord{
		Prompt:      prompt,
		Command:     cmd,
		Description: desc,
		Output:      out,
		Error:       errStr,
		ExitCode:    exitCode,
		Cwd:         cwd,
		Timestamp:   time.Now(),
	}
	_ = SaveLastExecution(rec)

	hr := NewHistoryReader(DetectShell())
	_ = hr.AppendCommand(cmd)
}

// Handle executes the interactive command review and runs the command if approved.
func (h *ToolHandler) Handle(ctx context.Context, in ShellInput) (ShellOutput, error) {
	cmdStr := in.Command
	if cmdStr == "" {
		return ShellOutput{Error: "empty command"}, nil
	}

	// Reconcile deterministic safety scanner with model semantic risk evaluation
	assessment := EvaluateCombinedRisk(cmdStr, in.Risk, in.IsDestructive)

	// Halt and clear any active background spinner before displaying interactive UI
	ui.StopActiveSpinner()

	outputMode := ""
	if h.cfg != nil {
		outputMode = strings.ToLower(strings.TrimSpace(h.cfg.Robo.OutputMode))
	}
	isInteractive := ui.IsStdoutTerminal() && (outputMode == "" || outputMode == "markdown" || outputMode == "md")

	// If not interactive terminal or non-markdown output format (piped to jq / script execution)
	if !isInteractive {
		if assessment.IsDestructive && (h.cfg == nil || !h.cfg.Robo.YoloApproveAll) {
			return ShellOutput{
				Error:     fmt.Sprintf("destructive command aborted in non-interactive mode: %s", assessment.Warning),
				ExitCode:  1,
				Cancelled: true,
				Tier:      assessment.Tier,
				RiskScore: assessment.Score,
			}, nil
		}
		out, exitCode, err := ExecuteInActiveShellWithCapture(ctx, cmdStr, false)
		recordExecution(in.Prompt, cmdStr, in.Description, out, exitCode, err)
		if err != nil {
			return ShellOutput{Output: out, Error: err.Error(), ExitCode: exitCode, Tier: assessment.Tier, RiskScore: assessment.Score}, nil
		}
		return ShellOutput{Output: out, ExitCode: 0, Tier: assessment.Tier, RiskScore: assessment.Score}, nil
	}

	// Interactive Mode: Display 3-Tier Risk Command Card on os.Stderr
	fmt.Fprintln(os.Stderr)
	title := "🤖 Proposed Shell Command"
	desc := strings.TrimSpace(in.Description)
	if desc != "" && !strings.Contains(desc, "\n") && len(desc) < 80 {
		title = fmt.Sprintf("🤖 Proposed: %s", desc)
	}
	fmt.Fprintln(os.Stderr, ui.RiskCommandCard(title, cmdStr, string(assessment.Tier), assessment.Warning))

	// If YOLO approve all is explicitly enabled, execute directly without prompting
	if h.cfg != nil && h.cfg.Robo.YoloApproveAll {
		out, exitCode, err := ExecuteInActiveShellWithCapture(ctx, cmdStr, true)
		recordExecution(in.Prompt, cmdStr, in.Description, out, exitCode, err)
		if err != nil {
			return ShellOutput{Output: out, Error: err.Error(), ExitCode: exitCode, Tier: assessment.Tier, RiskScore: assessment.Score}, nil
		}
		return ShellOutput{Output: out, ExitCode: 0, Tier: assessment.Tier, RiskScore: assessment.Score}, nil
	}

	// If Unattended YOLO / AutoAccept mode is active:
	if h.cfg != nil && h.cfg.Robo.AutoAccept {
		if assessment.Tier == RiskTierDestructive {
			// CIRCUIT BREAKER / HARD BRAKE: Destructive commands require explicit typed confirmation
			confirmed, err := ui.PromptDestructiveConfirm(assessment.Warning, "--yes-allow-destructive")
			if err != nil || !confirmed {
				fmt.Fprintln(os.Stderr, ui.BadgeWarning("Execution aborted: destructive confirmation not confirmed"))
				return ShellOutput{ExitCode: 1, Cancelled: true, Tier: assessment.Tier, RiskScore: assessment.Score}, nil
			}
		} else {
			// Auto-accept safe Tier 1 (Read-Only) and Tier 2 (Mutating) commands
			out, exitCode, err := ExecuteInActiveShellWithCapture(ctx, cmdStr, true)
			recordExecution(in.Prompt, cmdStr, in.Description, out, exitCode, err)
			if err != nil {
				return ShellOutput{Output: out, Error: err.Error(), ExitCode: exitCode, Tier: assessment.Tier, RiskScore: assessment.Score}, nil
			}
			return ShellOutput{Output: out, ExitCode: 0, Tier: assessment.Tier, RiskScore: assessment.Score}, nil
		}
	} else if assessment.Tier == RiskTierDestructive {
		// Standard Interactive Mode: Destructive Confirmation Gate
		confirmed, err := ui.PromptDestructiveConfirm(assessment.Warning, "--yes-allow-destructive")
		if err != nil || !confirmed {
			fmt.Fprintln(os.Stderr, ui.BadgeWarning("Execution aborted: destructive confirmation not confirmed"))
			return ShellOutput{ExitCode: 1, Cancelled: true, Tier: assessment.Tier, RiskScore: assessment.Score}, nil
		}
	} else {
		// Standard Interactive Mode: [Run] [Edit] [Cancel] Review Prompt
		action, editedCmd, err := ui.PromptCommandReview(cmdStr)
		if err != nil || action == ui.ActionCancel {
			fmt.Fprintln(os.Stderr, "Execution cancelled.")
			return ShellOutput{ExitCode: 0, Cancelled: true, Tier: assessment.Tier, RiskScore: assessment.Score}, nil
		}
		if editedCmd != cmdStr {
			cmdStr = editedCmd
			assessment = EvaluateCombinedRisk(cmdStr, "", false)
		}
	}

	out, exitCode, err := ExecuteInActiveShellWithCapture(ctx, cmdStr, true)
	recordExecution(in.Prompt, cmdStr, in.Description, out, exitCode, err)
	if err != nil {
		fmt.Fprintf(os.Stderr, "command error: %v\n", err)
		return ShellOutput{Output: out, Error: err.Error(), ExitCode: exitCode, Tier: assessment.Tier, RiskScore: assessment.Score}, nil
	}

	return ShellOutput{Output: out, ExitCode: 0, Tier: assessment.Tier, RiskScore: assessment.Score}, nil
}
