package cmd_test

import (
	"bytes"
	"testing"

	"github.com/vladimirvivien/robo/cmd"
)

func TestRootCmd_Help(t *testing.T) {
	var out bytes.Buffer
	cmd.RootCmd.SetOut(&out)
	cmd.RootCmd.SetErr(&out)
	cmd.RootCmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("RootCmd help returned error: %v", err)
	}

	helpOutput := out.String()
	if !bytes.Contains([]byte(helpOutput), []byte("robo")) {
		t.Errorf("expected help output to contain 'robo', got: %s", helpOutput)
	}
}

func TestRootCmd_Flags(t *testing.T) {
	flags := []string{"yolo", "one-shot", "dry-run", "max-steps", "system", "output", "config"}
	for _, f := range flags {
		flag := cmd.RootCmd.Flags().Lookup(f)
		if flag == nil {
			flag = cmd.RootCmd.PersistentFlags().Lookup(f)
		}
		if flag == nil {
			t.Errorf("expected flag --%s on RootCmd", f)
		}
	}
}

func TestDaemonCmd_Registration(t *testing.T) {
	foundDaemon := false
	for _, c := range cmd.RootCmd.Commands() {
		if c.Name() == "daemon" {
			foundDaemon = true
			break
		}
	}
	if !foundDaemon {
		t.Error("expected daemon subcommand to be registered on RootCmd")
	}
}
