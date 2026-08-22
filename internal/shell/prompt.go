package shell

import (
	"fmt"
	"strings"
)

// BuildSystemPrompt dynamically assembles a focused, token-efficient system prompt
// tailored specifically to the target OS, architecture, and shell dialect.
func BuildSystemPrompt(targetOS, targetArch string, shellType Type, customInstructions string, sc *Context) string {
	var sb strings.Builder

	// 1. Universal Core Protocol (~50 tokens)
	sb.WriteString("You are Robo, an on-device AI system assistant designed to interact directly with the operating system.\n\n")
	sb.WriteString("Tool Calling Rules:\n")
	sb.WriteString("- When an action, command, or query needs to run in the terminal, invoke the \"execute_shell\" tool.\n")
	sb.WriteString("- In \"execute_shell\", provide a concise 1-sentence description and the exact command string.\n")
	sb.WriteString("- Do NOT output duplicate conversational explanations, preambles, or markdown command fences when calling \"execute_shell\".\n")
	sb.WriteString("- For pure explanations, questions, or code not meant for immediate execution, output markdown without calling \"execute_shell\".\n")
	sb.WriteString("- Commands must be complete, runnable, and contain NO placeholder tokens (e.g. '<file>').\n\n")

	// 2. Dynamic Platform & Shell Module (~50-60 tokens)
	switch shellType {
	case ShellPowerShell:
		sb.WriteString("Target Environment: Windows PowerShell\n")
		sb.WriteString("Syntax & Execution:\n")
		sb.WriteString("- Generate non-interactive batch commands using standard cmdlets: Get-Process, Get-Service, Get-ChildItem, Where-Object, Select-Object, Select-String.\n")
		sb.WriteString("- For CPU/Memory inspection and process ranking: Use 'Get-Process | Sort-Object CPU -Descending | Select-Object -First <N>'. Never use Get-Counter.\n")
		sb.WriteString("- Use 'Select-String' (not 'grep'), 'Select-Object -First N' (not 'head').\n")
		sb.WriteString("Multi-Step Tasks:\n")
		sb.WriteString("- When a request has sequential steps (\"do X; then do Y\", \"wait N seconds and show Z\"), execute all steps connected by ';' or pipelines '|'.\n")
		sb.WriteString("- For time intervals or delays, use 'Start-Sleep -Seconds <N>' before the query command.\n")
		sb.WriteString("- For state modifications, use idempotent patterns (e.g. 'if (Test-Path dir) { ... }' or 'New-Item -Force').\n\n")

	case ShellFish:
		// Fish drops down to POSIX Bash for command execution
		sb.WriteString("Target Environment: POSIX Bash (executed via Bash compatibility layer for Fish terminal)\n")
		sb.WriteString("Syntax & Execution:\n")
		sb.WriteString("- Generate non-interactive batch commands; use standard POSIX Bash utilities (find, grep, awk, sed, curl, tar) with quoted variables (\"$VAR\").\n")
		sb.WriteString("Multi-Step Tasks:\n")
		sb.WriteString("- When a request has sequential steps (\"do X then Y\", \"wait N seconds and show Z\"), chain with '&&' for fail-fast safety.\n")
		sb.WriteString("- For time intervals or delays, use 'sleep <N>' before the query command.\n")
		sb.WriteString("- For CPU/Memory ranking: Use 'ps aux --sort=-%cpu | head -n 6'.\n")
		sb.WriteString("- Use idempotent patterns ('mkdir -p', 'rm -f', '[ -f ... ]').\n\n")

	default:
		switch targetOS {
		case "darwin":
			sb.WriteString("Target Environment: macOS POSIX (BSD)\n")
			sb.WriteString("Syntax & Execution:\n")
			sb.WriteString("- Generate non-interactive batch commands; avoid interactive tools (top, htop, less, nano, vi).\n")
			sb.WriteString("- For CPU/Memory ranking: Use 'ps aux -r | head -n 6' (CPU) or 'top -l 1 | grep PhysMem' (Memory). Do NOT use Linux 'free' or GNU '--sort'.\n")
			sb.WriteString("- Use 'sed -i \"\"' for in-place file editing.\n")
			sb.WriteString("- Quote all variables (\"$VAR\") and glob patterns.\n")
			sb.WriteString("Multi-Step Tasks:\n")
			sb.WriteString("- When a request has sequential steps (\"do X then Y\", \"wait N seconds and show Z\"), chain with '&&' for fail-fast safety.\n")
			sb.WriteString("- For time intervals or delays, use 'sleep <N>' before the query command.\n")
			sb.WriteString("- Use idempotent patterns ('mkdir -p', pre-existence checks).\n\n")
		case "windows":
			sb.WriteString("Target Environment: Windows Command Prompt (CMD)\n")
			sb.WriteString("Syntax & Execution:\n")
			sb.WriteString("- Use standard Windows batch/cmd utilities (dir, findstr, tasklist, type).\n")
			sb.WriteString("Multi-Step Tasks:\n")
			sb.WriteString("- When a request has sequential steps, chain operations with '&&'.\n\n")
		default:
			sb.WriteString("Target Environment: Linux POSIX (GNU)\n")
			sb.WriteString("Syntax & Execution:\n")
			sb.WriteString("- Generate non-interactive batch commands; avoid interactive tools (top, htop, less, nano, vi).\n")
			sb.WriteString("- For CPU/Memory ranking: Use 'ps aux --sort=-%cpu | head -n 6' or 'free -h'.\n")
			sb.WriteString("- Use standard GNU utilities (ps, grep, awk, sed, find, curl, tar) with quoted variables (\"$VAR\") and globs.\n")
			sb.WriteString("Multi-Step Tasks:\n")
			sb.WriteString("- When a request has sequential steps (\"do X then Y\", \"wait N seconds and show Z\"), chain with '&&' for fail-fast safety.\n")
			sb.WriteString("- For time intervals or delays, use 'sleep <N>' before the query command.\n")
			sb.WriteString("- Use idempotent patterns ('mkdir -p', 'rm -f', '[ -f ... ]').\n\n")
		}
	}

	// 3. User Instructions (if provided)
	if strings.TrimSpace(customInstructions) != "" {
		fmt.Fprintf(&sb, "User Instructions:\n%s\n\n", strings.TrimSpace(customInstructions))
	}

	// 4. Ambient Environment Context Grounding
	if sc != nil {
		sb.WriteString(sc.FormatPromptContext())
		sb.WriteString("\nContext Resolution:\n")
		sb.WriteString("- Resolve relative paths starting from Current Directory.\n")
		sb.WriteString("- Use Recent Shell History to resolve references like \"re-run that test\" or \"undo that commit\".\n")
		sb.WriteString("- Use Last Executed Action (if present) to understand prior commands, status, and error messages to resolve follow-ups like \"fix the previous error\" or \"what went wrong\".\n")
	}

	return strings.TrimSpace(sb.String())
}
