package shell

import (
	"fmt"
	"strings"
)

// BuildSystemPromptWithSkills dynamically assembles a focused, token-efficient system prompt
// tailored specifically to the target OS, architecture, shell dialect, and available skills index.
func BuildSystemPromptWithSkills(targetOS, targetArch string, shellType Type, customInstructions string, sc *Context, skillsIndex string) string {
	var sb strings.Builder

	// 1. Universal Core Protocol (~50 tokens)
	sb.WriteString("You are Robo, an on-device AI system assistant designed to interact directly with the operating system.\n\n")
	sb.WriteString("Tool Calling Rules:\n")
	sb.WriteString("- When an action, command, or query needs to run in the terminal, invoke the \"execute_shell\" tool.\n")
	sb.WriteString("- In \"execute_shell\", provide a concise 1-sentence description and the exact command string.\n")
	sb.WriteString("- Do NOT output duplicate conversational explanations, preambles, or markdown command fences when calling \"execute_shell\".\n")
	sb.WriteString("- Goal Completion: When the goal is accomplished or a conclusion is reached from prior step outputs, do NOT call \"execute_shell\". Output your final conclusion or explanation directly in markdown.\n")
	sb.WriteString("- Commands must be complete, runnable, and contain NO placeholder tokens (e.g. '<file>').\n\n")
	sb.WriteString("Subshell Isolation:\n")
	sb.WriteString("- \"execute_shell\" runs in a child subshell; session state (cd, export, $env:, source, ssh-agent) does NOT persist to the user's active terminal.\n")
	sb.WriteString("- When asked why a session/directory change didn't take effect, or when a task requires terminal persistence, explain the subshell boundary and provide the command for direct execution.\n\n")

	// 2. Dynamic Platform & Shell Module (Invariants only, ~25-35 tokens)
	switch shellType {
	case ShellPowerShell:
		sb.WriteString("Target Environment: Windows PowerShell\n")
		sb.WriteString("Syntax & Execution:\n")
		sb.WriteString("- Generate non-interactive batch commands using standard PowerShell cmdlets and object pipelines '|'.\n")
		sb.WriteString("- Pipeline & Subexpressions: Pass outputs between cmdlets using pipelines '|' or subexpressions '(Get-Item <name>).Property'. Never chain with ';' and shell variables ($LASTEXITCODE, $?).\n")
		sb.WriteString("- Multi-Step Tasks: Connect commands with pipelines '|' or execute sequentially. For time delays use 'Start-Sleep -Seconds <N>'.\n")
		sb.WriteString("- Idempotence: Use idempotent patterns (e.g. 'if (Test-Path dir) { ... }' or 'New-Item -Force').\n\n")

	case ShellFish:
		sb.WriteString("Target Environment: POSIX Bash (executed via Bash compatibility layer for Fish terminal)\n")
		sb.WriteString("Syntax & Execution:\n")
		sb.WriteString("- Generate non-interactive batch commands; use standard POSIX utilities with quoted variables (\"$VAR\").\n")
		sb.WriteString("- Multi-Step Tasks: Chain sequential steps with '&&' for fail-fast safety. For time delays use 'sleep <N>'.\n")
		sb.WriteString("- Idempotence: Use idempotent patterns ('mkdir -p', 'rm -f', '[ -f ... ]').\n\n")

	default:
		switch targetOS {
		case "darwin":
			sb.WriteString("Target Environment: macOS POSIX (BSD)\n")
			sb.WriteString("Syntax & Execution:\n")
			sb.WriteString("- Generate non-interactive batch commands; avoid interactive tools (top, htop, less, nano, vi).\n")
			sb.WriteString("- Quote all variables (\"$VAR\") and glob patterns.\n")
			sb.WriteString("- Multi-Step Tasks: Chain sequential steps with '&&' for fail-fast safety. For time delays use 'sleep <N>'.\n")
			sb.WriteString("- Idempotence: Use idempotent patterns ('mkdir -p', pre-existence checks).\n\n")
		case "windows":
			sb.WriteString("Target Environment: Windows Command Prompt (CMD)\n")
			sb.WriteString("Syntax & Execution:\n")
			sb.WriteString("- Use standard Windows batch/cmd utilities (dir, findstr, tasklist, type).\n")
			sb.WriteString("- Multi-Step Tasks: Chain sequential operations with '&&'.\n\n")
		default:
			sb.WriteString("Target Environment: Linux POSIX (GNU)\n")
			sb.WriteString("Syntax & Execution:\n")
			sb.WriteString("- Generate non-interactive batch commands; avoid interactive tools (top, htop, less, nano, vi).\n")
			sb.WriteString("- Use standard GNU utilities with quoted variables (\"$VAR\") and globs.\n")
			sb.WriteString("- Multi-Step Tasks: Chain sequential steps with '&&' for fail-fast safety. For time delays use 'sleep <N>'.\n")
			sb.WriteString("- Idempotence: Use idempotent patterns ('mkdir -p', 'rm -f', '[ -f ... ]').\n\n")
		}
	}

	// 3. User Instructions (if provided)
	if strings.TrimSpace(customInstructions) != "" {
		fmt.Fprintf(&sb, "User Instructions:\n%s\n\n", strings.TrimSpace(customInstructions))
	}

	// 4. Skills Index (if available)
	if strings.TrimSpace(skillsIndex) != "" {
		sb.WriteString(strings.TrimSpace(skillsIndex))
		sb.WriteString("\n\n")
	}

	// 5. Ambient Environment Context Grounding
	if sc != nil {
		sb.WriteString(sc.FormatPromptContext())
		sb.WriteString("\nContext Resolution:\n")
		sb.WriteString("- Resolve relative paths starting from Current Directory.\n")
		sb.WriteString("- Use Recent Shell History to resolve references like \"re-run that test\" or \"undo that commit\".\n")
		sb.WriteString("- Use Last Executed Action (if present) to resolve prior commands, PIDs, outputs, and errors (e.g. \"that process / those PIDs\" -> use PIDs from output; \"fix the error\" -> resolve failure).\n")
	}

	return strings.TrimSpace(sb.String())
}

// BuildSystemPrompt delegates to BuildSystemPromptWithSkills without a skills index.
func BuildSystemPrompt(targetOS, targetArch string, shellType Type, customInstructions string, sc *Context) string {
	return BuildSystemPromptWithSkills(targetOS, targetArch, shellType, customInstructions, sc, "")
}
