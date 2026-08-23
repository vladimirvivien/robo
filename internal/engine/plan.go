package engine

import (
	"fmt"
	"runtime"
	"strings"
)

// ExecutionPlan represents a compiled multi-step dry-run execution plan.
type ExecutionPlan struct {
	Goal          string       `json:"goal"`
	TargetOS      string       `json:"target_os"`
	ShellType     string       `json:"shell_type"`
	TotalSteps    int          `json:"total_steps"`
	HighestRisk   string       `json:"highest_risk"`
	Steps         []StepRecord `json:"steps"`
	ScriptContent string       `json:"script_content"`
}

// PlanCompiler compiles multi-stage session results into standalone shell scripts and execution plans.
type PlanCompiler struct {
	TargetOS string
}

// NewPlanCompiler creates a new plan compiler for the specified or active operating system.
func NewPlanCompiler(targetOS ...string) *PlanCompiler {
	tos := runtime.GOOS
	if len(targetOS) > 0 && targetOS[0] != "" {
		tos = targetOS[0]
	}
	return &PlanCompiler{TargetOS: tos}
}

// Compile generates an ExecutionPlan with standalone script content from a SessionResult.
func (c *PlanCompiler) Compile(res *SessionResult) *ExecutionPlan {
	if res == nil {
		return nil
	}

	highestRisk := "read-only"
	var maxScore float64

	for _, s := range res.Steps {
		if s.RiskScore > maxScore {
			maxScore = s.RiskScore
			highestRisk = s.RiskTier
		}
	}

	shellType := "bash"
	if c.TargetOS == "windows" {
		shellType = "powershell"
	}

	script := c.CompileScript(res.Goal, res.Steps)

	return &ExecutionPlan{
		Goal:          res.Goal,
		TargetOS:      c.TargetOS,
		ShellType:     shellType,
		TotalSteps:    len(res.Steps),
		HighestRisk:   highestRisk,
		Steps:         res.Steps,
		ScriptContent: script,
	}
}

// CompileScript produces a clean, self-contained shell script (.ps1 on Windows, .sh on POSIX).
func (c *PlanCompiler) CompileScript(goal string, steps []StepRecord) string {
	var sb strings.Builder
	isWindows := (c.TargetOS == "windows")

	if isWindows {
		sb.WriteString("<#\n")
		sb.WriteString(".SYNOPSIS\n")
		fmt.Fprintf(&sb, "  Robo Execution Plan: %s\n", goal)
		sb.WriteString(".DESCRIPTION\n")
		fmt.Fprintf(&sb, "  Compiled by Robo AI Assistant (%s/%s)\n", runtime.GOOS, runtime.GOARCH)
		sb.WriteString("#>\n\n")
		sb.WriteString("$ErrorActionPreference = 'Stop'\n\n")
	} else {
		sb.WriteString("#!/usr/bin/env bash\n")
		sb.WriteString("set -euo pipefail\n\n")
		fmt.Fprintf(&sb, "# Robo Execution Plan: %s\n", goal)
		fmt.Fprintf(&sb, "# Compiled by Robo AI Assistant (%s/%s)\n\n", runtime.GOOS, runtime.GOARCH)
	}

	for _, s := range steps {
		cmd := strings.TrimSpace(s.Command)
		if cmd == "" {
			continue
		}

		fmt.Fprintf(&sb, "# Step %d: %s\n", s.Step, s.Description)
		if s.RiskTier != "" {
			fmt.Fprintf(&sb, "# Risk: %s (score: %.2f)\n", s.RiskTier, s.RiskScore)
		}
		sb.WriteString(cmd + "\n\n")
	}

	return strings.TrimSpace(sb.String())
}
