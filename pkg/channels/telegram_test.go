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
