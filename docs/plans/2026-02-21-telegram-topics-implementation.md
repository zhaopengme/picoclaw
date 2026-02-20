# Telegram Topics Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Support Telegram group topics (threaded mode) by using a composite ChatID to isolate sessions per topic and reply to the correct thread.

**Architecture:** We will modify the `TelegramChannel` to construct a composite ChatID (`ChatID:MessageThreadID`) for topic messages on inbound, and parse this composite ID on outbound to specify the `MessageThreadID` when calling the Telegram API.

**Tech Stack:** Go, Telego (Telegram bot API wrapper)

---

### Task 1: Update ChatID Parsing Logic

**Files:**
- Modify: `pkg/channels/telegram.go`

**Step 1: Write the failing test**
There are no existing tests for `parseChatID`. We'll write a new test for the composite parser.
Create `pkg/channels/telegram_test.go`:
```go
package channels

import (
	"testing"
)

func TestParseCompositeChatID(t *testing.T) {
	tests := []struct {
		input       string
		wantChatID  int64
		wantThreadID int
		wantErr     bool
	}{
		{"12345", 12345, 0, false},
		{"-1001234567:5", -1001234567, 5, false},
		{"invalid", 0, 0, true},
		{"123:invalid", 123, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotChatID, gotThreadID, err := parseCompositeChatID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCompositeChatID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotChatID != tt.wantChatID {
				t.Errorf("parseCompositeChatID() gotChatID = %v, want %v", gotChatID, tt.wantChatID)
			}
			if gotThreadID != tt.wantThreadID {
				t.Errorf("parseCompositeChatID() gotThreadID = %v, want %v", gotThreadID, tt.wantThreadID)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**
Run: `go test -v ./pkg/channels -run TestParseCompositeChatID`
Expected: FAIL with "undefined: parseCompositeChatID"

**Step 3: Write minimal implementation**
Add to `pkg/channels/telegram.go` (replacing `parseChatID` or adding alongside it):
```go
import (
	"fmt"
	"strconv"
	"strings"
)

func parseCompositeChatID(chatIDStr string) (int64, int, error) {
	parts := strings.SplitN(chatIDStr, ":", 2)
	
	chatID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid chat ID format: %w", err)
	}

	var threadID int
	if len(parts) > 1 {
		threadID, err = strconv.Atoi(parts[1])
		if err != nil {
			return chatID, 0, fmt.Errorf("invalid thread ID format: %w", err)
		}
	}

	return chatID, threadID, nil
}
```

**Step 4: Run test to verify it passes**
Run: `go test -v ./pkg/channels -run TestParseCompositeChatID`
Expected: PASS

**Step 5: Commit**
```bash
git add pkg/channels/telegram.go pkg/channels/telegram_test.go
git commit -m "feat: add parseCompositeChatID for Telegram topics"
```

---

### Task 2: Update Inbound Message Handling (`handleMessage`)

**Files:**
- Modify: `pkg/channels/telegram.go`

**Step 1: Write the failing test**
(Skipping explicit unit test for complex Telego handler integration, relying on manual/system testing for this specific adapter boundary).

**Step 2: Implement inbound composite ChatID**
Modify `handleMessage` in `pkg/channels/telegram.go` (around line 219):
```go
	chatID := message.Chat.ID
	chatIDStr := fmt.Sprintf("%d", chatID)
	
	// Support for Forum Topics (Threads)
	if message.IsTopicMessage && message.MessageThreadID != 0 {
		chatIDStr = fmt.Sprintf("%d:%d", chatID, message.MessageThreadID)
	}
	
	c.chatIDs[senderID] = chatID
```
And update `stopThinking.Load` and `placeholders.Store` to use `chatIDStr` instead of `fmt.Sprintf("%d", chatID)`. 
Also, inject thread ID into metadata:
```go
	metadata := map[string]string{
		"message_id": fmt.Sprintf("%d", message.MessageID),
		"user_id":    fmt.Sprintf("%d", user.ID),
		"username":   user.Username,
		"first_name": user.FirstName,
		"is_group":   fmt.Sprintf("%t", message.Chat.Type != "private"),
		"peer_kind":  peerKind,
		"peer_id":    peerID,
	}
	if message.IsTopicMessage && message.MessageThreadID != 0 {
		metadata["thread_id"] = fmt.Sprintf("%d", message.MessageThreadID)
	}

	c.HandleMessage(senderID, chatIDStr, content, mediaPaths, metadata)
```

**Step 3: Update ChatAction and Thinking message**
In `handleMessage`, update the API calls to support threads:
```go
	// Thinking indicator
	chatActionParams := &telego.SendChatActionParams{
		ChatID: tu.ID(chatID),
		Action: telego.ChatActionTyping,
	}
	if message.IsTopicMessage && message.MessageThreadID != 0 {
		chatActionParams.MessageThreadID = message.MessageThreadID
	}
	err := c.bot.SendChatAction(ctx, chatActionParams)
	
	// ... (stop previous thinking animation logic using chatIDStr) ...

	thinkingMsgParams := &telego.SendMessageParams{
		ChatID: tu.ID(chatID),
		Text:   "Thinking... 💭",
	}
	if message.IsTopicMessage && message.MessageThreadID != 0 {
		thinkingMsgParams.MessageThreadID = message.MessageThreadID
	}
	pMsg, err := c.bot.SendMessage(ctx, thinkingMsgParams)
```

**Step 4: Commit**
```bash
git add pkg/channels/telegram.go
git commit -m "feat: handle inbound Telegram topic messages with composite chat ID"
```

---

### Task 3: Update Outbound Message Sending (`Send`)

**Files:**
- Modify: `pkg/channels/telegram.go`

**Step 1: Write the minimal implementation**
Modify `Send` in `pkg/channels/telegram.go` (around line 154) to use `parseCompositeChatID` and set `MessageThreadID`:
```go
	chatID, threadID, err := parseCompositeChatID(msg.ChatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	// ... (stop thinking animation logic remains the same using msg.ChatID) ...
	
	htmlContent := markdownToTelegramHTML(msg.Content)

	// Try to edit placeholder
	if pID, ok := c.placeholders.Load(msg.ChatID); ok {
		c.placeholders.Delete(msg.ChatID)
		editMsg := &telego.EditMessageTextParams{
			ChatID:    tu.ID(chatID),
			MessageID: pID.(int),
			Text:      htmlContent,
			ParseMode: telego.ModeHTML,
		}

		if _, err = c.bot.EditMessageText(ctx, editMsg); err == nil {
			return nil
		}
		// Fallback to new message if edit fails
	}

	tgMsg := &telego.SendMessageParams{
		ChatID:    tu.ID(chatID),
		Text:      htmlContent,
		ParseMode: telego.ModeHTML,
	}
	if threadID != 0 {
		tgMsg.MessageThreadID = threadID
	}

	if _, err = c.bot.SendMessage(ctx, tgMsg); err != nil {
		logger.ErrorCF("telegram", "HTML parse failed, falling back to plain text", map[string]interface{}{
			"error": err.Error(),
		})
		tgMsg.ParseMode = ""
		_, err = c.bot.SendMessage(ctx, tgMsg)
		return err
	}
```

**Step 2: Check compilation**
Run: `go build ./pkg/channels`
Expected: Compiles successfully without syntax errors.

**Step 3: Commit**
```bash
git add pkg/channels/telegram.go
git commit -m "feat: support sending outbound messages to Telegram topics"
```
