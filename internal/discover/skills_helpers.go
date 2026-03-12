// internal/discover/skills_helpers.go
package discover

import (
	"path/filepath"
	"strconv"
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

// skillsAncestor walks up from filePath's grandparent toward root and returns
// the full absolute path of the nearest ancestor directory whose base name
// contains "skills" (case-insensitive). Returns "" if none is found within root.
// The root directory itself is not considered an eligible ancestor.
// We start from the grandparent (not the parent) so that skill name dirs whose
// names happen to contain "skills" (e.g. "writing-skills/SKILL.md") are not
// mistaken for the skills root.
func skillsAncestor(filePath, root string) string {
	root = filepath.Clean(root)
	dir := filepath.Dir(filepath.Dir(filePath)) // start at grandparent of SKILL.md (skip skill name dir)
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

// parsePluginSource parses a source label of the form "plugin@registry:version"
// (as produced by claudePluginLabel) and returns the base ("plugin@registry"),
// the parsed semver, and true. Returns false if the label doesn't match.
func parsePluginSource(source string) (base string, version [3]int, ok bool) {
	atIdx := strings.Index(source, "@")
	if atIdx < 0 {
		return "", version, false
	}
	colonIdx := strings.LastIndex(source, ":")
	if colonIdx < 0 || colonIdx < atIdx {
		return "", version, false
	}
	base = source[:colonIdx]
	version, ok = parseSemver(source[colonIdx+1:])
	return base, version, ok
}

// parseSemver parses a "major.minor.patch" string. Returns false if it doesn't match.
func parseSemver(v string) ([3]int, bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		nums[i] = n
	}
	return nums, true
}

// compareVersion returns -1, 0, or 1 comparing semver a to b.
func compareVersion(a, b [3]int) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
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
