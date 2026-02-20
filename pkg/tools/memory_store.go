package tools

import (
	"context"
	"fmt"
)

// ProfileManager interface avoids import cycle with pkg/agent
type ProfileManager interface {
	WriteProfileKey(key, value string) error
	DeleteProfileKey(key string) error
}

type MemoryStoreTool struct {
	memoryStore ProfileManager
}

func NewMemoryStoreTool(ms ProfileManager) *MemoryStoreTool {
	return &MemoryStoreTool{
		memoryStore: ms,
	}
}

func (t *MemoryStoreTool) Name() string {
	return "memory_store"
}

func (t *MemoryStoreTool) Description() string {
	return "Store a permanent fact or user preference in the core profile. Use this to remember long-term information."
}

func (t *MemoryStoreTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
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
	}
}

func (t *MemoryStoreTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	key, ok1 := args["key"].(string)
	value, ok2 := args["value"].(string)

	if !ok1 || !ok2 {
		return ErrorResult("Missing or invalid 'key' or 'value' parameters")
	}

	err := t.memoryStore.WriteProfileKey(key, value)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to store memory: %v", err))
	}

	return NewToolResult(fmt.Sprintf("Successfully stored memory: %s = %s", key, value))
}
