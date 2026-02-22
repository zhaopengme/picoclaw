package gateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/zhaopengme/mobaiclaw/pkg/agent"
	"github.com/zhaopengme/mobaiclaw/pkg/bus"
	"github.com/zhaopengme/mobaiclaw/pkg/channels"
	"github.com/zhaopengme/mobaiclaw/pkg/providers"
	"github.com/zhaopengme/mobaiclaw/pkg/session"
)

type CommandGateway struct {
	bus            bus.Broker
	channelManager *channels.Manager
	agentBus       bus.Broker           // for forwarding to agent
	agentRegistry  *agent.AgentRegistry // to fetch models
	sessions       *session.SessionManager
}

func NewCommandGateway(b bus.Broker, agentBus bus.Broker, cm *channels.Manager, registry *agent.AgentRegistry, sessions *session.SessionManager) *CommandGateway {
	return &CommandGateway{
		bus:            b,
		agentBus:       agentBus,
		channelManager: cm,
		agentRegistry:  registry,
		sessions:       sessions,
	}
}

func (g *CommandGateway) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Forward outbound messages from agent back to channels
	if g.agentBus != nil {
		go func() {
			for {
				outMsg, ok := g.agentBus.SubscribeOutbound(ctx)
				if !ok {
					return
				}
				g.bus.PublishOutbound(outMsg)
			}
		}()
	}

	for {
		msg, ok := g.bus.ConsumeInbound(ctx)
		if !ok {
			return nil
		}

		if strings.HasPrefix(msg.Content, "/") {
			if response, handled := g.handleCommand(ctx, msg); handled {
				if response != "" {
					g.bus.PublishOutbound(bus.OutboundMessage{
						Channel: msg.Channel,
						ChatID:  msg.ChatID,
						Content: response,
					})
				}
			} else {
				// Forward unhandled commands to pure Agent Loop just in case
				if g.agentBus != nil {
					g.agentBus.PublishInbound(msg)
				}
			}
		} else {
			// Forward to pure Agent Loop
			if g.agentBus != nil {
				g.agentBus.PublishInbound(msg)
			}
		}
	}
}

func (g *CommandGateway) handleCommand(ctx context.Context, msg bus.InboundMessage) (string, bool) {
	content := strings.TrimSpace(msg.Content)
	if !strings.HasPrefix(content, "/") {
		return "", false
	}

	parts := strings.Fields(content)
	if len(parts) == 0 {
		return "", false
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "/start":
		return "Hello! I am MobaiClaw 🦞", true

	case "/help":
		return `/start - Start the bot
/help - Show this help message
/show [model|channel|agents] - Show current configuration
/list [models|channels|agents] - List available options
/switch [model|channel] to <name> - Switch current model or channel`, true

	case "/clear":
		if g.sessions == nil {
			return "sessions not available", true
		}
		g.sessions.SetHistory(msg.SessionKey, []providers.Message{})
		g.sessions.SetSummary(msg.SessionKey, "")
		if err := g.sessions.Save(msg.SessionKey); err != nil {
			return fmt.Sprintf("failed to save session: %v", err), true
		}
		return "🧹 当前会话已清空，我们可以重新开始了。", true

	case "/show":
		if len(args) < 1 {
			return "Usage: /show [model|channel|agents]", true
		}
		switch args[0] {
		case "model":
			if g.agentRegistry == nil {
				return "No registry available", true
			}
			defaultAgent := g.agentRegistry.GetDefaultAgent()
			if defaultAgent == nil {
				return "No default agent configured", true
			}
			return fmt.Sprintf("Current model: %s", defaultAgent.Model), true
		case "channel":
			return fmt.Sprintf("Current channel: %s", msg.Channel), true
		case "agents":
			if g.agentRegistry == nil {
				return "No registry available", true
			}
			agentIDs := g.agentRegistry.ListAgentIDs()
			return fmt.Sprintf("Registered agents: %s", strings.Join(agentIDs, ", ")), true
		default:
			return fmt.Sprintf("Unknown show target: %s", args[0]), true
		}

	case "/list":
		if len(args) < 1 {
			return "Usage: /list [models|channels|agents]", true
		}
		switch args[0] {
		case "models":
			return "Available models: configured in config.json per agent", true
		case "channels":
			if g.channelManager == nil {
				return "Channel manager not initialized", true
			}
			channels := g.channelManager.GetEnabledChannels()
			if len(channels) == 0 {
				return "No channels enabled", true
			}
			return fmt.Sprintf("Enabled channels: %s", strings.Join(channels, ", ")), true
		case "agents":
			if g.agentRegistry == nil {
				return "No registry available", true
			}
			agentIDs := g.agentRegistry.ListAgentIDs()
			return fmt.Sprintf("Registered agents: %s", strings.Join(agentIDs, ", ")), true
		default:
			return fmt.Sprintf("Unknown list target: %s", args[0]), true
		}

	case "/switch":
		if len(args) < 3 || args[1] != "to" {
			return "Usage: /switch [model|channel] to <name>", true
		}
		target := args[0]
		value := args[2]

		switch target {
		case "model":
			if g.agentRegistry == nil {
				return "No registry available", true
			}
			defaultAgent := g.agentRegistry.GetDefaultAgent()
			if defaultAgent == nil {
				return "No default agent configured", true
			}
			oldModel := defaultAgent.Model
			defaultAgent.Model = value
			return fmt.Sprintf("Switched model from %s to %s", oldModel, value), true
		case "channel":
			if g.channelManager == nil {
				return "Channel manager not initialized", true
			}
			if _, exists := g.channelManager.GetChannel(value); !exists && value != "cli" {
				return fmt.Sprintf("Channel '%s' not found or not enabled", value), true
			}
			return fmt.Sprintf("Switched target channel to %s", value), true
		default:
			return fmt.Sprintf("Unknown switch target: %s", target), true
		}
	}

	return "", false
}
