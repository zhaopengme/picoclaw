# Memory System Hybrid Architecture Design

## Overview
This document outlines the refactoring of PicoClaw's memory system from a pure unstructured Markdown approach to a safer, concurrent-friendly **Hybrid Architecture**.

**Core Philosophy**: Maintain the ultra-lightweight, zero-dependency nature of PicoClaw while preventing AI-driven file corruption and data races during memory updates.

## 1. Architecture & Data Structure
The memory is split into two distinct physical formats based on its access pattern and mutability:

### Long-term Memory (Structured Profile)
- **Path**: `workspace/memory/profile.json` (Replaces `MEMORY.md` as the core storage)
- **Format**: Flat Key-Value JSON object (`map[string]string`).
- **Purpose**: Stores permanent facts, user preferences, and state.
- **Example**:
  ```json
  {
    "user_name": "Mike",
    "theme_preference": "dark mode",
    "current_project": "picoclaw refactoring"
  }
  ```

### Episodic Memory (Unstructured Daily Notes)
- **Path**: `workspace/memory/YYYYMM/YYYYMMDD.md` (Unchanged)
- **Format**: Append-only Markdown text.
- **Purpose**: Acts as a temporal log of conversations, transient thoughts, and daily context flow.

## 2. Tools & Agent Interaction
The LLM is **stripped of its ability to arbitrarily edit the core memory file** using generic tools like `edit_file`. Instead, it manages long-term memory via explicit, structured tool calls.

### New Tools
1. **`memory_set`**
   - **Arguments**: `key` (string), `value` (string)
   - **Behavior**: The Go backend safely unmarshals `profile.json`, updates/inserts the key, and marshals it back.
2. **`memory_delete`**
   - **Arguments**: `key` (string)
   - **Behavior**: Removes a key from the JSON map.

### System Prompt Constraints
The System Prompt explicitly instructs the agent:
> "To remember permanent facts or preferences, you MUST use the `memory_set` tool. Do NOT attempt to edit profile.json directly using file editing tools."

## 3. Data Flow & Concurrency
To solve the critical issue of data races when multiple channels (e.g., Telegram, Slack) interact with the agent simultaneously, concurrency control is implemented at the Go backend level.

### Concurrency Control (File Locking)
The `MemoryStore` struct (`pkg/agent/memory.go`) is upgraded with a `sync.RWMutex`:
- **Read Operations** (Building Context): Uses `RLock()` to safely read the JSON without blocking other reads.
- **Write Operations** (`memory_set`/`delete`): Uses `Lock()` to ensure atomic updates to the JSON file, preventing data corruption during concurrent API calls.

### Context Injection (LLM Formatting)
While the underlying storage is JSON, the LLM consumes Markdown. The `ContextBuilder` dynamically formats the JSON into a clean Markdown list before injecting it into the System Prompt:

```markdown
## Core Profile (Facts & Preferences)
- **user_name**: Mike
- **theme_preference**: dark mode
- **current_project**: picoclaw refactoring

## Recent Daily Notes
(Appended YYYYMMDD.md content...)
```

## Advantages
- **Zero AI Syntax Errors**: The LLM outputs structured JSON arguments instead of relying on regex or line numbers to edit Markdown.
- **Thread Safety**: Go's RWMutex completely eliminates file write collisions.
- **Lightweight**: Still requires zero external databases (no SQLite/CGO needed).
- **Human Readable**: The flat JSON profile and Markdown daily logs remain easily readable and editable by humans.
