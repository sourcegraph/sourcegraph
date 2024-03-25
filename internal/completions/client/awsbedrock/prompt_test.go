package awsbedrock

import (
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
)

func TestGetPrompt(t *testing.T) {
	tests := []struct {
		name     string
		messages []types.Message
		want     string
		wantErr  bool
	}{
		{
			name: "success",
			messages: []types.Message{
				{Speaker: "human", Text: "Hello"},
				{Speaker: "assistant", Text: "Hi there!"},
			},
			want: "\n\nHuman: Hello\n\nAssistant: Hi there!",
		},
		{
			name: "empty message",
			messages: []types.Message{
				{Speaker: "human", Text: "Hello"},
				{Speaker: "assistant", Text: ""},
			},
			want: "\n\nHuman: Hello\n\nAssistant:",
		},
		{
			name: "consecutive same speaker error",
			messages: []types.Message{
				{Speaker: "human", Text: "Hello"},
				{Speaker: "human", Text: "Hi"},
			},
			wantErr: true,
		},
		{
			name: "invalid speaker",
			messages: []types.Message{
				{Speaker: "human1", Text: "Hello"},
				{Speaker: "human2", Text: "Hi"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetPrompt(tt.messages)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getPrompt() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("getPrompt() = %v, want %v", got, tt.want)
			}
		})
	}
}
