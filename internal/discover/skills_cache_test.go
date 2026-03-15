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
	data := `{"version":1,"entries":{"/path/to/SKILL.md":{"hash":"sha256:abc","name":"my-skill","description":"Does things"}}}`
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
	c.touched["/kept/SKILL.md"] = true

	c.Prune()

	if _, ok := c.Entries["/kept/SKILL.md"]; !ok {
		t.Error("expected /kept/SKILL.md to be retained")
	}
	if _, ok := c.Entries["/pruned/SKILL.md"]; ok {
		t.Error("expected /pruned/SKILL.md to be removed")
	}
}
