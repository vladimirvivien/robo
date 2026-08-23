package engine_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/engine"
)

func TestTruncateOutput_Short(t *testing.T) {
	input := "line 1\nline 2\nline 3"
	got := engine.TruncateOutput(input, 5, 5)
	if got != input {
		t.Errorf("expected unchanged text for short output, got:\n%s", got)
	}
}

func TestTruncateOutput_Long(t *testing.T) {
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	input := strings.Join(lines, "\n")

	got := engine.TruncateOutput(input, 3, 3)

	if !strings.Contains(got, "line 1\nline 2\nline 3") {
		t.Errorf("expected head lines preserved, got:\n%s", got)
	}
	if !strings.Contains(got, "line 98\nline 99\nline 100") {
		t.Errorf("expected tail lines preserved, got:\n%s", got)
	}
	if !strings.Contains(got, "[94 lines omitted]") {
		t.Errorf("expected omission marker with line count, got:\n%s", got)
	}
}

func TestTrajectoryManager_RollingCompression(t *testing.T) {
	tm := engine.NewTrajectoryManager()

	// Add Step 1 (older step with large output)
	tm.AddStep(engine.StepRecord{
		Step:        1,
		Command:     "pgrep robo",
		Description: "Find robo process ID",
		Output:      "8836\n8837\n8838",
		ExitCode:    0,
		Executed:    true,
	})

	// Add Step 2 (older step)
	tm.AddStep(engine.StepRecord{
		Step:        2,
		Command:     "Get-NetTCPConnection -OwningProcess 8836",
		Description: "Query port",
		Output:      "LocalPort 8765 State Listen",
		ExitCode:    0,
		Executed:    true,
	})

	// Add Step 3 (immediate previous step with verbose output)
	var buildLogs []string
	for i := 1; i <= 50; i++ {
		buildLogs = append(buildLogs, fmt.Sprintf("Building package %d...", i))
	}
	tm.AddStep(engine.StepRecord{
		Step:        3,
		Command:     "go test -v ./...",
		Description: "Run test suite",
		Output:      strings.Join(buildLogs, "\n"),
		ExitCode:    1,
		Error:       "FAIL: package auth (exit status 1)",
		Executed:    true,
	})

	formatted := tm.FormatPromptContext("restart robo and verify health")

	// Verify Step 1 is compacted to 1 line
	if !strings.Contains(formatted, "• Step 1: `pgrep robo` ──> Succeeded (Exit 0)") {
		t.Errorf("expected Step 1 to be folded into 1 line, got:\n%s", formatted)
	}

	// Verify Step 2 is compacted to 1 line
	if !strings.Contains(formatted, "• Step 2: `Get-NetTCPConnection -OwningProcess 8836` ──> Succeeded (Exit 0)") {
		t.Errorf("expected Step 2 to be folded into 1 line, got:\n%s", formatted)
	}

	// Verify Step 3 retains detailed truncated output
	if !strings.Contains(formatted, "[Most Recent Step Result]") {
		t.Errorf("expected Most Recent Step Result header for Step 3, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "Command: go test -v ./...") {
		t.Errorf("expected Step 3 command in detail, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "FAIL: package auth (exit status 1)") {
		t.Errorf("expected Step 3 error in detail, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "[30 lines omitted]") {
		t.Errorf("expected Step 3 output to be truncated with omission marker, got:\n%s", formatted)
	}
}

func TestTrajectoryManager_TokenBudgetConstraint(t *testing.T) {
	tm := engine.NewTrajectoryManager()

	// Simulate 5 steps with large 500-line outputs each
	for s := 1; s <= 5; s++ {
		var outputLines []string
		for i := 1; i <= 500; i++ {
			outputLines = append(outputLines, fmt.Sprintf("Step %d - log line %d: metric = %d.000", s, i, i*10))
		}
		tm.AddStep(engine.StepRecord{
			Step:        s,
			Command:     fmt.Sprintf("tool-step-%d --verbose", s),
			Description: fmt.Sprintf("Execute diagnostic step %d", s),
			Output:      strings.Join(outputLines, "\n"),
			ExitCode:    0,
			Executed:    true,
		})
	}

	formatted := tm.FormatPromptContext("diagnose complex multi-tier cluster")
	approxTokens := (len(formatted) + 3) / 4

	// Ensure 5-step multi-turn prompt stays well under 1,200 tokens
	if approxTokens > 1200 {
		t.Errorf("expected trajectory prompt to stay under 1,200 tokens, got %d tokens (~%d chars)", approxTokens, len(formatted))
	}
}

func TestTrajectoryManager_EmptyOutput(t *testing.T) {
	tm := engine.NewTrajectoryManager()
	tm.AddStep(engine.StepRecord{
		Step:        1,
		Command:     "Get-Service | Where-Object {$_.Name -like '*robo*'}",
		Description: "Check if robo service exists",
		Output:      "",
		ExitCode:    0,
		Executed:    true,
	})

	formatted := tm.FormatPromptContext("Is the a service running called robo?")
	if !strings.Contains(formatted, "(empty output - 0 matching records or lines produced)") {
		t.Errorf("expected explicit empty output indication, got:\n%s", formatted)
	}
}
