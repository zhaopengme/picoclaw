# /clear Command Migration to Gateway Layer

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move `/clear` command from Agent layer to Gateway layer for unified command management and improved efficiency.

**Architecture:**
- Inject `SessionManager` into `CommandGateway` to enable session operations
- Move `/clear` handling from `AgentLoop.ProcessMessage` to `CommandGateway.handleCommand`
- Remove `/clear` interception from Agent layer
- Update `/help` text to include `/clear`

**Tech Stack:** Go, session.SessionManager, bus.MessageBus

---

## Task 1: Add Sessions Field to CommandGateway

**Files:**
- Modify: `pkg/gateway/gateway.go:13-27`

**Step 1: Add Sessions field to struct**

```go
type CommandGateway struct {
    bus            bus.Broker
    channelManager *channels.Manager
    agentBus       bus.Broker           // for forwarding to agent
    agentRegistry  *agent.AgentRegistry // to fetch models
    sessions       *session.SessionManager // NEW: for /clear command
}
```

**Step 2: Update constructor to accept sessions parameter**

```go
func NewCommandGateway(b bus.Broker, agentBus bus.Broker, cm *channels.Manager, registry *agent.AgentRegistry, sessions *session.SessionManager) *CommandGateway {
    return &CommandGateway{
        bus:            b,
        agentBus:       agentBus,
        channelManager: cm,
        agentRegistry:  registry,
        sessions:       sessions, // NEW
    }
}
```

**Step 3: Add import for session package**

Add to imports at top of file:
```go
import (
    "context"
    "fmt"
    "strings"

    "github.com/zhaopengme/mobaiclaw/pkg/agent"
    "github.com/zhaopengme/mobaiclaw/pkg/bus"
    "github.com/zhaopengme/mobaiclaw/pkg/channels"
    "github.com/zhaopengme/mobaiclaw/pkg/session" // NEW
)
```

**Step 4: Commit**

```bash
git add pkg/gateway/gateway.go
git commit -m "refactor(gateway): add Sessions field to CommandGateway for session operations"
```

---

## Task 2: Add /clear Command Handler in Gateway

**Files:**
- Modify: `pkg/gateway/gateway.go:76-100` (handleCommand function)

**Step 1: Write failing test first**

Create test file `pkg/gateway/gateway_test.go`:

```go
package gateway

import (
    "context"
    "testing"

    "github.com/zhaopengme/mobaiclaw/pkg/bus"
    "github.com/zhaopengme/mobaiclaw/pkg/session"
)

func TestHandleClearCommand(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    sessions := session.NewSessionManager(tmpDir)
    sessions.AddHistory("test-session", "user", "some history")

    // Create gateway with sessions
    gw := &CommandGateway{
        sessions: sessions,
    }

    // Execute
    resp, handled := gw.handleCommand(context.Background(), bus.InboundMessage{
        Content:     "/clear",
        Channel:     "test",
        ChatID:      "test-chat",
        SessionKey:  "test-session",
    })

    // Verify
    if !handled {
        t.Fatal("/clear command should be handled")
    }
    if resp == "" {
        t.Fatal("response should not be empty")
    }

    // Verify history was cleared
    history := sessions.GetHistory("test-session")
    if len(history) != 0 {
        t.Errorf("history should be empty, got %d messages", len(history))
    }

    // Verify summary was cleared
    summary := sessions.GetSummary("test-session")
    if summary != "" {
        t.Errorf("summary should be empty, got %q", summary)
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/zhaopeng/projects/github/mobaiclaw
go test ./pkg/gateway/... -v -run TestHandleClearCommand
```

Expected: FAIL with `/clear` command not handled

**Step 3: Implement /clear handler in handleCommand**

Add case in `handleCommand` switch statement (after `/help` case):

```go
case "/clear":
    if g.sessions == nil {
        return "Session manager not available", true
    }

    // Clear history and summary for the current session
    g.sessions.SetHistory(msg.SessionKey, []providers.Message{})
    g.sessions.SetSummary(msg.SessionKey, "")
    g.sessions.Save(msg.SessionKey)

    // Return confirmation message
    return "🧹 当前会话已清空，我们可以重新开始了。", true
```

**Note:** Need to add `providers` import for `Message` type:

```go
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
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/gateway/... -v -run TestHandleClearCommand
```

Expected: PASS

**Step 5: Commit**

```bash
git add pkg/gateway/gateway.go pkg/gateway/gateway_test.go
git commit -m "feat(gateway): add /clear command handler"
```

---

## Task 3: Update /help Text to Include /clear

**Files:**
- Modify: `pkg/gateway/gateway.go:94-99`

**Step 1: Update /help case**

```go
case "/help":
    return `/start - Start the bot
/clear - Clear current session history and summary
/help - Show this help message
/show [model|channel|agents] - Show current configuration
/list [models|channels|agents] - List available options
/switch [model|channel] to <name> - Switch current model or channel`, true
```

**Step 2: Add test for /help output**

```go
func TestHelpCommandIncludesClear(t *testing.T) {
    gw := &CommandGateway{}

    resp, handled := gw.handleCommand(context.Background(), bus.InboundMessage{
        Content: "/help",
    })

    if !handled {
        t.Fatal("/help should be handled")
    }

    if !strings.Contains(resp, "/clear") {
        t.Error("/help output should include /clear command")
    }
}
```

**Step 3: Run test**

```bash
go test ./pkg/gateway/... -v -run TestHelpCommandIncludesClear
```

Expected: PASS

**Step 4: Commit**

```bash
git add pkg/gateway/gateway.go pkg/gateway/gateway_test.go
git commit -m "docs(gateway): add /clear to /help command output"
```

---

## Task 4: Remove /clear Handler from Agent Loop

**Files:**
- Modify: `pkg/agent/loop.go:307-326`

**Step 1: Remove /clear interception block**

Delete these lines:
```go
// Intercept built-in commands
if strings.ToLower(strings.TrimSpace(msg.Content)) == "/clear" {
    // Clear history and summary for the current session
    agent.Sessions.SetHistory(sessionKey, []providers.Message{})
    agent.Sessions.SetSummary(sessionKey, "")
    agent.Sessions.Save(sessionKey)

    // Return a confirmation message directly to the user
    clearMsg := "🧹 当前会话已清空，我们可以重新开始了。"

    // Optional: Log the action
    logger.InfoCF("agent", "User cleared session history",
        map[string]interface{}{
            "agent_id":    agent.ID,
            "session_key": sessionKey,
            "channel":     msg.Channel,
        })

    // return directly without running LLM loop
    return clearMsg, nil
}
```

**Step 2: Verify removal by checking code compiles**

```bash
go build ./pkg/agent/...
```

Expected: Success

**Step 3: Run existing agent tests to ensure no regression**

```bash
go test ./pkg/agent/... -v
```

Expected: All tests pass

**Step 4: Commit**

```bash
git add pkg/agent/loop.go
git commit -m "refactor(agent): remove /clear command handler (moved to gateway)"
```

---

## Task 5: Update Gateway Instantiation Calls

**Files:**
- Find all `NewCommandGateway` calls and update them

**Step 1: Find all instantiation sites**

```bash
grep -rn "NewCommandGateway" /Users/zhaopeng/projects/github/mobaiclaw --include="*.go"
```

**Step 2: Update each call to include sessions parameter**

Example (location may vary):
```go
// Before
gateway := gateway.NewCommandGateway(broker, agentBus, channelMgr, registry)

// After
gateway := gateway.NewCommandGateway(broker, agentBus, channelMgr, registry, sessions)
```

**Step 3: Verify compilation**

```bash
go build ./...
```

**Step 4: Run full test suite**

```bash
go test ./... -v
```

Expected: All tests pass

**Step 5: Commit**

```bash
git add .
git commit -m "refactor: update CommandGateway instantiation to include sessions parameter"
```

---

## Task 6: Integration Testing

**Step 1: Manual test via CLI**

```bash
cd /Users/zhaopeng/projects/github/mobaiclaw
go run main.go
```

Then in CLI:
1. Send some messages to establish history
2. Send `/help` command → verify `/clear` is listed
3. Send `/clear` command → verify session is cleared
4. Send new message → verify conversation starts fresh

**Step 2: Check logs for proper session clearing**

Look for confirmation message: `🧹 当前会话已清空，我们可以重新开始了。`

**Step 3: Verify no regressions**

- `/help` shows all commands
- Other commands (`/show`, `/list`, `/switch`) still work
- Normal conversation flow unaffected

**Step 4: Final commit if any adjustments needed**

```bash
git add .
git commit -m "test: verify /clear command migration integration"
```

---

## Verification Checklist

- [ ] `/clear` command works from Gateway layer
- [ ] `/help` includes `/clear` in output
- [ ] Agent layer no longer handles `/clear`
- [ ] All existing tests pass
- [ ] New tests for `/clear` in gateway package pass
- [ ] Manual testing confirms functionality
- [ ] No performance regression (command handling faster or same)

---

## Expected Performance Improvement

**Before:** `/clear` → Gateway (forward) → Agent (intercept) → Response
**After:** `/clear` → Gateway (handle) → Response

Eliminates:
- Routing lookup in Agent layer
- SessionKey derivation overhead
- Unnecessary message forwarding

---

## Notes

- The `/clear` command is case-insensitive in current implementation; Gateway layer should maintain this behavior if needed
- SessionKey is extracted from `msg.SessionKey` field, ensure it's populated correctly in all channel implementations
