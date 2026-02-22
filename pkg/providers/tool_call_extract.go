package providers

import "github.com/zhaopengme/mobaiclaw/pkg/providers/protocoltypes"

// extractToolCallsFromText delegates to protocoltypes.ExtractToolCallsFromText.
func extractToolCallsFromText(text string) []ToolCall {
	return protocoltypes.ExtractToolCallsFromText(text)
}

// stripToolCallsFromText delegates to protocoltypes.StripToolCallsFromText.
func stripToolCallsFromText(text string) string {
	return protocoltypes.StripToolCallsFromText(text)
}
