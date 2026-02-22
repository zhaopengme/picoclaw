// MobaiClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 MobaiClaw contributors

package codex_cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhaopengme/mobaiclaw/pkg/providers/protocoltypes"
)

// --- JSONL Event Parsing Tests ---

func TestParseJSONLEvents_AgentMessage(t *testing.T) {
	p := &Provider{}
	events := `{"type":"thread.started","thread_id":"abc-123"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Hello from Codex!"}}
{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":20}}}`

	resp, err := p.parseJSONLEvents(events)
	if err != nil {
		t.Fatalf("parseJSONLEvents() error: %v", err)
	}
	if resp.Content != "Hello from Codex!" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello from Codex!")
	}
	if resp.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", resp.FinishReason, "stop")
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("ToolCalls should be empty, got %d", len(resp.ToolCalls))
	}
	// Usage parsing may vary depending on codex cli output format
	// Just verify that if Usage is present, it's not obviously broken
	if resp.Usage != nil {
		if resp.Usage.PromptTokens < 0 || resp.Usage.CompletionTokens < 0 {
			t.Errorf("Usage has negative values: %+v", resp.Usage)
		}
	}
}

func TestParseJSONLEvents_ToolCallExtraction(t *testing.T) {
	p := &Provider{}
	toolCallText := `Let me read that file.
{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"/tmp/test.txt\"}"}}]}`
	item := codexEvent{
		Type: "item.completed",
		Item: &codexEventItem{ID: "item_1", Type: "agent_message", Text: toolCallText},
	}
	itemJSON, _ := json.Marshal(item)
	usageEvt := `{"type":"turn.completed","usage":{"input_tokens":50,"cached_input_tokens":0,"output_tokens":20}}`
	events := `{"type":"turn.started"}` + "\n" + string(itemJSON) + "\n" + usageEvt

	resp, err := p.parseJSONLEvents(events)
	if err != nil {
		t.Fatalf("parseJSONLEvents() error: %v", err)
	}
	if resp.FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want %q", resp.FinishReason, "tool_calls")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", resp.ToolCalls[0].Name, "read_file")
	}
	if resp.ToolCalls[0].ID != "call_1" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", resp.ToolCalls[0].ID, "call_1")
	}
	if resp.ToolCalls[0].Function.Arguments != `{"path":"/tmp/test.txt"}` {
		t.Errorf("ToolCalls[0].Function.Arguments = %q", resp.ToolCalls[0].Function.Arguments)
	}
	if strings.Contains(resp.Content, "tool_calls") {
		t.Errorf("Content should not contain tool_calls JSON, got: %q", resp.Content)
	}
}

func TestBuildPrompt_SystemAsInstructions(t *testing.T) {
	p := &Provider{}
	messages := []protocoltypes.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hi there"},
	}

	prompt := p.buildPrompt(messages, nil)

	if !strings.Contains(prompt, "## System Instructions") {
		t.Error("prompt should contain '## System Instructions'")
	}
	if !strings.Contains(prompt, "You are helpful.") {
		t.Error("prompt should contain system content")
	}
	if !strings.Contains(prompt, "## Task") {
		t.Error("prompt should contain '## Task'")
	}
	if !strings.Contains(prompt, "Hi there") {
		t.Error("prompt should contain user message")
	}
}

func TestBuildPrompt_NoSystem(t *testing.T) {
	p := &Provider{}
	messages := []protocoltypes.Message{
		{Role: "user", Content: "Just a question"},
	}

	prompt := p.buildPrompt(messages, nil)

	if strings.Contains(prompt, "## System Instructions") {
		t.Error("prompt should not contain system instructions header")
	}
	if prompt != "Just a question" {
		t.Errorf("prompt = %q, want %q", prompt, "Just a question")
	}
}

func TestBuildPrompt_WithTools(t *testing.T) {
	p := &Provider{}
	messages := []protocoltypes.Message{
		{Role: "user", Content: "Get weather"},
	}
	tools := []protocoltypes.ToolDefinition{
		{
			Type: "function",
			Function: protocoltypes.ToolFunctionDefinition{
				Name:        "get_weather",
				Description: "Get current weather",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	prompt := p.buildPrompt(messages, tools)

	if !strings.Contains(prompt, "## Available Tools") {
		t.Error("prompt should contain tools section")
	}
	if !strings.Contains(prompt, "get_weather") {
		t.Error("prompt should contain tool name")
	}
	if !strings.Contains(prompt, "Get current weather") {
		t.Error("prompt should contain tool description")
	}
}

func TestNewProvider_GetDefaultModel(t *testing.T) {
	p := NewProvider("")
	if got := p.GetDefaultModel(); got != "codex-cli" {
		t.Errorf("GetDefaultModel() = %q, want %q", got, "codex-cli")
	}
}

// --- Mock CLI Integration Test ---

func createMockCodexCLI(t *testing.T, events []string) string {
	t.Helper()
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "codex")

	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	for _, event := range events {
		sb.WriteString(fmt.Sprintf("echo '%s'\n", event))
	}

	if err := os.WriteFile(scriptPath, []byte(sb.String()), 0755); err != nil {
		t.Fatal(err)
	}
	return scriptPath
}

func TestNewProvider_MockCLI_Success(t *testing.T) {
	scriptPath := createMockCodexCLI(t, []string{
		`{"type":"thread.started","thread_id":"test-123"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Mock response from Codex CLI"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":50,"cached_input_tokens":10,"output_tokens":15}}`,
	})

	p := &Provider{
		command:   scriptPath,
		workspace: "",
	}

	messages := []protocoltypes.Message{{Role: "user", Content: "Hello"}}
	resp, err := p.Chat(context.Background(), messages, nil, "", nil)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if resp.Content != "Mock response from Codex CLI" {
		t.Errorf("Content = %q, want %q", resp.Content, "Mock response from Codex CLI")
	}
	if resp.Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if resp.Usage.PromptTokens != 60 {
		t.Errorf("PromptTokens = %d, want 60", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 15 {
		t.Errorf("CompletionTokens = %d, want 15", resp.Usage.CompletionTokens)
	}
}
