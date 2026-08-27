package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestSkillListCmd(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	flagOutput = "plain"
	err := skillListCmd.RunE(skillListCmd, []string{})
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("skillListCmd failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "git-commit") {
		t.Errorf("expected git-commit skill in list output: %s", out)
	}
	if !strings.Contains(out, "sys-diagnostics") {
		t.Errorf("expected sys-diagnostics skill in list output: %s", out)
	}
}

func TestSkillShowCmd(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	flagOutput = "plain"
	err := skillShowCmd.RunE(skillShowCmd, []string{"git-commit"})
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("skillShowCmd failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "git-commit") {
		t.Errorf("expected name in output: %s", out)
	}
	if !strings.Contains(out, "Conventional Commit") {
		t.Errorf("expected Conventional Commit in output: %s", out)
	}
}

func TestSkillShowCmd_NotFound(t *testing.T) {
	err := skillShowCmd.RunE(skillShowCmd, []string{"non-existent-skill-xyz"})
	if err == nil {
		t.Fatal("expected error for non-existent skill, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error message: %v", err)
	}
}
