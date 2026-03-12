# Skills Discovery Redesign

**Date:** 2026-03-12
**Branch:** feat/skills
**Status:** Approved

---

## Problem

Skills discovery in `internal/discover/skills.go` uses hardcoded, fixed-depth search paths with synthetic source labels (`claude-global`, `agents-project`, etc.). This misses skills in unconventional directories (e.g. `skills/`, `agent-skills/` at the workspace root), does not scale as new tool directories are added, and obscures where skills actually live.

---

## Goals

1. Discover skills recursively under each global path
2. Discover skills recursively across the entire workspace root, grouping by the nearest `*skills*` ancestor
3. Add a discovery cache to avoid re-parsing unchanged SKILL.md files
4. Replace synthetic source labels with tilde-substituted absolute paths

---

## Discovery

### Order

Global paths are searched first; workspace paths follow. This means global skills appear before workspace skills in `maestron skills` output.

### Global paths

Each of the following directories is walked recursively. Every `SKILL.md` found anywhere beneath a path gets that path as its source label (tilde-substituted).

| Directory | Source label |
|---|---|
| `~/.agents/skills` | `~/.agents/skills` |
| `~/.claude/skills` | `~/.claude/skills` |
| `~/.codex/skills` | `~/.codex/skills` |
| `~/.copilot/skills` | `~/.copilot/skills` |
| `~/.github/skills` | `~/.github/skills` |
| `~/.claude/plugins/cache` (existing Claude native walk) | `~/.claude/plugins/cache/<plugin>/<version>/skills` |

If a path does not exist, it is silently skipped.

### Workspace paths

If a workspace root is found (via `agents.FindAgentsConfig()`), walk from that root recursively. For each `SKILL.md` encountered:

1. Walk up from the skill file's parent directory toward the workspace root
2. Find the nearest ancestor directory whose name contains `"skills"` (case-insensitive)
3. Use that ancestor's absolute path, tilde-substituted, as the source label
4. If no such ancestor exists within the workspace root, skip the file

Examples:

| File path (relative to workspace) | Source label |
|---|---|
| `.codex/skills/foo/SKILL.md` | `.codex/skills` (tilde-subst if applicable) |
| `skills/bar/SKILL.md` | `<workspace>/skills` |
| `agent-skills/baz/SKILL.md` | `<workspace>/agent-skills` |
| `.codex/skills/category/foo/SKILL.md` | `.codex/skills` |
| `src/util.go` | skipped (no `*skills*` ancestor) |

Workspace paths that are already covered by a global path (same absolute path) are still included — deduplication by `name+source` handles overlaps.

### Deduplication

Deduplication is retained: skills with the same `name+source` keep the first occurrence. This is unchanged from current behaviour.

---

## Cache

### Location

`~/.agents/maestron/skills.json`

### Strategy

Always re-walk all directories (so new and deleted skills are detected on every run). For each `SKILL.md` found:

- Compute `sha256` of its content
- If the absolute path exists in the cache **and** the hash matches → use cached `name`, `description`, `source`; skip frontmatter parsing
- If hash differs or path is new → parse frontmatter, update cache entry
- After the walk, remove cache entries whose paths were not encountered (deleted skills)
- Write the updated cache back to disk

### Cache format

```json
{
  "version": 1,
  "entries": {
    "/abs/path/to/SKILL.md": {
      "hash": "sha256:<hex>",
      "name": "skill-name",
      "description": "One-line description",
      "source": "~/.claude/skills"
    }
  }
}
```

`source` is stored as the tilde-substituted path so the cache can be used directly without re-deriving it.

---

## Source labels

All source labels are the tilde-substituted absolute path of the skills root directory:

- `~/.claude/skills` for global Claude skills
- `~/.codex/skills` for global Codex skills
- `~/.claude/plugins/cache/superpowers/5.0.1/skills` for a Claude native plugin
- `/Users/scott/src/project/.codex/skills` for workspace skills under a workspace not inside home

Tilde substitution: replace the home directory prefix with `~` when building the source label. If the path is not under the home directory, use the absolute path as-is.

The `--source` filter on `maestron skills` remains a `strings.Contains` match, which works naturally against these path-based labels.

---

## Code changes

### `internal/discover/skills.go`

- Replace `ListSkills()` with updated implementation calling `walkGlobalSkills()` and `walkWorkspaceSkills()`
- Remove `discoverSkillsDir()` (fixed-path, one-level) and `discoverClaudeNativeSkills()` (replaced by recursive walk)
- Add `walkGlobalSkills(home string, cache *SkillCache) []SkillInfo`
- Add `walkWorkspaceSkills(root, home string, cache *SkillCache) []SkillInfo`
- Add `skillsAncestor(path, root string) string` — walks up to find `*skills*` ancestor, returns its path or `""`
- Add `tildeSubst(path, home string) string` — replaces home prefix with `~`

### `internal/discover/skills_cache.go` (new file)

```go
type CacheEntry struct {
    Hash        string `json:"hash"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Source      string `json:"source"`
}

type SkillCache struct {
    Version int                    `json:"version"`
    Entries map[string]CacheEntry  `json:"entries"`
    touched map[string]bool        // not serialised
}

func LoadCache() (*SkillCache, error)
func (c *SkillCache) Save() error
func (c *SkillCache) Lookup(absPath string, hash string) (CacheEntry, bool)
func (c *SkillCache) Set(absPath string, entry CacheEntry)
func (c *SkillCache) Prune() // removes entries not touched during current walk
```

Cache file path: `~/.agents/maestron/skills.json` via `platform.HomeDir()`.

### `internal/discover/types.go`

No structural changes. `SkillInfo.Source` remains a `string`; it will now hold a tilde-substituted path instead of a synthetic label.

### `cmd/skills.go`

No changes required. Grouping by source in `renderSkillsGrouped` already uses `SkillInfo.Source` as the group key — path-based labels will group correctly.

---

## Non-goals

- No configuration file for search paths
- No opt-in/opt-out per directory
- No watching for changes (always re-walk)
- No change to `SkillInfo` fields or the `--source` flag behaviour
