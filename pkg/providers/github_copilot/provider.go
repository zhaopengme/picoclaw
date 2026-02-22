// MobaiClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 MobaiClaw contributors

package github_copilot

import (
	"context"
	"fmt"

	json "encoding/json"

	copilot "github.com/github/copilot-sdk/go"
	"github.com/zhaopengme/mobaiclaw/pkg/providers/protocoltypes"
)

type Provider struct {
	uri         string
	connectMode string
	session     *copilot.Session
}

func NewProvider(uri string, connectMode string, model string) (*Provider, error) {
	var session *copilot.Session
	if connectMode == "" {
		connectMode = "grpc"
	}
	switch connectMode {
	case "stdio":
		//todo
	case "grpc":
		client := copilot.NewClient(&copilot.ClientOptions{
			CLIUrl: uri,
		})
		if err := client.Start(context.Background()); err != nil {
			return nil, fmt.Errorf("Can't connect to Github Copilot, https://github.com/github/copilot-sdk/blob/main/docs/getting-started.md#connecting-to-an-external-cli-server for details")
		}
		defer client.Stop()
		session, _ = client.CreateSession(context.Background(), &copilot.SessionConfig{
			Model: model,
			Hooks: &copilot.SessionHooks{},
		})
	}

	return &Provider{
		uri:         uri,
		connectMode: connectMode,
		session:     session,
	}, nil
}

func (p *Provider) Chat(ctx context.Context, messages []protocoltypes.Message, tools []protocoltypes.ToolDefinition, model string, options map[string]interface{}) (*protocoltypes.LLMResponse, error) {
	type tempMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	out := make([]tempMessage, 0, len(messages))

	for _, msg := range messages {
		out = append(out, tempMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	fullcontent, _ := json.Marshal(out)

	content, _ := p.session.Send(ctx, copilot.MessageOptions{
		Prompt: string(fullcontent),
	})

	return &protocoltypes.LLMResponse{
		FinishReason: "stop",
		Content:      content,
	}, nil
}

func (p *Provider) GetDefaultModel() string {
	return "gpt-4.1"
}
