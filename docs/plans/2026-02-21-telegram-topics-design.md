# Telegram Topics Support Design

## Overview
This design outlines how to support Telegram Forum Topics (threaded mode) in the Telegram adapter of the picoclaw project.

## Requirements
1. **Context Isolation**: Each topic thread should behave as an independent chat session with the bot.
2. **Reply Destination**: Responses should be sent back to the specific topic thread where they originated.
3. **Architectural Compatibility**: Avoid widespread changes to the core `MessageBus` or other adapters by keeping changes confined to the Telegram adapter.

## Approach: Composite ChatID

We will implement a "Composite ChatID" strategy. By combining the group's `ChatID` and the `MessageThreadID` at the adapter boundary, the rest of the system will naturally treat each topic as a separate session.

### 1. Inbound Messages (`handleMessage`)
When receiving a message from Telegram:
- Check if `message.IsTopicMessage` is true and `message.MessageThreadID` is present.
- If it is a topic message, construct a composite ChatID: `fmt.Sprintf("%d:%d", message.Chat.ID, message.MessageThreadID)`.
- If not, use the standard ChatID: `fmt.Sprintf("%d", message.Chat.ID)`.
- Inject the `MessageThreadID` into the message metadata for completeness.
- The `bus.InboundMessage` will be published with this composite `ChatID`.

### 2. Outbound Messages (`Send`)
When receiving a message to send from the bot engine:
- The `OutboundMessage.ChatID` will be the composite string if it originated from a topic.
- Parse this string: split by `:` to extract the true `ChatID` (int64) and the `MessageThreadID` (int).
- When calling `bot.SendMessage`, `bot.SendChatAction`, or editing placeholders, set the `MessageThreadID` field on the Telegram API request if it was extracted.

### 3. Thinking State
- The `Thinking... 💭` placeholder message and typing indicator must also use the parsed `MessageThreadID` so they appear in the correct thread.
- The tracking maps (`placeholders`, `stopThinking`) will naturally key off the composite ChatID string, maintaining isolation correctly.

## Affected Files
- `pkg/channels/telegram.go`:
  - `handleMessage`: Modify to build composite ChatID.
  - `Send`: Modify to parse composite ChatID and apply `MessageThreadID`.
  - `parseChatID` (or similar helper): Create a new helper `parseCompositeChatID` returning `(chatID int64, threadID int, err error)`.
  - Thinking animation logic within `handleMessage` needs to use the `MessageThreadID`.
