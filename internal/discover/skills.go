package discover

import (
	"bytes"
	"crypto/sha256"
	"fmt"
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

// ListSkills discovers all skills from global and workspace sources,
// using a content-hash cache to avoid re-parsing unchanged SKILL.md files.
// Global paths are searched first; workspace paths follow.
func ListSkills() ([]SkillInfo, error) {
	home, err := platform.HomeDir()
	if err != nil {
		return nil, err
	}

	cache, _ := LoadCache(home) // treat load failure as empty cache

	var skills []SkillInfo

	// 1. Global paths (searched first)
	skills = append(skills, walkGlobalSkills(home, cache)...)

	// 2. Workspace paths
	root, _, _ := agents.FindAgentsConfig()
	if root != "" {
		skills = append(skills, walkWorkspaceSkills(root, home, cache)...)
	}

	// Prune stale entries and persist cache (errors are non-fatal)
	cache.Prune()
	cache.Save(home) //nolint:errcheck

	// Pass 1: find the highest version for each versioned plugin skill.
	type pluginKey struct{ name, base string }
	type bestVersion struct {
		ver    [3]int
		source string
	}
	pluginBest := map[pluginKey]bestVersion{}
	for _, s := range skills {
		base, ver, isPlugin := parsePluginSource(s.Source)
		if !isPlugin {
			continue
		}
		k := pluginKey{s.Name, base}
		if existing, ok := pluginBest[k]; !ok || compareVersion(ver, existing.ver) > 0 {
			pluginBest[k] = bestVersion{ver, s.Source}
		}
	}

	// Pass 2: annotate with managed-skill relationship and deduplicate.
	//
	// Skills under ~/.agents/skills are "managed" (ManagedRelationIs).
	// Same-named skills at other paths are kept and annotated:
	//   - ManagedRelationMatches: content hash is identical to the managed copy
	//   - ManagedRelationDiffers: content hash differs from the managed copy
	// Skills with no managed counterpart use first-occurrence-wins per source.
	managedDir := filepath.Join(home, ".agents", "skills")
	managedByName := map[string]SkillInfo{}  // name → managed SkillInfo
	seenOther := map[string]bool{}           // source+name dedup for non-managed skills
	seenPlugin := map[string]bool{}          // name+base dedup for plugin skills

	var result []SkillInfo

	for _, s := range skills {
		// Plugin version filter: skip older versions.
		base, _, isPlugin := parsePluginSource(s.Source)
		if isPlugin {
			best := pluginBest[pluginKey{s.Name, base}]
			if s.Source != best.source {
				continue
			}
			pk := s.Name + "\x00" + base
			if seenPlugin[pk] {
				continue
			}
			seenPlugin[pk] = true
		}

		isManaged := strings.HasPrefix(s.Path, managedDir+string(filepath.Separator))

		if isManaged {
			// First managed copy of this name wins; record it for comparison.
			if _, exists := managedByName[s.Name]; !exists {
				s.ManagedRelation = ManagedRelationIs
				managedByName[s.Name] = s
				result = append(result, s)
			}
			continue
		}

		// Non-managed: annotate with match status if a managed copy exists.
		if ms, ok := managedByName[s.Name]; ok {
			if s.ContentHash == ms.ContentHash {
				s.ManagedRelation = ManagedRelationMatches
			} else {
				s.ManagedRelation = ManagedRelationDiffers
			}
		}

		// Deduplicate per source+name; keep the annotated skill.
		key := s.Source + ":" + s.Name
		if !seenOther[key] {
			seenOther[key] = true
			result = append(result, s)
		}
	}

	return result, nil
}

// loadSkillCached reads a SKILL.md file, using the cache to skip frontmatter
// parsing when the content hash matches. The source label is always provided
// by the caller (never read from cache).
func loadSkillCached(path, source string, cache *SkillCache) (SkillInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SkillInfo{}, err
	}

	sum := sha256.Sum256(data)
	hash := fmt.Sprintf("sha256:%x", sum)

	if entry, ok := cache.Lookup(path, hash); ok {
		return SkillInfo{
			Name:        entry.Name,
			Description: entry.Description,
			Source:      source,
			Path:        path,
			ContentHash: hash,
		}, nil
	}

	// Cache miss: parse frontmatter
	fm, _ := parseSkillFrontmatter(data)
	name := filepath.Base(filepath.Dir(path))
	desc := ""
	if fm != nil {
		if fm.Name != "" {
			name = fm.Name
		}
		desc = fm.Description
	}

	cache.Set(path, CacheEntry{Hash: hash, Name: name, Description: desc})
	return SkillInfo{
		Name:        name,
		Description: desc,
		Source:      source,
		Path:        path,
		ContentHash: hash,
	}, nil
}

// walkSkillsDir recursively walks dir for SKILL.md files, assigning source
// as the source label for all skills found. Silently skips missing dirs.
func walkSkillsDir(dir, source string, cache *SkillCache) []SkillInfo {
	var skills []SkillInfo
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error { //nolint:errcheck
		if err != nil {
			return nil
		}
		if info.IsDir() || info.Name() != "SKILL.md" {
			return nil
		}
		skill, err := loadSkillCached(path, source, cache)
		if err == nil {
			skills = append(skills, skill)
		}
		return nil
	})
	return skills
}

// walkGlobalSkills discovers skills from all standard global paths and the
// Claude plugins cache. Global paths are searched in a fixed priority order.
func walkGlobalSkills(home string, cache *SkillCache) []SkillInfo {
	var skills []SkillInfo

	standardPaths := []string{
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".copilot", "skills"),
		filepath.Join(home, ".github", "skills"),
	}
	for _, p := range standardPaths {
		source := tildeSubst(p, home)
		skills = append(skills, walkSkillsDir(p, source, cache)...)
	}

	cacheRoot := filepath.Join(home, ".claude", "plugins", "cache")
	skills = append(skills, walkClaudePlugins(cacheRoot, home, cache)...)

	return skills
}

// walkClaudePlugins walks the Claude plugins cache directory and discovers
// SKILL.md files, using claudePluginLabel to derive human-readable source labels.
func walkClaudePlugins(cacheRoot, home string, cache *SkillCache) []SkillInfo {
	var skills []SkillInfo
	filepath.Walk(cacheRoot, func(path string, info os.FileInfo, err error) error { //nolint:errcheck
		if err != nil {
			return nil
		}
		if info.IsDir() || info.Name() != "SKILL.md" {
			return nil
		}
		ancestor := skillsAncestor(path, cacheRoot)
		if ancestor == "" {
			return nil
		}
		source := claudePluginLabel(ancestor, cacheRoot, home)
		skill, err := loadSkillCached(path, source, cache)
		if err == nil {
			skills = append(skills, skill)
		}
		return nil
	})
	return skills
}

// walkWorkspaceSkills recursively walks the workspace root for SKILL.md files.
// Each skill's source label is the tilde-substituted path of its nearest
// ancestor directory whose name contains "skills". Skills with no such ancestor
// are skipped.
func walkWorkspaceSkills(root, home string, cache *SkillCache) []SkillInfo {
	var skills []SkillInfo
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error { //nolint:errcheck
		if err != nil {
			return nil
		}
		if info.IsDir() || info.Name() != "SKILL.md" {
			return nil
		}
		ancestor := skillsAncestor(path, root)
		if ancestor == "" {
			return nil
		}
		source := tildeSubst(ancestor, home)
		skill, err := loadSkillCached(path, source, cache)
		if err == nil {
			skills = append(skills, skill)
		}
		return nil
	})
	return skills
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
