package gateway

import (
	"context"
	"os"
	"testing"

	"github.com/zhaopengme/mobaiclaw/pkg/bus"
	"github.com/zhaopengme/mobaiclaw/pkg/providers"
	"github.com/zhaopengme/mobaiclaw/pkg/session"
)

func TestGatewayRoutesCommand(t *testing.T) {
	g := NewCommandGateway(nil, nil, nil, nil, nil)
	if g == nil {
		t.Fatal("expected gateway")
	}
}

func TestHandleClearCommand(t *testing.T) {
	// Setup: Create temp directory for session storage
	tmpDir, err := os.MkdirTemp("", "session-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create SessionManager
	sm := session.NewSessionManager(tmpDir)

	// Create test history
	testSessionKey := "telegram:123456"
	testHistory := []providers.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	// Get or create the session first
	_ = sm.GetOrCreate(testSessionKey)

	// Add test messages and summary to session
	sm.SetHistory(testSessionKey, testHistory)
	sm.SetSummary(testSessionKey, "Test summary")
	if err := sm.Save(testSessionKey); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Verify initial state
	initialHistory := sm.GetHistory(testSessionKey)
	if len(initialHistory) != 3 {
		t.Fatalf("Expected 3 messages in history, got %d", len(initialHistory))
	}
	initialSummary := sm.GetSummary(testSessionKey)
	if initialSummary != "Test summary" {
		t.Fatalf("Expected summary 'Test summary', got '%s'", initialSummary)
	}

	// Create CommandGateway with sessions
	g := &CommandGateway{
		sessions: sm,
	}

	// Execute: Call handleCommand with "/clear" message
	msg := bus.InboundMessage{
		Channel:    "telegram",
		ChatID:     "123456",
		Content:    "/clear",
		SessionKey: testSessionKey,
	}

	response, handled := g.handleCommand(context.Background(), msg)

	// Verify: handled should be true
	if !handled {
		t.Error("Expected /clear command to be handled, but handled was false")
	}

	// Verify: response should not be empty
	if response == "" {
		t.Error("Expected response to not be empty, but got empty string")
	}

	// Verify: history should be cleared
	clearedHistory := sm.GetHistory(testSessionKey)
	if len(clearedHistory) != 0 {
		t.Errorf("Expected 0 messages in history after clear, got %d", len(clearedHistory))
	}

	// Verify: summary should be cleared
	clearedSummary := sm.GetSummary(testSessionKey)
	if clearedSummary != "" {
		t.Errorf("Expected empty summary after clear, got '%s'", clearedSummary)
	}
}

func TestHandleClearCommandWithNilSessions(t *testing.T) {
	// Create CommandGateway with nil sessions
	g := &CommandGateway{
		sessions: nil,
	}

	// Execute: Call handleCommand with "/clear" message
	msg := bus.InboundMessage{
		Channel:    "telegram",
		ChatID:     "123456",
		Content:    "/clear",
		SessionKey: "telegram:123456",
	}

	response, handled := g.handleCommand(context.Background(), msg)

	// Verify: handled should be true
	if !handled {
		t.Error("Expected /clear command to be handled, but handled was false")
	}

	// Verify: response should contain error message
	if response == "" {
		t.Error("Expected error response to not be empty, but got empty string")
	}

	// Verify: response should indicate sessions not available
	expectedErrMsg := "sessions not available"
	if response != expectedErrMsg {
		t.Errorf("Expected response '%s', got '%s'", expectedErrMsg, response)
	}
}
