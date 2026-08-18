package shell

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Context encapsulates the developer's active terminal, repository, and shell history state.
type Context struct {
	OS             string   `json:"os"`
	Arch           string   `json:"arch"`
	Shell          Type     `json:"shell"`
	Cwd            string   `json:"cwd"`
	GitBranch      string   `json:"git_branch,omitempty"`
	GitStatus      string   `json:"git_status,omitempty"`
	RecentCommands []string `json:"recent_commands,omitempty"`
	LastCommand    string   `json:"last_command,omitempty"`
}

// Collector inspects the runtime environment and gathers ambient context.
type Collector struct {
	historyReader *HistoryReader
	workingDir    string
}

// NewCollector creates a new context Collector.
func NewCollector(hr *HistoryReader) *Collector {
	if hr == nil {
		hr = NewHistoryReader(DetectShell())
	}
	return &Collector{
		historyReader: hr,
	}
}

// WithWorkingDir overrides the current directory (useful for testing).
func (c *Collector) WithWorkingDir(dir string) *Collector {
	c.workingDir = dir
	return c
}

// Collect gathers the complete ambient context.
func (c *Collector) Collect(ctx context.Context, maxHistory int) (*Context, error) {
	if maxHistory <= 0 {
		maxHistory = 5
	}

	dir := c.workingDir
	if dir == "" {
		cwd, err := os.Getwd()
		if err == nil {
			dir = cwd
		}
	}

	sc := &Context{
		OS:    runtime.GOOS,
		Arch:  runtime.GOARCH,
		Shell: DetectShell(),
		Cwd:   dir,
	}

	// 1. Gather recent shell history
	cmds, _ := c.historyReader.ReadLastCommands(maxHistory)
	if len(cmds) > 0 {
		sc.RecentCommands = cmds
		sc.LastCommand = cmds[len(cmds)-1]
	}

	// 2. Gather Git repository context (with 1-second timeout)
	gitCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	branch, status := collectGitInfo(gitCtx, dir)
	sc.GitBranch = branch
	sc.GitStatus = status

	return sc, nil
}

func collectGitInfo(ctx context.Context, dir string) (string, string) {
	// Check git branch
	cmdBranch := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if dir != "" {
		cmdBranch.Dir = dir
	}
	var outBranch bytes.Buffer
	cmdBranch.Stdout = &outBranch
	if err := cmdBranch.Run(); err != nil {
		return "", ""
	}
	branch := strings.TrimSpace(outBranch.String())

	// Check git status summary
	cmdStatus := exec.CommandContext(ctx, "git", "status", "--porcelain")
	if dir != "" {
		cmdStatus.Dir = dir
	}
	var outStatus bytes.Buffer
	cmdStatus.Stdout = &outStatus
	var statusSummary string
	if err := cmdStatus.Run(); err == nil {
		lines := strings.Split(strings.TrimSpace(outStatus.String()), "\n")
		modifiedCount := 0
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				modifiedCount++
			}
		}
		if modifiedCount > 0 {
			statusSummary = fmt.Sprintf("%d uncommitted change(s)", modifiedCount)
		} else {
			statusSummary = "clean"
		}
	}

	return branch, statusSummary
}

// FormatPromptContext generates a concise, readable block for inclusion in LLM prompts.
func (c *Context) FormatPromptContext() string {
	var sb strings.Builder
	sb.WriteString("[Active Environment Context]\n")
	if c.OS != "" {
		fmt.Fprintf(&sb, "OS/Architecture: %s (%s)\n", c.OS, c.Arch)
	}
	if c.Shell != "" && c.Shell != ShellUnknown {
		fmt.Fprintf(&sb, "Active Shell: %s\n", c.Shell)
	}
	if c.Cwd != "" {
		fmt.Fprintf(&sb, "Current Directory: %s\n", c.Cwd)
	}
	if c.GitBranch != "" {
		status := c.GitStatus
		if status == "" {
			status = "clean"
		}
		fmt.Fprintf(&sb, "Git Repository: %s (%s)\n", c.GitBranch, status)
	}
	if len(c.RecentCommands) > 0 {
		sb.WriteString("Recent Shell History:\n")
		for i, cmd := range c.RecentCommands {
			fmt.Fprintf(&sb, "  %d. %s\n", i+1, cmd)
		}
	}
	return sb.String()
}
