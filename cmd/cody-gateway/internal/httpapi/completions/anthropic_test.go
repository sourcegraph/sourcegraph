package completions

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/shared/config"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/tokenizer"
)

func TestIsFlaggedAnthropicRequest(t *testing.T) {
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

	t.Run("missing known preamble", func(t *testing.T) {
		ar := anthropicRequest{Model: "claude-2", Prompt: "some prompt without known preamble"}
		result, err := isFlaggedAnthropicRequest(tk, ar, cfgWithPreamble)
		require.NoError(t, err)
		require.True(t, result.IsFlagged())
		require.False(t, result.shouldBlock)
		require.Contains(t, result.reasons, "unknown_prompt")
	})

	t.Run("preamble not configured ", func(t *testing.T) {
		ar := anthropicRequest{Model: "claude-2", Prompt: "some prompt without known preamble"}
		result, err := isFlaggedAnthropicRequest(tk, ar, cfg)
		require.NoError(t, err)
		require.False(t, result.IsFlagged())
	})

	t.Run("high max tokens to sample", func(t *testing.T) {
		ar := anthropicRequest{Model: "claude-2", MaxTokensToSample: 10000, Prompt: validPreamble}
		result, err := isFlaggedAnthropicRequest(tk, ar, cfg)
		require.NoError(t, err)
		require.True(t, result.IsFlagged())
		require.True(t, result.shouldBlock)
		require.Contains(t, result.reasons, "high_max_tokens_to_sample")
		require.Equal(t, int32(result.maxTokensToSample), ar.MaxTokensToSample)
	})

	t.Run("high prompt token count (above block limit)", func(t *testing.T) {
		tokenLengths, err := tk.Tokenize(validPreamble)
		require.NoError(t, err)

		validPreambleTokens := len(tokenLengths)
		longPrompt := strings.Repeat("word ", cfg.PromptTokenFlaggingLimit+1)
		ar := anthropicRequest{Model: "claude-2", Prompt: validPreamble + " " + longPrompt}
		result, err := isFlaggedAnthropicRequest(tk, ar, cfgWithPreamble)
		require.NoError(t, err)
		require.True(t, result.IsFlagged())
		require.False(t, result.shouldBlock)
		require.Contains(t, result.reasons, "high_prompt_token_count")
		require.Equal(t, result.promptTokenCount, validPreambleTokens+1+cfg.PromptTokenFlaggingLimit+1, cfg)
	})

	t.Run("high prompt token count and bad phrase", func(t *testing.T) {
		cfgWithBadPhrase := &cfgWithPreamble
		cfgWithBadPhrase.BlockedPromptPatterns = []string{"bad phrase"}
		longPrompt := strings.Repeat("word ", cfg.PromptTokenFlaggingLimit+1)
		ar := anthropicRequest{Model: "claude-2", Prompt: validPreamble + " " + longPrompt + "bad phrase"}
		result, err := isFlaggedAnthropicRequest(tk, ar, *cfgWithBadPhrase)
		require.NoError(t, err)
		require.True(t, result.IsFlagged())
		require.True(t, result.shouldBlock)
	})

	t.Run("low prompt token count and bad phrase", func(t *testing.T) {
		cfgWithBadPhrase := &cfgWithPreamble
		cfgWithBadPhrase.BlockedPromptPatterns = []string{"bad phrase"}
		longPrompt := strings.Repeat("word ", 5)
		ar := anthropicRequest{Model: "claude-2", Prompt: validPreamble + " " + longPrompt + "bad phrase"}
		result, err := isFlaggedAnthropicRequest(tk, ar, *cfgWithBadPhrase)
		require.NoError(t, err)
		// for now, we should not flag requests purely because of bad phrases
		require.False(t, result.IsFlagged())
	})

	t.Run("high prompt token count (above block limit)", func(t *testing.T) {
		tokenLengths, err := tk.Tokenize(validPreamble)
		require.NoError(t, err)

		validPreambleTokens := len(tokenLengths)
		longPrompt := strings.Repeat("word ", cfg.PromptTokenBlockingLimit+1)
		ar := anthropicRequest{Model: "claude-2", Prompt: validPreamble + " " + longPrompt}
		result, err := isFlaggedAnthropicRequest(tk, ar, cfgWithPreamble)
		require.NoError(t, err)
		require.True(t, result.IsFlagged())
		require.True(t, result.shouldBlock)
		require.Contains(t, result.reasons, "high_prompt_token_count")
		require.Equal(t, result.promptTokenCount, validPreambleTokens+1+cfg.PromptTokenBlockingLimit+1)
	})
}

func TestAnthropicRequestJSON(t *testing.T) {
	tk, err := tokenizer.NewAnthropicClaudeTokenizer()
	require.NoError(t, err)

	ar := anthropicRequest{Prompt: "Hello world"}
	_, _ = ar.GetPromptTokenCount(tk) // hydrate values that should not be marshalled

	b, err := json.MarshalIndent(ar, "", "\t")
	require.NoError(t, err)

	autogold.Expect(`{
"prompt": "Hello world",
"model": "",
"max_tokens_to_sample": 0
}`).Equal(t, string(b))
}

func TestAnthropicRequestGetPromptTokenCount(t *testing.T) {
	tk, err := tokenizer.NewAnthropicClaudeTokenizer()
	require.NoError(t, err)

	originalRequest := anthropicRequest{Prompt: "Hello world"}

	t.Run("values are hydrated", func(t *testing.T) {
		count, err := originalRequest.GetPromptTokenCount(tk)
		require.NoError(t, err)
		assert.Equal(t, originalRequest.promptTokens.count, count)
		assert.Nil(t, originalRequest.promptTokens.err)
	})

	t.Run("values are not recomputed", func(t *testing.T) {
		newRequest := originalRequest // copy
		// Contrived example, we never update the prompt.
		newRequest.Prompt = "Hello again! This is a much longer prompt now"
		count2, err := newRequest.GetPromptTokenCount(tk)
		require.NoError(t, err)
		assert.Equal(t, originalRequest.promptTokens.count, count2, "token count should be unchanged")
	})
}
