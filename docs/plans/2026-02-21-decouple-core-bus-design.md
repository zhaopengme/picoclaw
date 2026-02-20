# Decouple Core & Bus via Command Gateway

## Overview
This document outlines a structural refactoring of the `picoclaw` architecture to address tight coupling and God-Object anti-patterns identified in the `AgentLoop`. The primary goals are to abstract the `MessageBus` into an interface and introduce a `CommandGateway` to handle application-level commands, freeing the `AgentLoop` to be a pure domain engine for LLM interactions.

## Identified Anti-Patterns
1. **God Object (`AgentLoop`)**: `AgentLoop` handles LLM processing, tool dispatching, token compression, *and* command routing (`/list`, `/show`).
2. **Violation of Dependency Inversion**: `MessageBus` is a concrete struct. All modules (`channels`, `agent`, `tools`) depend on this concrete implementation, making it hard to swap for distributed brokers (e.g., Redis).
3. **Circular Dependency Bypass**: The `AgentLoop` holds a direct reference to the `ChannelManager` (injected via `SetChannelManager`), bypassing the bus to handle administrative commands.

## Architecture Solution: Mediator/Gateway

### 1. Abstracting the Message Bus
We will introduce interfaces for the message broker in `pkg/bus/interfaces.go`:
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
}
```
The existing `MessageBus` struct will be renamed to `MemoryBus` and will implement the `Broker` interface. All usages of `*bus.MessageBus` across `channels`, `agent`, and `tools` will be replaced with `bus.Broker`.

### 2. Introducing the Command Gateway
We will create a new layer (e.g., in `cmd/picoclaw` or a new `pkg/gateway`) called `CommandGateway`.
This gateway acts as the mediator between the raw `Broker` and the domain logic.

**Responsibilities of `CommandGateway`:**
- Subscribes to `Broker.ConsumeInbound()`.
- Inspects incoming messages. If a message is an administrative command (e.g., starts with `/` like `/list`, `/show`, `/switch`), the Gateway handles it directly by calling the `ChannelManager` or other application services.
- If the message is a regular chat message intended for the LLM, the Gateway forwards it to the `AgentLoop`.

### 3. Purifying the AgentLoop
With the Gateway handling commands:
- We will completely remove the `ChannelManager` dependency from `AgentLoop`.
- Delete `AgentLoop.SetChannelManager`.
- Delete all `handleCommand` methods (`/list`, `/show`, `/help`, etc.) from `pkg/agent/loop.go`.
- The `AgentLoop` becomes a pure engine that only processes text and tool calls using the LLM provider.

## Data Flow Comparison
**Old Flow:**
`Channel` -> `ConcreteBus` -> `AgentLoop` -> (Checks if Command) -> calls `ChannelManager` OR calls `LLM` -> `ConcreteBus` -> `Channel`

**New Flow:**
`Channel` -> `Broker(Interface)` -> **`CommandGateway`**
  -> (If Command) -> calls `ChannelManager` -> `Broker` -> `Channel`
  -> (If Chat) -> `AgentLoop` (Pure LLM Logic) -> `Broker` -> `Channel`

## Expected Outcomes
- **Testability**: The `Broker` interface allows mocking the bus for unit tests.
- **Maintainability**: `AgentLoop` shrinks significantly, focusing only on core AI logic (Single Responsibility Principle).
- **Extensibility**: Commands can be added or modified in the Gateway without touching the sensitive LLM iteration loop.
