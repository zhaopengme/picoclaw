// MobaiClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 MobaiClaw contributors

package codex_cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadCredentials(t *testing.T) {
	// Create a temporary auth.json file
	tmpDir := t.TempDir()
	authPath := filepath.Join(tmpDir, "auth.json")
	content := `{"tokens":{"access_token":"test-token","refresh_token":"test-refresh","account_id":"acc-123"}}`
	if err := os.WriteFile(authPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Set CODEX_HOME to point to temp dir
	oldHome := os.Getenv("CODEX_HOME")
	t.Cleanup(func() { os.Setenv("CODEX_HOME", oldHome) })
	os.Setenv("CODEX_HOME", tmpDir)

	accessToken, accountID, expiresAt, err := ReadCredentials()
	if err != nil {
		t.Fatalf("ReadCredentials() error: %v", err)
	}
	if accessToken != "test-token" {
		t.Errorf("AccessToken = %q, want %q", accessToken, "test-token")
	}
	if accountID != "acc-123" {
		t.Errorf("AccountID = %q, want %q", accountID, "acc-123")
	}
	// expiresAt should be approximately now + 1 hour
	expectedExpiry := time.Now().Add(time.Hour)
	diff := expiresAt.Sub(expectedExpiry)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("ExpiresAt = %v, want approximately %v", expiresAt, expectedExpiry)
	}
}

func TestCreateTokenSource(t *testing.T) {
	// This test just verifies the function returns a non-nil function
	ts := CreateTokenSource()
	if ts == nil {
		t.Fatal("CreateTokenSource() returned nil")
	}
	// Calling it without a valid auth.json will error, but we just check it doesn't crash
	_, _, err := ts()
	if err == nil {
		t.Error("Expected error when no auth.json exists, got nil")
	}
}
