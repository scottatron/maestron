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

**Global paths are searched first; workspace paths follow.** This is intentional: global skills represent the user's personal library, which should appear prominently and have first-match priority in deduplication over workspace-specific skills.

### Global paths

Each of the following directories is walked recursively. Every `SKILL.md` found anywhere beneath a given path gets that path's tilde-substituted form as its source label — so all skills under `~/.claude/skills/**` share the source `~/.claude/skills`.

| Directory | Source label |
|---|---|
| `~/.agents/skills` | `~/.agents/skills` |
| `~/.claude/skills` | `~/.claude/skills` |
| `~/.codex/skills` | `~/.codex/skills` |
| `~/.copilot/skills` | `~/.copilot/skills` |
| `~/.github/skills` | `~/.github/skills` |
| `~/.claude/plugins/cache` (Claude native) | `<plugin>@<registry>:<version>` |

If a path does not exist, it is silently skipped.

**Claude native grouping:** The plugins cache has the structure `~/.claude/plugins/cache/<registry>/<plugin>/<version>/skills`. Each plugin/version combination gets its own source group using the human-readable label `<plugin>@<registry>:<version>` (e.g. `superpowers@claude-plugins-official:5.0.1`). This replaces the previous single `claude-native` label. The label is derived by parsing the path components relative to the cache root — it is not a tilde-substituted path.

### Workspace paths

If a workspace root is found (via `agents.FindAgentsConfig()`), walk from that root recursively. For each `SKILL.md` encountered:

1. Walk up from the skill file's parent directory toward the workspace root
2. Find the nearest ancestor directory whose name contains `"skills"` (case-insensitive substring match)
3. Use that ancestor's full absolute path, tilde-substituted, as the source label
4. If no such ancestor exists within the workspace root, skip the file

The `skillsAncestor` function returns the **full absolute path** of the matching ancestor directory (not just its name). This becomes the source label after tilde substitution.

Examples:

| File path (relative to workspace) | Source label |
|---|---|
| `.codex/skills/foo/SKILL.md` | `<workspace>/.codex/skills` |
| `skills/bar/SKILL.md` | `<workspace>/skills` |
| `agent-skills/baz/SKILL.md` | `<workspace>/agent-skills` |
| `.codex/skills/category/foo/SKILL.md` | `<workspace>/.codex/skills` |
| `src/util.go` | skipped (no `*skills*` ancestor) |

**Workspace/global overlap:** If the workspace root happens to contain directories that are also global paths (e.g. a project rooted at `~`), those directories will produce different source labels in the workspace walk than in the global walk (different paths). This means they can appear twice under different groups. This is an acceptable edge case unlikely to occur in practice; no special handling is needed.

### Source re-derivation

Source labels are always re-derived during each walk; `SkillInfo.Source` is never loaded from the cache and is instead recomputed by the walking logic on each run. This avoids stale sources when the workspace root changes between invocations.

### Deduplication

Deduplication by `name+source` is retained: skills with the same name and source keep the first occurrence. With path-based source labels, two skills at different filesystem locations always have distinct sources and are never deduplicated against each other.

---

## Cache

### Location

`~/.agents/maestron/skills.json`

### Strategy

Always re-walk all directories (so new and deleted skills are detected on every run). For each `SKILL.md` found:

- Compute `sha256` of its content
- If the absolute path exists in the cache **and** the hash matches → use cached `name` and `description`; skip frontmatter parsing
- If hash differs or path is new → parse frontmatter, update cache entry
- After the walk, remove cache entries whose paths were not encountered (deleted skills)
- Write the updated cache back to disk

The cache provides `name` and `description` only. Source is always computed by the walk, not read from cache.

### Cache format

```json
{
  "version": 1,
  "entries": {
    "/abs/path/to/SKILL.md": {
      "hash": "sha256:<hex>",
      "name": "skill-name",
      "description": "One-line description"
    }
  }
}
```

---

## Source labels

All source labels are tilde-substituted absolute paths of the skills root directory:

- `~/.claude/skills` — all global Claude skills
- `~/.codex/skills` — all global Codex skills
- `superpowers@claude-plugins-official:5.0.1` — skills from a specific Claude plugin
- `/Users/scott/src/project/.codex/skills` — workspace skills where workspace is outside home

**Tilde substitution:** replace the home directory prefix (e.g. `/Users/scott`) with `~`. If the path does not begin with the home directory, use the absolute path as-is.

The `--source` filter on `maestron skills` uses `strings.Contains`, which works naturally against these labels (e.g. `--source codex`, `--source claude`, `--source superpowers`).

---

## Code changes

### `internal/discover/skills.go`

- Replace `ListSkills()` with updated implementation calling `walkGlobalSkills()` and `walkWorkspaceSkills()`
- Remove `discoverSkillsDir()` (fixed-path, one-level) and `discoverClaudeNativeSkills()` (replaced by recursive global walk)
- Add `walkGlobalSkills(home string, cache *SkillCache) []SkillInfo`
- Add `walkWorkspaceSkills(root, home string, cache *SkillCache) []SkillInfo`
- Add `skillsAncestor(filePath, root string) string` — walks up from `filePath`'s parent to `root`, returns the full absolute path of the nearest ancestor whose name contains `"skills"`, or `""` if none found
- Add `tildeSubst(path, home string) string` — replaces home prefix with `~`
- Add `claudePluginLabel(skillsDir, cacheRoot, home string) string` — parses `<cacheRoot>/<registry>/<plugin>/<version>/skills` to produce `<plugin>@<registry>:<version>`; falls back to tilde-substituted path using `home` if structure doesn't match

### `internal/discover/skills_cache.go` (new file)

```go
type CacheEntry struct {
    Hash        string `json:"hash"`
    Name        string `json:"name"`
    Description string `json:"description"`
}

type SkillCache struct {
    Version int                    `json:"version"`
    Entries map[string]CacheEntry  `json:"entries"`
    touched map[string]bool        // not serialised
}

func LoadCache(home string) (*SkillCache, error)
func (c *SkillCache) Save(home string) error
func (c *SkillCache) Lookup(absPath, hash string) (CacheEntry, bool)
func (c *SkillCache) Set(absPath string, entry CacheEntry)
func (c *SkillCache) Prune() // removes entries not touched during current walk
```

`LoadCache` and `Save` accept `home string` (the resolved home directory) to avoid a second independent call to `platform.HomeDir()`.

### `internal/discover/types.go`

No structural changes. `SkillInfo.Source` remains a `string`; it now holds a tilde-substituted path.

### `cmd/skills.go`

Update the `--source` flag help text from the current hardcoded examples to reflect path-based filtering:

```go
skillsCmd.Flags().StringVar(&skillsSource, "source", "", `filter by source path (e.g. "claude", "codex", "superpowers")`)
```

No other changes required. Grouping by source in `renderSkillsGrouped` uses `SkillInfo.Source` as the group key and works correctly with path-based labels.

---

## Testing

Add or update unit tests covering:

- `tildeSubst`: path under home → tilde prefix; path not under home → unchanged
- `skillsAncestor`: nested skill file finds correct `*skills*` ancestor; file with no `*skills*` ancestor returns `""`; file at workspace root boundary is handled
- `SkillCache.Lookup`: cache hit with matching hash returns entry; cache miss on hash mismatch; unknown path returns false
- `SkillCache.Prune`: untouched entries are removed; touched entries are retained
- `walkGlobalSkills`: missing directories are silently skipped; recursive walk finds nested SKILL.md files

---

## Non-goals

- No configuration file for search paths
- No opt-in/opt-out per directory
- No watching for changes (always re-walk)
- No change to `SkillInfo` fields
