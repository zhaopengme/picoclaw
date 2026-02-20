package channels

import (
	"testing"
)

func TestParseCompositeChatID(t *testing.T) {
	tests := []struct {
		input        string
		wantChatID   int64
		wantThreadID int
		wantErr      bool
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

func TestMarkdownToTelegramHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Normal text",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "Bold text",
			input: "**Hello**",
			want:  "<b>Hello</b>",
		},
		{
			name:  "Intra-word underscore (no italic)",
			input: "agent_id=main",
			want:  "agent_id=main",
		},
		{
			name:  "Multiple intra-word underscores",
			input: "session_key=agent:main, agent_id=main",
			want:  "session_key=agent:main, agent_id=main",
		},
		{
			name:  "Nested bold and italic",
			input: "**_Hello_**",
			want:  "<b><i>Hello</i></b>",
		},
		{
			name:  "Complex overlapping scenario from bug report",
			input: "**Channel（渠道）隔离**\n- feis... {session_key=agent:main:main, iterations=1, final_length=1108, agent_id=main}",
			want:  "<b>Channel（渠道）隔离</b>\n• feis... {session_key=agent:main:main, iterations=1, final_length=1108, agent_id=main}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markdownToTelegramHTML(tt.input)
			if got != tt.want {
				t.Errorf("markdownToTelegramHTML()\ninput: %q\ngot:  %q\nwant: %q", tt.input, got, tt.want)
			}
		})
	}
}
