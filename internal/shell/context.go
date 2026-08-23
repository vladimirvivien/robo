package shell

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

var roboVersion = "dev"

// SetRoboVersion sets the application version string used in environment context prompts.
func SetRoboVersion(v string) {
	if strings.TrimSpace(v) != "" {
		roboVersion = v
	}
}

// DetectDistro returns a short distro identifier on Linux (from /etc/os-release) or standard OS name.
func DetectDistro() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			for line := range strings.SplitSeq(string(data), "\n") {
				if after, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
					val := after
					return strings.Trim(val, `"'`)
				}
			}
		}
		return "Linux"
	}
	if runtime.GOOS == "darwin" {
		return "macOS"
	}
	if runtime.GOOS == "windows" {
		return "Windows"
	}
	return runtime.GOOS
}

// Context encapsulates the developer's active terminal and shell history state.
type Context struct {
	OS             string           `json:"os"`
	Arch           string           `json:"arch"`
	Distro         string           `json:"distro,omitempty"`
	RoboVersion    string           `json:"robo_version,omitempty"`
	Shell          Type             `json:"shell"`
	Cwd            string           `json:"cwd"`
	RecentCommands []string         `json:"recent_commands,omitempty"`
	LastCommand    string           `json:"last_command,omitempty"`
	LastExecution  *ExecutionRecord `json:"last_execution,omitempty"`
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
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Distro:      DetectDistro(),
		RoboVersion: roboVersion,
		Shell:       DetectShell(),
		Cwd:         dir,
	}

	// Gather recent shell history directly from history file (sub-millisecond)
	cmds, _ := c.historyReader.ReadLastCommands(maxHistory)
	if len(cmds) > 0 {
		sc.RecentCommands = cmds
		sc.LastCommand = cmds[len(cmds)-1]
	}

	// Load last execution record from SQLite store if within recent threshold (< 24 hours)
	if rec, err := LoadLastExecution(dir); err == nil && rec != nil {
		if rec.Timestamp.IsZero() || time.Since(rec.Timestamp) < 24*time.Hour {
			sc.LastExecution = rec
		}
	}

	return sc, nil
}

// FormatPromptContext generates a concise, readable block for inclusion in LLM prompts.
func (c *Context) FormatPromptContext() string {
	var sb strings.Builder
	sb.WriteString("[Active Environment Context]\n")

	systemInfo := fmt.Sprintf("%s (%s/%s)", c.OS, c.OS, c.Arch)
	if c.Distro != "" && !strings.EqualFold(c.Distro, c.OS) {
		systemInfo = fmt.Sprintf("%s (%s/%s)", c.Distro, c.OS, c.Arch)
	}
	if c.RoboVersion != "" && c.RoboVersion != "none" {
		fmt.Fprintf(&sb, "System: %s • robo %s\n", systemInfo, c.RoboVersion)
	} else {
		fmt.Fprintf(&sb, "System: %s\n", systemInfo)
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

	if c.LastExecution != nil && c.LastExecution.Command != "" {
		sb.WriteString("\nLast Executed Action:\n")
		if c.LastExecution.Prompt != "" {
			fmt.Fprintf(&sb, "  User Intent: %s\n", c.LastExecution.Prompt)
		}
		fmt.Fprintf(&sb, "  Command: %s\n", c.LastExecution.Command)
		if c.LastExecution.ExitCode != 0 {
			fmt.Fprintf(&sb, "  Status: Failed (Exit Code %d)\n", c.LastExecution.ExitCode)
		} else {
			sb.WriteString("  Status: Succeeded (Exit Code 0)\n")
		}
		outText := strings.TrimSpace(c.LastExecution.Output)
		if outText == "" {
			outText = strings.TrimSpace(c.LastExecution.Error)
		}
		if outText != "" {
			lines := strings.Split(outText, "\n")
			if len(lines) > 5 {
				lines = lines[:5]
			}
			fmt.Fprintf(&sb, "  Output/Error: %s\n", strings.Join(lines, "\n    "))
		}
	}

	return sb.String()
}
