package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Registry manages the discovery, precedence, and lookup of skills.
type Registry struct {
	mu           sync.RWMutex
	workspaceDir string
	globalDir    string
	skills       map[string]*Skill
}

// NewRegistry creates a new skill registry with designated workspace and global root paths.
func NewRegistry(workspaceDir, globalDir string) *Registry {
	if workspaceDir == "" {
		workspaceDir, _ = os.Getwd()
	}
	if globalDir == "" {
		home, _ := os.UserHomeDir()
		globalDir = filepath.Join(home, ".robo", "skills")
	}

	return &Registry{
		workspaceDir: workspaceDir,
		globalDir:    globalDir,
		skills:       make(map[string]*Skill),
	}
}

// Discover scans built-in, global, and workspace tiers, resolving precedence order.
func (r *Registry) Discover() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills = make(map[string]*Skill)

	// 1. Tier 3: Built-in Embedded Skills (Lowest priority)
	builtins, _ := LoadBuiltinSkills()
	for _, s := range builtins {
		r.skills[strings.ToLower(s.Name)] = s
	}

	// 2. Tier 2: Global User Skills (~/.robo/skills/)
	if r.globalDir != "" {
		_ = r.scanDirectory(r.globalDir, ScopeGlobal)
	}

	// 3. Tier 1: Project / Workspace Skills (Highest priority)
	if r.workspaceDir != "" {
		projectPaths := []string{
			filepath.Join(r.workspaceDir, ".robo", "skills"),
			filepath.Join(r.workspaceDir, ".agents", "skills"),
			filepath.Join(r.workspaceDir, ".agent", "skills"),
		}
		for _, p := range projectPaths {
			_ = r.scanDirectory(p, ScopeProject)
		}
	}

	return nil
}

// scanDirectory recursively inspects a directory for subdirectories containing SKILL.md.
func (r *Registry) scanDirectory(rootDir string, scope Scope) error {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillFile := filepath.Join(rootDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			continue
		}

		s, err := ParseSkillFile(skillFile, scope)
		if err != nil {
			continue // Skip malformed skills gracefully
		}

		// Precedence: Project overrides Global, Global overrides Builtin
		r.skills[strings.ToLower(s.Name)] = s
	}

	return nil
}

// Get retrieves a skill by its slug name (case-insensitive).
func (r *Registry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.skills[strings.ToLower(strings.TrimSpace(name))]
	return s, ok
}

// List returns all discovered skills sorted alphabetically by name.
func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		list = append(list, s)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	return list
}

// Match evaluates all registered skills against a user prompt and optional active file list.
func (r *Registry) Match(prompt string, modifiedFiles []string) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []*Skill
	for _, s := range r.List() {
		if Matches(s, prompt, modifiedFiles) {
			matched = append(matched, s)
		}
	}

	return matched
}

// BuildIndexPrompt renders the concise <available_skills> XML block for the system prompt.
func (r *Registry) BuildIndexPrompt() string {
	skills := r.List()
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	sb.WriteString("The following domain skills are available. Follow their operating instructions when relevant:\n")
	for _, s := range skills {
		fmt.Fprintf(&sb, "• %s: %s\n", s.Name, s.Description)
	}
	sb.WriteString("</available_skills>\n")

	return sb.String()
}

// BuildInstructionsPrompt renders the full instructions block for active skills.
func (r *Registry) BuildInstructionsPrompt(activeSkills []*Skill) string {
	if len(activeSkills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<active_skills>\n")
	for i, s := range activeSkills {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		sb.WriteString(s.FormatPromptInstructions())
		sb.WriteString("\n")
	}
	sb.WriteString("</active_skills>\n")

	return sb.String()
}
