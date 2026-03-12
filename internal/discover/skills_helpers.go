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
//
//	<cacheRoot>/<registry>/<plugin>/<version>/skills
//
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
