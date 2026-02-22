package gateway

import (
	"context"
	"strings"
	"testing"

	"github.com/zhaopengme/mobaiclaw/pkg/agent"
	"github.com/zhaopengme/mobaiclaw/pkg/bus"
	"github.com/zhaopengme/mobaiclaw/pkg/config"
	"github.com/zhaopengme/mobaiclaw/pkg/providers"
)

func TestGatewayRoutesCommand(t *testing.T) {
	g := NewCommandGateway(nil, nil, nil, nil, nil)
	if g == nil {
		t.Fatal("expected gateway")
	}
}

// newTestRegistry creates a minimal AgentRegistry with a default agent for testing.
func newTestRegistry(t *testing.T) *agent.AgentRegistry {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:     tmpDir,
				MaxTokens:     4096,
				ContextWindow: 32768,
			},
			List: []config.AgentConfig{
				{
					ID:      "main",
					Default: true,
				},
			},
		},
	}
	return agent.NewAgentRegistry(cfg, &nullProvider{})
}

// nullProvider is a no-op LLM provider for testing.
type nullProvider struct{}

func (p *nullProvider) Chat(_ context.Context, _ []providers.Message, _ []providers.ToolDefinition, _ string, _ map[string]interface{}) (*providers.LLMResponse, error) {
	return &providers.LLMResponse{Content: ""}, nil
}

func (p *nullProvider) GetDefaultModel() string { return "test" }

func TestHandleClearCommand(t *testing.T) {
	registry := newTestRegistry(t)

	// Get the default agent's session manager and pre-populate a session
	agentInst := registry.GetDefaultAgent()
	if agentInst == nil {
		t.Fatal("expected default agent")
	}

	// The session key that routing would produce for this agent
	sessionKey := "agent:main:main" // dm_scope=main by default
	sess := agentInst.Sessions.GetOrCreate(sessionKey)
	_ = sess

	testHistory := []providers.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}
	agentInst.Sessions.SetHistory(sessionKey, testHistory)
	agentInst.Sessions.SetSummary(sessionKey, "Test summary")

	// Verify initial state
	if len(agentInst.Sessions.GetHistory(sessionKey)) != 3 {
		t.Fatal("expected 3 messages in history")
	}

	g := &CommandGateway{
		agentRegistry: registry,
	}

	msg := bus.InboundMessage{
		Channel: "telegram",
		ChatID:  "123456",
		Content: "/clear",
		Metadata: map[string]string{
			"peer_kind": "direct",
			"peer_id":   "123456",
		},
	}

	response, handled := g.handleCommand(context.Background(), msg)

	if !handled {
		t.Error("Expected /clear command to be handled")
	}
	if response == "" {
		t.Error("Expected response to not be empty")
	}
	if strings.Contains(response, "failed") {
		t.Errorf("Expected success, got: %s", response)
	}

	// Verify: history should be cleared
	clearedHistory := agentInst.Sessions.GetHistory(sessionKey)
	if len(clearedHistory) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(clearedHistory))
	}

	// Verify: summary should be cleared
	clearedSummary := agentInst.Sessions.GetSummary(sessionKey)
	if clearedSummary != "" {
		t.Errorf("Expected empty summary after clear, got '%s'", clearedSummary)
	}
}

func TestHandleClearCommandWithNilRegistry(t *testing.T) {
	g := &CommandGateway{
		agentRegistry: nil,
	}

	msg := bus.InboundMessage{
		Channel: "telegram",
		ChatID:  "123456",
		Content: "/clear",
	}

	response, handled := g.handleCommand(context.Background(), msg)

	if !handled {
		t.Error("Expected /clear command to be handled")
	}
	if response != "agent registry not available" {
		t.Errorf("Expected 'agent registry not available', got '%s'", response)
	}
}

func TestHelpCommandIncludesClear(t *testing.T) {
	gw := &CommandGateway{}
	resp, handled := gw.handleCommand(context.Background(), bus.InboundMessage{Content: "/help"})
	if !handled {
		t.Fatal("/help should be handled")
	}
	if !strings.Contains(resp, "/clear") {
		t.Error("/help output should include /clear command")
	}
}

func TestHelpCommandIncludesReload(t *testing.T) {
	gw := &CommandGateway{}
	resp, handled := gw.handleCommand(context.Background(), bus.InboundMessage{Content: "/help"})
	if !handled {
		t.Fatal("/help should be handled")
	}
	if !strings.Contains(resp, "/reload") {
		t.Error("/help output should include /reload command")
	}
}
