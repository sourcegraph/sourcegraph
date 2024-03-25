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
	"github.com/sourcegraph/sourcegraph/internal/completions/client/openai"
	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/events"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/limiter"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/notify"
	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
)

func NewOpenAIHandler(
	baseLogger log.Logger,
	eventLogger events.Logger,
	rs limiter.RedisStore,
	rateLimitNotifier notify.RateLimitNotifier,
	httpClient httpcli.Doer,
	config config.OpenAIConfig,
	autoFlushStreamingResponses bool,
) http.Handler {
	return makeUpstreamHandler[openaiRequest](
		baseLogger,
		eventLogger,
		rs,
		rateLimitNotifier,
		httpClient,
		string(conftypes.CompletionsProviderNameOpenAI),
		config.AllowedModels,
		&OpenAIHandlerMethods{config: config},

		// OpenAI primarily uses tokens-per-minute ("TPM") to rate-limit spikes
		// in requests, so set a very high retry-after to discourage Sourcegraph
		// clients from retrying at all since retries are probably not going to
		// help in a minute-long rate limit window.
		30, // seconds
		autoFlushStreamingResponses,
	)
}

type openaiRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

type openaiRequest struct {
	Model            string                 `json:"model"`
	Messages         []openaiRequestMessage `json:"messages"`
	Temperature      float32                `json:"temperature,omitempty"`
	TopP             float32                `json:"top_p,omitempty"`
	N                int                    `json:"n,omitempty"`
	Stream           bool                   `json:"stream,omitempty"`
	Stop             []string               `json:"stop,omitempty"`
	MaxTokens        int                    `json:"max_tokens,omitempty"`
	PresencePenalty  float32                `json:"presence_penalty,omitempty"`
	FrequencyPenalty float32                `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float32     `json:"logit_bias,omitempty"`
	User             string                 `json:"user,omitempty"`
}

func (r openaiRequest) ShouldStream() bool {
	return r.Stream
}

func (r openaiRequest) GetModel() string {
	return r.Model
}

func (r openaiRequest) BuildPrompt() string {
	var sb strings.Builder
	for _, m := range r.Messages {
		sb.WriteString(m.Content + "\n")
	}
	return sb.String()
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openaiChoiceDelta struct {
	Content string `json:"content"`
}

type openaiChoice struct {
	Delta        openaiChoiceDelta `json:"delta"`
	Role         string            `json:"role"`
	Content      string            `json:"content"`
	FinishReason string            `json:"finish_reason"`
}

type openaiResponse struct {
	// Usage is only available for non-streaming requests.
	Usage   openaiUsage    `json:"usage"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
}

type OpenAIHandlerMethods struct {
	config config.OpenAIConfig
}

func (*OpenAIHandlerMethods) getAPIURLByFeature(feature codygateway.Feature) string {
	return "https://api.openai.com/v1/chat/completions"
}

func (*OpenAIHandlerMethods) validateRequest(_ context.Context, _ log.Logger, feature codygateway.Feature, _ openaiRequest) error {
	if feature == codygateway.FeatureCodeCompletions {
		return errors.Newf("feature %q is currently not supported for OpenAI", feature)
	}
	return nil
}

func (o *OpenAIHandlerMethods) shouldFlagRequest(ctx context.Context, logger log.Logger, req openaiRequest) (*flaggingResult, error) {
	result, err := isFlaggedRequest(
		nil, /* tokenzier, meaning token counts aren't considered when for flagging consideration. */
		flaggingRequest{
			FlattenedPrompt: req.BuildPrompt(),
			MaxTokens:       int(req.MaxTokens),
		},
		makeFlaggingConfig(o.config.FlaggingConfig))
	return result, err
}

func (*OpenAIHandlerMethods) transformBody(body *openaiRequest, identifier string) {
	// We don't want to let users generate multiple responses, as this would
	// mess with rate limit counting.
	if body.N > 1 {
		body.N = 1
	}
	// We forward the actor ID to support tracking.
	body.User = identifier
}

func (*OpenAIHandlerMethods) getRequestMetadata(body openaiRequest) (model string, additionalMetadata map[string]any) {
	return body.Model, map[string]any{"stream": body.Stream}
}

func (o *OpenAIHandlerMethods) transformRequest(r *http.Request) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+o.config.AccessToken)
	if o.config.OrgID != "" {
		r.Header.Set("OpenAI-Organization", o.config.OrgID)
	}
}

func (*OpenAIHandlerMethods) parseResponseAndUsage(logger log.Logger, body openaiRequest, r io.Reader) (promptUsage, completionUsage usageStats) {
	// First, extract prompt usage details from the request.
	for _, m := range body.Messages {
		promptUsage.characters += len(m.Content)
	}

	// Try to parse the request we saw, if it was non-streaming, we can simply parse
	// it as JSON.
	if !body.Stream {
		var res openaiResponse
		if err := json.NewDecoder(r).Decode(&res); err != nil {
			logger.Error("failed to parse OpenAI response as JSON", log.Error(err))
			return promptUsage, completionUsage
		}

		// Extract usage data from response
		promptUsage.tokens = res.Usage.PromptTokens
		completionUsage.tokens = res.Usage.CompletionTokens
		if len(res.Choices) > 0 {
			completionUsage.characters = len(res.Choices[0].Content)
		}
		return promptUsage, completionUsage
	}

	// Otherwise, we have to parse the event stream.
	//
	// Currently, OpenAI only reports usage on non-streaming requests
	// Until we can tokenize the response ourselves, just count
	// character usage, and set token counts to -1 as sentinel values.
	// TODO: https://github.com/sourcegraph/sourcegraph/issues/56590
	promptUsage.tokens = -1
	completionUsage.tokens = -1

	dec := openai.NewDecoder(r)
	// Consume all the messages, but we only care about the last completion data.
	for dec.Scan() {
		data := dec.Data()

		// Gracefully skip over any data that isn't JSON-like.
		if !bytes.HasPrefix(data, []byte("{")) {
			continue
		}

		var event openaiResponse
		if err := json.Unmarshal(data, &event); err != nil {
			logger.Error("failed to decode event payload", log.Error(err), log.String("body", string(data)))
			continue
		}
		if len(event.Choices) > 0 {
			completionUsage.characters += len(event.Choices[0].Delta.Content)
		}
		// These are only included in the last message, so we're not worried about overwriting
		if event.Usage.PromptTokens > 0 {
			promptUsage.tokens = event.Usage.PromptTokens
		}
		if event.Usage.CompletionTokens > 0 {
			completionUsage.tokens = event.Usage.CompletionTokens
		}
	}
	if completionUsage.tokens == -1 || promptUsage.tokens == -1 {
		logger.Warn("did not extract token counts from OpenAI streaming response", log.Int("prompt-tokens", promptUsage.tokens), log.Int("completion-tokens", completionUsage.tokens))
	}
	if err := dec.Err(); err != nil {
		logger.Error("failed to decode OpenAI streaming response", log.Error(err))
	}

	return promptUsage, completionUsage
}
