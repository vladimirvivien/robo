package shell

import (
	"context"
	"fmt"
	"os"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/ui"
)

// ShellInput represents the structured payload produced by the model.
type ShellInput struct {
	Command     string `json:"command" description:"The exact shell command or script to execute on the host operating system."`
	Description string `json:"description,omitempty" description:"A brief explanation of what this command accomplishes."`
}

// ShellOutput represents the outcome of command execution.
type ShellOutput struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// ToolHandler handles interactive review and execution of shell commands proposed by models.
type ToolHandler struct {
	cfg *config.Config
}

// NewToolHandler creates a new interactive shell tool handler.
func NewToolHandler(cfg *config.Config) *ToolHandler {
	return &ToolHandler{cfg: cfg}
}

// Handle executes the interactive command review and runs the command if approved.
func (h *ToolHandler) Handle(ctx context.Context, in ShellInput) (ShellOutput, error) {
	cmdStr := in.Command
	if cmdStr == "" {
		return ShellOutput{Error: "empty command"}, nil
	}

	// Halt and clear any active background spinner before displaying interactive UI
	ui.StopActiveSpinner()

	// Format proposed command card
	fmt.Println()
	title := "Proposed Shell Command"
	if in.Description != "" {
		title = fmt.Sprintf("Proposed: %s", in.Description)
	}
	fmt.Println(ui.CommandCard(title, cmdStr))

	// If YOLO approve all is enabled, execute directly without prompting
	if h.cfg != nil && h.cfg.Shell.YoloApproveAll {
		err := ExecuteInActiveShell(ctx, cmdStr)
		if err != nil {
			return ShellOutput{Error: err.Error(), ExitCode: 1}, nil
		}
		return ShellOutput{Output: "Command executed successfully.", ExitCode: 0}, nil
	}

	// Check for destructive command guardrails
	isDestructive, warning := IsDestructiveCommand(cmdStr)
	if isDestructive {
		confirmed, err := ui.PromptDestructiveConfirm(warning, "yes-delete")
		if err != nil || !confirmed {
			fmt.Println(ui.BadgeWarning("Execution aborted: destructive confirmation not confirmed"))
			return ShellOutput{Output: "Execution cancelled by user.", ExitCode: 1}, nil
		}
	} else if h.cfg != nil && h.cfg.Shell.AutoAccept {
		// Auto-accept safe command
		err := ExecuteInActiveShell(ctx, cmdStr)
		if err != nil {
			return ShellOutput{Error: err.Error(), ExitCode: 1}, nil
		}
		return ShellOutput{Output: "Command executed successfully.", ExitCode: 0}, nil
	}

	// Interactive review prompt: [Run] [Edit] [Cancel]
	action, editedCmd, err := ui.PromptCommandReview(cmdStr)
	if err != nil || action == ui.ActionCancel {
		fmt.Println("Execution cancelled.")
		return ShellOutput{Output: "Execution cancelled by user.", ExitCode: 0}, nil
	}

	if err := ExecuteInActiveShell(ctx, editedCmd); err != nil {
		fmt.Fprintf(os.Stderr, "command error: %v\n", err)
		return ShellOutput{Error: err.Error(), ExitCode: 1}, nil
	}

	return ShellOutput{Output: "Command executed successfully.", ExitCode: 0}, nil
}
