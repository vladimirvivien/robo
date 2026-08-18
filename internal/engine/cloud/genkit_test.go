package cloud_test

import (
	"testing"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/engine/cloud"
)

func TestCloudEngine_Interface(t *testing.T) {
	cfg := config.CloudConfig{
		Provider: "googleai",
		Model:    "googleai/gemini-2.5-flash",
	}

	e := cloud.New(cfg)

	// Verify implements engine.Engine interface
	var _ engine.Engine = e

	if e.Name() != "genkit" {
		t.Errorf("expected engine name 'genkit', got %q", e.Name())
	}
}

func TestCloudEngine_MissingAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	cfg := config.CloudConfig{
		Provider: "googleai",
		Model:    "googleai/gemini-2.5-flash",
	}

	e := cloud.New(cfg)
	_, err := e.Generate(t.Context(), engine.Request{Prompt: "hello"})
	if err == nil {
		t.Error("expected error when API key is missing, got nil")
	}
}
