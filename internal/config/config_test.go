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
	if cfg.Local.LibVersion != config.DefaultLocalLibVersion {
		t.Errorf("expected local lib version %q, got %q", config.DefaultLocalLibVersion, cfg.Local.LibVersion)
	}
	if cfg.Robod.IdleTTL != 15*time.Minute {
		t.Errorf("expected IdleTTL 15m, got %v", cfg.Robod.IdleTTL)
	}
	if cfg.Robod.URL != "http://127.0.0.1:8765" {
		t.Errorf("expected URL http://127.0.0.1:8765, got %s", cfg.Robod.URL)
	}
}

func TestConfig_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom-config.yaml")

	cfg := config.NewDefaultConfig()
	cfg.Routing.Strategy = "cloud-first"
	cfg.Local.Backend = "cpu"
	cfg.Local.LibVersion = "v0.17.0"
	cfg.Cloud.BaseURL = "https://custom.api.com"
	cfg.Robod.URL = "http://remote-server:9000"

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
	if loaded.Local.LibVersion != "v0.17.0" {
		t.Errorf("expected 'v0.17.0', got %q", loaded.Local.LibVersion)
	}
	if loaded.Cloud.BaseURL != "https://custom.api.com" {
		t.Errorf("expected 'https://custom.api.com', got %q", loaded.Cloud.BaseURL)
	}
	if loaded.Robod.URL != "http://remote-server:9000" {
		t.Errorf("expected URL 'http://remote-server:9000', got %s", loaded.Robod.URL)
	}
}

func TestConfig_EnvOverrides(t *testing.T) {
	t.Setenv("ROBO_ROUTING_STRATEGY", "local-only")
	t.Setenv("ROBO_LOCAL_BACKEND", "cpu")
	t.Setenv("ROBO_LOCAL_LIB_VERSION", "v0.18.0")
	t.Setenv("ROBO_CLOUD_BASE_URL", "https://env.api.com")
	t.Setenv("ROBO_ROBOD_URL", "http://env-robod:8888")
	t.Setenv("ROBO_ROBOD_TOKEN", "token-env-999")
	t.Setenv("ROBO_AUTO_ACCEPT", "true")
	t.Setenv("ROBO_YOLO_APPROVE_ALL", "1")
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
	if cfg.Local.LibVersion != "v0.18.0" {
		t.Errorf("expected env override 'v0.18.0', got %q", cfg.Local.LibVersion)
	}
	if cfg.Cloud.BaseURL != "https://env.api.com" {
		t.Errorf("expected env override 'https://env.api.com', got %q", cfg.Cloud.BaseURL)
	}
	if cfg.Robod.URL != "http://env-robod:8888" {
		t.Errorf("expected env override 'http://env-robod:8888', got %q", cfg.Robod.URL)
	}
	if cfg.Robod.AuthToken != "token-env-999" {
		t.Errorf("expected env override 'token-env-999', got %q", cfg.Robod.AuthToken)
	}
	if cfg.Cloud.APIKey != "test-key-123" {
		t.Errorf("expected APIKey 'test-key-123', got %q", cfg.Cloud.APIKey)
	}
	if !cfg.Shell.AutoAccept {
		t.Error("expected Shell.AutoAccept to be true via env")
	}
	if !cfg.Shell.YoloApproveAll {
		t.Error("expected Shell.YoloApproveAll to be true via env")
	}
}
