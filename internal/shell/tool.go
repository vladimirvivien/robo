package shell

import (
	"context"
	"fmt"
	"os"
	"strings"

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

	outputMode := ""
	if h.cfg != nil {
		outputMode = strings.ToLower(strings.TrimSpace(h.cfg.Shell.OutputMode))
	}
	isInteractive := ui.IsStdoutTerminal() && (outputMode == "" || outputMode == "markdown" || outputMode == "md")

	// If non-interactive (piped to jq, -o json, -o code, -o plain):
	if !isInteractive {
		if h.cfg != nil && h.cfg.Shell.YoloApproveAll {
			out, exitCode, err := ExecuteInActiveShellWithCapture(ctx, cmdStr, false)
			if err != nil {
				return ShellOutput{Output: out, Error: err.Error(), ExitCode: exitCode}, nil
			}
			return ShellOutput{Output: out, ExitCode: 0}, nil
		}

		isDestructive, _ := IsDestructiveCommand(cmdStr)
		if !isDestructive {
			out, exitCode, err := ExecuteInActiveShellWithCapture(ctx, cmdStr, false)
			if err != nil {
				return ShellOutput{Output: out, Error: err.Error(), ExitCode: exitCode}, nil
			}
			return ShellOutput{Output: out, ExitCode: 0}, nil
		}
		return ShellOutput{Error: "destructive command requires interactive confirmation", ExitCode: 1}, nil
	}

	// Interactive mode: display proposed command card
	fmt.Println()
	title := "🤖 Proposed Shell Command"
	desc := strings.TrimSpace(in.Description)
	if desc != "" && !strings.Contains(desc, "\n") && len(desc) < 80 {
		title = fmt.Sprintf("🤖 Proposed: %s", desc)
	}
	fmt.Println(ui.CommandCard(title, cmdStr))

	// If YOLO approve all is enabled, execute directly without prompting
	if h.cfg != nil && h.cfg.Shell.YoloApproveAll {
		out, exitCode, err := ExecuteInActiveShellWithCapture(ctx, cmdStr, true)
		if err != nil {
			return ShellOutput{Output: out, Error: err.Error(), ExitCode: exitCode}, nil
		}
		return ShellOutput{Output: out, ExitCode: 0}, nil
	}

	// Check for destructive command guardrails
	isDestructive, warning := IsDestructiveCommand(cmdStr)
	if isDestructive {
		confirmed, err := ui.PromptDestructiveConfirm(warning, "yes-delete")
		if err != nil || !confirmed {
			fmt.Println(ui.BadgeWarning("Execution aborted: destructive confirmation not confirmed"))
			return ShellOutput{ExitCode: 1}, nil
		}
	} else if h.cfg != nil && h.cfg.Shell.AutoAccept {
		// Auto-accept safe command
		out, exitCode, err := ExecuteInActiveShellWithCapture(ctx, cmdStr, true)
		if err != nil {
			return ShellOutput{Output: out, Error: err.Error(), ExitCode: exitCode}, nil
		}
		return ShellOutput{Output: out, ExitCode: 0}, nil
	}

	// Interactive review prompt: [Run] [Edit] [Cancel]
	action, editedCmd, err := ui.PromptCommandReview(cmdStr)
	if err != nil || action == ui.ActionCancel {
		fmt.Println("Execution cancelled.")
		return ShellOutput{ExitCode: 0}, nil
	}

	out, exitCode, err := ExecuteInActiveShellWithCapture(ctx, editedCmd, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "command error: %v\n", err)
		return ShellOutput{Output: out, Error: err.Error(), ExitCode: exitCode}, nil
	}

	return ShellOutput{Output: out, ExitCode: 0}, nil
}
