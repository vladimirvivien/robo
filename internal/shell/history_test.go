package shell_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vladimirvivien/robo/internal/shell"
)

func TestHistoryReader_ZshHistory(t *testing.T) {
	zshContent := `: 1629819230:0;git status
: 1629819235:0;docker build -t myapp .
ls -la
: 1629819240:0;cargo test --all
`
	hr := shell.NewHistoryReader(shell.ShellZsh)
	cmds, err := hr.ParseHistory(strings.NewReader(zshContent), 10)
	if err != nil {
		t.Fatalf("ParseHistory zsh failed: %v", err)
	}

	expected := []string{
		"git status",
		"docker build -t myapp .",
		"ls -la",
		"cargo test --all",
	}

	if len(cmds) != len(expected) {
		t.Fatalf("expected %d commands, got %d: %+v", len(expected), len(cmds), cmds)
	}

	for i, exp := range expected {
		if cmds[i] != exp {
			t.Errorf("cmd[%d] mismatch: got %q, want %q", i, cmds[i], exp)
		}
	}
}

func TestHistoryReader_BashHistory(t *testing.T) {
	bashContent := `#1629819230
git checkout main
#1629819235
git pull origin main
make build
`
	hr := shell.NewHistoryReader(shell.ShellBash)
	cmds, err := hr.ParseHistory(strings.NewReader(bashContent), 10)
	if err != nil {
		t.Fatalf("ParseHistory bash failed: %v", err)
	}

	expected := []string{
		"git checkout main",
		"git pull origin main",
		"make build",
	}

	if len(cmds) != len(expected) {
		t.Fatalf("expected %d commands, got %d: %+v", len(expected), len(cmds), cmds)
	}

	for i, exp := range expected {
		if cmds[i] != exp {
			t.Errorf("cmd[%d] mismatch: got %q, want %q", i, cmds[i], exp)
		}
	}
}

func TestHistoryReader_FishHistory(t *testing.T) {
	fishContent := `- cmd: npm install
  when: 1629819230
- cmd: npm run build
  when: 1629819235
`
	hr := shell.NewHistoryReader(shell.ShellFish)
	cmds, err := hr.ParseHistory(strings.NewReader(fishContent), 10)
	if err != nil {
		t.Fatalf("ParseHistory fish failed: %v", err)
	}

	expected := []string{
		"npm install",
		"npm run build",
	}

	if len(cmds) != len(expected) {
		t.Fatalf("expected %d commands, got %d: %+v", len(expected), len(cmds), cmds)
	}

	for i, exp := range expected {
		if cmds[i] != exp {
			t.Errorf("cmd[%d] mismatch: got %q, want %q", i, cmds[i], exp)
		}
	}
}

func TestHistoryReader_PowerShellHistory(t *testing.T) {
	pwshContent := `Get-Process | Select-Object -First 5
git log --oneline -n 3
pwsh ./build.ps1
`
	hr := shell.NewHistoryReader(shell.ShellPowerShell)
	cmds, err := hr.ParseHistory(strings.NewReader(pwshContent), 10)
	if err != nil {
		t.Fatalf("ParseHistory pwsh failed: %v", err)
	}

	expected := []string{
		"Get-Process | Select-Object -First 5",
		"git log --oneline -n 3",
		"pwsh ./build.ps1",
	}

	if len(cmds) != len(expected) {
		t.Fatalf("expected %d commands, got %d: %+v", len(expected), len(cmds), cmds)
	}

	for i, exp := range expected {
		if cmds[i] != exp {
			t.Errorf("cmd[%d] mismatch: got %q, want %q", i, cmds[i], exp)
		}
	}
}

func TestHistoryReader_LimitWindow(t *testing.T) {
	content := "cmd1\ncmd2\ncmd3\ncmd4\ncmd5\ncmd6\n"
	hr := shell.NewHistoryReader(shell.ShellBash)
	cmds, err := hr.ParseHistory(strings.NewReader(content), 3)
	if err != nil {
		t.Fatalf("ParseHistory failed: %v", err)
	}

	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	if cmds[0] != "cmd4" || cmds[1] != "cmd5" || cmds[2] != "cmd6" {
		t.Errorf("unexpected window: %+v", cmds)
	}
}

func TestHistoryReader_FileRead(t *testing.T) {
	dir := t.TempDir()
	histFile := filepath.Join(dir, "custom_history")
	if err := os.WriteFile(histFile, []byte("echo 1\necho 2\n"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	hr := shell.NewHistoryReader(shell.ShellBash).WithCustomPath(histFile)
	cmds, err := hr.ReadLastCommands(5)
	if err != nil {
		t.Fatalf("ReadLastCommands failed: %v", err)
	}

	if len(cmds) != 2 || cmds[0] != "echo 1" || cmds[1] != "echo 2" {
		t.Errorf("unexpected read file commands: %+v", cmds)
	}
}

func TestHistoryReader_AppendCommand(t *testing.T) {
	dir := t.TempDir()
	histFile := filepath.Join(dir, "custom_history")
	hr := shell.NewHistoryReader(shell.ShellBash).WithCustomPath(histFile)

	if err := hr.AppendCommand("git status"); err != nil {
		t.Fatalf("AppendCommand failed: %v", err)
	}
	if err := hr.AppendCommand("go build"); err != nil {
		t.Fatalf("AppendCommand failed: %v", err)
	}

	cmds, err := hr.ReadLastCommands(5)
	if err != nil {
		t.Fatalf("ReadLastCommands failed: %v", err)
	}

	if len(cmds) != 2 || cmds[0] != "git status" || cmds[1] != "go build" {
		t.Errorf("unexpected appended commands: %+v", cmds)
	}
}
