package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type MockProfileManager struct {
	Store map[string]string
}

func (m *MockProfileManager) WriteProfileKey(key, value string) error {
	m.Store[key] = value
	return nil
}

func (m *MockProfileManager) DeleteProfileKey(key string) error {
	delete(m.Store, key)
	return nil
}

func TestMemoryStoreTool(t *testing.T) {
	mockStore := &MockProfileManager{Store: make(map[string]string)}
	tool := NewMemoryStoreTool(mockStore)

	argsJSON := `{"key": "test_key", "value": "test_value"}`
	var args map[string]interface{}
	json.Unmarshal([]byte(argsJSON), &args)

	res := tool.Execute(context.Background(), args)
	if res.IsError {
		t.Fatalf("Tool returned error: %s", res.ForLLM)
	}

	if mockStore.Store["test_key"] != "test_value" {
		t.Errorf("Key not stored correctly in MemoryStore")
	}
}

func TestMemoryDeleteTool(t *testing.T) {
	mockStore := &MockProfileManager{Store: map[string]string{"obsolete_key": "old_value"}}
	tool := NewMemoryDeleteTool(mockStore)

	argsJSON := `{"key": "obsolete_key"}`
	var args map[string]interface{}
	json.Unmarshal([]byte(argsJSON), &args)

	res := tool.Execute(context.Background(), args)
	if res.IsError {
		t.Fatalf("Execute failed: %s", res.ForLLM)
	}

	if _, exists := mockStore.Store["obsolete_key"]; exists {
		t.Errorf("Key not deleted")
	}
}
