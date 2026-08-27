package skill

import "testing"

func TestTriggerMatching(t *testing.T) {
	s := &Skill{
		SkillMetadata: SkillMetadata{
			Name: "git-commit-helper",
			Triggers: TriggerConfig{
				Keywords: []string{"commit", "git commit", "staged", "review diff"},
				Files:    []string{".git/*", ".gitignore", "pkg/*.go"},
			},
		},
	}

	promptTests := []struct {
		prompt      string
		expectMatch bool
	}{
		{"Please help me commit these changes", true},
		{"Write a git commit message", true},
		{"Inspect staged files", true},
		{"Can you review diff for me?", true},
		{"What is the current CPU usage?", false},
		{"Restart the docker container", false},
		{"", false},
	}

	for _, tt := range promptTests {
		t.Run("prompt: "+tt.prompt, func(t *testing.T) {
			matched := MatchesPrompt(s, tt.prompt)
			if matched != tt.expectMatch {
				t.Errorf("MatchesPrompt(%q) = %v, expected %v", tt.prompt, matched, tt.expectMatch)
			}
		})
	}

	fileTests := []struct {
		files       []string
		expectMatch bool
	}{
		{[]string{".git/config"}, true},
		{[]string{".gitignore"}, true},
		{[]string{"pkg/auth.go"}, true},
		{[]string{"cmd/root.go"}, false},
		{[]string{"README.md"}, false},
		{nil, false},
	}

	for _, tt := range fileTests {
		t.Run("files", func(t *testing.T) {
			matched := MatchesFiles(s, tt.files)
			if matched != tt.expectMatch {
				t.Errorf("MatchesFiles(%v) = %v, expected %v", tt.files, matched, tt.expectMatch)
			}
		})
	}
}
