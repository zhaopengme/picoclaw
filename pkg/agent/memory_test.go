package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileReadWrite(t *testing.T) {
	tempDir := t.TempDir()
	ms := NewMemoryStore(tempDir)

	// Write
	err := ms.WriteProfileKey("test_key", "test_value")
	if err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Read
	profile := ms.ReadProfile()
	if profile["test_key"] != "test_value" {
		t.Errorf("Expected 'test_value', got '%v'", profile["test_key"])
	}

	// Delete
	err = ms.DeleteProfileKey("test_key")
	if err != nil {
		t.Fatalf("Failed to delete profile key: %v", err)
	}

	profile2 := ms.ReadProfile()
	if _, exists := profile2["test_key"]; exists {
		t.Errorf("Key 'test_key' should have been deleted")
	}
}

func TestGetMemoryContextFormatting(t *testing.T) {
	tempDir := t.TempDir()
	ms := NewMemoryStore(tempDir)
	ms.WriteProfileKey("user", "Mike")

	ctx := ms.GetMemoryContext()
	expectedContains := "- **user**: Mike"

	if !strings.Contains(ctx, expectedContains) {
		t.Errorf("Context missing expected formatting. Got:\n%s", ctx)
	}
}

func TestProfileCorruptionProtection(t *testing.T) {
	tempDir := t.TempDir()
	ms := NewMemoryStore(tempDir)

	// Create a corrupted profile.json
	corruptData := []byte(`{ "user": "Mike", "broken_key" }`)
	err := os.WriteFile(ms.profileFile, corruptData, 0644)
	if err != nil {
		t.Fatalf("Failed to write corrupt data: %v", err)
	}

	// Try to write a new key, should fail because of parsing error
	err = ms.WriteProfileKey("new_key", "value")
	if err == nil {
		t.Errorf("Expected an error when writing to a corrupted profile, got nil")
	} else if !strings.Contains(err.Error(), "corrupted") {
		t.Errorf("Expected corruption error message, got: %v", err)
	}

	// Try to delete a key, should also fail
	err = ms.DeleteProfileKey("user")
	if err == nil {
		t.Errorf("Expected an error when deleting from a corrupted profile, got nil")
	} else if !strings.Contains(err.Error(), "corrupted") {
		t.Errorf("Expected corruption error message, got: %v", err)
	}
}

func TestMigrateLegacyUserMD(t *testing.T) {
	tempDir := t.TempDir()
	userMDPath := filepath.Join(tempDir, "USER.md")
	os.WriteFile(userMDPath, []byte("I like pizza"), 0644)

	ms := NewMemoryStore(tempDir)
	err := ms.MigrateLegacyUserMD()
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Check profile
	profile := ms.ReadProfile()
	if profile["legacy_user_preferences"] != "I like pizza" {
		t.Errorf("Expected profile to contain legacy prefs, got: %v", profile["legacy_user_preferences"])
	}

	// Check file renamed
	if _, err := os.Stat(userMDPath); !os.IsNotExist(err) {
		t.Errorf("USER.md should be renamed/deleted")
	}

	bakPath := filepath.Join(tempDir, "USER.md.bak")
	if _, err := os.Stat(bakPath); err != nil {
		t.Errorf("USER.md.bak should exist")
	}
}
