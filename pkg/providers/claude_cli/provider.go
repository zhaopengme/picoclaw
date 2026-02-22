// MobaiClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 MobaiClaw contributors

package claude_cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/zhaopengme/mobaiclaw/pkg/providers/protocoltypes"
)

type Provider struct {
	command   string
	workspace string
}

func NewProvider(workspace string) *Provider {
	return &Provider{
		command:   "claude",
		workspace: workspace,
	}
}

func (p *Provider) Chat(ctx context.Context, messages []protocoltypes.Message, tools []protocoltypes.ToolDefinition, model string, options map[string]interface{}) (*protocoltypes.LLMResponse, error) {
	systemPrompt := p.buildSystemPrompt(messages, tools)
	prompt := p.messagesToPrompt(messages)

	args := []string{"-p", "--output-format", "json", "--dangerously-skip-permissions", "--no-chrome"}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}
	if model != "" && model != "claude-code" {
		args = append(args, "--model", model)
	}
	args = append(args, "-")

	cmd := exec.CommandContext(ctx, p.command, args...)
	if p.workspace != "" {
		cmd.Dir = p.workspace
	}
	cmd.Stdin = bytes.NewReader([]byte(prompt))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderrStr := stderr.String(); stderrStr != "" {
			return nil, fmt.Errorf("claude cli error: %s", stderrStr)
		}
		return nil, fmt.Errorf("claude cli error: %w", err)
	}

	return p.parseResponse(stdout.String())
}

func (p *Provider) GetDefaultModel() string {
	return "claude-code"
}

func (p *Provider) messagesToPrompt(messages []protocoltypes.Message) string {
	var parts []string

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			// handled via --system-prompt flag
		case "user":
			parts = append(parts, "User: "+msg.Content)
		case "assistant":
			parts = append(parts, "Assistant: "+msg.Content)
		case "tool":
			parts = append(parts, fmt.Sprintf("[Tool Result for %s]: %s", msg.ToolCallID, msg.Content))
		}
	}

	if len(parts) == 1 && strings.HasPrefix(parts[0], "User: ") {
		return strings.TrimPrefix(parts[0], "User: ")
	}

	return strings.Join(parts, "\n")
}

func (p *Provider) buildSystemPrompt(messages []protocoltypes.Message, tools []protocoltypes.ToolDefinition) string {
	var parts []string

	for _, msg := range messages {
		if msg.Role == "system" {
			parts = append(parts, msg.Content)
		}
	}

	if len(tools) > 0 {
		parts = append(parts, p.buildToolsPrompt(tools))
	}

	return strings.Join(parts, "\n\n")
}

func (p *Provider) buildToolsPrompt(tools []protocoltypes.ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("When you need to use a tool, respond with ONLY a JSON object:\n\n")
	sb.WriteString("```json\n")
	sb.WriteString(`{"tool_calls":[{"id":"call_xxx","type":"function","function":{"name":"tool_name","arguments":"{...}"}}]}`)
	sb.WriteString("\n```\n\n")
	sb.WriteString("CRITICAL: The 'arguments' field MUST be a JSON-encoded STRING.\n\n")
	sb.WriteString("### Tool Definitions:\n\n")

	for _, tool := range tools {
		if tool.Type != "function" {
			continue
		}
		sb.WriteString(fmt.Sprintf("#### %s\n", tool.Function.Name))
		if tool.Function.Description != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n", tool.Function.Description))
		}
		if len(tool.Function.Parameters) > 0 {
			paramsJSON, _ := json.Marshal(tool.Function.Parameters)
			sb.WriteString(fmt.Sprintf("Parameters:\n```json\n%s\n```\n", string(paramsJSON)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

type jsonResponse struct {
	Type         string     `json:"type"`
	Subtype      string     `json:"subtype"`
	IsError      bool       `json:"is_error"`
	Result       string     `json:"result"`
	SessionID    string     `json:"session_id"`
	TotalCostUSD float64    `json:"total_cost_usd"`
	DurationMS   int        `json:"duration_ms"`
	DurationAPI  int        `json:"duration_api_ms"`
	NumTurns     int        `json:"num_turns"`
	Usage        usageInfo  `json:"usage"`
}

type usageInfo struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

func (p *Provider) parseResponse(output string) (*protocoltypes.LLMResponse, error) {
	var resp jsonResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse claude cli response: %w", err)
	}

	if resp.IsError {
		return nil, fmt.Errorf("claude cli returned error: %s", resp.Result)
	}

	toolCalls := protocoltypes.ExtractToolCallsFromText(resp.Result)

	finishReason := "stop"
	content := resp.Result
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
		content = protocoltypes.StripToolCallsFromText(resp.Result)
	}

	var usage *protocoltypes.UsageInfo
	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		usage = &protocoltypes.UsageInfo{
			PromptTokens:     resp.Usage.InputTokens + resp.Usage.CacheCreationInputTokens + resp.Usage.CacheReadInputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.CacheCreationInputTokens + resp.Usage.CacheReadInputTokens + resp.Usage.OutputTokens,
		}
	}

	return &protocoltypes.LLMResponse{
		Content:      strings.TrimSpace(content),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}
