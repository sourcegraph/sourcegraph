package completions

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/shared/config"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/tokenizer"
)

func TestIsFlaggedAnthropicMessagesRequest(t *testing.T) {
	validPreamble := "You are cody-gateway."

	cfg := config.AnthropicConfig{
		PromptTokenFlaggingLimit:       18000,
		PromptTokenBlockingLimit:       20000,
		MaxTokensToSampleFlaggingLimit: 1000,
		ResponseTokenBlockingLimit:     1000,
	}
	cfgWithPreamble := config.AnthropicConfig{
		PromptTokenFlaggingLimit:       18000,
		PromptTokenBlockingLimit:       20000,
		MaxTokensToSampleFlaggingLimit: 1000,
		ResponseTokenBlockingLimit:     1000,
		AllowedPromptPatterns:          []string{strings.ToLower(validPreamble)},
	}
	tk, err := tokenizer.NewAnthropicClaudeTokenizer()
	require.NoError(t, err)

	t.Run("works for known preamble", func(t *testing.T) {
		r := anthropicMessagesRequest{Model: "anthropic/claude-3-sonnet-20240229", Messages: []anthropicMessage{
			{Role: "system", Content: []anthropicMessageContent{{Type: "text", Text: validPreamble}}},
		}}
		result, err := isFlaggedAnthropicMessagesRequest(tk, r, cfgWithPreamble)
		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("missing known preamble", func(t *testing.T) {
		r := anthropicMessagesRequest{Model: "anthropic/claude-3-sonnet-20240229", Messages: []anthropicMessage{
			{Role: "system", Content: []anthropicMessageContent{{Type: "text", Text: "some prompt without known preamble"}}},
		}}
		result, err := isFlaggedAnthropicMessagesRequest(tk, r, cfgWithPreamble)
		require.NoError(t, err)
		require.True(t, result.IsFlagged())
		require.False(t, result.shouldBlock)
		require.Contains(t, result.reasons, "unknown_prompt")
	})

	t.Run("preamble not configured ", func(t *testing.T) {
		r := anthropicMessagesRequest{Model: "anthropic/claude-3-sonnet-20240229", Messages: []anthropicMessage{
			{Role: "system", Content: []anthropicMessageContent{{Type: "text", Text: "some prompt without known preamble"}}},
		}}
		result, err := isFlaggedAnthropicMessagesRequest(tk, r, cfg)
		require.NoError(t, err)
		require.False(t, result.IsFlagged())
	})

	t.Run("high max tokens to sample", func(t *testing.T) {
		r := anthropicMessagesRequest{Model: "anthropic/claude-3-sonnet-20240229", MaxTokens: 10000, Messages: []anthropicMessage{
			{Role: "system", Content: []anthropicMessageContent{{Type: "text", Text: validPreamble}}},
		}}
		result, err := isFlaggedAnthropicMessagesRequest(tk, r, cfg)
		require.NoError(t, err)
		require.True(t, result.IsFlagged())
		require.True(t, result.shouldBlock)
		require.Contains(t, result.reasons, "high_max_tokens_to_sample")
		require.Equal(t, int32(result.maxTokensToSample), r.MaxTokens)
	})

	t.Run("high prompt token count and bad phrase", func(t *testing.T) {
		cfgWithBadPhrase := &cfgWithPreamble
		cfgWithBadPhrase.BlockedPromptPatterns = []string{"bad phrase"}
		longPrompt := strings.Repeat("word ", cfg.PromptTokenFlaggingLimit+1)
		r := anthropicMessagesRequest{Model: "anthropic/claude-3-sonnet-20240229", Messages: []anthropicMessage{
			{Role: "system", Content: []anthropicMessageContent{{Type: "text", Text: validPreamble + " " + longPrompt + "bad phrase"}}},
		}}
		result, err := isFlaggedAnthropicMessagesRequest(tk, r, *cfgWithBadPhrase)
		require.NoError(t, err)
		require.True(t, result.IsFlagged())
		require.True(t, result.shouldBlock)
	})

	t.Run("low prompt token count and bad phrase", func(t *testing.T) {
		cfgWithBadPhrase := &cfgWithPreamble
		cfgWithBadPhrase.BlockedPromptPatterns = []string{"bad phrase"}
		longPrompt := strings.Repeat("word ", 5)
		r := anthropicMessagesRequest{Model: "anthropic/claude-3-sonnet-20240229", Messages: []anthropicMessage{
			{Role: "system", Content: []anthropicMessageContent{{Type: "text", Text: validPreamble + " " + longPrompt + "bad phrase"}}},
		}}
		result, err := isFlaggedAnthropicMessagesRequest(tk, r, *cfgWithBadPhrase)
		require.NoError(t, err)
		// for now, we should not flag requests purely because of bad phrases
		require.False(t, result.IsFlagged())
	})

	t.Run("high prompt token count (above block limit)", func(t *testing.T) {
		tokenLengths, err := tk.Tokenize(validPreamble)
		require.NoError(t, err)

		validPreambleTokens := len(tokenLengths)
		longPrompt := strings.Repeat("word ", cfg.PromptTokenFlaggingLimit+1)
		r := anthropicMessagesRequest{Model: "anthropic/claude-3-sonnet-20240229", Messages: []anthropicMessage{
			{Role: "system", Content: []anthropicMessageContent{{Type: "text", Text: validPreamble}}},
			{Role: "user", Content: []anthropicMessageContent{{Type: "text", Text: longPrompt}}},
		}}

		result, err := isFlaggedAnthropicMessagesRequest(tk, r, cfgWithPreamble)
		require.NoError(t, err)
		require.True(t, result.IsFlagged())
		require.False(t, result.shouldBlock)
		require.Contains(t, result.reasons, "high_prompt_token_count")
		require.Equal(t, result.promptTokenCount, validPreambleTokens+4+cfg.PromptTokenFlaggingLimit+4, cfg)
	})

	t.Run("high prompt token count (below block limit)", func(t *testing.T) {
		tokenLengths, err := tk.Tokenize(validPreamble)
		require.NoError(t, err)

		validPreambleTokens := len(tokenLengths)
		longPrompt := strings.Repeat("word ", cfg.PromptTokenBlockingLimit+1)
		r := anthropicMessagesRequest{Model: "anthropic/claude-3-sonnet-20240229", Messages: []anthropicMessage{
			{Role: "system", Content: []anthropicMessageContent{{Type: "text", Text: validPreamble}}},
			{Role: "user", Content: []anthropicMessageContent{{Type: "text", Text: longPrompt}}},
		}}

		result, err := isFlaggedAnthropicMessagesRequest(tk, r, cfgWithPreamble)
		require.NoError(t, err)
		require.True(t, result.IsFlagged())
		require.True(t, result.shouldBlock)
		require.Contains(t, result.reasons, "high_prompt_token_count")
		require.Equal(t, result.promptTokenCount, validPreambleTokens+4+cfg.PromptTokenBlockingLimit+4)
	})
}

func TestAnthropicMessagesRequestJSON(t *testing.T) {
	_, err := tokenizer.NewAnthropicClaudeTokenizer()
	require.NoError(t, err)

	r := anthropicMessagesRequest{Model: "anthropic/claude-3-sonnet-20240229", Messages: []anthropicMessage{
		{Role: "user", Content: []anthropicMessageContent{{Type: "text", Text: "Hello world"}}},
	}}

	b, err := json.MarshalIndent(r, "", "\t")
	require.NoError(t, err)

	autogold.Expect(`{
"messages": [
		{
			"role": "user",
			"content": [
				{
					"type": "text",
					"text": "Hello world"
				}
			]
		}
],
"model": "anthropic/claude-3-sonnet-20240229"
}`).Equal(t, string(b))
}
