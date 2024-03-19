package completions

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/shared/config"
	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/events"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/limiter"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/notify"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/tokenizer"
	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/anthropicmessages"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
)

const anthropicMessagesAPIURL = "https://api.anthropic.com/v1/messages"

// This implements the newer `/messages` API by Anthropic
// https://docs.anthropic.com/claude/reference/messages_post
func NewAnthropicMessagesHandler(
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
	tokenizer, err := tokenizer.NewAnthropicClaudeTokenizer()
	if err != nil {
		return nil, err
	}
	return makeUpstreamHandler[anthropicMessagesRequest](
		baseLogger,
		eventLogger,
		rs,
		rateLimitNotifier,
		httpClient,
		string(conftypes.CompletionsProviderNameAnthropic),
		func(_ codygateway.Feature) string { return anthropicMessagesAPIURL },
		config.AllowedModels,
		&AnthropicMessagesHandlerMethods{config: config, tokenizer: tokenizer, promptRecorder: promptRecorder},

		// Anthropic primarily uses concurrent requests to rate-limit spikes
		// in requests, so set a default retry-after that is likely to be
		// acceptable for Sourcegraph clients to retry (the default
		// SRC_HTTP_CLI_EXTERNAL_RETRY_AFTER_MAX_DURATION) since we might be
		// able to circumvent concurrents limits without raising an error to the
		// user.
		2, // seconds
		autoFlushStreamingResponses,
		nil,
	), nil
}

// AnthropicMessagesRequest captures all known fields from https://console.anthropic.com/docs/api/reference.
type anthropicMessagesRequest struct {
	Messages      []anthropicMessage `json:"messages,omitempty"`
	Model         string             `json:"model"`
	MaxTokens     int32              `json:"max_tokens,omitempty"`
	Temperature   float32            `json:"temperature,omitempty"`
	TopP          float32            `json:"top_p,omitempty"`
	TopK          int32              `json:"top_k,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`

	// These are not accepted from the client an instead are only used to talk
	// to the upstream LLM APIs.
	Metadata *anthropicMessagesRequestMetadata `json:"metadata,omitempty"`
	System   string                            `json:"system,omitempty"`
}

type anthropicMessage struct {
	Role    string                    `json:"role"` // "user", "assistant", or "system" (only allowed for the first message)
	Content []anthropicMessageContent `json:"content"`
}

type anthropicMessageContent struct {
	Type string `json:"type"` // "text" or "image" (not yet supported)
	Text string `json:"text"`
}

type anthropicMessagesRequestMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

func (ar anthropicMessagesRequest) ShouldStream() bool {
	return ar.Stream
}

func (ar anthropicMessagesRequest) GetModel() string {
	return ar.Model
}

// Note: This is not the actual prompt send to Anthropic but it's a good
// approximation to measure tokens.
func (r anthropicMessagesRequest) BuildPrompt() string {
	var sb strings.Builder
	for _, m := range r.Messages {
		switch m.Role {
		case "user":
			sb.WriteString("Human: ")
		case "assistant":
			sb.WriteString("Assistant: ")
		case "system":
			sb.WriteString("System: ")
		default:
			return ""
		}

		for _, c := range m.Content {
			if c.Type == "text" {
				sb.WriteString(c.Text)
			}
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// AnthropicMessagesNonStreamingResponse captures all relevant-to-us fields from https://docs.anthropic.com/claude/reference/messages_post.
type anthropicMessagesNonStreamingResponse struct {
	Content    []anthropicMessageContent      `json:"content"`
	Usage      anthropicMessagesResponseUsage `json:"usage"`
	StopReason string                         `json:"stop_reason"`
}

// AnthropicMessagesStreamingResponse captures all relevant-to-us fields from each relevant SSE event from https://docs.anthropic.com/claude/reference/messages_post.
type anthropicMessagesStreamingResponse struct {
	Type         string                                        `json:"type"`
	Delta        *anthropicMessagesStreamingResponseTextBucket `json:"delta"`
	ContentBlock *anthropicMessagesStreamingResponseTextBucket `json:"content_block"`
	Usage        *anthropicMessagesResponseUsage               `json:"usage"`
	Message      *anthropicStreamingResponseMessage            `json:"message"`
}

type anthropicStreamingResponseMessage struct {
	Usage *anthropicMessagesResponseUsage `json:"usage"`
}

type anthropicMessagesStreamingResponseTextBucket struct {
	Text string `json:"text"`
}

type anthropicMessagesResponseUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type AnthropicMessagesHandlerMethods struct {
	tokenizer      *tokenizer.Tokenizer
	promptRecorder PromptRecorder
	config         config.AnthropicConfig
}

func (a *AnthropicMessagesHandlerMethods) validateRequest(ctx context.Context, logger log.Logger, _ codygateway.Feature, ar anthropicMessagesRequest) (int, *flaggingResult, error) {
	if ar.MaxTokens > int32(a.config.MaxTokensToSample) {
		return http.StatusBadRequest, nil, errors.Errorf("max_tokens exceeds maximum allowed value of %d: %d", a.config.MaxTokensToSample, ar.MaxTokens)
	}

	if result, err := isFlaggedAnthropicMessagesRequest(a.tokenizer, ar, a.config); err != nil {
		logger.Error("error checking AnthropicMessages request - treating as non-flagged",
			log.Error(err))
	} else if result.IsFlagged() {
		// Record flagged prompts in hotpath - they usually take a long time on the backend side, so this isn't going to make things meaningfully worse
		if err := a.promptRecorder.Record(ctx, ar.BuildPrompt()); err != nil {
			logger.Warn("failed to record flagged prompt", log.Error(err))
		}
		if a.config.RequestBlockingEnabled && result.shouldBlock {
			return http.StatusBadRequest, result, requestBlockedError(ctx)
		}
		return 0, result, nil
	}

	return 0, nil, nil
}
func (a *AnthropicMessagesHandlerMethods) transformBody(body *anthropicMessagesRequest, identifier string) {
	// Overwrite the metadata field, we don't want to allow users to specify it:
	body.Metadata = &anthropicMessagesRequestMetadata{
		// We forward the actor ID to support tracking.
		UserID: identifier,
	}

	// Remove the `anthropic/` prefix from the model string
	body.Model = strings.TrimPrefix(body.Model, "anthropic/")

	// Convert the eventual first message from `system` to a top-level system prompt
	body.System = "" // prevent the upstream API from setting this
	if len(body.Messages) > 0 && body.Messages[0].Role == "system" {
		body.System = body.Messages[0].Content[0].Text
		body.Messages = body.Messages[1:]
	}
}
func (a *AnthropicMessagesHandlerMethods) getRequestMetadata(body anthropicMessagesRequest) (model string, additionalMetadata map[string]any) {
	return body.Model, map[string]any{
		"stream":     body.Stream,
		"max_tokens": body.MaxTokens,
	}
}
func (a *AnthropicMessagesHandlerMethods) transformRequest(r *http.Request) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-API-Key", a.config.AccessToken)
	r.Header.Set("anthropic-version", "2023-06-01")
}
func (a *AnthropicMessagesHandlerMethods) parseResponseAndUsage(logger log.Logger, body anthropicMessagesRequest, r io.Reader) (promptUsage, completionUsage usageStats) {
	// First, extract prompt usage details from the request.
	for _, m := range body.Messages {
		promptUsage.characters += len(m.Content)
	}
	promptUsage.characters = len(body.System)

	// Try to parse the request we saw, if it was non-streaming, we can simply parse
	// it as JSON.
	if !body.ShouldStream() {
		var res anthropicMessagesNonStreamingResponse
		if err := json.NewDecoder(r).Decode(&res); err != nil {
			logger.Error("failed to parse Anthropic response as JSON", log.Error(err))
			return promptUsage, completionUsage
		}

		// Extract character data from response by summing up all text
		for _, c := range res.Content {
			completionUsage.characters += len(c.Text)
		}
		// Extract prompt usage data from the response
		completionUsage.tokens = res.Usage.OutputTokens
		promptUsage.tokens = res.Usage.InputTokens

		return promptUsage, completionUsage
	}

	// Otherwise, we have to parse the event stream from anthropic.
	dec := anthropicmessages.NewDecoder(r)
	for dec.Scan() {
		data := dec.Data()

		// Gracefully skip over any data that isn't JSON-like. Anthropic's API sometimes sends
		// non-documented data over the stream, like timestamps.
		if !bytes.HasPrefix(data, []byte("{")) {
			continue
		}

		var event anthropicMessagesStreamingResponse
		if err := json.Unmarshal(data, &event); err != nil {
			logger.Error("failed to decode event payload", log.Error(err), log.String("body", string(data)))
			continue
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil && event.Message.Usage != nil {
				promptUsage.tokens = event.Message.Usage.InputTokens
			}
		case "content_block_delta":
			if event.Delta != nil {
				completionUsage.characters += len(event.Delta.Text)
			}
		case "message_delta":
			if event.Usage != nil {
				completionUsage.tokens = event.Usage.OutputTokens
			}
		}
	}
	if err := dec.Err(); err != nil {
		logger.Error("failed to decode Anthropic streaming response", log.Error(err))
	}

	return promptUsage, completionUsage
}

func isFlaggedAnthropicMessagesRequest(tk *tokenizer.Tokenizer, r anthropicMessagesRequest, cfg config.AnthropicConfig) (*flaggingResult, error) {
	return isFlaggedRequest(tk,
		flaggingRequest{
			FlattenedPrompt: r.BuildPrompt(),
			MaxTokens:       int(r.MaxTokens),
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
