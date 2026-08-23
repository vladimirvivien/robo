package config_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/vladimirvivien/robo/internal/config"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := config.NewDefaultConfig()

	if cfg.Robo.InferenceMode != "slm" {
		t.Errorf("expected inference_mode 'slm', got %q", cfg.Robo.InferenceMode)
	}
	if cfg.SLM.Model != config.DefaultLocalModel {
		t.Errorf("expected SLM model %q, got %q", config.DefaultLocalModel, cfg.SLM.Model)
	}
	if cfg.SLM.Version != config.DefaultLocalVersion {
		t.Errorf("expected SLM version %q, got %q", config.DefaultLocalVersion, cfg.SLM.Version)
	}
	if cfg.Robo.OutputMode != "markdown" {
		t.Errorf("expected output_mode 'markdown', got %q", cfg.Robo.OutputMode)
	}
	if cfg.Robo.MaxHistoryLines != 10 {
		t.Errorf("expected max_history_lines 10, got %d", cfg.Robo.MaxHistoryLines)
	}
	if cfg.Robod.IdleTTL != 15*time.Minute {
		t.Errorf("expected IdleTTL 15m, got %v", cfg.Robod.IdleTTL)
	}
	if !cfg.Robod.Enabled {
		t.Errorf("expected Robod.Enabled true, got %v", cfg.Robod.Enabled)
	}
}

func TestConfig_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom-config.yaml")

	cfg := config.NewDefaultConfig()
	cfg.Robo.InferenceMode = "llm"
	cfg.SLM.Backend = "cpu"
	cfg.SLM.Version = "v0.17.0"
	cfg.LLM.BaseURL = "https://custom.api.com"
	cfg.Robod.Enabled = false
	cfg.Robo.OutputMode = "json"
	cfg.Robo.MaxHistoryLines = 15

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Robo.InferenceMode != "llm" {
		t.Errorf("expected inference_mode 'llm', got %q", loaded.Robo.InferenceMode)
	}
	if loaded.SLM.Backend != "cpu" {
		t.Errorf("expected 'cpu', got %q", loaded.SLM.Backend)
	}
	if loaded.SLM.Version != "v0.17.0" {
		t.Errorf("expected 'v0.17.0', got %q", loaded.SLM.Version)
	}
	if loaded.LLM.BaseURL != "https://custom.api.com" {
		t.Errorf("expected 'https://custom.api.com', got %q", loaded.LLM.BaseURL)
	}
	if loaded.Robod.Enabled != false {
		t.Errorf("expected Robod.Enabled false, got %v", loaded.Robod.Enabled)
	}
	if loaded.Robo.OutputMode != "json" {
		t.Errorf("expected output_mode 'json', got %q", loaded.Robo.OutputMode)
	}
	if loaded.Robo.MaxHistoryLines != 15 {
		t.Errorf("expected max_history_lines 15, got %d", loaded.Robo.MaxHistoryLines)
	}
}

func TestConfig_EnvOverrides(t *testing.T) {
	t.Setenv("ROBO_INFERENCE_MODE", "llm")
	t.Setenv("ROBO_SLM_BACKEND", "cpu")
	t.Setenv("ROBO_SLM_VERSION", "v0.18.0")
	t.Setenv("ROBO_LLM_BASE_URL", "https://env.api.com")
	t.Setenv("ROBO_ROBOD_ENABLED", "false")
	t.Setenv("ROBO_OUTPUT_MODE", "code")
	t.Setenv("ROBO_AUTO_ACCEPT", "true")
	t.Setenv("ROBO_YOLO_APPROVE_ALL", "1")
	t.Setenv("GEMINI_API_KEY", "test-key-123")

	dir := t.TempDir()
	nonExistent := filepath.Join(dir, "none.yaml")

	cfg, err := config.Load(nonExistent)
	if err != nil {
		t.Fatalf("Load non-existent failed: %v", err)
	}

	if cfg.Robo.InferenceMode != "llm" {
		t.Errorf("expected env override inference_mode 'llm', got %q", cfg.Robo.InferenceMode)
	}
	if cfg.SLM.Backend != "cpu" {
		t.Errorf("expected env override 'cpu', got %q", cfg.SLM.Backend)
	}
	if cfg.SLM.Version != "v0.18.0" {
		t.Errorf("expected env override 'v0.18.0', got %q", cfg.SLM.Version)
	}
	if cfg.LLM.BaseURL != "https://env.api.com" {
		t.Errorf("expected env override 'https://env.api.com', got %q", cfg.LLM.BaseURL)
	}
	if cfg.Robod.Enabled != false {
		t.Errorf("expected env override Robod.Enabled false, got %v", cfg.Robod.Enabled)
	}
	if cfg.LLM.APIKey != "test-key-123" {
		t.Errorf("expected APIKey 'test-key-123', got %q", cfg.LLM.APIKey)
	}
	if cfg.Robo.OutputMode != "code" {
		t.Errorf("expected Robo.OutputMode 'code', got %q", cfg.Robo.OutputMode)
	}
	if !cfg.Robo.AutoAccept {
		t.Error("expected Robo.AutoAccept to be true via env")
	}
	if !cfg.Robo.YoloApproveAll {
		t.Error("expected Robo.YoloApproveAll to be true via env")
	}
}
