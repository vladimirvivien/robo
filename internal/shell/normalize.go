package shell

import (
	"regexp"
	"strings"
)

var (
	// PowerShell process property hallucinations produced by small language models
	reWSMemoryUsage     = regexp.MustCompile(`(?i)\bWS_MemoryUsage\b`)
	reWorkingSetMemory  = regexp.MustCompile(`(?i)\bWorkingSet_Memory\b`)
	reCPUUsage          = regexp.MustCompile(`(?i)\bCPU_Usage\b`)
	reProcessNameSnake  = regexp.MustCompile(`(?i)\bProcess_Name\b`)
	reExpandPID         = regexp.MustCompile(`(?i)(-ExpandProperty\s+)PID\b`)
	reExpandMemoryUsage = regexp.MustCompile(`(?i)(-ExpandProperty\s+)MemoryUsage\b`)
)

// NormalizeCommand performs lightweight, fast post-processing on model-generated commands
// to fix common small-model naming conflations and platform-specific property typos.
func NormalizeCommand(targetOS string, shellType Type, cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}

	if shellType == ShellPowerShell || targetOS == "windows" {
		cmd = normalizePowerShell(cmd)
	}

	return cmd
}

func normalizePowerShell(cmd string) string {
	cmd = reWSMemoryUsage.ReplaceAllString(cmd, "WS")
	cmd = reWorkingSetMemory.ReplaceAllString(cmd, "WS")
	cmd = reCPUUsage.ReplaceAllString(cmd, "CPU")
	cmd = reProcessNameSnake.ReplaceAllString(cmd, "ProcessName")
	cmd = reExpandPID.ReplaceAllString(cmd, "${1}Id")
	cmd = reExpandMemoryUsage.ReplaceAllString(cmd, "${1}WS")
	return cmd
}
