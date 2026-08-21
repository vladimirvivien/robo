package cmd_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/cmd"
	"github.com/vladimirvivien/robo/internal/config"
)

func TestStatusCmd_Registration(t *testing.T) {
	foundStatus := false
	for _, c := range cmd.RootCmd.Commands() {
		if c.Name() == "status" {
			foundStatus = true
			break
		}
	}
	if !foundStatus {
		t.Error("expected 'status' subcommand to be registered on RootCmd")
	}
}

func TestStatusCmd_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := config.NewDefaultConfig()
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("save config: %v", err)
	}

	var out bytes.Buffer
	cmd.RootCmd.SetOut(&out)
	cmd.RootCmd.SetErr(&out)
	cmd.RootCmd.SetArgs([]string{"status", "--config", cfgPath, "--json"})

	err := cmd.RootCmd.Execute()
	if err != nil {
		t.Fatalf("status --json failed: %v", err)
	}
}

func TestStatusCmd_MissingConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.yaml")

	var out bytes.Buffer
	cmd.RootCmd.SetOut(&out)
	cmd.RootCmd.SetErr(&out)
	cmd.RootCmd.SetArgs([]string{"status", "--config", cfgPath, "-o", "plain"})

	err := cmd.RootCmd.Execute()
	if err != nil {
		t.Fatalf("status execution failed: %v", err)
	}

	output := out.String()
	if strings.Contains(output, "exists: true") {
		t.Errorf("expected config exists to be false for nonexistent config, got: %s", output)
	}
}
