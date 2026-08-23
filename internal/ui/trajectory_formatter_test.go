package ui_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/ui"
)

func TestTrajectoryFormatter_JSON(t *testing.T) {
	tf := ui.NewTrajectoryFormatter("json", false, 80)
	var buf bytes.Buffer

	data := ui.SessionOutputData{
		Goal:       "find robo process and ports",
		Status:     "completed",
		TotalSteps: 2,
		Steps: []ui.TrajectoryStep{
			{
				Step:        1,
				Command:     "pgrep -f robo",
				Description: "Find PID",
				Output:      "8836",
				ExitCode:    0,
				Executed:    true,
				RiskTier:    "read-only",
				RiskScore:   0.1,
			},
			{
				Step:        2,
				Command:     "ss -tlpn | grep 8836",
				Description: "Check port",
				Output:      "LISTEN 0 128 0.0.0.0:8765",
				ExitCode:    0,
				Executed:    true,
				RiskTier:    "read-only",
				RiskScore:   0.1,
			},
		},
		FinalResponse: "Robo is listening on port 8765.",
		Provider:      "litertlm",
		Model:         "litert-community/gemma-4-E4B-it",
		Local:         true,
	}

	if err := tf.FormatSession(&buf, data); err != nil {
		t.Fatalf("FormatSession json error: %v", err)
	}

	var parsed ui.SessionOutputData
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated json: %v\nJSON:\n%s", err, buf.String())
	}

	if parsed.Goal != "find robo process and ports" {
		t.Errorf("expected goal preserved, got: %s", parsed.Goal)
	}
	if len(parsed.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(parsed.Steps))
	}
	if parsed.Steps[0].Command != "pgrep -f robo" {
		t.Errorf("unexpected step 1 command: %s", parsed.Steps[0].Command)
	}
	if parsed.Steps[1].RiskTier != "read-only" {
		t.Errorf("unexpected step 2 risk tier: %s", parsed.Steps[1].RiskTier)
	}
}

func TestTrajectoryFormatter_CodeReplay(t *testing.T) {
	tf := ui.NewTrajectoryFormatter("code", false, 80)
	var buf bytes.Buffer

	data := ui.SessionOutputData{
		Goal:       "create directories and initialize project",
		Status:     "completed",
		TotalSteps: 2,
		Steps: []ui.TrajectoryStep{
			{
				Step:        1,
				Command:     "mkdir -p src/cmd",
				Description: "Create src directory",
				Executed:    true,
			},
			{
				Step:        2,
				Command:     "go mod init myapp",
				Description: "Initialize Go module",
				Executed:    true,
			},
		},
	}

	if err := tf.FormatSession(&buf, data); err != nil {
		t.Fatalf("FormatSession code error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "mkdir -p src/cmd") {
		t.Errorf("expected step 1 command in replay script, got:\n%s", out)
	}
	if !strings.Contains(out, "go mod init myapp") {
		t.Errorf("expected step 2 command in replay script, got:\n%s", out)
	}
	if !strings.Contains(out, "Goal: create directories and initialize project") {
		t.Errorf("expected goal header in replay script, got:\n%s", out)
	}
}

func TestTrajectoryFormatter_Plain(t *testing.T) {
	tf := ui.NewTrajectoryFormatter("plain", false, 80)
	var buf bytes.Buffer

	data := ui.SessionOutputData{
		Goal:          "what is robo",
		FinalResponse: "Robo is an on-device AI agent.",
	}

	if err := tf.FormatSession(&buf, data); err != nil {
		t.Fatalf("FormatSession plain error: %v", err)
	}

	out := strings.TrimSpace(buf.String())
	if out != "Robo is an on-device AI agent." {
		t.Errorf("unexpected plain output: %s", out)
	}
}
