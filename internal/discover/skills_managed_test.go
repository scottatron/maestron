package discover

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAnnotateManagedSync_DirHashNotJustSKILLmd verifies that the managed-sync
// comparison is based on the full skill directory, not SKILL.md alone.
// A non-managed skill whose SKILL.md is identical to the managed copy but has
// additional or different files should be reported as ManagedRelationDiffers.
func TestAnnotateManagedSync_DirHashNotJustSKILLmd(t *testing.T) {
	home := t.TempDir()
	managedDir := filepath.Join(home, ".agents", "skills")

	skillMD := "---\nname: my-skill\ndescription: A test skill\n---\n# My Skill\n"

	// Create managed copy with SKILL.md + a helper script.
	managedSkillDir := filepath.Join(managedDir, "my-skill")
	if err := os.MkdirAll(managedSkillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(managedSkillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(managedSkillDir, "helper.sh"), []byte("#!/bin/sh\necho managed\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create non-managed copy with identical SKILL.md but different helper script.
	otherSkillDir := filepath.Join(home, ".claude", "skills", "my-skill")
	if err := os.MkdirAll(otherSkillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherSkillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherSkillDir, "helper.sh"), []byte("#!/bin/sh\necho different\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cache := newCache()
	skills := walkGlobalSkills(home, cache)
	result := annotateManagedSync(skills, managedDir)

	// Find the non-managed skill entry.
	var found *SkillInfo
	for i := range result {
		if result[i].ManagedRelation != ManagedRelationIs {
			found = &result[i]
		}
	}
	if found == nil {
		t.Fatal("expected a non-managed skill entry, got none")
	}
	if found.ManagedRelation != ManagedRelationDiffers {
		t.Errorf("ManagedRelation = %q, want %q (files outside SKILL.md differ)", found.ManagedRelation, ManagedRelationDiffers)
	}
}

// TestAnnotateManagedSync_MatchesWhenDirIdentical verifies that identical
// skill directories are reported as ManagedRelationMatches.
func TestAnnotateManagedSync_MatchesWhenDirIdentical(t *testing.T) {
	home := t.TempDir()
	managedDir := filepath.Join(home, ".agents", "skills")

	skillMD := "---\nname: my-skill\ndescription: A test skill\n---\n# My Skill\n"
	script := "#!/bin/sh\necho same\n"

	for _, dir := range []string{
		filepath.Join(managedDir, "my-skill"),
		filepath.Join(home, ".claude", "skills", "my-skill"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "helper.sh"), []byte(script), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cache := newCache()
	skills := walkGlobalSkills(home, cache)
	result := annotateManagedSync(skills, managedDir)

	var found *SkillInfo
	for i := range result {
		if result[i].ManagedRelation != ManagedRelationIs {
			found = &result[i]
		}
	}
	if found == nil {
		t.Fatal("expected a non-managed skill entry, got none")
	}
	if found.ManagedRelation != ManagedRelationMatches {
		t.Errorf("ManagedRelation = %q, want %q", found.ManagedRelation, ManagedRelationMatches)
	}
}
