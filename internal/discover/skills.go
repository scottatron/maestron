package discover

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

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
	return ListSkillsWithOptions(ListSkillsOptions{})
}

// ListSkillsOptions configures skill discovery behavior.
type ListSkillsOptions struct {
	// ScanWorkspace forces workspace scanning when not inside a git repo. The
	// scan starts at the current working directory in that case.
	ScanWorkspace bool
}

// ListSkillsWithOptions discovers all skills from global and workspace sources,
// using a content-hash cache to avoid re-parsing unchanged SKILL.md files.
// Workspace scanning starts from the git repo root when inside a repo. Outside
// a repo, only global paths are scanned unless ScanWorkspace is true.
func ListSkillsWithOptions(opts ListSkillsOptions) ([]SkillInfo, error) {
	home, err := platform.HomeDir()
	if err != nil {
		return nil, err
	}

	cache, _ := LoadCache(home) // treat load failure as empty cache

	var skills []SkillInfo

	// 1. Global paths (searched first)
	skills = append(skills, walkGlobalSkills(home, cache)...)

	// 2. Workspace paths
	root, err := workspaceSkillsRoot(opts)
	if err != nil {
		return nil, err
	}
	if root != "" {
		skills = append(skills, walkWorkspaceSkills(root, home, cache)...)
	}

	// Prune stale entries and persist cache (errors are non-fatal)
	cache.Prune()
	cache.Save(home) //nolint:errcheck

	managedDir := filepath.Join(home, ".agents", "skills")
	result := annotateManagedSync(skills, managedDir)

	return result, nil
}

// annotateManagedSync applies plugin deduplication and managed-skill annotations
// to a flat list of discovered skills. managedDir is the directory that holds
// managed skill copies (e.g. ~/.agents/skills).
//
// Skills whose SKILL.md lives directly under managedDir are marked ManagedRelationIs.
// Same-named skills at other paths are annotated:
//   - ManagedRelationMatches: full directory hash is identical to the managed copy
//   - ManagedRelationDiffers: full directory hash differs from the managed copy
//
// Skills with no managed counterpart use first-occurrence-wins per source.
func annotateManagedSync(skills []SkillInfo, managedDir string) []SkillInfo {
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

	managedByKey := map[string]string{} // managed dir key → full-dir hash of managed copy
	seenOther := map[string]bool{}      // source+managed-key dedup for non-managed skills
	seenPlugin := map[string]bool{}     // name+base dedup for plugin skills

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
		managedKey := skillDirKey(s.Path)

		if isManaged {
			// First managed copy of this install dir wins; record its directory hash.
			if _, exists := managedByKey[managedKey]; !exists {
				s.ManagedRelation = ManagedRelationIs
				h, _ := dirContentHash(filepath.Dir(s.Path))
				managedByKey[managedKey] = h
				result = append(result, s)
			}
			continue
		}

		// Non-managed: annotate with match status if a managed copy exists.
		if managedHash, ok := managedByKey[managedKey]; ok {
			skillHash, _ := dirContentHash(filepath.Dir(s.Path))
			if skillHash != "" && skillHash == managedHash {
				s.ManagedRelation = ManagedRelationMatches
			} else {
				s.ManagedRelation = ManagedRelationDiffers
			}
		}

		// Deduplicate per source+name; keep the annotated skill.
		key := s.Source + ":" + managedKey
		if !seenOther[key] {
			seenOther[key] = true
			result = append(result, s)
		}
	}

	return result
}

// dirContentHash computes a sha256 hash over all files in dir, sorted by
// relative path. VCS metadata directories are skipped to match the file set
// copied by InstallFromLocal.
func dirContentHash(dir string) (string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if vcsDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	h := sha256.New()
	for _, rel := range files {
		fmt.Fprintf(h, "%s\n", rel)
		data, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			return "", err
		}
		h.Write(data)
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

func workspaceSkillsRoot(opts ListSkillsOptions) (string, error) {
	if root, err := gitRepoRoot(); err != nil {
		return "", err
	} else if root != "" {
		return root, nil
	}
	if !opts.ScanWorkspace {
		return "", nil
	}
	return os.Getwd()
}

func gitRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", nil
	}
	return filepath.Clean(strings.TrimSpace(string(out))), nil
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
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error { //nolint:errcheck
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if vcsDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "SKILL.md" {
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

func skillDirKey(path string) string {
	return filepath.Base(filepath.Dir(path))
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
// vcsDir reports whether name is a version-control metadata directory to skip.
func vcsDir(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", ".bzr", "_darcs":
		return true
	}
	return false
}

func walkWorkspaceSkills(root, home string, cache *SkillCache) []SkillInfo {
	var skills []SkillInfo
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error { //nolint:errcheck
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if vcsDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() != "SKILL.md" {
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
