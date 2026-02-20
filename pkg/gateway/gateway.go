package gateway

import (
	"context"
	"strings"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
)

type CommandGateway struct {
	bus            bus.Broker
	channelManager *channels.Manager
	agentBus       bus.Broker // for forwarding to agent
}

func NewCommandGateway(b bus.Broker, agentBus bus.Broker, cm *channels.Manager) *CommandGateway {
	return &CommandGateway{
		bus:            b,
		agentBus:       agentBus,
		channelManager: cm,
	}
}

func (g *CommandGateway) Run(ctx context.Context) error {
	for {
		msg, ok := g.bus.ConsumeInbound(ctx)
		if !ok {
			return nil
		}

		if strings.HasPrefix(msg.Content, "/") {
			g.handleCommand(ctx, msg)
		} else {
			// Forward to pure Agent Loop
			if g.agentBus != nil {
				g.agentBus.PublishInbound(msg)
			}
		}
	}
}

func (g *CommandGateway) handleCommand(ctx context.Context, msg bus.InboundMessage) {
	// We'll move the logic from AgentLoop here in the next task.
	// For now, just echo "Command received"
	g.bus.PublishOutbound(bus.OutboundMessage{
		Channel: msg.Channel,
		ChatID:  msg.ChatID,
		Content: "Command acknowledged by Gateway",
	})
}
