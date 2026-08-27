package skill

import (
	"fmt"
	"strings"
)

// Scope represents the discovery origin and priority of a skill.
type Scope string

const (
	// ScopeProject indicates a skill located in .robo/skills/ within the active workspace.
	ScopeProject Scope = "project"
	// ScopeGlobal indicates a skill located in ~/.robo/skills/ in the user's home directory.
	ScopeGlobal Scope = "global"
	// ScopeBuiltin indicates a skill bundled directly into the robo binary.
	ScopeBuiltin Scope = "builtin"
)

// TriggerConfig defines the conditions under which a skill is dynamically activated.
type TriggerConfig struct {
	Keywords []string `yaml:"keywords,omitempty" json:"keywords,omitempty"`
	Files    []string `yaml:"files,omitempty" json:"files,omitempty"`
}

// SkillMetadata encapsulates the YAML frontmatter declaration of a skill.
type SkillMetadata struct {
	Name         string        `yaml:"name" json:"name"`
	Description  string        `yaml:"description" json:"description"`
	Version      string        `yaml:"version,omitempty" json:"version,omitempty"`
	Triggers     TriggerConfig `yaml:"triggers,omitempty" json:"triggers,omitempty"`
	ToolsEnabled bool          `yaml:"tools_enabled,omitempty" json:"tools_enabled,omitempty"`
}

// Skill represents a fully parsed skill containing metadata and markdown instructions.
type Skill struct {
	SkillMetadata
	Body  string `json:"body"`
	Path  string `json:"path"`
	Scope Scope  `json:"scope"`
}

// FormatPromptInstructions formats the skill's instructions for inclusion in the model context envelope.
func (s *Skill) FormatPromptInstructions() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "### Skill: %s\n", s.Name)
	if s.Description != "" {
		fmt.Fprintf(&sb, "*Description:* %s\n\n", s.Description)
	}
	sb.WriteString(strings.TrimSpace(s.Body))
	return sb.String()
}

// FormatIndexEntry formats a single-line summary of the skill for the system prompt index.
func (s *Skill) FormatIndexEntry() string {
	return fmt.Sprintf("• %s: %s", s.Name, s.Description)
}
