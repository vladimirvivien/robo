package shell_test

import (
	"context"
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/shell"
)

func TestToolHandler_EmptyCommand(t *testing.T) {
	cfg := config.NewDefaultConfig()
	handler := shell.NewToolHandler(cfg)

	out, err := handler.Handle(context.Background(), shell.ShellInput{Command: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Error != "empty command" {
		t.Errorf("expected empty command error, got: %s", out.Error)
	}
}

func TestToolHandler_YoloApproveAll(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Robo.YoloApproveAll = true
	handler := shell.NewToolHandler(cfg)

	out, err := handler.Handle(context.Background(), shell.ShellInput{
		Command:     "echo testing-tool-call",
		Description: "Echo test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d (error: %s)", out.ExitCode, out.Error)
	}
	if !strings.Contains(out.Output, "testing-tool-call") {
		t.Errorf("expected output to contain 'testing-tool-call', got: %q", out.Output)
	}
}

func TestToolHandler_NonInteractiveJSON(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.Robo.OutputMode = "json"
	handler := shell.NewToolHandler(cfg)

	out, err := handler.Handle(context.Background(), shell.ShellInput{
		Command:     "echo hello-from-json-tool",
		Description: "Echo hello test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", out.ExitCode)
	}
	if !strings.Contains(out.Output, "hello-from-json-tool") {
		t.Errorf("expected captured execution output 'hello-from-json-tool', got %q", out.Output)
	}
}
