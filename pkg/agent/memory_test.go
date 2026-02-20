package agent

import (
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
