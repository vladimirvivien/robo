package shell

import (
	"regexp"
	"strings"
)

// RiskTier categorizes the operational danger of proposed shell commands.
type RiskTier string

const (
	// RiskTierReadOnly represents low-risk inspection and diagnostic commands (0.0 - 0.2).
	RiskTierReadOnly RiskTier = "read-only"
	// RiskTierMutating represents state-modifying actions like file edits, builds, and package installs (0.3 - 0.6).
	RiskTierMutating RiskTier = "mutating"
	// RiskTierDestructive represents high-risk deletions, disk writes, process termination, and hard resets (0.7 - 1.0).
	RiskTierDestructive RiskTier = "destructive"
)

type safetyRule struct {
	pattern *regexp.Regexp
	reason  string
	tier    RiskTier
	score   float64
}

var destructiveRules = []safetyRule{
	{
		pattern: regexp.MustCompile(`(?i)\brm\s+-[a-z]*r[a-z]*f*\b|\brm\s+-[a-z]*f[a-z]*r*\b|\brm\s+--recursive\b`),
		reason:  "Recursive file deletion (`rm -rf` / `rm -r`)",
		tier:    RiskTierDestructive,
		score:   1.0,
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(Remove-Item|del|rmdir|rd)\b.*(\s+[-/](Recurse|Force|r|s|q)\b)`),
		reason:  "PowerShell/Windows recursive file or directory deletion (`Remove-Item` / `del /s`)",
		tier:    RiskTierDestructive,
		score:   1.0,
	},
	{
		pattern: regexp.MustCompile(`(?i)\bdd\s+if=\S+`),
		reason:  "Direct disk block overwrite (`dd`)",
		tier:    RiskTierDestructive,
		score:   1.0,
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(mkfs(\.\w+)?|fdisk|parted|gdisk|Format-Volume|Clear-Disk)\b`),
		reason:  "Disk partition / filesystem formatting (`mkfs` / `Format-Volume`)",
		tier:    RiskTierDestructive,
		score:   1.0,
	},
	{
		pattern: regexp.MustCompile(`(?i)\bkill\s+-9\b|\bkillall\s+-9\b|\bStop-Process\b.*(\s+-Force\b)`),
		reason:  "Forced process termination (`kill -9` / `Stop-Process -Force`)",
		tier:    RiskTierDestructive,
		score:   0.8,
	},
	{
		pattern: regexp.MustCompile(`(?i)\bchmod\s+(-[a-z]*R[a-z]*\s+)?(777|000)\s+[/~]`),
		reason:  "Root or home filesystem permission rewrite (`chmod 777 /`)",
		tier:    RiskTierDestructive,
		score:   0.9,
	},
	{
		pattern: regexp.MustCompile(`(?i)\bgit\s+reset\s+--hard\b`),
		reason:  "Git destructive worktree discard (`git reset --hard`)",
		tier:    RiskTierDestructive,
		score:   0.85,
	},
	{
		pattern: regexp.MustCompile(`(?i)\bgit\s+clean\s+-[a-z]*f[a-z]*\b`),
		reason:  "Git uncommitted file removal (`git clean -f`)",
		tier:    RiskTierDestructive,
		score:   0.8,
	},
	{
		pattern: regexp.MustCompile(`(?i)\bgit\s+push\b.*(\s+--force|-f\b)`),
		reason:  "Git remote history rewrite (`git push --force`)",
		tier:    RiskTierDestructive,
		score:   0.9,
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(DROP\s+DATABASE|DROP\s+TABLE|TRUNCATE\s+TABLE)\b`),
		reason:  "Database destruction statement (`DROP DATABASE` / `DROP TABLE`)",
		tier:    RiskTierDestructive,
		score:   0.95,
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(reboot|shutdown|init\s+0|Stop-Computer)\b`),
		reason:  "Host system shutdown / reboot (`shutdown` / `Stop-Computer`)",
		tier:    RiskTierDestructive,
		score:   0.9,
	},
}

var mutatingRules = []safetyRule{
	{
		pattern: regexp.MustCompile(`(?i)\b(npm|yarn|pnpm)\s+(install|i|add|remove|uninstall)\b`),
		reason:  "Node package modification (`npm install` / `yarn add`)",
		tier:    RiskTierMutating,
		score:   0.4,
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(pip|pip3|conda)\s+(install|uninstall)\b`),
		reason:  "Python package installation / removal (`pip install`)",
		tier:    RiskTierMutating,
		score:   0.4,
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(go\s+get|go\s+install|cargo\s+install|brew\s+install|winget\s+install|choco\s+install|apt|apt-get|yum|dnf|pacman)\b`),
		reason:  "System / language package management (`go install` / `brew install`)",
		tier:    RiskTierMutating,
		score:   0.45,
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(touch|mkdir|New-Item|Set-Content|Add-Content|Out-File|sed\s+-i)\b`),
		reason:  "File or directory creation / in-place modification",
		tier:    RiskTierMutating,
		score:   0.35,
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(cp|mv|Copy-Item|Move-Item|Rename-Item)\b`),
		reason:  "File / directory copy or move",
		tier:    RiskTierMutating,
		score:   0.35,
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(systemctl|service)\s+(restart|start|stop|enable|disable)\b`),
		reason:  "Service state modification (`systemctl restart`)",
		tier:    RiskTierMutating,
		score:   0.5,
	},
	{
		pattern: regexp.MustCompile(`(?i)\b(docker|podman)\s+(run|rm|rmi|stop|kill|exec|compose)\b`),
		reason:  "Container state modification (`docker run` / `docker stop`)",
		tier:    RiskTierMutating,
		score:   0.5,
	},
	{
		pattern: regexp.MustCompile(`(?i)\bgit\s+(commit|checkout\s+-b|switch\s+-c|merge|rebase|pull|branch\s+-[dD])\b`),
		reason:  "Git repository state mutation (`git commit` / `git merge`)",
		tier:    RiskTierMutating,
		score:   0.4,
	},
}

// SafetyAssessment represents the structured risk evaluation of a shell command.
type SafetyAssessment struct {
	Tier          RiskTier `json:"tier"`
	Score         float64  `json:"score"`
	Level         string   `json:"level"` // "safe", "destructive" (backward compatibility)
	Warning       string   `json:"warning,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	IsDestructive bool     `json:"is_destructive"`
}

// AssessSafety evaluates the risk tier, score, and warning of a shell command.
func AssessSafety(cmd string) SafetyAssessment {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return SafetyAssessment{
			Tier:          RiskTierReadOnly,
			Score:         0.0,
			Level:         "safe",
			IsDestructive: false,
		}
	}

	// 1. Check Destructive (Tier 3: 0.7 - 1.0)
	for _, rule := range destructiveRules {
		if rule.pattern.MatchString(trimmed) {
			return SafetyAssessment{
				Tier:          RiskTierDestructive,
				Score:         rule.score,
				Level:         "destructive",
				Warning:       rule.reason,
				Reason:        rule.reason,
				IsDestructive: true,
			}
		}
	}

	// 2. Check Mutating (Tier 2: 0.3 - 0.6)
	for _, rule := range mutatingRules {
		if rule.pattern.MatchString(trimmed) {
			return SafetyAssessment{
				Tier:          RiskTierMutating,
				Score:         rule.score,
				Level:         "safe",
				Reason:        rule.reason,
				IsDestructive: false,
			}
		}
	}

	// 3. Default: Read-Only (Tier 1: 0.0 - 0.2)
	return SafetyAssessment{
		Tier:          RiskTierReadOnly,
		Score:         0.1,
		Level:         "safe",
		Reason:        "Read-only diagnostic command",
		IsDestructive: false,
	}
}

// EvaluateCombinedRisk reconciles the deterministic regex scanner with model-provided semantic risk.
func EvaluateCombinedRisk(cmd string, modelRisk string, modelDestructive bool) SafetyAssessment {
	assess := AssessSafety(cmd)

	if modelDestructive || strings.EqualFold(modelRisk, "destructive") || strings.EqualFold(modelRisk, "tier-3") {
		if assess.Tier != RiskTierDestructive {
			assess.Tier = RiskTierDestructive
			assess.Score = 0.9
			assess.Level = "destructive"
			assess.IsDestructive = true
			if assess.Warning == "" {
				assess.Warning = "Model classified this command as destructive"
			}
		}
	} else if strings.EqualFold(modelRisk, "mutating") || strings.EqualFold(modelRisk, "tier-2") {
		if assess.Tier == RiskTierReadOnly {
			assess.Tier = RiskTierMutating
			assess.Score = 0.5
		}
	}

	return assess
}

// IsDestructiveCommand checks if a shell command string matches high-risk destructive patterns.
func IsDestructiveCommand(cmd string) (bool, string) {
	assess := AssessSafety(cmd)
	return assess.IsDestructive, assess.Warning
}
