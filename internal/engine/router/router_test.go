package router_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/engine/router"
)

func TestRouter_DecideRoute(t *testing.T) {
	tests := []struct {
		name         string
		cfg          config.RoutingConfig
		req          engine.Request
		wantStrategy router.Strategy
	}{
		{
			name:         "flag local-only",
			cfg:          config.RoutingConfig{Strategy: "auto", MaxLocalTokens: 4096},
			req:          engine.Request{Prompt: "test", ForceBackend: "local-only"},
			wantStrategy: router.StrategyLocalOnly,
		},
		{
			name:         "flag cloud-only",
			cfg:          config.RoutingConfig{Strategy: "auto", MaxLocalTokens: 4096},
			req:          engine.Request{Prompt: "test", ForceBackend: "cloud-only"},
			wantStrategy: router.StrategyCloudOnly,
		},
		{
			name:         "config local-only",
			cfg:          config.RoutingConfig{Strategy: "local-only", MaxLocalTokens: 4096},
			req:          engine.Request{Prompt: "test"},
			wantStrategy: router.StrategyLocalOnly,
		},
		{
			name:         "config cloud-only",
			cfg:          config.RoutingConfig{Strategy: "cloud-only", MaxLocalTokens: 4096},
			req:          engine.Request{Prompt: "test"},
			wantStrategy: router.StrategyCloudOnly,
		},
		{
			name: "heuristic images require cloud",
			cfg:  config.RoutingConfig{Strategy: "auto", MaxLocalTokens: 4096},
			req: engine.Request{
				Prompt: "look at this image",
				Images: []engine.ImageContext{{MimeType: "image/png", Data: []byte("fake-png")}},
			},
			wantStrategy: router.StrategyCloudOnly,
		},
		{
			name: "heuristic token limit exceeded",
			cfg:  config.RoutingConfig{Strategy: "auto", MaxLocalTokens: 100},
			req: engine.Request{
				Prompt: strings.Repeat("hello world ", 100), // ~300 tokens > 100
			},
			wantStrategy: router.StrategyCloudOnly,
		},
		{
			name:         "default auto",
			cfg:          config.RoutingConfig{Strategy: "auto", MaxLocalTokens: 4096},
			req:          engine.Request{Prompt: "simple question"},
			wantStrategy: router.StrategyAuto,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := router.NewRouter(engine.NewMockEngine("local"), engine.NewMockEngine("cloud"), tc.cfg)
			strategy, reason := r.DecideRoute(tc.req)
			if strategy != tc.wantStrategy {
				t.Errorf("got strategy %v, want %v (reason: %s)", strategy, tc.wantStrategy, reason)
			}
		})
	}
}

func TestRouter_Generate_LocalSuccess(t *testing.T) {
	local := engine.NewMockEngine("local-engine")
	cloud := engine.NewMockEngine("cloud-engine")

	cfg := config.RoutingConfig{Strategy: "auto", EscalateOnError: true, MaxLocalTokens: 4096}
	r := router.NewRouter(local, cloud, cfg)

	resp, err := r.Generate(context.Background(), engine.Request{Prompt: "explain memory management"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "local-engine") {
		t.Errorf("expected local engine response, got: %s", resp.Text)
	}
	if len(local.Calls) != 1 || len(cloud.Calls) != 0 {
		t.Errorf("expected 1 local call and 0 cloud calls, got %d local, %d cloud", len(local.Calls), len(cloud.Calls))
	}
}

func TestRouter_Generate_LocalFailure_CloudEscalation(t *testing.T) {
	local := engine.NewMockEngine("local-engine")
	local.GenerateFn = func(ctx context.Context, req engine.Request) (*engine.Response, error) {
		return nil, errors.New("daemon connection refused")
	}

	cloud := engine.NewMockEngine("cloud-engine")

	cfg := config.RoutingConfig{Strategy: "auto", EscalateOnError: true, MaxLocalTokens: 4096}
	r := router.NewRouter(local, cloud, cfg)

	resp, err := r.Generate(context.Background(), engine.Request{Prompt: "explain memory management"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "cloud-engine") {
		t.Errorf("expected cloud escalation response, got: %s", resp.Text)
	}
	if len(local.Calls) != 1 || len(cloud.Calls) != 1 {
		t.Errorf("expected 1 local call and 1 cloud escalation call, got %d local, %d cloud", len(local.Calls), len(cloud.Calls))
	}
}

func TestRouter_Generate_LocalOnly_NoEscalation(t *testing.T) {
	local := engine.NewMockEngine("local-engine")
	local.GenerateFn = func(ctx context.Context, req engine.Request) (*engine.Response, error) {
		return nil, errors.New("local model oom")
	}

	cloud := engine.NewMockEngine("cloud-engine")

	cfg := config.RoutingConfig{Strategy: "auto", EscalateOnError: true, MaxLocalTokens: 4096}
	r := router.NewRouter(local, cloud, cfg)

	_, err := r.Generate(context.Background(), engine.Request{Prompt: "do not escalate", ForceBackend: "local-only"})
	if err == nil {
		t.Fatal("expected error with local-only failure, got nil")
	}

	if len(cloud.Calls) != 0 {
		t.Errorf("cloud engine should NOT have been called on local-only, got %d calls", len(cloud.Calls))
	}
}

func TestRouter_GenerateStream_LocalSuccess(t *testing.T) {
	local := engine.NewMockEngine("local-engine")
	cloud := engine.NewMockEngine("cloud-engine")

	cfg := config.RoutingConfig{Strategy: "auto", EscalateOnError: true, MaxLocalTokens: 4096}
	r := router.NewRouter(local, cloud, cfg)

	stream, err := r.GenerateStream(context.Background(), engine.Request{Prompt: "hello stream"})
	if err != nil {
		t.Fatalf("GenerateStream failed: %v", err)
	}

	var sb strings.Builder
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		sb.WriteString(chunk.Text)
	}

	if !strings.Contains(sb.String(), "hello stream") {
		t.Errorf("unexpected streamed text: %s", sb.String())
	}
}

func TestRouter_GenerateStream_ImmediateFailure_CloudEscalation(t *testing.T) {
	local := engine.NewMockEngine("local-engine")
	local.GenerateStreamFn = func(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
		ch := make(chan engine.StreamChunk, 1)
		ch <- engine.StreamChunk{Error: errors.New("immediate stream crash")}
		close(ch)
		return ch, nil
	}

	cloud := engine.NewMockEngine("cloud-engine")

	cfg := config.RoutingConfig{Strategy: "auto", EscalateOnError: true, MaxLocalTokens: 4096}
	r := router.NewRouter(local, cloud, cfg)

	stream, err := r.GenerateStream(context.Background(), engine.Request{Prompt: "escalate this stream"})
	if err != nil {
		t.Fatalf("GenerateStream failed: %v", err)
	}

	var sb strings.Builder
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("unexpected chunk error: %v", chunk.Error)
		}
		sb.WriteString(chunk.Text)
	}

	if !strings.Contains(sb.String(), "escalate this stream") {
		t.Errorf("expected escalated stream from cloud, got: %s", sb.String())
	}
}

func TestRouter_Close(t *testing.T) {
	local := engine.NewMockEngine("local")
	cloud := engine.NewMockEngine("cloud")

	r := router.NewRouter(local, cloud, config.RoutingConfig{})
	if err := r.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !local.Closed || !cloud.Closed {
		t.Errorf("expected both engines closed: local=%v, cloud=%v", local.Closed, cloud.Closed)
	}
}

func TestRouter_ActiveEscalateSignal_Unary(t *testing.T) {
	local := engine.NewMockEngine("local")
	local.GenerateFn = func(ctx context.Context, req engine.Request) (*engine.Response, error) {
		return &engine.Response{Text: router.EscalateToCloudSignal + " task too complex"}, nil
	}

	cloud := engine.NewMockEngine("cloud-frontier")
	cloud.GenerateFn = func(ctx context.Context, req engine.Request) (*engine.Response, error) {
		return &engine.Response{Text: "cloud solution for complex task"}, nil
	}

	cfg := config.RoutingConfig{Strategy: "auto", EscalateOnError: true}
	r := router.NewRouter(local, cloud, cfg)

	resp, err := r.Generate(context.Background(), engine.Request{Prompt: "complex architecture"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "cloud solution") {
		t.Errorf("expected escalation to cloud on signal, got: %s", resp.Text)
	}
}

func TestRouter_ActiveEscalateSignal_Stream(t *testing.T) {
	local := engine.NewMockEngine("local")
	local.GenerateStreamFn = func(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
		ch := make(chan engine.StreamChunk, 2)
		ch <- engine.StreamChunk{Text: router.EscalateToCloudSignal}
		close(ch)
		return ch, nil
	}

	cloud := engine.NewMockEngine("cloud-frontier")
	cloud.GenerateStreamFn = func(ctx context.Context, req engine.Request) (<-chan engine.StreamChunk, error) {
		ch := make(chan engine.StreamChunk, 2)
		ch <- engine.StreamChunk{Text: "streamed cloud answer"}
		close(ch)
		return ch, nil
	}

	cfg := config.RoutingConfig{Strategy: "auto", EscalateOnError: true}
	r := router.NewRouter(local, cloud, cfg)

	stream, err := r.GenerateStream(context.Background(), engine.Request{Prompt: "complex stream"})
	if err != nil {
		t.Fatalf("GenerateStream failed: %v", err)
	}

	var sb strings.Builder
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("unexpected chunk error: %v", chunk.Error)
		}
		sb.WriteString(chunk.Text)
	}

	if !strings.Contains(sb.String(), "streamed cloud answer") {
		t.Errorf("expected escalated cloud stream on signal, got: %s", sb.String())
	}
}
