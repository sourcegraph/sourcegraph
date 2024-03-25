package awsbedrock

import (
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const HUMAN_PROMPT = "\n\nHuman:"
const ASSISTANT_PROMPT = "\n\nAssistant:"

func GetPrompt(messages []types.Message) (string, error) {
	prompt := make([]string, 0, len(messages))
	for idx, message := range messages {
		if idx > 0 && messages[idx-1].Speaker == message.Speaker {
			return "", errors.Newf("found consecutive messages with the same speaker '%s'", message.Speaker)
		}

		messagePrompt, err := message.GetPrompt(HUMAN_PROMPT, ASSISTANT_PROMPT)
		if err != nil {
			return "", err
		}
		prompt = append(prompt, messagePrompt)
	}
	return strings.Join(prompt, ""), nil
}
