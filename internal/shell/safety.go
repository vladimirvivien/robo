package shell

import (
	"regexp"
	"strings"
)

type destructiveRule struct {
	pattern *regexp.Regexp
	reason  string
}

var destructiveRules = []destructiveRule{
	{
		pattern: regexp.MustCompile(`(?i)\brm\s+-[a-z]*r[a-z]*f*\b|\brm\s+-[a-z]*f[a-z]*r*\b|\brm\s+--recursive\b`),
		reason:  "Recursive file deletion (`rm -rf` / `rm -r`)",
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(Remove-Item|del|rmdir|rd)\b.*(\s+-(Recurse|Force|r|s)\b)`),
		reason:  "PowerShell/Windows recursive folder removal",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bdd\s+if=\S+`),
		reason:  "Direct disk block overwrite (`dd`)",
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(mkfs(\.\w+)?|fdisk|parted|gdisk)\b`),
		reason:  "Disk partition/filesystem formatting (`mkfs` / `fdisk`)",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bkill\s+-9\s+1\b|\bkillall\s+-9\b`),
		reason:  "Critical system process termination (`kill -9 1`)",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bchmod\s+(-[a-z]*R[a-z]*\s+)?(777|000)\s+[/~]`),
		reason:  "Root or home filesystem permission rewrite (`chmod 777 /`)",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bgit\s+reset\s+--hard\b`),
		reason:  "Git destructive worktree discard (`git reset --hard`)",
	},
	{
		pattern: regexp.MustCompile(`(?i)\bgit\s+clean\s+-[a-z]*f[a-z]*\b`),
		reason:  "Git uncommitted file removal (`git clean -f`)",
	},
}

// IsDestructiveCommand checks if a shell command string contains high-risk or destructive patterns.
func IsDestructiveCommand(cmd string) (bool, string) {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return false, ""
	}

	for _, rule := range destructiveRules {
		if rule.pattern.MatchString(trimmed) {
			return true, rule.reason
		}
	}

	return false, ""
}
