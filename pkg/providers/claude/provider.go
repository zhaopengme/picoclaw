// MobaiClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 MobaiClaw contributors

package claude

import (
	"context"

	anthropicprovider "github.com/zhaopengme/mobaiclaw/pkg/providers/anthropic"
	"github.com/zhaopengme/mobaiclaw/pkg/providers/protocoltypes"
)

type Provider struct {
	delegate *anthropicprovider.Provider
}

func NewProvider(token string) *Provider {
	return &Provider{
		delegate: anthropicprovider.NewProvider(token),
	}
}

func NewProviderWithBaseURL(token, apiBase string) *Provider {
	return &Provider{
		delegate: anthropicprovider.NewProviderWithBaseURL(token, apiBase),
	}
}

func NewProviderWithTokenSource(token string, tokenSource func() (string, error)) *Provider {
	return &Provider{
		delegate: anthropicprovider.NewProviderWithTokenSource(token, tokenSource),
	}
}

func NewProviderWithTokenSourceAndBaseURL(token string, tokenSource func() (string, error), apiBase string) *Provider {
	return &Provider{
		delegate: anthropicprovider.NewProviderWithTokenSourceAndBaseURL(token, tokenSource, apiBase),
	}
}

// NewProviderWithDelegate creates a Provider from an existing anthropic.Provider delegate.
// Exported for testing purposes.
func NewProviderWithDelegate(delegate *anthropicprovider.Provider) *Provider {
	return &Provider{delegate: delegate}
}

func (p *Provider) Chat(ctx context.Context, messages []protocoltypes.Message, tools []protocoltypes.ToolDefinition, model string, options map[string]interface{}) (*protocoltypes.LLMResponse, error) {
	resp, err := p.delegate.Chat(ctx, messages, tools, model, options)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (p *Provider) GetDefaultModel() string {
	return p.delegate.GetDefaultModel()
}
