package config_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/vladimirvivien/robo/internal/config"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := config.NewDefaultConfig()

	if !cfg.LLM.AutoRoute {
		t.Errorf("expected auto_route true, got %v", cfg.LLM.AutoRoute)
	}
	if !cfg.LLM.Local.Enabled {
		t.Errorf("expected local.enabled true, got %v", cfg.LLM.Local.Enabled)
	}
	if cfg.LLM.Local.Model != config.DefaultLocalModel {
		t.Errorf("expected local model %q, got %q", config.DefaultLocalModel, cfg.LLM.Local.Model)
	}
	if cfg.LLM.Local.Version != config.DefaultLocalVersion {
		t.Errorf("expected local version %q, got %q", config.DefaultLocalVersion, cfg.LLM.Local.Version)
	}
	if cfg.Shell.OutputMode != "markdown" {
		t.Errorf("expected output_mode 'markdown', got %q", cfg.Shell.OutputMode)
	}
	if cfg.Shell.MaxHistoryLines != 10 {
		t.Errorf("expected max_history_lines 10, got %d", cfg.Shell.MaxHistoryLines)
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
	cfg.LLM.AutoRoute = false
	cfg.LLM.Local.Enabled = true
	cfg.LLM.Local.Backend = "cpu"
	cfg.LLM.Local.Version = "v0.17.0"
	cfg.LLM.Cloud.BaseURL = "https://custom.api.com"
	cfg.Robod.Enabled = false
	cfg.Shell.OutputMode = "json"
	cfg.Shell.MaxHistoryLines = 15

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.LLM.AutoRoute != false {
		t.Errorf("expected auto_route false, got %v", loaded.LLM.AutoRoute)
	}
	if loaded.LLM.Local.Backend != "cpu" {
		t.Errorf("expected 'cpu', got %q", loaded.LLM.Local.Backend)
	}
	if loaded.LLM.Local.Version != "v0.17.0" {
		t.Errorf("expected 'v0.17.0', got %q", loaded.LLM.Local.Version)
	}
	if loaded.LLM.Cloud.BaseURL != "https://custom.api.com" {
		t.Errorf("expected 'https://custom.api.com', got %q", loaded.LLM.Cloud.BaseURL)
	}
	if loaded.Robod.Enabled != false {
		t.Errorf("expected Robod.Enabled false, got %v", loaded.Robod.Enabled)
	}
	if loaded.Shell.OutputMode != "json" {
		t.Errorf("expected output_mode 'json', got %q", loaded.Shell.OutputMode)
	}
	if loaded.Shell.MaxHistoryLines != 15 {
		t.Errorf("expected max_history_lines 15, got %d", loaded.Shell.MaxHistoryLines)
	}
}

func TestConfig_EnvOverrides(t *testing.T) {
	t.Setenv("ROBO_LLM_AUTO_ROUTE", "false")
	t.Setenv("ROBO_LOCAL_ENABLED", "true")
	t.Setenv("ROBO_LOCAL_BACKEND", "cpu")
	t.Setenv("ROBO_LOCAL_VERSION", "v0.18.0")
	t.Setenv("ROBO_CLOUD_BASE_URL", "https://env.api.com")
	t.Setenv("ROBO_ROBOD_ENABLED", "false")
	t.Setenv("ROBO_OUTPUT_MODE", "code")
	t.Setenv("ROBO_INPUT_PROMPT_PREFIX", "custom-prompt>")
	t.Setenv("ROBO_AUTO_ACCEPT", "true")
	t.Setenv("ROBO_YOLO_APPROVE_ALL", "1")
	t.Setenv("GEMINI_API_KEY", "test-key-123")

	dir := t.TempDir()
	nonExistent := filepath.Join(dir, "none.yaml")

	cfg, err := config.Load(nonExistent)
	if err != nil {
		t.Fatalf("Load non-existent failed: %v", err)
	}

	if cfg.LLM.AutoRoute != false {
		t.Errorf("expected env override auto_route false, got %v", cfg.LLM.AutoRoute)
	}
	if cfg.LLM.Local.Backend != "cpu" {
		t.Errorf("expected env override 'cpu', got %q", cfg.LLM.Local.Backend)
	}
	if cfg.LLM.Local.Version != "v0.18.0" {
		t.Errorf("expected env override 'v0.18.0', got %q", cfg.LLM.Local.Version)
	}
	if cfg.LLM.Cloud.BaseURL != "https://env.api.com" {
		t.Errorf("expected env override 'https://env.api.com', got %q", cfg.LLM.Cloud.BaseURL)
	}
	if cfg.Robod.Enabled != false {
		t.Errorf("expected env override Robod.Enabled false, got %v", cfg.Robod.Enabled)
	}
	if cfg.LLM.Cloud.APIKey != "test-key-123" {
		t.Errorf("expected APIKey 'test-key-123', got %q", cfg.LLM.Cloud.APIKey)
	}
	if cfg.Shell.OutputMode != "code" {
		t.Errorf("expected Shell.OutputMode 'code', got %q", cfg.Shell.OutputMode)
	}
	if cfg.Shell.InputPromptPrefix != "custom-prompt>" {
		t.Errorf("expected Shell.InputPromptPrefix 'custom-prompt>', got %q", cfg.Shell.InputPromptPrefix)
	}
	if !cfg.Shell.AutoAccept {
		t.Error("expected Shell.AutoAccept to be true via env")
	}
	if !cfg.Shell.YoloApproveAll {
		t.Error("expected Shell.YoloApproveAll to be true via env")
	}
}
