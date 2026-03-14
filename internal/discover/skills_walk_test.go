package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnnotateManagedSync_UsesDirKeyWhenFrontmatterDiffers(t *testing.T) {
	home := t.TempDir()
	managedDir := filepath.Join(home, ".agents", "skills")

	managedSkillDir := filepath.Join(managedDir, "repo-skill")
	sourceSkillDir := filepath.Join(home, "src", "repo-skill")

	skillMD := "---\nname: frontmatter-name\ndescription: A test skill\n---\n# My Skill\n"

	for _, dir := range []string{managedSkillDir, sourceSkillDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cache := newCache()
	skills := []SkillInfo{}
	for _, source := range []struct {
		dir   string
		label string
	}{
		{managedSkillDir, "~/.agents/skills"},
		{sourceSkillDir, filepath.Join(home, "src")},
	} {
		skill, err := loadSkillCached(filepath.Join(source.dir, "SKILL.md"), source.label, cache)
		if err != nil {
			t.Fatal(err)
		}
		skills = append(skills, skill)
	}

	result := annotateManagedSync(skills, managedDir)

	var managed, source *SkillInfo
	for i := range result {
		switch result[i].Path {
		case filepath.Join(managedSkillDir, "SKILL.md"):
			managed = &result[i]
		case filepath.Join(sourceSkillDir, "SKILL.md"):
			source = &result[i]
		}
	}

	if managed == nil || source == nil {
		t.Fatalf("expected both managed and source skills, got %#v", result)
	}
	if managed.ManagedRelation != ManagedRelationIs {
		t.Fatalf("managed relation = %q, want %q", managed.ManagedRelation, ManagedRelationIs)
	}
	if source.ManagedRelation != ManagedRelationMatches {
		t.Fatalf("source relation = %q, want %q", source.ManagedRelation, ManagedRelationMatches)
	}
}

func TestWalkSkillsDirSkipsVCSDirectories(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".claude", "skills")

	keep := filepath.Join(root, "kept-skill")
	if err := os.MkdirAll(keep, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keep, "SKILL.md"), []byte("# kept"), 0644); err != nil {
		t.Fatal(err)
	}

	skipped := filepath.Join(root, ".git", "hidden-skill")
	if err := os.MkdirAll(skipped, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skipped, "SKILL.md"), []byte("# skipped"), 0644); err != nil {
		t.Fatal(err)
	}

	cache := newCache()
	skills := walkSkillsDir(root, "~/.claude/skills", cache)

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill after VCS pruning, got %d", len(skills))
	}
	if got := filepath.Base(filepath.Dir(skills[0].Path)); got != "kept-skill" {
		t.Fatalf("walked skill dir = %q, want %q", got, "kept-skill")
	}
}

func TestDirContentHashIncludesHiddenNonVCSAndSkipsVCS(t *testing.T) {
	t.Run("includes hidden non-vcs directories", func(t *testing.T) {
		left := t.TempDir()
		right := t.TempDir()

		for _, dir := range []string{left, right} {
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill"), 0644); err != nil {
				t.Fatal(err)
			}
			githubDir := filepath.Join(dir, ".github")
			if err := os.MkdirAll(githubDir, 0755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(left, ".github", "config.yml"), []byte("one"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(right, ".github", "config.yml"), []byte("two"), 0644); err != nil {
			t.Fatal(err)
		}

		leftHash, err := dirContentHash(left)
		if err != nil {
			t.Fatal(err)
		}
		rightHash, err := dirContentHash(right)
		if err != nil {
			t.Fatal(err)
		}
		if leftHash == rightHash {
			t.Fatal("expected hidden non-VCS files to affect content hash")
		}
	})

	t.Run("skips vcs directories", func(t *testing.T) {
		left := t.TempDir()
		right := t.TempDir()

		for _, dir := range []string{left, right} {
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill"), 0644); err != nil {
				t.Fatal(err)
			}
			gitDir := filepath.Join(dir, ".git")
			if err := os.MkdirAll(gitDir, 0755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(left, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(right, ".git", "HEAD"), []byte("ref: refs/heads/feature\n"), 0644); err != nil {
			t.Fatal(err)
		}

		leftHash, err := dirContentHash(left)
		if err != nil {
			t.Fatal(err)
		}
		rightHash, err := dirContentHash(right)
		if err != nil {
			t.Fatal(err)
		}
		if leftHash != rightHash {
			t.Fatal("expected VCS metadata to be ignored by content hash")
		}
	})
}
