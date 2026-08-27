package skill

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseSkillFile reads and parses a SKILL.md file from disk.
func ParseSkillFile(filePath string, scope Scope) (*Skill, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read skill file: %w", err)
	}
	return ParseSkillContent(data, filePath, scope)
}

// ParseSkillContent parses raw SKILL.md bytes with YAML frontmatter.
func ParseSkillContent(data []byte, path string, scope Scope) (*Skill, error) {
	// Strip UTF-8 BOM if present
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	content := string(data)

	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return nil, fmt.Errorf("invalid skill format: missing leading '---' YAML frontmatter delimiter")
	}

	// Remove leading "---"
	afterStart := strings.TrimPrefix(trimmed, "---")
	idx := strings.Index(afterStart, "---")
	if idx == -1 {
		return nil, fmt.Errorf("invalid skill format: missing closing '---' YAML frontmatter delimiter")
	}

	frontmatterText := afterStart[:idx]
	bodyText := strings.TrimSpace(afterStart[idx+3:])

	var meta SkillMetadata
	if err := yaml.Unmarshal([]byte(frontmatterText), &meta); err != nil {
		return nil, fmt.Errorf("unmarshal skill frontmatter: %w", err)
	}

	meta.Name = strings.TrimSpace(meta.Name)
	meta.Description = strings.TrimSpace(meta.Description)

	if meta.Name == "" {
		return nil, fmt.Errorf("invalid skill metadata: 'name' is required")
	}
	if meta.Description == "" {
		return nil, fmt.Errorf("invalid skill metadata: 'description' is required")
	}

	return &Skill{
		SkillMetadata: meta,
		Body:          bodyText,
		Path:          path,
		Scope:         scope,
	}, nil
}
