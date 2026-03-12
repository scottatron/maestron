// internal/discover/skills_helpers_test.go
package discover

import (
	"os"
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
		// Skill whose dir name contains "skills" — must not be mistaken for the root
		{"/workspace/skills/writing-skills/SKILL.md", "/workspace/skills"},
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
	// root is a temp dir not under home, so tildeSubst returns the absolute path
	if !sources[filepath.Join(root, ".codex", "skills")] {
		t.Errorf("expected .codex/skills source, got: %v", sources)
	}
	if !sources[filepath.Join(root, "agent-skills")] {
		t.Errorf("expected agent-skills source, got: %v", sources)
	}
}
