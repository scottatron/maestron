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
	Version int                   `json:"version"`
	Entries map[string]CacheEntry `json:"entries"`
	touched map[string]bool       // not serialised; tracks entries seen this run
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
// intermediate directories as needed.
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
