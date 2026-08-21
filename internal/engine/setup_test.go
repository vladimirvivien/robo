package engine_test

import (
	"os"
	"path/filepath"
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

	t.Run("fails when local library directory is missing", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.LLM.Local.LibDir = "/nonexistent/lib/dir"
		cfg.LLM.Local.Model = "/nonexistent/model.bin"
		err := engine.ValidateInferenceSetup(cfg, "auto")
		if err == nil {
			t.Error("expected error when local library is missing, got nil")
		}
	})

	t.Run("fails when local model file is missing", func(t *testing.T) {
		dir := t.TempDir()
		libDir := filepath.Join(dir, "lib")
		_ = os.MkdirAll(libDir, 0755)
		_ = os.WriteFile(filepath.Join(libDir, "dummy.dll"), []byte("fake-lib"), 0644)

		cfg := config.NewDefaultConfig()
		cfg.LLM.Local.LibDir = libDir
		cfg.LLM.Local.Model = "/nonexistent/model.bin"
		err := engine.ValidateInferenceSetup(cfg, "auto")
		if err == nil {
			t.Error("expected error when local model is missing, got nil")
		}
	})

	t.Run("succeeds when local files exist", func(t *testing.T) {
		dir := t.TempDir()
		libDir := filepath.Join(dir, "lib")
		_ = os.MkdirAll(libDir, 0755)
		_ = os.WriteFile(filepath.Join(libDir, "dummy.dll"), []byte("fake-lib"), 0644)
		modelFile := filepath.Join(dir, "model.litertlm")
		_ = os.WriteFile(modelFile, []byte("fake-weights"), 0644)

		cfg := config.NewDefaultConfig()
		cfg.LLM.Local.LibDir = libDir
		cfg.LLM.Local.Model = modelFile
		err := engine.ValidateInferenceSetup(cfg, "auto")
		if err != nil {
			t.Errorf("expected success when local files exist, got %v", err)
		}
	})

	t.Run("cloud-only fails without API key even if local exists", func(t *testing.T) {
		dir := t.TempDir()
		libDir := filepath.Join(dir, "lib")
		_ = os.MkdirAll(libDir, 0755)
		_ = os.WriteFile(filepath.Join(libDir, "dummy.dll"), []byte("fake-lib"), 0644)
		modelFile := filepath.Join(dir, "model.litertlm")
		_ = os.WriteFile(modelFile, []byte("fake-weights"), 0644)

		cfg := config.NewDefaultConfig()
		cfg.LLM.Local.LibDir = libDir
		cfg.LLM.Local.Model = modelFile
		cfg.LLM.Cloud.APIKey = ""
		err := engine.ValidateInferenceSetup(cfg, "cloud-only")
		if err == nil {
			t.Error("expected error for cloud-only without API key, got nil")
		}
	})

	t.Run("cloud-only succeeds with API key and local files", func(t *testing.T) {
		dir := t.TempDir()
		libDir := filepath.Join(dir, "lib")
		_ = os.MkdirAll(libDir, 0755)
		_ = os.WriteFile(filepath.Join(libDir, "dummy.dll"), []byte("fake-lib"), 0644)
		modelFile := filepath.Join(dir, "model.litertlm")
		_ = os.WriteFile(modelFile, []byte("fake-weights"), 0644)

		cfg := config.NewDefaultConfig()
		cfg.LLM.Local.LibDir = libDir
		cfg.LLM.Local.Model = modelFile
		cfg.LLM.Cloud.APIKey = "valid-key"
		err := engine.ValidateInferenceSetup(cfg, "cloud-only")
		if err != nil {
			t.Errorf("expected success for cloud-only with API key, got %v", err)
		}
	})
}

func TestFindLocalModelPath_StrictRoboCache(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	_ = os.MkdirAll(cacheDir, 0755)

	modelFile := filepath.Join(cacheDir, "gemma-4-E2B-it.litertlm")
	_ = os.WriteFile(modelFile, []byte("fake-weights"), 0644)

	found := engine.FindLocalModelPath("gemma-4-e2b", cacheDir)
	if found != modelFile {
		t.Errorf("expected to find model at %s, got %s", modelFile, found)
	}

	notFound := engine.FindLocalModelPath("gemma-4-12b", cacheDir)
	if notFound != "" {
		t.Errorf("expected notFound to be empty, got %s", notFound)
	}
}

func TestIsModelDownloaded(t *testing.T) {
	if engine.IsModelDownloaded("") {
		t.Error("expected empty string not to be downloaded")
	}
	if engine.IsModelDownloaded("nonexistent/invalid/model/path/file.litertlm") {
		t.Error("expected invalid model not to be downloaded")
	}
}
