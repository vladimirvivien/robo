package shell

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

var (
	codeBlockRegex = regexp.MustCompile("(?s)```(?:bash|sh|zsh|fish|powershell|pwsh|cmd)?\\s*\\n(.*?)\\n```")
)

// StripCodeBlock removes markdown code fences and returns any surrounding text.
func StripCodeBlock(text string) string {
	return codeBlockRegex.ReplaceAllString(text, "")
}

// ExtractProposedCommand extracts a shell command from LLM response text if formatted as code or command block.
func ExtractProposedCommand(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	// 1. Look for markdown code block
	matches := codeBlockRegex.FindStringSubmatch(trimmed)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// 2. Look for single line prefixed with prompt symbol
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 1 {
		line := strings.TrimSpace(lines[0])
		if strings.HasPrefix(line, "$ ") {
			return strings.TrimSpace(line[2:])
		}
		if strings.HasPrefix(line, "> ") {
			return strings.TrimSpace(line[2:])
		}
		// If single line has no markdown and looks like a command (e.g. starts with common cli tool)
		firstWord := strings.Fields(line)
		if len(firstWord) > 0 {
			switch firstWord[0] {
			case "git", "docker", "kubectl", "find", "grep", "ls", "cat", "go", "cargo", "npm", "yarn", "pnpm", "python", "tar", "zip", "unzip", "curl", "wget", "ps", "kill", "systemctl":
				return line
			}
		}
	}

	return ""
}

// ExecuteInActiveShell executes a command string directly inside the user's active shell with stdio attached.
func ExecuteInActiveShell(ctx context.Context, cmdStr string) error {
	shellType := DetectShell()

	var cmd *exec.Cmd
	switch shellType {
	case ShellPowerShell:
		shellBin := "powershell"
		if _, err := exec.LookPath("pwsh"); err == nil {
			shellBin = "pwsh"
		}
		cmd = exec.CommandContext(ctx, shellBin, "-NoProfile", "-Command", cmdStr)
	default:
		shellBin := os.Getenv("SHELL")
		if shellBin == "" {
			if runtime.GOOS == "windows" {
				shellBin = "cmd.exe"
				cmd = exec.CommandContext(ctx, shellBin, "/c", cmdStr)
			} else {
				shellBin = "/bin/sh"
				cmd = exec.CommandContext(ctx, shellBin, "-c", cmdStr)
			}
		} else {
			cmd = exec.CommandContext(ctx, shellBin, "-c", cmdStr)
		}
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
