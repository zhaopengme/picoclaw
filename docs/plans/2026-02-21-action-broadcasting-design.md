# Action Status Broadcasting Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Provide real-time feedback in chat interfaces (like Telegram) when the agent decides to execute a long-running tool, replacing the static "Thinking... 💭" placeholder with the current action (e.g., "⚙️ 正在执行: web_search") without spamming the chat history.

**Architecture:** We will extend the internal event bus `OutboundMessage` to support `Metadata`. When `AgentLoop` encounters tool calls, it will publish a status update message with `Metadata: {"status_update": "true"}` before executing the tool. The Telegram channel's `Send` method will intercept these status updates and `EditMessageText` on the existing placeholder bubble, but crucially, it will *not* delete the placeholder ID from memory until a final, non-status message arrives.

**Tech Stack:** Go, internal event bus (`pkg/bus`), Telegram API (`pkg/channels/telegram.go`), Agent Loop (`pkg/agent/loop.go`).

---

### Task 1: Extend OutboundMessage with Metadata

**Files:**
- Modify: `pkg/bus/types.go`

**Step 1: Write the failing test**
(Skipped as this is a simple struct field addition)

**Step 2: Write minimal implementation**
In `pkg/bus/types.go`, add `Metadata map[string]string` to `OutboundMessage`:
```go
type OutboundMessage struct {
	Channel  string            `json:"channel"`
	ChatID   string            `json:"chat_id"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}
```

**Step 3: Commit**
```bash
git add pkg/bus/types.go
git commit -m "feat(bus): add Metadata to OutboundMessage to support status signals"
```

### Task 2: Update Telegram placeholder logic to respect status updates

**Files:**
- Modify: `pkg/channels/telegram.go` (in the `Send` method, approx line 173)

**Step 1: Write the failing test**
(Skipped due to external API dependency mocking complexity)

**Step 2: Write minimal implementation**
In `pkg/channels/telegram.go`, inside `Send(...)`:
Check if it's a status update:
```go
	isStatusUpdate := msg.Metadata != nil && msg.Metadata["status_update"] == "true"
```
Modify the placeholder deletion block (around `if pID, ok := c.placeholders.Load(msg.ChatID); ok {`):
```go
			if pID, ok := c.placeholders.Load(msg.ChatID); ok {
				// Only remove the placeholder if this is the final message,
				// allowing status updates to reuse the same bubble multiple times.
				if !isStatusUpdate {
					c.placeholders.Delete(msg.ChatID)
				}

				editMsg := &telego.EditMessageTextParams{
```

**Step 3: Commit**
```bash
git add pkg/channels/telegram.go
git commit -m "feat(telegram): support persistent placeholders for status updates"
```

### Task 3: Broadcast status updates from AgentLoop

**Files:**
- Modify: `pkg/agent/loop.go` (in `runLLMIteration`, before tool execution loop)

**Step 1: Write the failing test**
(Skipped as it involves bus event side-effects)

**Step 2: Write minimal implementation**
In `pkg/agent/loop.go`, before `// Execute tool calls` inside `runLLMIteration` (approx line 630):
Add a broadcast loop that announces the tools being executed:
```go
		// Broadcast status update to channel before running potentially slow tools
		if len(normalizedToolCalls) > 0 && !constants.IsInternalChannel(opts.Channel) {
			var toolNamesDisplay []string
			for _, tc := range normalizedToolCalls {
				toolNamesDisplay = append(toolNamesDisplay, tc.Name)
			}
			
			statusMsg := fmt.Sprintf("⚙️ 正在执行: %s...", strings.Join(toolNamesDisplay, ", "))
			al.bus.PublishOutbound(bus.OutboundMessage{
				Channel:  opts.Channel,
				ChatID:   opts.ChatID,
				Content:  statusMsg,
				Metadata: map[string]string{"status_update": "true"},
			})
		}
```

**Step 3: Commit**
```bash
git add pkg/agent/loop.go
git commit -m "feat(agent): broadcast tool execution status to channels"
```

