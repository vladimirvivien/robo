package shell_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vladimirvivien/robo/internal/shell"
)

func TestCollector_CollectAndFormat(t *testing.T) {
	dir := t.TempDir()
	histFile := filepath.Join(dir, ".zsh_history")
	histContent := `: 1629819230:0;git status
: 1629819235:0;go test ./...
`
	if err := os.WriteFile(histFile, []byte(histContent), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	hr := shell.NewHistoryReader(shell.ShellZsh).WithCustomPath(histFile)
	collector := shell.NewCollector(hr).WithWorkingDir(dir)

	ctx := context.Background()
	sc, err := collector.Collect(ctx, 5)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if sc.Cwd != dir {
		t.Errorf("expected cwd %q, got %q", dir, sc.Cwd)
	}

	if len(sc.RecentCommands) != 2 {
		t.Fatalf("expected 2 recent commands, got %d: %+v", len(sc.RecentCommands), sc.RecentCommands)
	}
	if sc.LastCommand != "go test ./..." {
		t.Errorf("expected last command 'go test ./...', got %q", sc.LastCommand)
	}

	formatted := sc.FormatPromptContext()
	if !strings.Contains(formatted, "[Active Environment Context]") {
		t.Errorf("missing header in formatted context: %s", formatted)
	}
	if !strings.Contains(formatted, "go test ./...") {
		t.Errorf("missing command in formatted context: %s", formatted)
	}
}

func TestCollector_FiltersSelfReferentialRoboCommands(t *testing.T) {
	dir := t.TempDir()
	histFile := filepath.Join(dir, "ConsoleHost_history.txt")
	histContent := "git status\nrobo \"what is my cpu\"\n.\\bin\\robo status\nkubectl get pods\n./robo get --model 2b\ndocker ps\n"
	if err := os.WriteFile(histFile, []byte(histContent), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	hr := shell.NewHistoryReader(shell.ShellPowerShell).WithCustomPath(histFile)
	collector := shell.NewCollector(hr).WithWorkingDir(dir)

	ctx := context.Background()
	sc, err := collector.Collect(ctx, 10)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	formatted := sc.FormatPromptContext()

	// Ensure non-robo commands are present
	for _, expected := range []string{"git status", "kubectl get pods", "docker ps"} {
		if !strings.Contains(formatted, expected) {
			t.Errorf("expected %q in formatted context, got:\n%s", expected, formatted)
		}
	}

	// Ensure robo commands are filtered out
	for _, filtered := range []string{"robo \"what is my cpu\"", ".\\bin\\robo status", "./robo get --model 2b"} {
		if strings.Contains(formatted, filtered) {
			t.Errorf("expected %q to be filtered out, got:\n%s", filtered, formatted)
		}
	}
}

func TestCollector_FormatsLastExecution(t *testing.T) {
	dir := t.TempDir()
	rec := shell.ExecutionRecord{
		Prompt:    "add .ssh/id_ed25519 to my ssh keychain",
		Command:   "ssh-add ~/.ssh/id_ed25519",
		Output:    "Could not open a connection to your authentication agent.",
		ExitCode:  2,
		Timestamp: time.Now(),
		Cwd:       dir,
	}
	_ = rec

	sc := &shell.Context{
		OS:    "linux",
		Arch:  "amd64",
		Shell: shell.ShellBash,
		Cwd:   dir,
		LastExecution: &shell.ExecutionRecord{
			Prompt:   "add .ssh/id_ed25519 to my ssh keychain",
			Command:  "ssh-add ~/.ssh/id_ed25519",
			Output:   "Could not open a connection to your authentication agent.",
			ExitCode: 2,
			Cwd:      dir,
		},
	}

	formatted := sc.FormatPromptContext()

	if !strings.Contains(formatted, "Last Executed Action:") {
		t.Errorf("expected 'Last Executed Action:' in prompt context, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "ssh-add ~/.ssh/id_ed25519") {
		t.Errorf("expected command in prompt context, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "Status: Failed (Exit Code 2)") {
		t.Errorf("expected failure exit code in prompt context, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "Could not open a connection to your authentication agent.") {
		t.Errorf("expected error output in prompt context, got:\n%s", formatted)
	}
}
