package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Type represents a supported shell dialect.
type Type string

const (
	ShellZsh        Type = "zsh"
	ShellBash       Type = "bash"
	ShellFish       Type = "fish"
	ShellPowerShell Type = "powershell"
	ShellUnknown    Type = "unknown"
)

// DetectShell identifies the user's active shell from environment or OS defaults.
func DetectShell() Type {
	shellEnv := os.Getenv("SHELL")
	if shellEnv != "" {
		base := strings.ToLower(filepath.Base(shellEnv))
		switch {
		case strings.Contains(base, "zsh"):
			return ShellZsh
		case strings.Contains(base, "bash"):
			return ShellBash
		case strings.Contains(base, "fish"):
			return ShellFish
		case strings.Contains(base, "pwsh") || strings.Contains(base, "powershell"):
			return ShellPowerShell
		}
	}

	// PowerShell / Windows specific checks
	if os.Getenv("PSModulePath") != "" || runtime.GOOS == "windows" {
		return ShellPowerShell
	}

	return ShellUnknown
}
