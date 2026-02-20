// pkg/agent/context_test.go
package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBootstrapFiles_NoUserMD(t *testing.T) {
	tempDir := t.TempDir()
	// Create dummy files
	os.WriteFile(filepath.Join(tempDir, "AGENTS.md"), []byte("agents content"), 0644)
	os.WriteFile(filepath.Join(tempDir, "USER.md"), []byte("legacy user prefs"), 0644)

	cb := NewContextBuilder(tempDir, NewMemoryStore(tempDir))
	content := cb.LoadBootstrapFiles()

	if strings.Contains(content, "legacy user prefs") {
		t.Errorf("LoadBootstrapFiles should not load USER.md into context")
	}
	if !strings.Contains(content, "agents content") {
		t.Errorf("LoadBootstrapFiles should load AGENTS.md")
	}
}
