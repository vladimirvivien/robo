package shell_test

import (
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/shell"
)

func TestBuildSystemPrompt_PowerShell(t *testing.T) {
	sc := &shell.Context{
		OS:             "windows",
		Arch:           "amd64",
		Shell:          shell.ShellPowerShell,
		Cwd:            `C:\Users\dev\project`,
		RecentCommands: []string{"git status", "go test ./..."},
	}

	prompt := shell.BuildSystemPrompt("windows", "amd64", shell.ShellPowerShell, "", sc)

	if !strings.Contains(prompt, "Target Environment: Windows PowerShell") {
		t.Errorf("expected PowerShell target environment, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Get-ChildItem") || !strings.Contains(prompt, "Select-String") {
		t.Errorf("expected PowerShell cmdlets in prompt, got:\n%s", prompt)
	}
	// Ensure POSIX rules are omitted
	if strings.Contains(prompt, "GNU utilities") || strings.Contains(prompt, "Target Environment: Linux") {
		t.Errorf("PowerShell prompt should not contain Linux GNU rules")
	}
	if !strings.Contains(prompt, "Subshell Isolation:") {
		t.Errorf("expected Subshell Isolation in prompt, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Context Resolution:") {
		t.Errorf("expected Context Resolution in prompt, got:\n%s", prompt)
	}
}

func TestBuildSystemPrompt_LinuxBash(t *testing.T) {
	sc := &shell.Context{
		OS:             "linux",
		Arch:           "amd64",
		Shell:          shell.ShellBash,
		Cwd:            "/home/dev/project",
		RecentCommands: []string{"ls -la", "docker ps"},
	}

	prompt := shell.BuildSystemPrompt("linux", "amd64", shell.ShellBash, "", sc)

	if !strings.Contains(prompt, "Target Environment: Linux POSIX") {
		t.Errorf("expected Linux POSIX target environment, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "GNU utilities") || !strings.Contains(prompt, "&&") {
		t.Errorf("expected GNU utilities and && chaining in prompt, got:\n%s", prompt)
	}
	// Ensure PowerShell rules are omitted
	if strings.Contains(prompt, "Get-ChildItem") || strings.Contains(prompt, "Windows PowerShell") {
		t.Errorf("Linux prompt should not contain PowerShell rules")
	}
}

func TestBuildSystemPrompt_DarwinZsh(t *testing.T) {
	sc := &shell.Context{
		OS:             "darwin",
		Arch:           "arm64",
		Shell:          shell.ShellZsh,
		Cwd:            "/Users/dev/project",
		RecentCommands: []string{"git diff"},
	}

	prompt := shell.BuildSystemPrompt("darwin", "arm64", shell.ShellZsh, "", sc)

	if !strings.Contains(prompt, "Target Environment: macOS POSIX (BSD)") {
		t.Errorf("expected macOS target environment, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "BSD") || !strings.Contains(prompt, "sed -i") {
		t.Errorf("expected BSD compatibility notes in macOS prompt, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "Get-ChildItem") {
		t.Errorf("macOS prompt should not contain PowerShell cmdlets")
	}
}

func TestBuildSystemPrompt_FishDropDown(t *testing.T) {
	sc := &shell.Context{
		OS:             "linux",
		Arch:           "amd64",
		Shell:          shell.ShellFish,
		Cwd:            "/home/dev/fishproject",
		RecentCommands: []string{"git status"},
	}

	prompt := shell.BuildSystemPrompt("linux", "amd64", shell.ShellFish, "", sc)

	// Fish should map to POSIX Bash compatibility
	if !strings.Contains(prompt, "Target Environment: POSIX Bash") {
		t.Errorf("expected POSIX Bash target environment for fish drop-down, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Bash compatibility layer for Fish") {
		t.Errorf("expected fish compatibility note, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "Get-ChildItem") {
		t.Errorf("Fish prompt should not contain PowerShell cmdlets")
	}
}

func TestBuildSystemPrompt_CustomInstructions(t *testing.T) {
	prompt := shell.BuildSystemPrompt("linux", "amd64", shell.ShellBash, "Always use verbose flags", nil)

	if !strings.Contains(prompt, "User Instructions:") || !strings.Contains(prompt, "Always use verbose flags") {
		t.Errorf("expected custom user instructions in prompt, got:\n%s", prompt)
	}
}
