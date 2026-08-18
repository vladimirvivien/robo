package cmd_test

import (
	"testing"

	"github.com/vladimirvivien/robo/cmd"
)

func TestDoCmd_Registration(t *testing.T) {
	foundDo := false
	for _, c := range cmd.RootCmd.Commands() {
		if c.Name() == "do" {
			foundDo = true
			break
		}
	}
	if !foundDo {
		t.Error("expected 'do' subcommand to be registered on RootCmd")
	}
}

func TestDoCmd_Flags(t *testing.T) {
	flagAutoAccept := cmd.DoCmd.Flag("auto-accept")
	if flagAutoAccept == nil {
		t.Error("expected --auto-accept flag on DoCmd")
	}

	flagYolo := cmd.DoCmd.Flag("yolo-approve-all")
	if flagYolo == nil {
		t.Error("expected --yolo-approve-all flag on DoCmd")
	}
}
