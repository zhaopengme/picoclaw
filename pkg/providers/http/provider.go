// MobaiClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 MobaiClaw contributors

package httpprovider

import (
	"context"

	"github.com/zhaopengme/mobaiclaw/pkg/providers/openai_compat"
	"github.com/zhaopengme/mobaiclaw/pkg/providers/protocoltypes"
)

type Provider struct {
	delegate *openai_compat.Provider
}

func NewProvider(apiKey, apiBase, proxy string) *Provider {
	return &Provider{
		delegate: openai_compat.NewProvider(apiKey, apiBase, proxy),
	}
}

func NewProviderWithMaxTokensField(apiKey, apiBase, proxy, maxTokensField string) *Provider {
	return &Provider{
		delegate: openai_compat.NewProviderWithMaxTokensField(apiKey, apiBase, proxy, maxTokensField),
	}
}

func (p *Provider) Chat(ctx context.Context, messages []protocoltypes.Message, tools []protocoltypes.ToolDefinition, model string, options map[string]interface{}) (*protocoltypes.LLMResponse, error) {
	return p.delegate.Chat(ctx, messages, tools, model, options)
}

func (p *Provider) GetDefaultModel() string {
	return ""
}
