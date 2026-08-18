package shell

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// HistoryReader locates and parses shell history from the filesystem.
type HistoryReader struct {
	shellType  Type
	customPath string
}

// NewHistoryReader creates a HistoryReader for the given shell (or auto-detected).
func NewHistoryReader(shellType Type) *HistoryReader {
	if shellType == "" || shellType == ShellUnknown {
		shellType = DetectShell()
	}
	return &HistoryReader{shellType: shellType}
}

// WithCustomPath overrides the history file path (useful for testing or custom $HISTFILE).
func (hr *HistoryReader) WithCustomPath(path string) *HistoryReader {
	hr.customPath = path
	return hr
}

// DefaultHistoryPath returns the standard history file path for the active shell and OS.
func (hr *HistoryReader) DefaultHistoryPath() string {
	if hr.customPath != "" {
		return hr.customPath
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check explicit HISTFILE environment variable first
	if env := os.Getenv("HISTFILE"); env != "" {
		return env
	}

	switch hr.shellType {
	case ShellZsh:
		return filepath.Join(home, ".zsh_history")
	case ShellBash:
		return filepath.Join(home, ".bash_history")
	case ShellFish:
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome == "" {
			dataHome = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(dataHome, "fish", "fish_history")
	case ShellPowerShell:
		if runtime.GOOS == "windows" {
			appData := os.Getenv("APPDATA")
			if appData == "" {
				appData = filepath.Join(home, "AppData", "Roaming")
			}
			return filepath.Join(appData, "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt")
		}
		return filepath.Join(home, ".local", "share", "powershell", "PSReadLine", "ConsoleHost_history.txt")
	default:
		// Fallback check common files
		zsh := filepath.Join(home, ".zsh_history")
		if _, err := os.Stat(zsh); err == nil {
			return zsh
		}
		return filepath.Join(home, ".bash_history")
	}
}

// ReadLastCommands reads the most recent N commands from the shell history.
func (hr *HistoryReader) ReadLastCommands(limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}

	path := hr.DefaultHistoryPath()
	if path == "" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return hr.ParseHistory(f, limit)
}

// ParseHistory parses command lines from a history stream.
func (hr *HistoryReader) ParseHistory(r io.Reader, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}

	var rawLines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			rawLines = append(rawLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var commands []string
	switch hr.shellType {
	case ShellZsh:
		commands = parseZshHistory(rawLines)
	case ShellFish:
		commands = parseFishHistory(rawLines)
	case ShellBash:
		commands = parseBashHistory(rawLines)
	case ShellPowerShell:
		commands = parsePowerShellHistory(rawLines)
	default:
		commands = parseGenericHistory(rawLines)
	}

	// Return the last N commands
	if len(commands) > limit {
		commands = commands[len(commands)-limit:]
	}

	return commands, nil
}

func parseZshHistory(lines []string) []string {
	var cmds []string
	for _, line := range lines {
		// Check extended history format: ": <timestamp>:<elapsed>;<command>"
		if strings.HasPrefix(line, ": ") {
			if _, after, ok := strings.Cut(line, ";"); ok {
				cmd := strings.TrimSpace(after)
				if cmd != "" {
					cmds = append(cmds, cmd)
				}
				continue
			}
		}
		cmd := strings.TrimSpace(line)
		if cmd != "" {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

func parseBashHistory(lines []string) []string {
	var cmds []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check timestamp line "#1629819230"
		if strings.HasPrefix(trimmed, "#") {
			if _, err := strconv.ParseInt(trimmed[1:], 10, 64); err == nil {
				continue
			}
		}
		if trimmed != "" {
			cmds = append(cmds, trimmed)
		}
	}
	return cmds
}

func parseFishHistory(lines []string) []string {
	var cmds []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "- cmd:"); ok {
			cmd := strings.TrimSpace(after)
			if cmd != "" {
				cmds = append(cmds, cmd)
			}
		} else if after, ok := strings.CutPrefix(trimmed, "cmd:"); ok {
			cmd := strings.TrimSpace(after)
			if cmd != "" {
				cmds = append(cmds, cmd)
			}
		}
	}
	return cmds
}

func parsePowerShellHistory(lines []string) []string {
	var cmds []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cmds = append(cmds, trimmed)
		}
	}
	return cmds
}

func parseGenericHistory(lines []string) []string {
	var cmds []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			cmds = append(cmds, trimmed)
		}
	}
	return cmds
}
