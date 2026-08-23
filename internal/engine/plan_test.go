package engine_test

import (
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/engine"
)

func TestPlanCompiler_WindowsPowerShell(t *testing.T) {
	compiler := engine.NewPlanCompiler("windows")

	res := &engine.SessionResult{
		Goal:       "find process robo and inspect ports",
		Status:     "completed",
		TotalSteps: 2,
		Steps: []engine.StepRecord{
			{
				Step:        1,
				Command:     "Get-Process -Name robo",
				Description: "Find robo process ID",
				Executed:    false,
				RiskTier:    "read-only",
				RiskScore:   0.1,
			},
			{
				Step:        2,
				Command:     "Get-NetTCPConnection -OwningProcess 8836",
				Description: "Query listening port",
				Executed:    false,
				RiskTier:    "read-only",
				RiskScore:   0.1,
			},
		},
	}

	plan := compiler.Compile(res)
	if plan == nil {
		t.Fatal("expected compiled plan, got nil")
	}

	if plan.TargetOS != "windows" {
		t.Errorf("expected target OS windows, got %s", plan.TargetOS)
	}
	if plan.ShellType != "powershell" {
		t.Errorf("expected shell powershell, got %s", plan.ShellType)
	}
	if plan.TotalSteps != 2 {
		t.Errorf("expected 2 steps, got %d", plan.TotalSteps)
	}

	script := plan.ScriptContent
	if !strings.Contains(script, "$ErrorActionPreference = 'Stop'") {
		t.Errorf("expected PowerShell ErrorActionPreference header, got:\n%s", script)
	}
	if !strings.Contains(script, "Get-Process -Name robo") {
		t.Errorf("expected step 1 command in script, got:\n%s", script)
	}
	if !strings.Contains(script, "Get-NetTCPConnection -OwningProcess 8836") {
		t.Errorf("expected step 2 command in script, got:\n%s", script)
	}
}

func TestPlanCompiler_LinuxBash(t *testing.T) {
	compiler := engine.NewPlanCompiler("linux")

	res := &engine.SessionResult{
		Goal:       "build and run container",
		Status:     "completed",
		TotalSteps: 2,
		Steps: []engine.StepRecord{
			{
				Step:        1,
				Command:     "docker build -t myapp:latest .",
				Description: "Build image",
				Executed:    false,
				RiskTier:    "mutating",
				RiskScore:   0.5,
			},
			{
				Step:        2,
				Command:     "docker run -d -p 8080:80 myapp:latest",
				Description: "Launch container",
				Executed:    false,
				RiskTier:    "mutating",
				RiskScore:   0.5,
			},
		},
	}

	plan := compiler.Compile(res)
	if plan == nil {
		t.Fatal("expected compiled plan, got nil")
	}

	if plan.TargetOS != "linux" {
		t.Errorf("expected target OS linux, got %s", plan.TargetOS)
	}
	if plan.ShellType != "bash" {
		t.Errorf("expected shell bash, got %s", plan.ShellType)
	}
	if plan.HighestRisk != "mutating" {
		t.Errorf("expected highest risk mutating, got %s", plan.HighestRisk)
	}

	script := plan.ScriptContent
	if !strings.Contains(script, "#!/usr/bin/env bash") {
		t.Errorf("expected bash shebang, got:\n%s", script)
	}
	if !strings.Contains(script, "set -euo pipefail") {
		t.Errorf("expected strict flags in bash script, got:\n%s", script)
	}
	if !strings.Contains(script, "docker build -t myapp:latest .") {
		t.Errorf("expected step 1 command in script, got:\n%s", script)
	}
}
