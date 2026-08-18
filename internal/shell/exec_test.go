package shell_test

import (
	"testing"

	"github.com/vladimirvivien/robo/internal/shell"
)

func TestExtractProposedCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "markdown bash block",
			input:    "Here is the command:\n```bash\nfind . -type f -name '*.go'\n```\nLet me know if you need anything else.",
			expected: "find . -type f -name '*.go'",
		},
		{
			name:     "markdown generic block",
			input:    "```\ngit status\n```",
			expected: "git status",
		},
		{
			name:     "single line with dollar prefix",
			input:    "$ docker compose up -d",
			expected: "docker compose up -d",
		},
		{
			name:     "single line command directly",
			input:    "kubectl get pods -n kube-system",
			expected: "kubectl get pods -n kube-system",
		},
		{
			name:     "pure explanatory text",
			input:    "Go channels provide synchronization across goroutines.",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			extracted := shell.ExtractProposedCommand(tc.input)
			if extracted != tc.expected {
				t.Errorf("got %q, want %q", extracted, tc.expected)
			}
		})
	}
}
