package anthropic

import (
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func removeWhitespaceOnlySequences(sequences []string) []string {
	var result []string
	for _, sequence := range sequences {
		if len(strings.TrimSpace(sequence)) > 0 {
			result = append(result, sequence)
		}
	}
	return result
}

func toAnthropicMessages(messages []types.Message) ([]anthropicMessage, error) {
	anthropicMessages := make([]anthropicMessage, 0, len(messages))

	for i, message := range messages {
		speaker := message.Speaker
		text := message.Text

		anthropicRole := message.Speaker

		switch speaker {
		case types.SYSTEM_MESSAGE_SPEAKER:
			if i != 0 {
				return nil, errors.New("system role can only be used in the first message")
			}
		case types.ASSISTANT_MESSAGE_SPEAKER:
		case types.HUMAN_MESSAGE_SPEAKER:
			anthropicRole = "user"
		default:
			return nil, errors.Errorf("unexpected role: %s", text)
		}

		if text == "" {
			return nil, errors.New("message content cannot be empty")
		}

		anthropicMessages = append(anthropicMessages, anthropicMessage{
			Role:    anthropicRole,
			Content: []anthropicMessageContent{{Text: text, Type: "text"}},
		})
	}

	return anthropicMessages, nil
}
