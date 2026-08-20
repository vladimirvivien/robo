package shell

import (
	"bytes"
	"context"
	"io"
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
			case "git", "jj", "docker", "kubectl", "find", "grep", "ls", "cat", "go", "cargo", "npm", "yarn", "pnpm", "python", "tar", "zip", "unzip", "curl", "wget", "ps", "kill", "systemctl":
				return line
			}
		}
	}

	return ""
}

// ExecuteInActiveShell executes a command string directly inside the user's active shell with stdio attached.
func ExecuteInActiveShell(ctx context.Context, cmdStr string) error {
	_, _, err := ExecuteInActiveShellWithCapture(ctx, cmdStr, true)
	return err
}

// ExecuteInActiveShellWithCapture executes a command string directly inside the user's active shell,
// displaying to os.Stdout/os.Stderr if interactive, and returning captured combined output and exit code.
func ExecuteInActiveShellWithCapture(ctx context.Context, cmdStr string, isInteractive bool) (string, int, error) {
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

	var buf bytes.Buffer
	if isInteractive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &buf)
	} else {
		cmd.Stdout = &buf
		cmd.Stderr = &buf
	}

	setSysProcAttr(cmd, isInteractive)

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return strings.TrimRight(buf.String(), "\r\n"), exitCode, err
}
