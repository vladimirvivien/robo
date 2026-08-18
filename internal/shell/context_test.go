package shell_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
