package manage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInstallFromLocal_MissingSourcePreservesInstalled verifies that when the
// source path does not exist, the already-installed skill is not deleted.
func TestInstallFromLocal_MissingSourcePreservesInstalled(t *testing.T) {
	home := t.TempDir()

	// Create an already-installed skill.
	skillName := "my-skill"
	installDest := filepath.Join(home, ".agents", "skills", skillName)
	if err := os.MkdirAll(installDest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDest, "SKILL.md"), []byte("# my-skill\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Attempt to update from a non-existent source path.
	nonExistentSrc := filepath.Join(home, "does-not-exist")
	_, err := InstallFromLocal(home, nonExistentSrc, skillName)
	if err == nil {
		t.Fatal("expected error for missing source, got nil")
	}

	// The installed skill must still exist after the failed update.
	if _, statErr := os.Stat(filepath.Join(installDest, "SKILL.md")); os.IsNotExist(statErr) {
		t.Error("installed skill was deleted when source validation failed")
	}
}
