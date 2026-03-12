package discover

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/scottatron/maestron/internal/agents"
	"github.com/scottatron/maestron/internal/platform"
)

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// ListSkills discovers all skills from project and Claude native sources.
// Skills with the same name+source are deduplicated (first occurrence wins).
func ListSkills() ([]SkillInfo, error) {
	var skills []SkillInfo

	root, _, _ := agents.FindAgentsConfig()

	// Project-local skills (highest priority)
	if root != "" {
		for _, rel := range []string{".agents/skills", ".claude/skills", ".codex/skills", ".github/skills"} {
			src := rel[:strings.Index(rel, "/")]
			if s, err := discoverSkillsDir(filepath.Join(root, rel), src+"-project"); err == nil {
				skills = append(skills, s...)
			}
		}
	}

	// User-global skills
	if home, err := platform.HomeDir(); err == nil {
		for _, rel := range []string{".agents/skills", ".claude/skills", ".codex/skills", ".github/skills"} {
			src := rel[:strings.Index(rel, "/")]
			if s, err := discoverSkillsDir(filepath.Join(home, rel), src+"-global"); err == nil {
				skills = append(skills, s...)
			}
		}
	}

	// Claude native skills from plugins cache
	if claudeSkills, err := discoverClaudeNativeSkills(); err == nil {
		skills = append(skills, claudeSkills...)
	}

	// Deduplicate by name+source (keep first occurrence)
	seen := map[string]bool{}
	deduped := skills[:0]
	for _, s := range skills {
		key := s.Source + ":" + s.Name
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, s)
		}
	}

	return deduped, nil
}


func discoverSkillsDir(skillsDir, source string) ([]SkillInfo, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []SkillInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		skill, err := loadSkillFromPath(skillPath, source)
		if err != nil {
			continue
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

func discoverClaudeNativeSkills() ([]SkillInfo, error) {
	cacheDir, err := platform.ClaudePluginsCacheDir()
	if err != nil {
		return nil, err
	}

	var skills []SkillInfo

	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() || info.Name() != "SKILL.md" {
			return nil
		}
		// Accept if path contains a "skills" directory component
		inSkills := false
		for _, part := range strings.Split(filepath.Dir(path), string(filepath.Separator)) {
			if part == "skills" {
				inSkills = true
				break
			}
		}
		if !inSkills {
			return nil
		}

		skill, err := loadSkillFromPath(path, "claude-native")
		if err != nil {
			return nil
		}
		skills = append(skills, skill)
		return nil
	})

	return skills, err
}

func loadSkillFromPath(path, source string) (SkillInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SkillInfo{}, err
	}

	fm, err := parseSkillFrontmatter(data)
	if err != nil || fm == nil {
		dir := filepath.Base(filepath.Dir(path))
		return SkillInfo{
			Name:   dir,
			Source: source,
			Path:   path,
		}, nil
	}

	name := fm.Name
	if name == "" {
		name = filepath.Base(filepath.Dir(path))
	}

	return SkillInfo{
		Name:        name,
		Description: fm.Description,
		Source:      source,
		Path:        path,
	}, nil
}

func parseSkillFrontmatter(data []byte) (*skillFrontmatter, error) {
	if !bytes.HasPrefix(data, []byte("---")) {
		return &skillFrontmatter{}, nil
	}
	end := bytes.Index(data[3:], []byte("\n---"))
	if end < 0 {
		return &skillFrontmatter{}, nil
	}
	fmData := data[3 : end+3]
	var fm skillFrontmatter
	if err := yaml.Unmarshal(fmData, &fm); err != nil {
		return nil, err
	}
	return &fm, nil
}
