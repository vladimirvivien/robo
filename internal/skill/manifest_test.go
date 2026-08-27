package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkillContent(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		check       func(t *testing.T, s *Skill)
	}{
		{
			name: "valid skill with keywords and files",
			content: `---
name: git-commit-helper
description: Generates conventional commits from staged diffs.
version: 1.2.0
triggers:
  keywords: ["commit", "diff"]
  files: [".git/*"]
tools_enabled: true
---

# Instructions

1. Check git status.
2. Format message as feat(scope): subject.
`,
			expectError: false,
			check: func(t *testing.T, s *Skill) {
				if s.Name != "git-commit-helper" {
					t.Errorf("expected name 'git-commit-helper', got '%s'", s.Name)
				}
				if s.Description != "Generates conventional commits from staged diffs." {
					t.Errorf("unexpected description: %s", s.Description)
				}
				if s.Version != "1.2.0" {
					t.Errorf("expected version 1.2.0, got %s", s.Version)
				}
				if len(s.Triggers.Keywords) != 2 {
					t.Errorf("expected 2 keywords, got %d", len(s.Triggers.Keywords))
				}
				if len(s.Triggers.Files) != 1 {
					t.Errorf("expected 1 file trigger, got %d", len(s.Triggers.Files))
				}
				if !s.ToolsEnabled {
					t.Errorf("expected tools_enabled true")
				}
				if s.Body == "" {
					t.Errorf("expected non-empty body")
				}
			},
		},
		{
			name: "missing leading frontmatter delimiter",
			content: `name: missing-delim
description: Foo
---
Body text
`,
			expectError: true,
		},
		{
			name: "missing closing frontmatter delimiter",
			content: `---
name: missing-closing
description: Foo
Body text without closing
`,
			expectError: true,
		},
		{
			name: "missing required name",
			content: `---
description: Only description
---
Body
`,
			expectError: true,
		},
		{
			name: "missing required description",
			content: `---
name: only-name
---
Body
`,
			expectError: true,
		},
		{
			name:        "utf-8 BOM prefix handled",
			content:     "\xef\xbb\xbf---\nname: bom-skill\ndescription: Has BOM\n---\n# Body",
			expectError: false,
			check: func(t *testing.T, s *Skill) {
				if s.Name != "bom-skill" {
					t.Errorf("expected name 'bom-skill', got '%s'", s.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ParseSkillContent([]byte(tt.content), "test/SKILL.md", ScopeProject)
			if (err != nil) != tt.expectError {
				t.Fatalf("expected error=%v, got err=%v", tt.expectError, err)
			}
			if err == nil && tt.check != nil {
				tt.check(t, s)
			}
		})
	}
}

func TestParseSkillFile(t *testing.T) {
	tempDir := t.TempDir()
	skillPath := filepath.Join(tempDir, "SKILL.md")

	content := `---
name: file-test-skill
description: Tests file parsing
---
Instructions here.
`
	if err := os.WriteFile(skillPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test skill file: %v", err)
	}

	s, err := ParseSkillFile(skillPath, ScopeGlobal)
	if err != nil {
		t.Fatalf("ParseSkillFile failed: %v", err)
	}
	if s.Name != "file-test-skill" {
		t.Errorf("expected name file-test-skill, got %s", s.Name)
	}
	if s.Scope != ScopeGlobal {
		t.Errorf("expected ScopeGlobal, got %s", s.Scope)
	}
}
