package skill

import (
	"path/filepath"
	"strings"
)

// MatchesPrompt checks if any keyword in the skill's triggers appears in the prompt (case-insensitive).
func MatchesPrompt(skill *Skill, prompt string) bool {
	if skill == nil || len(skill.Triggers.Keywords) == 0 || strings.TrimSpace(prompt) == "" {
		return false
	}

	lowerPrompt := strings.ToLower(prompt)
	words := strings.Fields(lowerPrompt)

	for _, kw := range skill.Triggers.Keywords {
		kw = strings.TrimSpace(strings.ToLower(kw))
		if kw == "" {
			continue
		}

		// Multi-word keyword phrase match
		if strings.Contains(kw, " ") {
			if strings.Contains(lowerPrompt, kw) {
				return true
			}
			continue
		}

		// Single word exact match against tokens or substring match for punctuation-separated
		for _, w := range words {
			wClean := strings.Trim(w, `.,!?:;'"()[]{}<>/-_`)
			if wClean == kw || strings.Contains(lowerPrompt, kw) {
				return true
			}
		}
	}

	return false
}

// MatchesFiles checks if any file pattern in the skill's triggers matches any file in the provided list.
func MatchesFiles(skill *Skill, files []string) bool {
	if skill == nil || len(skill.Triggers.Files) == 0 || len(files) == 0 {
		return false
	}

	for _, pat := range skill.Triggers.Files {
		pat = strings.TrimSpace(pat)
		if pat == "" {
			continue
		}

		for _, f := range files {
			f = filepath.ToSlash(strings.TrimSpace(f))
			patSlash := filepath.ToSlash(pat)

			// Direct equality or glob match
			if f == patSlash {
				return true
			}
			matched, err := filepath.Match(patSlash, f)
			if err == nil && matched {
				return true
			}

			// Handle folder globs like .git/* or src/**
			if strings.HasSuffix(patSlash, "/*") {
				prefix := strings.TrimSuffix(patSlash, "/*")
				if strings.HasPrefix(f, prefix+"/") {
					return true
				}
			}
		}
	}

	return false
}

// Matches returns true if the skill matches either the prompt keywords or active files.
func Matches(skill *Skill, prompt string, files []string) bool {
	if MatchesPrompt(skill, prompt) {
		return true
	}
	if len(files) > 0 && MatchesFiles(skill, files) {
		return true
	}
	return false
}
