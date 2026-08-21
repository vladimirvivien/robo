package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/cmd"
)

func TestGetCmd_Registration(t *testing.T) {
	foundGet := false
	for _, c := range cmd.RootCmd.Commands() {
		if c.Name() == "get" {
			foundGet = true
			break
		}
	}
	if !foundGet {
		t.Error("expected 'get' subcommand to be registered on RootCmd")
	}
}

func TestGetCmd_Flags(t *testing.T) {
	flags := []string{"model", "litertlm-lib", "lib", "cache-dir", "no-ui", "quiet", "force"}
	for _, f := range flags {
		flag := cmd.GetCmd.Flags().Lookup(f)
		if flag == nil {
			t.Errorf("expected flag --%s on GetCmd", f)
		}
	}
}

func TestGetCmd_RequiresFlag(t *testing.T) {
	var out bytes.Buffer
	cmd.RootCmd.SetOut(&out)
	cmd.RootCmd.SetErr(&out)
	cmd.RootCmd.SetArgs([]string{"get"})

	err := cmd.RootCmd.Execute()
	if err == nil {
		t.Error("expected error when neither --model nor --litertlm-lib is specified")
	}
	if !strings.Contains(err.Error(), "please specify at least one asset") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetCmd_CachedModelDetection(t *testing.T) {
	dir := t.TempDir()

	var out bytes.Buffer
	cmd.RootCmd.SetOut(&out)
	cmd.RootCmd.SetErr(&out)
	cmd.RootCmd.SetArgs([]string{
		"get",
		"--model", "gemma-4-e2b",
		"--cache-dir", dir,
		"--no-ui",
	})

	// When pointing to an empty cache dir and dummy model without live internet or already cached,
	// error should appropriately reflect fetch or cache state without panic
	_ = cmd.RootCmd.Execute()
}
