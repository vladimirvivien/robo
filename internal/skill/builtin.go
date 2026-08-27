package skill

import (
	"embed"
	"io/fs"
	"path/filepath"
	"strings"
)

//go:embed builtin/*
var builtinFS embed.FS

// LoadBuiltinSkills loads all embedded built-in skills from the binary.
func LoadBuiltinSkills() ([]*Skill, error) {
	var skills []*Skill

	err := fs.WalkDir(builtinFS, "builtin", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(d.Name(), "SKILL.md") {
			return nil
		}

		data, err := builtinFS.ReadFile(path)
		if err != nil {
			return err
		}

		s, err := ParseSkillContent(data, path, ScopeBuiltin)
		if err != nil {
			return nil // Skip malformed embedded skills gracefully
		}

		// Fallback name if needed
		if s.Name == "" {
			dirName := filepath.Base(filepath.Dir(path))
			s.Name = dirName
		}

		skills = append(skills, s)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return skills, nil
}
