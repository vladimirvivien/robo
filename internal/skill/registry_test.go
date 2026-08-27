package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistry_DiscoveryAndPrecedence(t *testing.T) {
	tempWorkspace := t.TempDir()
	tempGlobal := t.TempDir()

	// 1. Create global skill "custom-skill"
	globalSkillDir := filepath.Join(tempGlobal, "custom-skill")
	if err := os.MkdirAll(globalSkillDir, 0755); err != nil {
		t.Fatal(err)
	}
	globalContent := `---
name: custom-skill
description: Global version of custom skill
triggers:
  keywords: ["custom"]
---
Global body instructions.
`
	if err := os.WriteFile(filepath.Join(globalSkillDir, "SKILL.md"), []byte(globalContent), 0600); err != nil {
		t.Fatal(err)
	}

	// 2. Create project skill with same name "custom-skill" (should override global)
	projectSkillDir := filepath.Join(tempWorkspace, ".robo", "skills", "custom-skill")
	if err := os.MkdirAll(projectSkillDir, 0755); err != nil {
		t.Fatal(err)
	}
	projectContent := `---
name: custom-skill
description: Project version of custom skill (Overridden)
triggers:
  keywords: ["custom"]
---
Project body instructions.
`
	if err := os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"), []byte(projectContent), 0600); err != nil {
		t.Fatal(err)
	}

	// 3. Create another project skill in .agents/skills/
	agentsSkillDir := filepath.Join(tempWorkspace, ".agents", "skills", "agents-skill")
	if err := os.MkdirAll(agentsSkillDir, 0755); err != nil {
		t.Fatal(err)
	}
	agentsContent := `---
name: agents-skill
description: Discovered in .agents/skills
triggers:
  keywords: ["agents"]
---
Agents body.
`
	if err := os.WriteFile(filepath.Join(agentsSkillDir, "SKILL.md"), []byte(agentsContent), 0600); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry(tempWorkspace, tempGlobal)
	if err := reg.Discover(); err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	// Verify project override
	s, ok := reg.Get("custom-skill")
	if !ok {
		t.Fatal("expected custom-skill to be found")
	}
	if s.Scope != ScopeProject {
		t.Errorf("expected ScopeProject override, got %s", s.Scope)
	}
	if !strings.Contains(s.Description, "Project version") {
		t.Errorf("expected project description, got: %s", s.Description)
	}

	// Verify .agents/skills discovery
	s2, ok := reg.Get("agents-skill")
	if !ok {
		t.Fatal("expected agents-skill to be found")
	}
	if s2.Scope != ScopeProject {
		t.Errorf("expected ScopeProject, got %s", s2.Scope)
	}

	// Verify builtin skills are also loaded
	builtinSkill, ok := reg.Get("git-commit")
	if !ok {
		t.Fatal("expected builtin git-commit skill to be found")
	}
	if builtinSkill.Scope != ScopeBuiltin {
		t.Errorf("expected ScopeBuiltin, got %s", builtinSkill.Scope)
	}

	// Verify Match
	matched := reg.Match("I need to write a git commit", nil)
	if len(matched) == 0 {
		t.Fatal("expected git-commit to match prompt")
	}

	// Verify prompt generation
	indexPrompt := reg.BuildIndexPrompt()
	if !strings.Contains(indexPrompt, "<available_skills>") || !strings.Contains(indexPrompt, "git-commit") {
		t.Errorf("invalid index prompt: %s", indexPrompt)
	}

	instructionsPrompt := reg.BuildInstructionsPrompt([]*Skill{s})
	if !strings.Contains(instructionsPrompt, "<active_skills>") || !strings.Contains(instructionsPrompt, "Project body instructions") {
		t.Errorf("invalid instructions prompt: %s", instructionsPrompt)
	}
}

func TestRegistry_RepositoryExampleSkills(t *testing.T) {
	// Find repo root skills directory relative to this package
	skillsDir := filepath.Join("..", "..", "skills")
	if _, err := os.Stat(skillsDir); err != nil {
		t.Skip("skipping repository skills directory test (not run from repo layout)")
	}

	reg := NewRegistry("", skillsDir)
	if err := reg.Discover(); err != nil {
		t.Fatalf("Discover from skills directory failed: %v", err)
	}

	expectedSkills := []string{
		"process-management",
		"network-triage",
		"docker-workloads",
		"docker-cleanup",
		"tool-installer",
		"disk-storage-triage",
		"git-workflow",
		"service-management",
		"log-analyzer",
		"security-permissions",
		"device-discovery",
	}

	for _, name := range expectedSkills {
		s, ok := reg.Get(name)
		if !ok {
			t.Errorf("expected skill '%s' to be discovered", name)
			continue
		}
		if s.Description == "" {
			t.Errorf("skill '%s' has empty description", name)
		}
		if s.Body == "" {
			t.Errorf("skill '%s' has empty body", name)
		}
	}
}

func TestRegistry_AllBuiltinSkills(t *testing.T) {
	reg := NewRegistry(t.TempDir(), t.TempDir())
	if err := reg.Discover(); err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	expectedBuiltins := []string{
		"git-commit",
		"sys-diagnostics",
		"process-management",
		"network-triage",
		"docker-workloads",
		"docker-cleanup",
		"tool-installer",
		"disk-storage-triage",
		"git-workflow",
		"service-management",
		"log-analyzer",
		"security-permissions",
		"device-discovery",
	}

	for _, name := range expectedBuiltins {
		s, ok := reg.Get(name)
		if !ok {
			t.Errorf("expected embedded builtin skill '%s' to be loaded", name)
			continue
		}
		if s.Scope != ScopeBuiltin {
			t.Errorf("expected skill '%s' to have ScopeBuiltin, got %s", name, s.Scope)
		}
		if s.Description == "" {
			t.Errorf("builtin skill '%s' has empty description", name)
		}
		if s.Body == "" {
			t.Errorf("builtin skill '%s' has empty body", name)
		}
	}
}
