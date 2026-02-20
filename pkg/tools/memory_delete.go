package tools

import (
	"context"
	"fmt"
)

type MemoryDeleteTool struct {
	memoryStore ProfileManager
}

func NewMemoryDeleteTool(ms ProfileManager) *MemoryDeleteTool {
	return &MemoryDeleteTool{
		memoryStore: ms,
	}
}

func (t *MemoryDeleteTool) Name() string {
	return "memory_delete"
}

func (t *MemoryDeleteTool) Description() string {
	return "Delete a specific fact or preference from the core profile."
}

func (t *MemoryDeleteTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"key": map[string]interface{}{
				"type":        "string",
				"description": "The unique identifier of the memory to delete",
			},
		},
		"required": []string{"key"},
	}
}

func (t *MemoryDeleteTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	key, ok := args["key"].(string)
	if !ok {
		return ErrorResult("Missing or invalid 'key' parameter")
	}

	err := t.memoryStore.DeleteProfileKey(key)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to delete memory: %v", err))
	}

	return NewToolResult(fmt.Sprintf("Successfully deleted memory key: %s", key))
}
