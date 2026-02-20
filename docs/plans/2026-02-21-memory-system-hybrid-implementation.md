# Memory System Hybrid Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor PicoClaw's memory system to use a concurrency-safe, structured JSON profile for long-term memory while retaining Markdown for daily notes, and introduce dedicated LLM memory management tools.

**Architecture:** We will update `pkg/agent/memory.go` to use `sync.RWMutex` and `profile.json`. We will create new `Tool` implementations in `pkg/tools/` for `memory_store` and `memory_delete`, and update the `ContextBuilder` to format the JSON profile into Markdown for the system prompt.

**Tech Stack:** Go (Standard Library: `encoding/json`, `sync`, `os`, `filepath`)

---

### Task 1: Refactor `MemoryStore` Data Structure

**Files:**
- Modify: `pkg/agent/memory.go`

**Step 1: Write the failing test**
(No dedicated tests currently exist for memory.go, so we will skip TDD for this specific structural change, but we will add a test in Task 2).

**Step 2: Update struct and initialization**
Modify `pkg/agent/memory.go` to replace `memoryFile` with `profileFile` and add a `sync.RWMutex`.

```go
package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MemoryStore manages persistent memory for the agent.
type MemoryStore struct {
	mu          sync.RWMutex
	workspace   string
	memoryDir   string
	profileFile string
}

func NewMemoryStore(workspace string) *MemoryStore {
	memoryDir := filepath.Join(workspace, "memory")
	profileFile := filepath.Join(memoryDir, "profile.json")

	os.MkdirAll(memoryDir, 0755)

	return &MemoryStore{
		workspace:   workspace,
		memoryDir:   memoryDir,
		profileFile: profileFile,
	}
}
```

**Step 3: Commit**
```bash
git add pkg/agent/memory.go
git commit -m "refactor: update MemoryStore to use profile.json and RWMutex"
```

---

### Task 2: Implement JSON Read/Write Logic

**Files:**
- Modify: `pkg/agent/memory.go`
- Create: `pkg/agent/memory_test.go`

**Step 1: Write the failing test**
Create `pkg/agent/memory_test.go`:
```go
package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileReadWrite(t *testing.T) {
	tempDir := t.TempDir()
	ms := NewMemoryStore(tempDir)

	// Write
	err := ms.WriteProfileKey("test_key", "test_value")
	if err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	// Read
	profile := ms.ReadProfile()
	if profile["test_key"] != "test_value" {
		t.Errorf("Expected 'test_value', got '%v'", profile["test_key"])
	}

	// Delete
	err = ms.DeleteProfileKey("test_key")
	if err != nil {
		t.Fatalf("Failed to delete profile key: %v", err)
	}

	profile2 := ms.ReadProfile()
	if _, exists := profile2["test_key"]; exists {
		t.Errorf("Key 'test_key' should have been deleted")
	}
}
```

**Step 2: Run test to verify it fails**
Run: `go test -v ./pkg/agent -run TestProfileReadWrite`
Expected: Compilation failure (methods not defined).

**Step 3: Write minimal implementation**
Add these methods to `pkg/agent/memory.go` (replacing `ReadLongTerm` and `WriteLongTerm`):

```go
// ReadProfile reads the long-term profile JSON safely.
func (ms *MemoryStore) ReadProfile() map[string]string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	profile := make(map[string]string)
	data, err := os.ReadFile(ms.profileFile)
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &profile)
	}
	return profile
}

// WriteProfileKey safely updates or adds a key in the profile.
func (ms *MemoryStore) WriteProfileKey(key, value string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	profile := make(map[string]string)
	data, err := os.ReadFile(ms.profileFile)
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &profile)
	}

	profile[key] = value

	newData, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ms.profileFile, newData, 0644)
}

// DeleteProfileKey safely removes a key from the profile.
func (ms *MemoryStore) DeleteProfileKey(key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	profile := make(map[string]string)
	data, err := os.ReadFile(ms.profileFile)
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &profile)
	}

	if _, exists := profile[key]; !exists {
		return nil // Key doesn't exist, nothing to do
	}

	delete(profile, key)

	newData, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ms.profileFile, newData, 0644)
}
```

**Step 4: Run test to verify it passes**
Run: `go test -v ./pkg/agent -run TestProfileReadWrite`
Expected: PASS

**Step 5: Commit**
```bash
git add pkg/agent/memory.go pkg/agent/memory_test.go
git commit -m "feat(memory): implement thread-safe JSON profile read/write logic"
```

---

### Task 3: Update Context Formatting

**Files:**
- Modify: `pkg/agent/memory.go`

**Step 1: Write the failing test**
Add to `pkg/agent/memory_test.go`:
```go
func TestGetMemoryContextFormatting(t *testing.T) {
	tempDir := t.TempDir()
	ms := NewMemoryStore(tempDir)
	ms.WriteProfileKey("user", "Mike")

	ctx := ms.GetMemoryContext()
	expectedContains := "- **user**: Mike"

	if !strings.Contains(ctx, expectedContains) {
		t.Errorf("Context missing expected formatting. Got:\n%s", ctx)
	}
}
```

**Step 2: Run test to verify it fails**
Run: `go test -v ./pkg/agent -run TestGetMemoryContextFormatting`
Expected: Compilation failure or missing text (since we removed `ReadLongTerm`).

**Step 3: Write minimal implementation**
Update `GetMemoryContext` in `pkg/agent/memory.go`:

```go
// GetMemoryContext returns formatted memory context for the agent prompt.
func (ms *MemoryStore) GetMemoryContext() string {
	var parts []string

	// Long-term memory (Profile)
	profile := ms.ReadProfile()
	if len(profile) > 0 {
		var profileStr string
		for k, v := range profile {
			profileStr += fmt.Sprintf("- **%s**: %s\n", k, v)
		}
		parts = append(parts, "## Core Profile (Facts & Preferences)\n\n"+profileStr)
	}

	// Recent daily notes (last 3 days)
	recentNotes := ms.GetRecentDailyNotes(3)
	if recentNotes != "" {
		parts = append(parts, "## Recent Daily Notes\n\n"+recentNotes)
	}

	if len(parts) == 0 {
		return ""
	}

	var result string
	for i, part := range parts {
		if i > 0 {
			result += "\n\n---\n\n"
		}
		result += part
	}
	return fmt.Sprintf("# Memory\n\n%s", result)
}
```

**Step 4: Run test to verify it passes**
Run: `go test -v ./pkg/agent -run TestGetMemoryContextFormatting`
Expected: PASS

**Step 5: Commit**
```bash
git add pkg/agent/memory.go pkg/agent/memory_test.go
git commit -m "feat(agent): format JSON profile as Markdown list for system prompt"
```

---

### Task 4: Create `memory_store` Tool

**Files:**
- Create: `pkg/tools/memory_store.go`
- Create: `pkg/tools/memory_store_test.go`

**Step 1: Write the failing test**
Create `pkg/tools/memory_store_test.go`:
```go
package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"github.com/sipeed/picoclaw/pkg/agent"
)

func TestMemoryStoreTool(t *testing.T) {
	tempDir := t.TempDir()
	ms := agent.NewMemoryStore(tempDir)
	tool := NewMemoryStoreTool(ms)

	argsJSON := `{"key": "test_key", "value": "test_value"}`
	var args map[string]interface{}
	json.Unmarshal([]byte(argsJSON), &args)

	res, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("Tool returned error: %s", res.Error)
	}

	profile := ms.ReadProfile()
	if profile["test_key"] != "test_value" {
		t.Errorf("Key not stored correctly in MemoryStore")
	}
}
```

**Step 2: Run test to verify it fails**
Run: `go test -v ./pkg/tools -run TestMemoryStoreTool`
Expected: Compilation failure.

**Step 3: Write minimal implementation**
Create `pkg/tools/memory_store.go`:
```go
package tools

import (
	"fmt"
	"github.com/sipeed/picoclaw/pkg/agent"
)

type MemoryStoreTool struct {
	BaseTool
	memoryStore *agent.MemoryStore
}

func NewMemoryStoreTool(ms *agent.MemoryStore) *MemoryStoreTool {
	return &MemoryStoreTool{
		BaseTool: BaseTool{
			Name:        "memory_store",
			Description: "Store a permanent fact or user preference in the core profile. Use this to remember long-term information.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "The unique identifier for the memory (e.g., 'user_name', 'coding_lang')",
					},
					"value": map[string]interface{}{
						"type":        "string",
						"description": "The information to remember",
					},
				},
				"required": []string{"key", "value"},
			},
		},
		memoryStore: ms,
	}
}

func (t *MemoryStoreTool) Execute(args map[string]interface{}) (ToolResult, error) {
	key, ok1 := args["key"].(string)
	value, ok2 := args["value"].(string)

	if !ok1 || !ok2 {
		return ToolResult{Error: "Missing or invalid 'key' or 'value' parameters"}, nil
	}

	err := t.memoryStore.WriteProfileKey(key, value)
	if err != nil {
		return ToolResult{Error: fmt.Sprintf("Failed to store memory: %v", err)}, nil
	}

	return ToolResult{
		Output: fmt.Sprintf("Successfully stored memory: %s = %s", key, value),
	}, nil
}
```

**Step 4: Run test to verify it passes**
Run: `go test -v ./pkg/tools -run TestMemoryStoreTool`
Expected: PASS

**Step 5: Commit**
```bash
git add pkg/tools/memory_store.go pkg/tools/memory_store_test.go
git commit -m "feat(tools): add memory_store tool for structured profile updates"
```

---

### Task 5: Create `memory_delete` Tool

**Files:**
- Create: `pkg/tools/memory_delete.go`
- Modify: `pkg/tools/memory_store_test.go` (Add delete test)

**Step 1: Write the failing test**
Add to `pkg/tools/memory_store_test.go`:
```go
func TestMemoryDeleteTool(t *testing.T) {
	tempDir := t.TempDir()
	ms := agent.NewMemoryStore(tempDir)
	ms.WriteProfileKey("obsolete_key", "old_value")

	tool := NewMemoryDeleteTool(ms)
	argsJSON := `{"key": "obsolete_key"}`
	var args map[string]interface{}
	json.Unmarshal([]byte(argsJSON), &args)

	res, err := tool.Execute(args)
	if err != nil || res.Error != "" {
		t.Fatalf("Execute failed")
	}

	profile := ms.ReadProfile()
	if _, exists := profile["obsolete_key"]; exists {
		t.Errorf("Key not deleted")
	}
}
```

**Step 2: Run test to verify it fails**
Run: `go test -v ./pkg/tools -run TestMemoryDeleteTool`
Expected: Compilation failure.

**Step 3: Write minimal implementation**
Create `pkg/tools/memory_delete.go`:
```go
package tools

import (
	"fmt"
	"github.com/sipeed/picoclaw/pkg/agent"
)

type MemoryDeleteTool struct {
	BaseTool
	memoryStore *agent.MemoryStore
}

func NewMemoryDeleteTool(ms *agent.MemoryStore) *MemoryDeleteTool {
	return &MemoryDeleteTool{
		BaseTool: BaseTool{
			Name:        "memory_delete",
			Description: "Delete a specific fact or preference from the core profile.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "The unique identifier of the memory to delete",
					},
				},
				"required": []string{"key"},
			},
		},
		memoryStore: ms,
	}
}

func (t *MemoryDeleteTool) Execute(args map[string]interface{}) (ToolResult, error) {
	key, ok := args["key"].(string)
	if !ok {
		return ToolResult{Error: "Missing or invalid 'key' parameter"}, nil
	}

	err := t.memoryStore.DeleteProfileKey(key)
	if err != nil {
		return ToolResult{Error: fmt.Sprintf("Failed to delete memory: %v", err)}, nil
	}

	return ToolResult{
		Output: fmt.Sprintf("Successfully deleted memory key: %s", key),
	}, nil
}
```

**Step 4: Run test to verify it passes**
Run: `go test -v ./pkg/tools -run TestMemoryDeleteTool`
Expected: PASS

**Step 5: Commit**
```bash
git add pkg/tools/memory_delete.go pkg/tools/memory_store_test.go
git commit -m "feat(tools): add memory_delete tool"
```

---

### Task 6: Update Tool Registry & Context Prompt

**Files:**
- Modify: `cmd/picoclaw/main.go`
- Modify: `pkg/agent/context.go`

**Step 1: Refactor Dependency Injection & Register Tools**
In `pkg/agent/context.go`, modify `NewContextBuilder` to accept a `*MemoryStore` instance rather than creating it internally:
```go
// Change signature
func NewContextBuilder(workspace string, memoryStore *MemoryStore) *ContextBuilder {
    // ...
    return &ContextBuilder{
        workspace:    workspace,
        skillsLoader: skills.NewSkillsLoader(workspace, globalSkillsDir, builtinSkillsDir),
        memory:       memoryStore,
    }
}
```

In `cmd/picoclaw/main.go` (or wherever `ContextBuilder` and tools are initialized), instantiate the `MemoryStore` first and pass it to both:
```go
// In initialization flow:
memoryStore := agent.NewMemoryStore(workspacePath)
cb := agent.NewContextBuilder(workspacePath, memoryStore)

// Register tools
memoryStoreTool := tools.NewMemoryStoreTool(memoryStore)
toolRegistry.Register(memoryStoreTool)
memoryDeleteTool := tools.NewMemoryDeleteTool(memoryStore)
toolRegistry.Register(memoryDeleteTool)
```

**Step 2: Update System Prompt with Defensive Constraints**
In `pkg/agent/context.go`, rigorously update `getIdentity` system prompt instructions.
Change:
```go
3. **Memory** - When remembering something, write to %s/memory/MEMORY.md
```
To:
```go
3. **Memory Management (CRITICAL RULES)**:
   - **Core Profile**: Permanent facts about the user. You MUST use the 'memory_store' tool to update this and 'memory_delete' to remove items.
   - **Daily Notes**: Short-term session logs at %s/memory/YYYYMM/YYYYMMDD.md. The system manages this automatically.
   - **FORBIDDEN ACTIONS**: You are STRICTLY FORBIDDEN from using `edit_file`, `write_file`, or `shell` tools to modify any files inside the `memory/` directory directly. Any memory updates must go through the dedicated memory tools.
```

**Step 3: Commit**
```bash
git add cmd/picoclaw/main.go pkg/agent/context.go
git commit -m "feat: register memory tools and update system prompt constraints"
```
