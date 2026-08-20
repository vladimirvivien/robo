package shell

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// Context encapsulates the developer's active terminal and shell history state.
type Context struct {
	OS             string   `json:"os"`
	Arch           string   `json:"arch"`
	Shell          Type     `json:"shell"`
	Cwd            string   `json:"cwd"`
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

	// Gather recent shell history directly from history file (sub-millisecond)
	cmds, _ := c.historyReader.ReadLastCommands(maxHistory)
	if len(cmds) > 0 {
		sc.RecentCommands = cmds
		sc.LastCommand = cmds[len(cmds)-1]
	}

	return sc, nil
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
	if len(c.RecentCommands) > 0 {
		var filtered []string
		for _, cmd := range c.RecentCommands {
			clean := strings.TrimSpace(cmd)
			// Filter out self-referential robo invocations to avoid confusing SLMs
			lower := strings.ToLower(clean)
			if strings.HasPrefix(lower, "robo ") || lower == "robo" ||
				strings.HasPrefix(lower, ".\\robo") || strings.HasPrefix(lower, "./robo") ||
				strings.HasPrefix(lower, ".\\bin\\robo") || strings.HasPrefix(lower, "./bin/robo") {
				continue
			}
			filtered = append(filtered, clean)
		}
		if len(filtered) > 0 {
			sb.WriteString("Recent Shell History:\n")
			for i, cmd := range filtered {
				fmt.Fprintf(&sb, "  %d. %s\n", i+1, cmd)
			}
		}
	}
	return sb.String()
}
