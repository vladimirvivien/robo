package engine_test

import (
	"context"
	"testing"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
)

type mockEngine struct {
	responses []engine.Response
	callCount int
}

func (m *mockEngine) Name() string { return "mock" }
func (m *mockEngine) Close() error { return nil }

func (m *mockEngine) Generate(ctx context.Context, req engine.Request) (*engine.Response, error) {
	if m.callCount < len(m.responses) {
		resp := m.responses[m.callCount]
		m.callCount++
		return &resp, nil
	}
	return &engine.Response{Text: "default mock response"}, nil
}

func (m *mockEngine) GenerateStream(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
	out := make(chan engine.StreamChunk, 1)
	go func() {
		defer close(out)
		if m.callCount < len(m.responses) {
			resp := m.responses[m.callCount]
			m.callCount++
			out <- engine.StreamChunk{
				Text:      resp.Text,
				ToolCalls: resp.ToolCalls,
				Final:     true,
			}
			return
		}
		out <- engine.StreamChunk{Text: "default mock text", Final: true}
	}()
	return out, nil
}

func TestSessionRunner_ImmediateTextCompletion(t *testing.T) {
	mock := &mockEngine{
		responses: []engine.Response{
			{Text: "Robo is a local AI assistant."},
		},
	}

	cfg := config.NewDefaultConfig()
	runner := engine.NewSessionRunner(mock, cfg, engine.SessionConfig{
		MaxSteps: 5,
	})

	res, err := runner.Run(context.Background(), "what is robo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "completed" {
		t.Errorf("expected completed status, got %s", res.Status)
	}
	if res.FinalResponse != "Robo is a local AI assistant." {
		t.Errorf("unexpected final response: %s", res.FinalResponse)
	}
	if len(res.Steps) != 0 {
		t.Errorf("expected 0 tool steps, got %d", len(res.Steps))
	}
}

func TestSessionRunner_DryRunMultiStep(t *testing.T) {
	mock := &mockEngine{
		responses: []engine.Response{
			{
				Text: "Finding process",
				ToolCalls: []engine.ToolCall{
					{Name: "execute_shell", Command: "pgrep robo", Description: "Find PID"},
				},
			},
			{
				Text: "Querying ports",
				ToolCalls: []engine.ToolCall{
					{Name: "execute_shell", Command: "ss -tlpn | grep 8836", Description: "Find ports"},
				},
			},
			{
				Text: "Process robo is listening on port 8765.",
			},
		},
	}

	cfg := config.NewDefaultConfig()
	runner := engine.NewSessionRunner(mock, cfg, engine.SessionConfig{
		MaxSteps: 5,
		DryRun:   true,
	})

	res, err := runner.Run(context.Background(), "what port is robo on")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "completed" {
		t.Errorf("expected status completed, got %s", res.Status)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("expected 2 steps in dry run, got %d", len(res.Steps))
	}
	if res.Steps[0].Executed {
		t.Errorf("expected step 1 to not be executed in dry run")
	}
	if res.Steps[0].Command != "pgrep robo" {
		t.Errorf("unexpected step 1 command: %s", res.Steps[0].Command)
	}
	if res.Steps[1].Command != "ss -tlpn | grep 8836" {
		t.Errorf("unexpected step 2 command: %s", res.Steps[1].Command)
	}
}

func TestSessionRunner_OneShot(t *testing.T) {
	mock := &mockEngine{
		responses: []engine.Response{
			{
				ToolCalls: []engine.ToolCall{
					{Name: "execute_shell", Command: "echo hello", Description: "print hello"},
				},
			},
			{
				Text: "Done",
			},
		},
	}

	cfg := config.NewDefaultConfig()
	runner := engine.NewSessionRunner(mock, cfg, engine.SessionConfig{
		OneShot: true,
		DryRun:  true,
	})

	res, err := runner.Run(context.Background(), "say hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Steps) != 1 {
		t.Errorf("expected exactly 1 step in OneShot mode, got %d", len(res.Steps))
	}
	if mock.callCount != 1 {
		t.Errorf("expected exactly 1 model call in OneShot mode, got %d", mock.callCount)
	}
}

func TestSessionRunner_LoopDetection(t *testing.T) {
	// Model returns the exact same tool call twice in a row
	mock := &mockEngine{
		responses: []engine.Response{
			{
				ToolCalls: []engine.ToolCall{
					{Name: "execute_shell", Command: "Get-Service | Where-Object {$_.Name -like '*robo*'}", Description: "Find service"},
				},
			},
			{
				ToolCalls: []engine.ToolCall{
					{Name: "execute_shell", Command: "Get-Service | Where-Object {$_.Name -like '*robo*'}", Description: "Find service again"},
				},
			},
		},
	}

	cfg := config.NewDefaultConfig()
	// Disable interactive prompt for headless tests
	cfg.Robo.OutputMode = "json"
	runner := engine.NewSessionRunner(mock, cfg, engine.SessionConfig{
		MaxSteps: 5,
		Yolo:     true,
	})

	res, err := runner.Run(context.Background(), "Is there a service running called robo?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != "completed" {
		t.Errorf("expected status completed after loop detection, got %s", res.Status)
	}
	if len(res.Steps) != 1 {
		t.Errorf("expected only 1 executed step before loop termination, got %d", len(res.Steps))
	}
}
