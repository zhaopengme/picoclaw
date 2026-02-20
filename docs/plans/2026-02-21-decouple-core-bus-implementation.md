# Decouple Core & Bus Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Decouple the `AgentLoop` from the concrete `MessageBus` and `ChannelManager` by introducing a `Broker` interface and a `CommandGateway`.

**Architecture:** First, we abstract the `MessageBus` into a `Broker` interface, replacing concrete pointers across the project. Then, we create a `CommandGateway` to intercept `/` commands, allowing us to strip command-handling and `ChannelManager` dependencies out of `AgentLoop`.

**Tech Stack:** Go

---

### Task 1: Create Broker Interfaces

**Files:**
- Create: `pkg/bus/interfaces.go`
- Modify: `pkg/bus/bus.go`

**Step 1: Write the failing test**

We don't need a specific test just for defining an interface, but we will write a quick type-assertion test to ensure the existing `MessageBus` implements the new `Broker` interface.
Create `pkg/bus/interfaces_test.go`:
```go
package bus

import (
	"testing"
)

func TestMessageBusImplementsBroker(t *testing.T) {
	var _ Broker = (*MessageBus)(nil)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/bus -run TestMessageBusImplementsBroker -v`
Expected: FAIL with "undefined: Broker"

**Step 3: Write minimal implementation**

Create `pkg/bus/interfaces.go`:
```go
package bus

import "context"

type Publisher interface {
	PublishInbound(InboundMessage)
	PublishOutbound(OutboundMessage)
}

type Subscriber interface {
	ConsumeInbound(context.Context) (InboundMessage, bool)
	SubscribeOutbound(context.Context) (OutboundMessage, bool)
}

type Broker interface {
	Publisher
	Subscriber
	RegisterHandler(channel string, handler MessageHandler)
	GetHandler(channel string) (MessageHandler, bool)
	Close()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/bus -run TestMessageBusImplementsBroker -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/bus/interfaces.go pkg/bus/interfaces_test.go
git commit -m "feat: define Broker interfaces for MessageBus abstraction"
```

---

### Task 2: Refactor Agent to Use Broker Interface

**Files:**
- Modify: `pkg/agent/loop.go`
- Modify: `pkg/agent/agent.go` (if it passes bus)

**Step 1: Write the failing build (No test needed, just compile failure)**

In `pkg/agent/loop.go`, change the `bus` field type from `*bus.MessageBus` to `bus.Broker`.
```go
type AgentLoop struct {
	agent   *Agent
	bus     bus.Broker
	// ...
}

func NewAgentLoop(cfg *config.Config, b bus.Broker, provider providers.Provider) *AgentLoop {
    // ...
}
```

**Step 2: Run build to verify it fails**

Run: `go build ./pkg/agent`
Expected: Likely no failures yet if the methods match, but we need to ensure anywhere `*bus.MessageBus` was passed into `AgentLoop` or tools is updated. Let's update `Agent` struct if it holds it.

**Step 3: Write minimal implementation**

Also update `cmd/picoclaw/cmd_gateway.go` and `cmd/picoclaw/cmd_agent.go` where `NewAgentLoop` is called, ensuring we pass `msgBus` (which implements `bus.Broker`).

Update `pkg/agent/tools/spawn.go` and `pkg/agent/tools/message.go` to accept `bus.Broker` instead of `*bus.MessageBus`.

**Step 4: Run build to verify it passes**

Run: `go build ./pkg/agent ./pkg/tools ./cmd/picoclaw`
Expected: Compiles successfully (or you fix the remaining `*bus.MessageBus` references in tools/agent).

**Step 5: Commit**

```bash
git add pkg/agent/ pkg/tools/ cmd/picoclaw/
git commit -m "refactor: update Agent and Tools to depend on bus.Broker interface"
```

---

### Task 3: Refactor Channels to Use Broker Interface

**Files:**
- Modify: `pkg/channels/base.go`
- Modify: `pkg/channels/manager.go`

**Step 1: Write the failing build**

In `pkg/channels/base.go`:
```go
type BaseChannel struct {
	config    interface{}
	bus       bus.Broker
    // ...
}

func NewBaseChannel(name string, config interface{}, b bus.Broker, allowList []string) *BaseChannel {
    // ...
}
```

In `pkg/channels/manager.go`:
```go
type Manager struct {
	config   *config.Config
	bus      bus.Broker
    // ...
}

func NewManager(cfg *config.Config, b bus.Broker) *Manager {
    // ...
}
```

**Step 2: Run build to verify it fails**

Run: `go build ./pkg/channels`
Expected: Failures in individual channel implementations (e.g. `telegram.go`, `discord.go`) where `NewBaseChannel` is called.

**Step 3: Write minimal implementation**

Update all channel constructors (e.g., `NewTelegramChannel(cfg *config.Config, b bus.Broker)`) to take `bus.Broker` and pass it to `NewBaseChannel`.
You will need to search and replace `*bus.MessageBus` with `bus.Broker` in all files inside `pkg/channels/`.

**Step 4: Run build to verify it passes**

Run: `go build ./pkg/channels`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/channels/
git commit -m "refactor: update Channels to depend on bus.Broker interface"
```

---

### Task 4: Extract CommandGateway

**Files:**
- Create: `pkg/gateway/gateway.go`
- Create: `pkg/gateway/gateway_test.go`

**Step 1: Write the failing test**

Create `pkg/gateway/gateway_test.go`:
```go
package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
)

// MockBroker and MockChannelManager needed here, or just test logic.
// We will test that Gateway routes commands properly.
func TestGatewayRoutesCommand(t *testing.T) {
	// A simple test showing gateway creation exists
	g := NewCommandGateway(nil, nil)
	if g == nil {
		t.Fatal("expected gateway")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/gateway -v`
Expected: FAIL "undefined: NewCommandGateway"

**Step 3: Write minimal implementation**

Create `pkg/gateway/gateway.go`:
```go
package gateway

import (
	"context"
	"strings"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/logger"
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/gateway -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/gateway/
git commit -m "feat: introduce CommandGateway structure"
```

---

### Task 5: Move Command Logic and Purify AgentLoop

**Files:**
- Modify: `pkg/agent/loop.go`
- Modify: `pkg/gateway/gateway.go`
- Modify: `cmd/picoclaw/cmd_gateway.go`

**Step 1: Write minimal implementation**

1. In `pkg/agent/loop.go`:
   - Delete `SetChannelManager(cm *channels.Manager)`
   - Delete `handleCommand(ctx context.Context, msg bus.InboundMessage)` and its sub-methods (`handleShow`, `handleSwitch`, `handleList`, etc.)
   - Ensure `processMessage` no longer checks for `strings.HasPrefix(msg.Content, "/")`.
   - Remove `ChannelManager` field from `AgentLoop`.

2. In `pkg/gateway/gateway.go`:
   - Paste the `handleCommand` logic (and `handleShow`, `handleList`, etc.) that was deleted from `AgentLoop`.
   - Update references to use `g.channelManager` instead of `al.channelManager`.
   - Update references to use `g.bus.PublishOutbound` instead of `al.bus.PublishOutbound`.

3. In `cmd/picoclaw/cmd_gateway.go`:
   - Create *two* buses: `mainBus := bus.NewMessageBus()` and `agentBus := bus.NewMessageBus()`.
   - Pass `mainBus` to `channels.NewManager`.
   - Pass `agentBus` to `agent.NewAgentLoop`.
   - Create Gateway: `gw := gateway.NewCommandGateway(mainBus, agentBus, channelManager)`.
   - Start Gateway: `go gw.Run(ctx)`.
   - Remove `agentLoop.SetChannelManager(channelManager)`.

**Step 2: Check compilation**

Run: `go build ./cmd/picoclaw`
Expected: You will likely need to fix imports and minor syntax adjustments in `gateway.go` to match the moved logic.

**Step 3: Fix compilation errors**

Iteratively run `go build ./...` and fix any missing imports (e.g. `fmt`, `strings`, `pkg/config`) in `gateway.go` until the build succeeds.

**Step 4: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/agent/loop.go pkg/gateway/gateway.go cmd/picoclaw/cmd_gateway.go
git commit -m "refactor: purify AgentLoop and move command logic to CommandGateway"
```
