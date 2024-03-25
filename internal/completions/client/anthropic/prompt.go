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

func convertFromLegacyMessages(messages []types.Message) []types.Message {
	filteredMessages := make([]types.Message, 0)
	skipNext := false
	for i, message := range messages {
		if skipNext {
			skipNext = false
			continue
		}

		// 1. If the first message is "system prompt like" convert it to an actual system prompt
		//
		// Note: The prefix we scan for here is used in the current chat prompts for VS Code and the
		//       old Web UI prompt.
		if i == 0 && strings.HasPrefix(message.Text, "You are Cody, an AI") {
			message.Speaker = types.SYSTEM_MESSAGE_SPEAKER
			skipNext = true
		}

		if i == len(messages)-1 && message.Speaker == types.ASSISTANT_MESSAGE_SPEAKER {
			// 2. If the last message is from an `assistant` with no or empty `text`, omit it
			if message.Text == "" {
				continue
			}

			// 3. Final assistant content cannot end with trailing whitespace
			message.Text = strings.TrimRight(message.Text, " \t\n\r")

		}

		// 4. If there is any assistant message in the middle of the messages without a `text`, omit
		//    both the empty assistant message as well as the unanswered question from the `user`

		// Don't apply this to the human message before the last message (it should always be included)
		if i >= len(messages)-2 {
			filteredMessages = append(filteredMessages, message)
			continue
		}
		// If the next message is an assistant message with no or empty `content`, omit the current and
		// the next one
		nextMessage := messages[i+1]
		if (nextMessage.Speaker == types.ASSISTANT_MESSAGE_SPEAKER && nextMessage.Text == "") ||
			(message.Speaker == types.ASSISTANT_MESSAGE_SPEAKER && message.Text == "") {
			continue
		}
		filteredMessages = append(filteredMessages, message)
	}

	return filteredMessages
}
