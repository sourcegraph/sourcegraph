package completions

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/shared/config"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/anthropic"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/events"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/limiter"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/notify"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/tokenizer"
	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/complete"

const (
	logPromptPrefixLength = 250
)

// PromptRecorder implementations should save select completions prompts for
// a short amount of time for security review.
type PromptRecorder interface {
	Record(ctx context.Context, prompt string) error
}

func NewAnthropicHandler(
	baseLogger log.Logger,
	eventLogger events.Logger,
	rs limiter.RedisStore,
	rateLimitNotifier notify.RateLimitNotifier,
	httpClient httpcli.Doer,
	config config.AnthropicConfig,
	promptRecorder PromptRecorder,
	autoFlushStreamingResponses bool,
) (http.Handler, error) {
	// Tokenizer only needs to be initialized once, and can be shared globally.
	anthropicTokenizer, err := tokenizer.NewAnthropicClaudeTokenizer()
	if err != nil {
		return nil, err
	}

	return makeUpstreamHandler[anthropicRequest](
		baseLogger,
		eventLogger,
		rs,
		rateLimitNotifier,
		httpClient,
		string(conftypes.CompletionsProviderNameAnthropic),
		func(_ codygateway.Feature) string { return anthropicAPIURL },
		config.AllowedModels,
		&AnthropicHandlerMethods{config: config, anthropicTokenizer: anthropicTokenizer, promptRecorder: promptRecorder},

		// Anthropic primarily uses concurrent requests to rate-limit spikes
		// in requests, so set a default retry-after that is likely to be
		// acceptable for Sourcegraph clients to retry (the default
		// SRC_HTTP_CLI_EXTERNAL_RETRY_AFTER_MAX_DURATION) since we might be
		// able to circumvent concurrents limits without raising an error to the
		// user.
		2, // seconds
		autoFlushStreamingResponses,
		config.DetectedPromptPatterns,
	), nil
}

// anthropicRequest captures all known fields from https://console.anthropic.com/docs/api/reference.
type anthropicRequest struct {
	Prompt            string                    `json:"prompt"`
	Model             string                    `json:"model"`
	MaxTokensToSample int32                     `json:"max_tokens_to_sample"`
	StopSequences     []string                  `json:"stop_sequences,omitempty"`
	Stream            bool                      `json:"stream,omitempty"`
	Temperature       float32                   `json:"temperature,omitempty"`
	TopK              int32                     `json:"top_k,omitempty"`
	TopP              float32                   `json:"top_p,omitempty"`
	Metadata          *anthropicRequestMetadata `json:"metadata,omitempty"`

	// Use (*anthropicRequest).GetTokenCount()
	promptTokens *anthropicTokenCount
}

func (ar anthropicRequest) ShouldStream() bool {
	return ar.Stream
}

func (ar anthropicRequest) GetModel() string {
	return ar.Model
}

func (ar anthropicRequest) BuildPrompt() string {
	return ar.Prompt
}

type anthropicTokenCount struct {
	count int
	err   error
}

// GetPromptTokenCount computes the token count of the prompt exactly once using
// the given tokenizer. It is not concurrency-safe.
func (ar *anthropicRequest) GetPromptTokenCount(tk *tokenizer.Tokenizer) (int, error) {
	if ar.promptTokens == nil {
		tokens, err := tk.Tokenize(ar.Prompt)
		ar.promptTokens = &anthropicTokenCount{
			count: len(tokens),
			err:   err,
		}
	}
	return ar.promptTokens.count, ar.promptTokens.err
}

type anthropicRequestMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

// anthropicResponse captures all relevant-to-us fields from https://console.anthropic.com/docs/api/reference.
type anthropicResponse struct {
	Completion string `json:"completion,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
}

type AnthropicHandlerMethods struct {
	anthropicTokenizer *tokenizer.Tokenizer
	promptRecorder     PromptRecorder
	config             config.AnthropicConfig
}

func (a *AnthropicHandlerMethods) validateRequest(ctx context.Context, logger log.Logger, _ codygateway.Feature, ar anthropicRequest) (int, *flaggingResult, error) {
	if ar.MaxTokensToSample > int32(a.config.MaxTokensToSample) {
		return http.StatusBadRequest, nil, errors.Errorf("max_tokens_to_sample exceeds maximum allowed value of %d: %d", a.config.MaxTokensToSample, ar.MaxTokensToSample)
	}

	if result, err := isFlaggedAnthropicRequest(a.anthropicTokenizer, ar, a.config); err != nil {
		logger.Error("error checking anthropic request - treating as non-flagged",
			log.Error(err))
	} else if result.IsFlagged() {
		// Record flagged prompts in hotpath - they usually take a long time on the backend side, so this isn't going to make things meaningfully worse
		if err := a.promptRecorder.Record(ctx, ar.Prompt); err != nil {
			logger.Warn("failed to record flagged prompt", log.Error(err))
		}
		if a.config.RequestBlockingEnabled && result.shouldBlock {
			return http.StatusBadRequest, result, requestBlockedError(ctx)
		}
		return 0, result, nil
	}

	return 0, nil, nil
}
func (a *AnthropicHandlerMethods) transformBody(body *anthropicRequest, identifier string) {
	// Overwrite the metadata field, we don't want to allow users to specify it:
	body.Metadata = &anthropicRequestMetadata{
		// We forward the actor ID to support tracking.
		UserID: identifier,
	}
}
func (a *AnthropicHandlerMethods) getRequestMetadata(body anthropicRequest) (model string, additionalMetadata map[string]any) {
	return body.Model, map[string]any{
		"stream":               body.Stream,
		"max_tokens_to_sample": body.MaxTokensToSample,
	}
}
func (a *AnthropicHandlerMethods) transformRequest(r *http.Request) {
	// Mimic headers set by the official Anthropic client:
	// https://sourcegraph.com/github.com/anthropics/anthropic-sdk-typescript@493075d70f50f1568a276ed0cb177e297f5fef9f/-/blob/src/index.ts
	r.Header.Set("Cache-Control", "no-cache")
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Client", "sourcegraph-cody-gateway/1.0")
	r.Header.Set("X-API-Key", a.config.AccessToken)
	r.Header.Set("anthropic-version", "2023-01-01")
}
func (a *AnthropicHandlerMethods) parseResponseAndUsage(logger log.Logger, reqBody anthropicRequest, r io.Reader) (promptUsage, completionUsage usageStats) {
	var err error
	// First, extract prompt usage details from the request.
	promptUsage.characters = len(reqBody.Prompt)
	promptUsage.tokens, err = reqBody.GetPromptTokenCount(a.anthropicTokenizer)
	if err != nil {
		logger.Error("failed to count tokens in Anthropic response", log.Error(err))
	}

	// Try to parse the request we saw, if it was non-streaming, we can simply parse
	// it as JSON.
	if !reqBody.Stream {
		var res anthropicResponse
		if err := json.NewDecoder(r).Decode(&res); err != nil {
			logger.Error("failed to parse Anthropic response as JSON", log.Error(err))
			return promptUsage, completionUsage
		}

		// Extract usage data from response
		completionUsage.characters = len(res.Completion)
		if tokens, err := a.anthropicTokenizer.Tokenize(res.Completion); err != nil {
			logger.Error("failed to count tokens in Anthropic response", log.Error(err))
		} else {
			completionUsage.tokens = len(tokens)
		}
		return promptUsage, completionUsage
	}

	// Otherwise, we have to parse the event stream from anthropic.
	dec := anthropic.NewDecoder(r)
	var lastCompletion string
	// Consume all the messages, but we only care about the last completion data.
	for dec.Scan() {
		data := dec.Data()

		// Gracefully skip over any data that isn't JSON-like. Anthropic's API sometimes sends
		// non-documented data over the stream, like timestamps.
		if !bytes.HasPrefix(data, []byte("{")) {
			continue
		}

		var event anthropicResponse
		if err := json.Unmarshal(data, &event); err != nil {
			logger.Error("failed to decode event payload", log.Error(err), log.String("body", string(data)))
			continue
		}
		lastCompletion = event.Completion
	}
	if err := dec.Err(); err != nil {
		logger.Error("failed to decode Anthropic streaming response", log.Error(err))
	}

	// Extract usage data from streamed response.
	completionUsage.characters = len(lastCompletion)
	if tokens, err := a.anthropicTokenizer.Tokenize(lastCompletion); err != nil {
		logger.Warn("failed to count tokens in Anthropic response", log.Error(err))
		completionUsage.tokens = -1
	} else {
		completionUsage.tokens = len(tokens)
	}
	return promptUsage, completionUsage
}

func isFlaggedAnthropicRequest(tk *tokenizer.Tokenizer, ar anthropicRequest, cfg config.AnthropicConfig) (*flaggingResult, error) {
	// Only usage of chat models us currently flagged, so if the request
	// is using another model, we skip other checks.
	if ar.Model != "claude-2" && ar.Model != "claude-2.0" && ar.Model != "claude-2.1" && ar.Model != "claude-v1" {
		return nil, nil
	}

	return isFlaggedRequest(tk,
		flaggingRequest{
			FlattenedPrompt: ar.Prompt,
			MaxTokens:       int(ar.MaxTokensToSample),
		},
		flaggingConfig{
			AllowedPromptPatterns:          cfg.AllowedPromptPatterns,
			BlockedPromptPatterns:          cfg.BlockedPromptPatterns,
			PromptTokenFlaggingLimit:       cfg.PromptTokenFlaggingLimit,
			PromptTokenBlockingLimit:       cfg.PromptTokenBlockingLimit,
			MaxTokensToSampleFlaggingLimit: cfg.MaxTokensToSampleFlaggingLimit,
			ResponseTokenBlockingLimit:     cfg.ResponseTokenBlockingLimit,
		},
	)
}
