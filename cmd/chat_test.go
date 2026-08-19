package cmd_test

import (
	"testing"

	"github.com/vladimirvivien/robo/cmd"
)

func TestChatCmd_Registration(t *testing.T) {
	foundChat := false
	for _, c := range cmd.RootCmd.Commands() {
		if c.Name() == "chat" {
			foundChat = true
			break
		}
	}
	if !foundChat {
		t.Error("expected 'chat' subcommand to be registered on RootCmd")
	}
}

func TestChatCmd_Flags(t *testing.T) {
	flagSession := cmd.ChatCmd.Flag("session")
	if flagSession == nil {
		t.Fatalf("expected --session flag on ChatCmd")
	}
	if flagSession.Shorthand != "s" {
		t.Errorf("expected shorthand 's', got %q", flagSession.Shorthand)
	}

	flagLocal := cmd.ChatCmd.Flag("local-only")
	if flagLocal == nil {
		t.Fatalf("expected --local-only flag on ChatCmd")
	}
}
