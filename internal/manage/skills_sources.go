package manage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SkillSource describes where a skill was installed from.
type SkillSource struct {
	Type string `json:"type"` // "git" or "local"
	// git fields
	URL         string `json:"url,omitempty"`
	Ref         string `json:"ref,omitempty"`
	ResolvedSHA string `json:"resolved_sha,omitempty"`
	Subpath     string `json:"subpath,omitempty"`
	// local fields
	Path     string `json:"path,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

// SkillRecord represents a managed skill in the manifest.
type SkillRecord struct {
	Name        string      `json:"name"`
	InstallPath string      `json:"install_path"`
	Source      SkillSource `json:"source"`
	InstalledAt time.Time   `json:"installed_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	ContentHash string      `json:"content_hash"`
}

// SkillsManifest is the top-level manifest tracking managed skills.
type SkillsManifest struct {
	Version int                     `json:"version"`
	Skills  map[string]*SkillRecord `json:"skills"`
}

// manifestPath returns the path to the skills manifest file.
func manifestPath(home string) string {
	return filepath.Join(home, ".agents", "maestron", "skills-sources.json")
}

// LoadManifest reads the skills manifest. Returns an empty manifest if the file
// does not exist, and an error if the file exists but cannot be parsed.
// Tilde paths in the manifest are expanded to absolute paths using home.
func LoadManifest(home string) (*SkillsManifest, error) {
	path := manifestPath(home)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &SkillsManifest{Version: 1, Skills: make(map[string]*SkillRecord)}, nil
	}
	if err != nil {
		return nil, err
	}
	var m SkillsManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Skills == nil {
		m.Skills = make(map[string]*SkillRecord)
	}
	// Expand ~ so all in-memory paths are absolute.
	for _, r := range m.Skills {
		r.InstallPath = expandTilde(r.InstallPath, home)
		r.Source.Path = expandTilde(r.Source.Path, home)
	}
	return &m, nil
}

// Save writes the manifest to disk, creating parent directories as needed.
// Absolute paths under home are stored as ~ paths for portability.
func (m *SkillsManifest) Save(home string) error {
	path := manifestPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// Build a copy with tilde-substituted paths for on-disk portability.
	portable := SkillsManifest{Version: m.Version, Skills: make(map[string]*SkillRecord, len(m.Skills))}
	for k, r := range m.Skills {
		cp := *r
		cp.InstallPath = substituteHome(cp.InstallPath, home)
		cp.Source.Path = substituteHome(cp.Source.Path, home)
		portable.Skills[k] = &cp
	}
	data, err := json.MarshalIndent(portable, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// expandTilde replaces a leading "~/" with home.
func expandTilde(path, home string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// substituteHome replaces a leading home directory prefix with "~/".
func substituteHome(path, home string) string {
	if home != "" && strings.HasPrefix(path, home+"/") {
		return "~/" + path[len(home)+1:]
	}
	return path
}
