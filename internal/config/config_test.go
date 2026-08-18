package config_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/vladimirvivien/robo/internal/config"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := config.NewDefaultConfig()

	if cfg.Routing.Strategy != "auto" {
		t.Errorf("expected strategy 'auto', got %q", cfg.Routing.Strategy)
	}
	if cfg.Local.Model != config.DefaultLocalModel {
		t.Errorf("expected local model %q, got %q", config.DefaultLocalModel, cfg.Local.Model)
	}
	if cfg.Daemon.IdleTTL != 15*time.Minute {
		t.Errorf("expected IdleTTL 15m, got %v", cfg.Daemon.IdleTTL)
	}
	if cfg.Daemon.Port != 8765 {
		t.Errorf("expected port 8765, got %d", cfg.Daemon.Port)
	}
}

func TestConfig_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom-config.yaml")

	cfg := config.NewDefaultConfig()
	cfg.Routing.Strategy = "cloud-first"
	cfg.Local.Backend = "cpu"
	cfg.Daemon.Port = 9000

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Routing.Strategy != "cloud-first" {
		t.Errorf("expected 'cloud-first', got %q", loaded.Routing.Strategy)
	}
	if loaded.Local.Backend != "cpu" {
		t.Errorf("expected 'cpu', got %q", loaded.Local.Backend)
	}
	if loaded.Daemon.Port != 9000 {
		t.Errorf("expected port 9000, got %d", loaded.Daemon.Port)
	}
}

func TestConfig_EnvOverrides(t *testing.T) {
	t.Setenv("ROBO_ROUTING_STRATEGY", "local-only")
	t.Setenv("ROBO_LOCAL_BACKEND", "cpu")
	t.Setenv("GEMINI_API_KEY", "test-key-123")

	dir := t.TempDir()
	nonExistent := filepath.Join(dir, "none.yaml")

	cfg, err := config.Load(nonExistent)
	if err != nil {
		t.Fatalf("Load non-existent failed: %v", err)
	}

	if cfg.Routing.Strategy != "local-only" {
		t.Errorf("expected env override 'local-only', got %q", cfg.Routing.Strategy)
	}
	if cfg.Local.Backend != "cpu" {
		t.Errorf("expected env override 'cpu', got %q", cfg.Local.Backend)
	}
	if cfg.Cloud.APIKey != "test-key-123" {
		t.Errorf("expected APIKey 'test-key-123', got %q", cfg.Cloud.APIKey)
	}
}
