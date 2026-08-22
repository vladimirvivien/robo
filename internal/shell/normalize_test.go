package shell_test

import (
	"testing"

	"github.com/vladimirvivien/robo/internal/shell"
)

func TestNormalizeCommand_PowerShell(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "WS_MemoryUsage to WS",
			input:    "Get-Process -Name robo | Select-Object -ExpandProperty WS_MemoryUsage",
			expected: "Get-Process -Name robo | Select-Object -ExpandProperty WS",
		},
		{
			name:     "WorkingSet_Memory to WS",
			input:    "Get-Process | Sort-Object WorkingSet_Memory -Descending",
			expected: "Get-Process | Sort-Object WS -Descending",
		},
		{
			name:     "CPU_Usage to CPU",
			input:    "Get-Process | Sort-Object CPU_Usage -Descending",
			expected: "Get-Process | Sort-Object CPU -Descending",
		},
		{
			name:     "ExpandProperty PID to Id",
			input:    "Get-Process -Name robo | Select-Object -First 1 -ExpandProperty PID",
			expected: "Get-Process -Name robo | Select-Object -First 1 -ExpandProperty Id",
		},
		{
			name:     "ExpandProperty MemoryUsage to WS",
			input:    "Get-Process -Name robo | Select-Object -ExpandProperty MemoryUsage",
			expected: "Get-Process -Name robo | Select-Object -ExpandProperty WS",
		},
		{
			name:     "Process_Name to ProcessName",
			input:    "Get-Process | Select-Object Process_Name, Id",
			expected: "Get-Process | Select-Object ProcessName, Id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shell.NormalizeCommand("windows", shell.ShellPowerShell, tt.input)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNormalizeCommand_NonPowerShell(t *testing.T) {
	input := "ps aux | grep WS_MemoryUsage"
	got := shell.NormalizeCommand("linux", shell.ShellBash, input)
	if got != input {
		t.Errorf("expected no change on bash, got %q", got)
	}
}
