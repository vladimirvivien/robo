package engine_test

import (
	"testing"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
)

func TestCheckCloudSetup(t *testing.T) {
	t.Run("with API key", func(t *testing.T) {
		t.Setenv("GEMINI_API_KEY", "test-key-abc")
		cfg := config.CloudConfig{
			Provider:  "googleai",
			APIKeyEnv: "GEMINI_API_KEY",
		}
		status := engine.CheckCloudSetup(cfg)
		if !status.Configured {
			t.Error("expected cloud to be configured when GEMINI_API_KEY is set")
		}
	})

	t.Run("without API key", func(t *testing.T) {
		t.Setenv("GEMINI_API_KEY", "")
		t.Setenv("GOOGLE_API_KEY", "")
		t.Setenv("ANTHROPIC_API_KEY", "")
		t.Setenv("OPENAI_API_KEY", "")
		cfg := config.CloudConfig{
			Provider:  "googleai",
			APIKeyEnv: "GEMINI_API_KEY",
		}
		status := engine.CheckCloudSetup(cfg)
		if status.Configured {
			t.Error("expected cloud not to be configured when no API keys are present")
		}
	})
}

func TestValidateInferenceSetup(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	t.Run("cloud-only fails without API key", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.LLM.Cloud.APIKey = ""
		err := engine.ValidateInferenceSetup(cfg, "cloud-only")
		if err == nil {
			t.Error("expected error for cloud-only without API key, got nil")
		}
	})

	t.Run("cloud-only succeeds with API key", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.LLM.Cloud.APIKey = "valid-key"
		err := engine.ValidateInferenceSetup(cfg, "cloud-only")
		if err != nil {
			t.Errorf("expected success for cloud-only with API key, got %v", err)
		}
	})

	t.Run("local-only fails when missing and auto-download disabled", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.LLM.Local.AutoDownload = false
		cfg.LLM.Local.LibDir = "/nonexistent/lib/dir"
		cfg.LLM.Local.Model = "/nonexistent/model.bin"
		err := engine.ValidateInferenceSetup(cfg, "local-only")
		if err == nil {
			t.Error("expected error for local-only when missing and auto-download disabled, got nil")
		}
	})

	t.Run("auto succeeds when auto-download is enabled", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.LLM.Local.AutoDownload = true
		err := engine.ValidateInferenceSetup(cfg, "auto")
		if err != nil {
			t.Errorf("expected success for auto when auto-download is enabled, got %v", err)
		}
	})

	t.Run("auto fails when neither local nor cloud is available", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.LLM.Local.AutoDownload = false
		cfg.LLM.Local.LibDir = "/nonexistent/lib/dir"
		cfg.LLM.Local.Model = "/nonexistent/model.bin"
		cfg.LLM.Cloud.APIKey = ""
		cfg.LLM.Cloud.Enabled = false
		err := engine.ValidateInferenceSetup(cfg, "auto")
		if err == nil {
			t.Error("expected error when neither local nor cloud is available, got nil")
		}
	})
}

func TestIsModelDownloaded(t *testing.T) {
	if engine.IsModelDownloaded("") {
		t.Error("expected empty string not to be downloaded")
	}
	if engine.IsModelDownloaded("nonexistent/invalid/model/path/file.litertlm") {
		t.Error("expected invalid model not to be downloaded")
	}
}
