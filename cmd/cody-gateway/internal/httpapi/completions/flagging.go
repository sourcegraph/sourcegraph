package completions

import (
	"context"
	"slices"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/shared/config"
	"github.com/sourcegraph/sourcegraph/internal/completions/tokenizer"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type flaggingConfig struct {
	// Phrases we look for in the prompt to consider it valid.
	// Each phrase is lower case.
	AllowedPromptPatterns []string
	// Phrases we look for in a flagged request to consider blocking the response.
	// Each phrase is converted to lower case. Can be empty (to disable blocking).
	BlockedPromptPatterns []string
	// Phrases we look for in a request to collect data.
	// Each phrase is lower case. Can be empty (to disable data collection).
	PromptTokenFlaggingLimit       int
	PromptTokenBlockingLimit       int
	MaxTokensToSampleFlaggingLimit int
	ResponseTokenBlockingLimit     int

	// FlaggedModelNames is a slice of LLM model names, e.g. "gpt-3.5-turbo",
	// that will lead to the request getting flagged.
	FlaggedModelNames []string

	// If false, flaggingResult.shouldBlock will always be false when returned by isFlaggedRequest.
	RequestBlockingEnabled bool
}

// makeFlaggingConfig converts the config.FlaggingConfig into the type used in this package.
// (This just avoids taking a hard dependency, allowing the config package to change independently, etc.)
func makeFlaggingConfig(cfg config.FlaggingConfig) flaggingConfig {

	return flaggingConfig{
		AllowedPromptPatterns:          cfg.AllowedPromptPatterns,
		BlockedPromptPatterns:          cfg.BlockedPromptPatterns,
		PromptTokenFlaggingLimit:       cfg.PromptTokenFlaggingLimit,
		PromptTokenBlockingLimit:       cfg.PromptTokenBlockingLimit,
		MaxTokensToSampleFlaggingLimit: cfg.MaxTokensToSampleFlaggingLimit,
		ResponseTokenBlockingLimit:     cfg.ResponseTokenBlockingLimit,
		FlaggedModelNames:              cfg.FlaggedModelNames,
		RequestBlockingEnabled:         cfg.RequestBlockingEnabled,
	}
}

type flaggingRequest struct {
	// ModelName is the slug for the specific LLM model.
	// e.g. "llama-v2-13b-code"
	ModelName       string
	FlattenedPrompt string
	MaxTokens       int
}
type flaggingResult struct {
	shouldBlock       bool
	blockedPhrase     *string
	reasons           []string
	promptPrefix      string
	maxTokensToSample int
	promptTokenCount  int
}

// isFlaggedRequest inspects the request and determines if it should be "flagged". This is how we
// perform basic abuse-detection and filtering. The implementation should err on the side of efficency,
// as the goal isn't for 100% accuracy - isFlaggedRequest should catch obvious abuse patterns, and let other backend
// systems do a more thorough review async.
func isFlaggedRequest(tk tokenizer.Tokenizer, r flaggingRequest, cfg flaggingConfig) (*flaggingResult, error) {
	// Verify that we were given a legitimate flaggingConfig. Blocking all requests is
	// kinda lame. But failing loudly is preferable to banning users because 100% of their
	// requests get flagged.
	if cfg.MaxTokensToSampleFlaggingLimit == 0 || cfg.ResponseTokenBlockingLimit == 0 {
		return nil, errors.New("flaggingConfig object is invalid")
	}

	var reasons []string
	prompt := strings.ToLower(r.FlattenedPrompt)

	if r.ModelName != "" && slices.Contains(cfg.FlaggedModelNames, r.ModelName) {
		reasons = append(reasons, "model_used")
	}

	if hasValidPattern, _ := containsAny(prompt, cfg.AllowedPromptPatterns); len(cfg.AllowedPromptPatterns) > 0 && !hasValidPattern {
		reasons = append(reasons, "unknown_prompt")
	}

	// If this request has a very high token count for responses, then flag it.
	if r.MaxTokens > cfg.MaxTokensToSampleFlaggingLimit {
		reasons = append(reasons, "high_max_tokens_to_sample")
	}

	// For more accurate flagging, we need to take the actual tokenization of the prompt
	// into account. However, not every LLM integration has that available.
	tokenCount := -1
	if tk != nil {
		tokens, err := tk.Tokenize(r.FlattenedPrompt)
		if err != nil {
			return &flaggingResult{}, errors.Wrap(err, "tokenizing prompt")
		}

		tokenCount = len(tokens)
		if tokenCount > cfg.PromptTokenFlaggingLimit {
			reasons = append(reasons, "high_prompt_token_count")
		}
	}

	// The request has been flagged. Now we determine if it is serious enough to outright block the request.
	var blocked bool
	hasBlockedPhrase, phrase := containsAny(prompt, cfg.BlockedPromptPatterns)
	if tokenCount > cfg.PromptTokenBlockingLimit || r.MaxTokens > cfg.ResponseTokenBlockingLimit || hasBlockedPhrase {
		blocked = true
		reasons = append(reasons, "blocked_phrase")
	}

	if len(reasons) == 0 {
		return nil, nil
	}

	// Maximum number of characters of the prompt prefix we include in logs and telemetry.
	const logPromptPrefixLength = 250
	promptPrefix := r.FlattenedPrompt
	if len(promptPrefix) > logPromptPrefixLength {
		promptPrefix = promptPrefix[:logPromptPrefixLength]
	}

	res := &flaggingResult{
		reasons:           reasons,
		maxTokensToSample: r.MaxTokens,
		promptPrefix:      promptPrefix,
		promptTokenCount:  tokenCount,
		shouldBlock:       blocked,
	}
	if hasBlockedPhrase {
		res.blockedPhrase = &phrase
	}

	// Honor the configuration setting for disabling request blocking.
	res.shouldBlock = res.shouldBlock && cfg.RequestBlockingEnabled

	return res, nil
}

func (f *flaggingResult) IsFlagged() bool {
	return f != nil
}

func containsAny(prompt string, patterns []string) (bool, string) {
	prompt = strings.ToLower(prompt)
	for _, pattern := range patterns {
		if strings.Contains(prompt, pattern) {
			return true, pattern
		}
	}
	return false, ""
}

// requestBlockedError returns an error indicating that the request was blocked, including the trace ID.
func requestBlockedError(ctx context.Context) error {
	traceID := trace.FromContext(ctx).SpanContext().TraceID().String()
	return errors.Errorf("We blocked your request because we detected your prompt to be against our Acceptable Use Policy (https://sourcegraph.com/terms/aup). Try again by removing any phrases that may violate our AUP. If you think this is a mistake, please contact support@sourcegraph.com and reference this ID: %s", traceID)
}

// PromptRecorder implementations should save select completions prompts for
// a short amount of time for security review.
type PromptRecorder interface {
	Record(ctx context.Context, prompt string) error
}
