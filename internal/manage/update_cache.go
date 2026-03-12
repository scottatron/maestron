package manage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// UpdateCheckEntry records the result of a single skill update check.
type UpdateCheckEntry struct {
	HasUpdate bool      `json:"has_update"`
	RemoteSHA string    `json:"remote_sha,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
	ErrMsg    string    `json:"error,omitempty"`
}

// UpdateCheckCache persists the results of the last update check per skill.
type UpdateCheckCache struct {
	Skills map[string]*UpdateCheckEntry `json:"skills"`
}

func updateCachePath(home string) string {
	return filepath.Join(home, ".agents", "maestron", "update-check.json")
}

// LoadUpdateCache reads the update check cache. Returns an empty cache on missing file.
func LoadUpdateCache(home string) (*UpdateCheckCache, error) {
	c := &UpdateCheckCache{Skills: map[string]*UpdateCheckEntry{}}
	data, err := os.ReadFile(updateCachePath(home))
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, err
	}
	if c.Skills == nil {
		c.Skills = map[string]*UpdateCheckEntry{}
	}
	return c, nil
}

// Save writes the update check cache to disk.
func (c *UpdateCheckCache) Save(home string) error {
	p := updateCachePath(home)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
