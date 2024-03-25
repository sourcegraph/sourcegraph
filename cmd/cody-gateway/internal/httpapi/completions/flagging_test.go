package completions

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/tokenizer"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/shared/config"
)

func TestMakeFlaggingConfig(t *testing.T) {
	configConfig := config.FlaggingConfig{
		AllowedPromptPatterns:  []string{"allowed", "prompt", "patterns"},
		BlockedPromptPatterns:  []string{"blocked", "prompt", "patterns"},
		RequestBlockingEnabled: true,

		// NOTE: This field is NOT part of flagging.go's flaggingConfig struct, it
		// only uses MaxTokensToSampleFlaggingLimit. Instead, this is the hard-cap
		// for the LLM provider.
		MaxTokensToSample: 111,

		PromptTokenFlaggingLimit:       222,
		PromptTokenBlockingLimit:       333,
		MaxTokensToSampleFlaggingLimit: 444,
		ResponseTokenBlockingLimit:     555,
	}

	// Confirm that everything is copied over as expected.
	convertedConfig := makeFlaggingConfig(configConfig)
	assert.Equal(t, configConfig.AllowedPromptPatterns, convertedConfig.AllowedPromptPatterns)
	assert.Equal(t, configConfig.BlockedPromptPatterns, convertedConfig.BlockedPromptPatterns)
	assert.Equal(t, configConfig.RequestBlockingEnabled, convertedConfig.RequestBlockingEnabled)
	assert.Equal(t, configConfig.MaxTokensToSampleFlaggingLimit, convertedConfig.MaxTokensToSampleFlaggingLimit)
	assert.Equal(t, configConfig.PromptTokenFlaggingLimit, convertedConfig.PromptTokenFlaggingLimit)
	assert.Equal(t, configConfig.PromptTokenFlaggingLimit, convertedConfig.PromptTokenFlaggingLimit)
}

func TestIsFlaggedRequest(t *testing.T) {
	validPreamble := "You are cody-gateway."

	basicCfg := flaggingConfig{
		PromptTokenFlaggingLimit:       18000,
		PromptTokenBlockingLimit:       20000,
		MaxTokensToSampleFlaggingLimit: 1000,
		ResponseTokenBlockingLimit:     1000,
		RequestBlockingEnabled:         true,
	}
	cfgWithPreamble := flaggingConfig{
		PromptTokenFlaggingLimit:       18000,
		PromptTokenBlockingLimit:       20000,
		MaxTokensToSampleFlaggingLimit: 1000,
		ResponseTokenBlockingLimit:     1000,
		RequestBlockingEnabled:         true,
		AllowedPromptPatterns:          []string{strings.ToLower(validPreamble)},
	}

	// Create a generic tokenizer. If provided to isFlaggedRequest, it will enable
	// a few more checks.
	tokenizer, err := tokenizer.NewAnthropicClaudeTokenizer()
	require.NoError(t, err)

	// callIsFlaggedRequest just wraps the call to isFlaggedResult.
	callIsFlaggedRequest := func(t *testing.T, prompt string, cfg flaggingConfig) (*flaggingResult, error) {
		return isFlaggedRequest(
			tokenizer,
			flaggingRequest{
				FlattenedPrompt: prompt,
				MaxTokens:       200,
			},
			cfg)
	}

	// Request is missing the preamble.
	t.Run("MissingPreamble", func(t *testing.T) {
		result, err := callIsFlaggedRequest(t, "prompt without known preamble", cfgWithPreamble)
		require.NoError(t, err)

		require.True(t, result.IsFlagged())
		require.False(t, result.shouldBlock)
		require.Contains(t, result.reasons, "unknown_prompt")
	})

	// If the configuration doesn't include a preamble, the same request won't get flagged.
	t.Run("PremableNotConfigured", func(t *testing.T) {
		result, err := callIsFlaggedRequest(t, "some prompt without known premable", basicCfg)
		require.NoError(t, err)
		require.False(t, result.IsFlagged())
	})

	t.Run("WithPreamble", func(t *testing.T) {
		result, err := callIsFlaggedRequest(t, "yadda yadda"+validPreamble+"yadda yadda", cfgWithPreamble)
		require.NoError(t, err)
		require.False(t, result.IsFlagged())
	})

	t.Run("high max tokens to sample", func(t *testing.T) {
		result, err := isFlaggedRequest(
			tokenizer,
			flaggingRequest{
				FlattenedPrompt: validPreamble,
				MaxTokens:       basicCfg.MaxTokensToSampleFlaggingLimit + 1,
			},
			basicCfg)
		require.NoError(t, err)
		assert.True(t, result.IsFlagged())
		assert.True(t, result.shouldBlock)
		assert.Contains(t, result.reasons, "high_max_tokens_to_sample")

		// NB. In practice, this is essentially us returning to the client what the configured
		// MaxTokensToSampleFlaggingLimit is. e.g. "flagged, because maxTokensToSample was set to xxx".
		assert.Equal(t, result.maxTokensToSample, basicCfg.MaxTokensToSampleFlaggingLimit+1)
	})

	t.Run("missing preamble and bad phrase", func(t *testing.T) {
		cfgWithBadPhrase := cfgWithPreamble
		cfgWithBadPhrase.BlockedPromptPatterns = []string{"bad phrase"}
		result, err := callIsFlaggedRequest(
			t,
			"never going to give you up... bad phrase never going to... ",
			cfgWithBadPhrase)
		require.NoError(t, err)
		assert.True(t, result.IsFlagged())
		assert.True(t, result.shouldBlock)
		assert.Contains(t, result.reasons, "unknown_prompt")
	})

	// If the prompt is NOT flagged, then we do not perform the "blocking due to bad phrase" check.
	// In other words, a valid prompt with a bad phrase is allowed through.
	t.Run("bad phrase only", func(t *testing.T) {
		cfgWithBadPhrase := cfgWithPreamble
		cfgWithBadPhrase.BlockedPromptPatterns = []string{"bad phrase"}
		result, err := callIsFlaggedRequest(
			t,
			validPreamble+" ... bad phrase ...",
			cfgWithBadPhrase)
		require.NoError(t, err)
		assert.False(t, result.IsFlagged())
	})

	t.Run("TokenCountChecks", func(t *testing.T) {
		// Set up a prompt with a well-enough known prompt count based on tokenizer.
		repeatedWords := strings.Repeat("never going to give you up ", 10)
		prompt := validPreamble + repeatedWords

		promptTokens, err := tokenizer.Tokenize(prompt)
		require.NoError(t, err)
		promptTokenCount := len(promptTokens)

		// Flagging config's with the flagging limit equal to the token count of the prompt.
		tokenCountConfig := cfgWithPreamble
		tokenCountConfig.PromptTokenFlaggingLimit = promptTokenCount
		tokenCountConfig.PromptTokenBlockingLimit = promptTokenCount + 10

		// If no tokenizer is available when checking if the request should be flagged,
		// we simply skip those checks. (And do not panic, etc.)
		t.Run("NilTokenizer", func(t *testing.T) {
			reallyLongPrompt := strings.Repeat(prompt, 10)
			result, err := isFlaggedRequest(
				nil,
				flaggingRequest{
					FlattenedPrompt: reallyLongPrompt,
					MaxTokens:       200,
				},
				tokenCountConfig)
			require.NoError(t, err)

			// Other than the long-prompt check (which requires the tokenizer),
			// the request is legit.
			assert.False(t, result.IsFlagged())
		})

		t.Run("BelowFlaggingLimit", func(t *testing.T) {
			shoterPrompt := string(prompt[:len(prompt)-8])
			result, err := callIsFlaggedRequest(t, shoterPrompt, tokenCountConfig)
			require.NoError(t, err)
			assert.False(t, result.IsFlagged())
			assert.Nil(t, result)
		})

		t.Run("AboveFlaggingLimitBelowBlockLimit", func(t *testing.T) {
			longerPrompt := prompt + " qed" // NB. Must be fewer than XX tokens, as to not be blocked.
			result, err := callIsFlaggedRequest(t, longerPrompt, tokenCountConfig)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsFlagged())
			assert.False(t, result.shouldBlock)
			assert.Contains(t, result.reasons, "high_prompt_token_count")
			assert.Greater(t, result.promptTokenCount, promptTokenCount)
		})

		t.Run("AboveFlaggingLimitAboveBlockLimit", func(t *testing.T) {
			// Create an even longer prompt, more than XX tokens in length to
			// exceed the blocking limit.
			longerPrompt := prompt + " qed. Along with additional information, which I intend to use in order to..."
			result, err := callIsFlaggedRequest(t, longerPrompt, tokenCountConfig)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsFlagged())
			assert.True(t, result.shouldBlock)
			assert.Contains(t, result.reasons, "high_prompt_token_count")
			assert.Greater(t, result.promptTokenCount, tokenCountConfig.PromptTokenBlockingLimit)
		})
	})
}
