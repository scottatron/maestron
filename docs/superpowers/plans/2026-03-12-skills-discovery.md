# Skills Discovery Redesign Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Track implementation work in `bd` issues instead of markdown task lists.
> The checkbox-formatted steps below are a sequencing aid in the document, not an approved tracking mechanism for live work.

**Goal:** Replace hardcoded fixed-depth skills discovery with recursive walks, path-based source labels, and a sha256 content cache.

**Architecture:** Two discovery phases — global paths first (`~/{.agents,.claude,.codex,.copilot,.github}/skills` + Claude plugins cache), then workspace (recursive from root, grouped by nearest `*skills*` ancestor). A JSON cache at `~/.agents/maestron/skills.json` avoids re-parsing unchanged SKILL.md frontmatter using sha256 hashes.

**Tech Stack:** Go stdlib (`crypto/sha256`, `encoding/json`, `path/filepath`, `os`), existing `gopkg.in/yaml.v3` for frontmatter, existing `internal/platform` for home dir.

---

## Chunk 1: Cache infrastructure and helper functions

### Task 1: Helper functions — `tildeSubst`, `skillsAncestor`, `claudePluginLabel`

**Files:**
- Create: `internal/discover/skills_helpers.go`
- Create: `internal/discover/skills_helpers_test.go`

These are pure functions with no side effects, testable in isolation.

- [ ] **Step 1: Create the test file with failing tests**

```go
// internal/discover/skills_helpers_test.go
package discover

import (
	"path/filepath"
	"testing"
)

func TestTildeSubst(t *testing.T) {
	home := "/Users/scott"
	tests := []struct {
		path string
		want string
	}{
		{"/Users/scott/.claude/skills", "~/.claude/skills"},
		{"/Users/scott/.claude/skills/foo/SKILL.md", "~/.claude/skills/foo/SKILL.md"},
		{"/Users/scott", "~"},
		{"/Users/other/.claude/skills", "/Users/other/.claude/skills"},
		{"/tmp/somewhere", "/tmp/somewhere"},
		{"", ""},
	}
	for _, tt := range tests {
		got := tildeSubst(tt.path, home)
		if got != tt.want {
			t.Errorf("tildeSubst(%q, %q) = %q, want %q", tt.path, home, got, tt.want)
		}
	}
}

func TestSkillsAncestor(t *testing.T) {
	root := "/workspace"
	tests := []struct {
		filePath string
		want     string
	}{
		// Standard: .codex/skills/foo/SKILL.md -> .codex/skills
		{"/workspace/.codex/skills/foo/SKILL.md", "/workspace/.codex/skills"},
		// Nested category: .codex/skills/category/foo/SKILL.md -> .codex/skills
		{"/workspace/.codex/skills/category/foo/SKILL.md", "/workspace/.codex/skills"},
		// Root-level skills dir: skills/bar/SKILL.md -> skills
		{"/workspace/skills/bar/SKILL.md", "/workspace/skills"},
		// Custom name: agent-skills/baz/SKILL.md -> agent-skills
		{"/workspace/agent-skills/baz/SKILL.md", "/workspace/agent-skills"},
		// No skills ancestor: src/foo/SKILL.md -> ""
		{"/workspace/src/foo/SKILL.md", ""},
		// Boundary: directly in workspace root -> ""
		{"/workspace/SKILL.md", ""},
	}
	for _, tt := range tests {
		got := skillsAncestor(tt.filePath, root)
		if got != tt.want {
			t.Errorf("skillsAncestor(%q, %q) = %q, want %q", tt.filePath, root, got, tt.want)
		}
	}
}

func TestClaudePluginLabel(t *testing.T) {
	home := "/Users/scott"
	cacheRoot := "/Users/scott/.claude/plugins/cache"
	tests := []struct {
		skillsDir string
		want      string
	}{
		// Standard: <registry>/<plugin>/<version>/skills
		{
			filepath.Join(cacheRoot, "claude-plugins-official", "superpowers", "5.0.1", "skills"),
			"superpowers@claude-plugins-official:5.0.1",
		},
		// Different registry/plugin
		{
			filepath.Join(cacheRoot, "my-registry", "my-tool", "1.2.3", "skills"),
			"my-tool@my-registry:1.2.3",
		},
		// Malformed path (not enough segments): fall back to tilde-subst
		{
			filepath.Join(cacheRoot, "only-one"),
			"~/.claude/plugins/cache/only-one",
		},
		// Path not ending in "skills": fall back
		{
			filepath.Join(cacheRoot, "reg", "plugin", "1.0", "other"),
			"~/.claude/plugins/cache/reg/plugin/1.0/other",
		},
	}
	for _, tt := range tests {
		got := claudePluginLabel(tt.skillsDir, cacheRoot, home)
		if got != tt.want {
			t.Errorf("claudePluginLabel(%q) = %q, want %q", tt.skillsDir, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/discover/ -run 'TestTildeSubst|TestSkillsAncestor|TestClaudePluginLabel' -v
```

Expected: compile error (functions not defined yet).

- [ ] **Step 3: Create the implementation file**

```go
// internal/discover/skills_helpers.go
package discover

import (
	"path/filepath"
	"strings"
)

// tildeSubst replaces the home directory prefix with "~".
// If path does not begin with home, it is returned unchanged.
func tildeSubst(path, home string) string {
	if home == "" || path == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}

// skillsAncestor walks up from filePath's parent toward root and returns
// the full absolute path of the nearest ancestor directory whose base name
// contains "skills" (case-insensitive). Returns "" if none is found within root.
// The root directory itself is not considered an eligible ancestor.
func skillsAncestor(filePath, root string) string {
	root = filepath.Clean(root)
	dir := filepath.Dir(filePath) // start at the skill name dir (parent of SKILL.md)
	for {
		// Stop at root or if we've walked above root
		if dir == root || !strings.HasPrefix(dir, root+string(filepath.Separator)) {
			return ""
		}
		if strings.Contains(strings.ToLower(filepath.Base(dir)), "skills") {
			return dir
		}
		dir = filepath.Dir(dir)
	}
}

// claudePluginLabel derives a human-readable source label for a Claude plugin
// skills directory. It expects skillsDir to follow the structure:
//   <cacheRoot>/<registry>/<plugin>/<version>/skills
// and returns "<plugin>@<registry>:<version>".
// Falls back to tildeSubst(skillsDir, home) if the structure doesn't match.
func claudePluginLabel(skillsDir, cacheRoot, home string) string {
	rel, err := filepath.Rel(cacheRoot, skillsDir)
	if err != nil {
		return tildeSubst(skillsDir, home)
	}
	parts := strings.Split(rel, string(filepath.Separator))
	// expected parts: [registry, plugin, version, "skills"]
	if len(parts) != 4 || parts[3] != "skills" {
		return tildeSubst(skillsDir, home)
	}
	registry, plugin, version := parts[0], parts[1], parts[2]
	return plugin + "@" + registry + ":" + version
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/discover/ -run 'TestTildeSubst|TestSkillsAncestor|TestClaudePluginLabel' -v
```

Expected: all PASS.

- [ ] **Step 5: Run full test suite to check for regressions**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/discover/skills_helpers.go internal/discover/skills_helpers_test.go
git commit -m "feat(skills): add tildeSubst, skillsAncestor, claudePluginLabel helpers"
```

---

### Task 2: Cache type with Load/Save

**Files:**
- Create: `internal/discover/skills_cache.go`
- Create: `internal/discover/skills_cache_test.go`

- [ ] **Step 1: Write failing tests for cache Load/Save**

```go
// internal/discover/skills_cache_test.go
package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCache_Missing(t *testing.T) {
	home := t.TempDir()
	cache, err := LoadCache(home)
	if err != nil {
		t.Fatalf("LoadCache on missing file: %v", err)
	}
	if cache == nil {
		t.Fatal("expected non-nil cache for missing file")
	}
	if len(cache.Entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(cache.Entries))
	}
}

func TestLoadCache_Valid(t *testing.T) {
	home := t.TempDir()
	cacheDir := filepath.Join(home, ".agents", "maestron")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	data := `{
		"version": 1,
		"entries": {
			"/path/to/SKILL.md": {
				"hash": "sha256:abc",
				"name": "my-skill",
				"description": "Does things"
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(cacheDir, "skills.json"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := LoadCache(home)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	entry, ok := cache.Entries["/path/to/SKILL.md"]
	if !ok {
		t.Fatal("expected entry not found")
	}
	if entry.Name != "my-skill" {
		t.Errorf("name = %q, want %q", entry.Name, "my-skill")
	}
}

func TestSaveCache(t *testing.T) {
	home := t.TempDir()
	cache := newCache()
	cache.Entries["/path/SKILL.md"] = CacheEntry{
		Hash:        "sha256:xyz",
		Name:        "foo",
		Description: "bar",
	}

	if err := cache.Save(home); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".agents", "maestron", "skills.json"))
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	var loaded SkillCache
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.Version != 1 {
		t.Errorf("version = %d, want 1", loaded.Version)
	}
	if _, ok := loaded.Entries["/path/SKILL.md"]; !ok {
		t.Error("expected entry not found in saved file")
	}
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./internal/discover/ -run 'TestLoadCache|TestSaveCache' -v
```

Expected: compile error.

- [ ] **Step 3: Implement the cache type**

```go
// internal/discover/skills_cache.go
package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CacheEntry holds the cached metadata for a single SKILL.md file.
type CacheEntry struct {
	Hash        string `json:"hash"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SkillCache is the in-memory representation of ~/.agents/maestron/skills.json.
type SkillCache struct {
	Version int                    `json:"version"`
	Entries map[string]CacheEntry  `json:"entries"`
	touched map[string]bool        // not serialised; tracks entries seen this run
}

func newCache() *SkillCache {
	return &SkillCache{
		Version: 1,
		Entries: make(map[string]CacheEntry),
		touched: make(map[string]bool),
	}
}

// cachePath returns ~/.agents/maestron/skills.json.
func cachePath(home string) string {
	return filepath.Join(home, ".agents", "maestron", "skills.json")
}

// LoadCache reads the cache from disk. Returns an empty cache if the file
// does not exist. Returns an error only for unexpected read/parse failures.
func LoadCache(home string) (*SkillCache, error) {
	path := cachePath(home)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return newCache(), nil
	}
	if err != nil {
		return newCache(), err
	}
	var c SkillCache
	if err := json.Unmarshal(data, &c); err != nil {
		return newCache(), nil // treat corrupt cache as empty
	}
	if c.Entries == nil {
		c.Entries = make(map[string]CacheEntry)
	}
	c.touched = make(map[string]bool)
	return &c, nil
}

// Save writes the cache to ~/.agents/maestron/skills.json, creating
// intermediate directories as needed. Errors are non-fatal (cache is
// opportunistic).
func (c *SkillCache) Save(home string) error {
	path := cachePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/discover/ -run 'TestLoadCache|TestSaveCache' -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/discover/skills_cache.go internal/discover/skills_cache_test.go
git commit -m "feat(skills): add SkillCache type with LoadCache/Save"
```

---

### Task 3: Cache Lookup, Set, Prune methods

**Files:**
- Modify: `internal/discover/skills_cache.go`
- Modify: `internal/discover/skills_cache_test.go`

- [ ] **Step 1: Add failing tests for Lookup, Set, Prune**

Append to `internal/discover/skills_cache_test.go`:

```go
func TestCacheLookup_Hit(t *testing.T) {
	c := newCache()
	c.Entries["/path/SKILL.md"] = CacheEntry{Hash: "sha256:abc", Name: "foo", Description: "bar"}

	entry, ok := c.Lookup("/path/SKILL.md", "sha256:abc")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if entry.Name != "foo" {
		t.Errorf("name = %q, want %q", entry.Name, "foo")
	}
	// Lookup should mark as touched
	if !c.touched["/path/SKILL.md"] {
		t.Error("expected path to be marked touched after Lookup hit")
	}
}

func TestCacheLookup_HashMismatch(t *testing.T) {
	c := newCache()
	c.Entries["/path/SKILL.md"] = CacheEntry{Hash: "sha256:abc", Name: "foo"}

	_, ok := c.Lookup("/path/SKILL.md", "sha256:different")
	if ok {
		t.Error("expected cache miss on hash mismatch")
	}
	// Should NOT be marked touched on a miss
	if c.touched["/path/SKILL.md"] {
		t.Error("expected path NOT to be touched on miss")
	}
}

func TestCacheLookup_UnknownPath(t *testing.T) {
	c := newCache()
	_, ok := c.Lookup("/unknown/SKILL.md", "sha256:abc")
	if ok {
		t.Error("expected cache miss for unknown path")
	}
}

func TestCacheSet(t *testing.T) {
	c := newCache()
	c.Set("/path/SKILL.md", CacheEntry{Hash: "sha256:abc", Name: "foo"})

	if _, ok := c.Entries["/path/SKILL.md"]; !ok {
		t.Error("expected entry to be stored")
	}
	if !c.touched["/path/SKILL.md"] {
		t.Error("expected path to be marked touched after Set")
	}
}

func TestCachePrune(t *testing.T) {
	c := newCache()
	c.Entries["/kept/SKILL.md"] = CacheEntry{Hash: "sha256:a", Name: "kept"}
	c.Entries["/pruned/SKILL.md"] = CacheEntry{Hash: "sha256:b", Name: "pruned"}
	c.touched["/kept/SKILL.md"] = true // only this one was seen

	c.Prune()

	if _, ok := c.Entries["/kept/SKILL.md"]; !ok {
		t.Error("expected /kept/SKILL.md to be retained")
	}
	if _, ok := c.Entries["/pruned/SKILL.md"]; ok {
		t.Error("expected /pruned/SKILL.md to be removed")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/discover/ -run 'TestCacheLookup|TestCacheSet|TestCachePrune' -v
```

Expected: compile error (methods not defined).

- [ ] **Step 3: Implement Lookup, Set, Prune**

Append to `internal/discover/skills_cache.go`:

```go
// Lookup checks the cache for absPath. If found and hash matches, returns the
// entry and marks it as touched. Returns false on miss or hash mismatch.
func (c *SkillCache) Lookup(absPath, hash string) (CacheEntry, bool) {
	entry, ok := c.Entries[absPath]
	if !ok || entry.Hash != hash {
		return CacheEntry{}, false
	}
	c.touched[absPath] = true
	return entry, true
}

// Set stores an entry and marks it as touched.
func (c *SkillCache) Set(absPath string, entry CacheEntry) {
	c.Entries[absPath] = entry
	c.touched[absPath] = true
}

// Prune removes cache entries not touched during the current walk.
func (c *SkillCache) Prune() {
	for path := range c.Entries {
		if !c.touched[path] {
			delete(c.Entries, path)
		}
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/discover/ -run 'TestCacheLookup|TestCacheSet|TestCachePrune' -v
```

Expected: all PASS.

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/discover/skills_cache.go internal/discover/skills_cache_test.go
git commit -m "feat(skills): add Lookup, Set, Prune to SkillCache"
```

---

## Chunk 2: Discovery rewrite and CLI update

### Task 4: `loadSkillCached` and `walkSkillsDir`

**Files:**
- Modify: `internal/discover/skills.go`

These are the core building blocks used by all walk functions. `loadSkillCached` replaces `loadSkillFromPath`. `walkSkillsDir` is the generic recursive walker used for standard global paths.

- [ ] **Step 1: Add `loadSkillCached` and `walkSkillsDir` to `skills.go`**

Add these functions to `internal/discover/skills.go` (keep existing functions for now — they will be removed in Task 7):

```go
import (
	"crypto/sha256"
	"fmt"
	// ... existing imports
)

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
	}, nil
}

// walkSkillsDir recursively walks dir for SKILL.md files, assigning source
// as the source label for all skills found. Silently skips missing dirs.
func walkSkillsDir(dir, source string, cache *SkillCache) []SkillInfo {
	var skills []SkillInfo
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error { //nolint:errcheck
		if err != nil || info.IsDir() || info.Name() != "SKILL.md" {
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
```

- [ ] **Step 2: Build to confirm it compiles**

```bash
go build ./...
```

Expected: clean compile.

- [ ] **Step 3: Commit**

```bash
git add internal/discover/skills.go
git commit -m "feat(skills): add loadSkillCached and walkSkillsDir building blocks"
```

---

### Task 5: `walkGlobalSkills` and `walkClaudePlugins`

**Files:**
- Modify: `internal/discover/skills.go`

- [ ] **Step 1: Add `walkGlobalSkills` and `walkClaudePlugins`**

Append to `internal/discover/skills.go`:

```go
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
		if err != nil || info.IsDir() || info.Name() != "SKILL.md" {
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
```

- [ ] **Step 2: Build to confirm it compiles**

```bash
go build ./...
```

Expected: clean compile.

- [ ] **Step 3: Commit**

```bash
git add internal/discover/skills.go
git commit -m "feat(skills): add walkGlobalSkills and walkClaudePlugins"
```

---

### Task 6: `walkWorkspaceSkills`

**Files:**
- Modify: `internal/discover/skills.go`

- [ ] **Step 1: Add `walkWorkspaceSkills`**

Append to `internal/discover/skills.go`:

```go
// walkWorkspaceSkills recursively walks the workspace root for SKILL.md files.
// Each skill's source label is the tilde-substituted path of its nearest
// ancestor directory whose name contains "skills". Skills with no such ancestor
// are skipped.
func walkWorkspaceSkills(root, home string, cache *SkillCache) []SkillInfo {
	var skills []SkillInfo
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error { //nolint:errcheck
		if err != nil || info.IsDir() || info.Name() != "SKILL.md" {
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
```

- [ ] **Step 2: Build to confirm it compiles**

```bash
go build ./...
```

Expected: clean compile.

- [ ] **Step 3: Commit**

```bash
git add internal/discover/skills.go
git commit -m "feat(skills): add walkWorkspaceSkills"
```

---

### Task 7: Replace `ListSkills` and remove old functions

**Files:**
- Modify: `internal/discover/skills.go`

This task wires everything together and removes the old hardcoded discovery code.

- [ ] **Step 1: Replace `ListSkills` with the new orchestrator**

Replace the entire `ListSkills` function body in `internal/discover/skills.go`:

```go
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

	// Deduplicate by name+source (first occurrence wins)
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
```

- [ ] **Step 2: Remove old functions**

Delete these functions from `internal/discover/skills.go` (they are fully replaced):
- `discoverSkillsDir`
- `discoverClaudeNativeSkills`
- `loadSkillFromPath`

Also remove the `skillFrontmatter` struct and `parseSkillFrontmatter` if they are only used by the removed functions. Check: `loadSkillCached` calls `parseSkillFrontmatter`, so keep `skillFrontmatter` and `parseSkillFrontmatter`.

- [ ] **Step 3: Remove unused imports (if any)**

Check the import block at the top of `skills.go`. The `strings` import was used by old source label derivation — verify it is still needed (it is, since `parseSkillFrontmatter` uses it). Remove any imports that are now unused.

- [ ] **Step 4: Build**

```bash
go build ./...
```

Expected: clean compile. Fix any remaining compile errors before proceeding.

- [ ] **Step 5: Run existing integration test**

```bash
go test ./tests/ -run TestListSkills -v
```

Expected: PASS. The test is a smoke test — it logs found skills without asserting format. Manually inspect the log output to confirm source labels are now paths (e.g. `~/.claude/skills`, `superpowers@claude-plugins-official:5.0.1`) rather than the old synthetic labels (`claude-global`, `claude-native`). No code change to this test is needed.

- [ ] **Step 6: Run full suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/discover/skills.go
git commit -m "feat(skills): rewrite ListSkills with recursive walk and cache"
```

---

### Task 8: Update CLI help text and add unit tests for walk functions

**Files:**
- Modify: `cmd/skills.go`
- Modify: `internal/discover/skills_helpers_test.go`

- [ ] **Step 1: Update the `--source` flag help text in `cmd/skills.go`**

Find the line (around line 26):
```go
skillsCmd.Flags().StringVar(&skillsSource, "source", "", `filter by source (e.g. "project", "global", "claude", "codex")`)
```

Replace with:
```go
skillsCmd.Flags().StringVar(&skillsSource, "source", "", `filter by source path (e.g. "claude", "codex", "superpowers")`)
```

- [ ] **Step 2: Add integration-level walk test to `skills_helpers_test.go`**

Note: these tests are in `package discover` (same package as `newCache`, `walkGlobalSkills`, `walkWorkspaceSkills`), so all unexported functions and constructors are accessible without any exports.

Append a filesystem-based test that exercises the full walk path through temp directories:

```go
func TestWalkGlobalSkills_MissingDirs(t *testing.T) {
	home := t.TempDir() // empty home — no skills dirs exist
	cache := newCache()
	skills := walkGlobalSkills(home, cache)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills from empty home, got %d", len(skills))
	}
}

func TestWalkGlobalSkills_NestedSkill(t *testing.T) {
	home := t.TempDir()
	// Create ~/.claude/skills/my-skill/SKILL.md
	skillDir := filepath.Join(home, ".claude", "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: my-skill\ndescription: A test skill\n---\n# My Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cache := newCache()
	skills := walkGlobalSkills(home, cache)

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "my-skill" {
		t.Errorf("name = %q, want %q", skills[0].Name, "my-skill")
	}
	if skills[0].Source != "~/.claude/skills" {
		t.Errorf("source = %q, want %q", skills[0].Source, "~/.claude/skills")
	}
}

func TestWalkWorkspaceSkills_AncestorGrouping(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()

	// Create .codex/skills/foo/SKILL.md and agent-skills/bar/SKILL.md
	for _, rel := range []string{
		filepath.Join(".codex", "skills", "foo", "SKILL.md"),
		filepath.Join("agent-skills", "bar", "SKILL.md"),
		filepath.Join("src", "util", "SKILL.md"), // should be skipped — no "skills" ancestor
	} {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# skill"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cache := newCache()
	skills := walkWorkspaceSkills(root, home, cache)

	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d: %v", len(skills), skills)
	}
	sources := map[string]bool{}
	for _, s := range skills {
		sources[s.Source] = true
	}
	if !sources[filepath.Join(root, ".codex", "skills")] {
		t.Errorf("expected source %q, got sources: %v", filepath.Join(root, ".codex", "skills"), sources)
	}
	if !sources[filepath.Join(root, "agent-skills")] {
		t.Errorf("expected source %q, got sources: %v", filepath.Join(root, "agent-skills"), sources)
	}
}
```

Note: `walkWorkspaceSkills` uses absolute paths for source labels when the workspace is not under home — this is expected and tested above.

- [ ] **Step 3: Run the new tests**

```bash
go test ./internal/discover/ -run 'TestWalkGlobal|TestWalkWorkspace' -v
```

Expected: all PASS.

- [ ] **Step 4: Run full suite**

```bash
go test ./...
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/skills.go internal/discover/skills_helpers_test.go
git commit -m "feat(skills): update --source help text; add walk integration tests"
```

---

### Task 9: Final verification

- [ ] **Step 1: Build the binary**

```bash
go build -o /tmp/maestron .
```

Expected: clean compile, binary produced.

- [ ] **Step 2: Smoke-test skills output**

```bash
/tmp/maestron skills
```

Expected: output grouped by path-based source labels (e.g. `~/.claude/skills`, `superpowers@claude-plugins-official:5.0.1`) rather than synthetic labels.

- [ ] **Step 3: Test `--source` filter**

```bash
/tmp/maestron skills --source superpowers
/tmp/maestron skills --source claude
```

Expected: filtered output matching the substring.

- [ ] **Step 4: Verify cache file is created**

```bash
cat ~/.agents/maestron/skills.json
```

Expected: valid JSON with `version: 1` and an `entries` object.

- [ ] **Step 5: Run skills a second time and confirm it is fast (cache hit)**

```bash
time /tmp/maestron skills
```

Expected: noticeably fast on second run (no frontmatter parsing for unchanged files).

- [ ] **Step 6: Final commit if any cleanup needed, then push**

```bash
go test ./...
git log --oneline -10
```

Verify all commits on this branch use conventional commit prefixes per `AGENTS.md`.
