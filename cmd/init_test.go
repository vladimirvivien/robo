package cmd_test

import (
	"path/filepath"
	"testing"

	"github.com/vladimirvivien/robo/cmd"
	"github.com/vladimirvivien/robo/internal/config"
)

func TestInitCmd_Registration(t *testing.T) {
	foundInit := false
	for _, c := range cmd.RootCmd.Commands() {
		if c.Name() == "init" {
			foundInit = true
			break
		}
	}
	if !foundInit {
		t.Error("expected 'init' subcommand to be registered on RootCmd")
	}
}

func TestInitCmd_Flags(t *testing.T) {
	flags := []string{"version", "model", "backend", "non-interactive", "force"}
	for _, f := range flags {
		flag := cmd.InitCmd.Flags().Lookup(f)
		if flag == nil {
			t.Errorf("expected flag --%s on InitCmd", f)
		}
	}
}

func TestInitCmd_NonInteractive(t *testing.T) {
	dir := t.TempDir()
	targetConfig := filepath.Join(dir, "config.yaml")

	cmd.RootCmd.SetArgs([]string{
		"init",
		"--config", targetConfig,
		"--non-interactive",
		"--version", "v0.16.0",
		"--model", "litert-community/gemma-4-E4B-it-litert-lm",
		"--backend", "cpu",
		"--force",
	})

	cfg := config.NewDefaultConfig()
	cfg.SLM.Version = "v0.16.0"
	cfg.SLM.Model = "litert-community/gemma-4-E4B-it-litert-lm"
	cfg.SLM.Backend = "cpu"
	cfg.SLM.AutoDownload = false // Don't download large weights during unit test

	if err := cfg.Save(targetConfig); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := config.Load(targetConfig)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.SLM.Version != "v0.16.0" {
		t.Errorf("expected version 'v0.16.0', got %q", loaded.SLM.Version)
	}
	if loaded.SLM.Model != "litert-community/gemma-4-E4B-it-litert-lm" {
		t.Errorf("expected model 'litert-community/gemma-4-E4B-it-litert-lm', got %q", loaded.SLM.Model)
	}
	if loaded.SLM.Backend != "cpu" {
		t.Errorf("expected backend 'cpu', got %q", loaded.SLM.Backend)
	}
}
