package daemon_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/daemon"
	"github.com/vladimirvivien/robo/internal/engine"
)

func TestClient_DirectAndStream(t *testing.T) {
	mockEngine := engine.NewMockEngine("local-test")

	dir := t.TempDir()
	statePath := filepath.Join(dir, "daemon.json")

	token := "auth-tok-xyz"
	server, err := daemon.NewServer(mockEngine, daemon.ServerOptions{
		Port:      0,
		AuthToken: token,
		ModelName: "test-model",
		StatePath: statePath,
		IdleTTL:   5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	if err := server.Listen(0); err != nil {
		t.Fatalf("Listen: %v", err)
	}

	ctx := t.Context()

	go func() { _ = server.Serve(ctx) }()

	cfg := *config.NewDefaultConfig()
	cfg.Daemon.Enabled = true

	client := daemon.NewClient(cfg,
		daemon.WithStatePath(statePath),
		daemon.WithLauncher(func() error { return nil }),
	)

	// 1. Test Generate
	resp, err := client.Generate(ctx, engine.Request{Prompt: "hello from client"})
	if err != nil {
		t.Fatalf("client.Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "hello from client") {
		t.Errorf("unexpected response: %s", resp.Text)
	}

	// 2. Test GenerateStream
	streamCh, err := client.GenerateStream(ctx, engine.Request{Prompt: "stream word sequence"})
	if err != nil {
		t.Fatalf("client.GenerateStream failed: %v", err)
	}

	var gathered []string
	for chunk := range streamCh {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		if chunk.Text != "" {
			gathered = append(gathered, chunk.Text)
		}
	}

	joined := strings.Join(gathered, "")
	if !strings.Contains(joined, "stream") || !strings.Contains(joined, "word") {
		t.Errorf("streamed text mismatch: %q", joined)
	}

	_ = server.Shutdown(ctx)
}

func TestClient_InProcessFallbackWhenDaemonDown(t *testing.T) {
	fallbackEngine := engine.NewMockEngine("in-proc-fallback")

	dir := t.TempDir()
	statePath := filepath.Join(dir, "nonexistent-daemon.json")

	cfg := *config.NewDefaultConfig()
	cfg.Daemon.Enabled = false // disable daemon to force immediate fallback

	client := daemon.NewClient(cfg,
		daemon.WithStatePath(statePath),
		daemon.WithInProcEngine(fallbackEngine),
		daemon.WithLauncher(func() error { return nil }),
	)

	ctx := context.Background()

	// 1. Unary fallback
	resp, err := client.Generate(ctx, engine.Request{Prompt: "test fallback"})
	if err != nil {
		t.Fatalf("fallback Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "in-proc-fallback") {
		t.Errorf("expected response from fallback engine, got: %s", resp.Text)
	}

	// 2. Streaming fallback
	streamCh, err := client.GenerateStream(ctx, engine.Request{Prompt: "test streaming fallback"})
	if err != nil {
		t.Fatalf("fallback GenerateStream failed: %v", err)
	}

	var gathered []string
	for chunk := range streamCh {
		if chunk.Error != nil {
			t.Fatalf("fallback chunk error: %v", chunk.Error)
		}
		if chunk.Text != "" {
			gathered = append(gathered, chunk.Text)
		}
	}

	joined := strings.Join(gathered, "")
	if !strings.Contains(joined, "streaming") {
		t.Errorf("expected streamed response from fallback engine, got: %s", joined)
	}
}
