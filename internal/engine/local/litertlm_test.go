package local_test

import (
	"testing"

	"github.com/vladimirvivien/robo/internal/config"
	"github.com/vladimirvivien/robo/internal/engine"
	"github.com/vladimirvivien/robo/internal/engine/local"
)

func TestLocalEngine_Interface(t *testing.T) {
	cfg := config.SLMConfig{
		Model: "test-model",
	}

	e := local.New(cfg)

	// Verify implements engine.Engine interface
	var _ engine.Engine = e

	if e.Name() != "litertlm" {
		t.Errorf("expected engine name 'litertlm', got %q", e.Name())
	}
}
