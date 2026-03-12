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
		{"/workspace/.codex/skills/foo/SKILL.md", "/workspace/.codex/skills"},
		{"/workspace/.codex/skills/category/foo/SKILL.md", "/workspace/.codex/skills"},
		{"/workspace/skills/bar/SKILL.md", "/workspace/skills"},
		{"/workspace/agent-skills/baz/SKILL.md", "/workspace/agent-skills"},
		{"/workspace/src/foo/SKILL.md", ""},
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
		{
			filepath.Join(cacheRoot, "claude-plugins-official", "superpowers", "5.0.1", "skills"),
			"superpowers@claude-plugins-official:5.0.1",
		},
		{
			filepath.Join(cacheRoot, "my-registry", "my-tool", "1.2.3", "skills"),
			"my-tool@my-registry:1.2.3",
		},
		{
			filepath.Join(cacheRoot, "only-one"),
			"~/.claude/plugins/cache/only-one",
		},
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
