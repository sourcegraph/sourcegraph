package completions

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"strings"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/shared/config"

	"github.com/sourcegraph/sourcegraph/internal/completions/client/fireworks"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/events"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/limiter"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/notify"
	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
)

func NewFireworksHandler(
	baseLogger log.Logger,
	eventLogger events.Logger,
	rs limiter.RedisStore,
	rateLimitNotifier notify.RateLimitNotifier,
	httpClient httpcli.Doer,
	config config.FireworksConfig,
	autoFlushStreamingResponses bool,
) http.Handler {
	return makeUpstreamHandler[fireworksRequest](
		baseLogger,
		eventLogger,
		rs,
		rateLimitNotifier,
		httpClient,
		string(conftypes.CompletionsProviderNameFireworks),
		config.AllowedModels,
		&FireworksHandlerMethods{
			baseLogger:  baseLogger,
			eventLogger: eventLogger,
			config:      config,
		},
		// Setting to a valuer higher than SRC_HTTP_CLI_EXTERNAL_RETRY_AFTER_MAX_DURATION to not
		// do any retries
		30, // seconds
		autoFlushStreamingResponses,
	)
}

// fireworksRequest captures fields from https://readme.fireworks.ai/reference/createcompletion and
// https://readme.fireworks.ai/reference/createchatcompletion.
type fireworksRequest struct {
	Prompt      string    `json:"prompt,omitempty"`
	Messages    []message `json:"messages,omitempty"`
	Model       string    `json:"model"`
	MaxTokens   int32     `json:"max_tokens,omitempty"`
	Temperature float32   `json:"temperature,omitempty"`
	TopP        float32   `json:"top_p,omitempty"`
	N           int32     `json:"n,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Echo        bool      `json:"echo,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

func (fr fireworksRequest) ShouldStream() bool {
	return fr.Stream
}

func (fr fireworksRequest) GetModel() string {
	return fr.Model
}

func (fr fireworksRequest) BuildPrompt() string {
	if fr.Prompt != "" {
		return fr.Prompt
	}
	var sb strings.Builder
	for _, m := range fr.Messages {
		sb.WriteString(m.Content + "\n")
	}
	return sb.String()
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type fireworksResponse struct {
	Choices []struct {
		Text         string `json:"text"`
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		TotalTokens      int `json:"total_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type FireworksHandlerMethods struct {
	baseLogger  log.Logger
	eventLogger events.Logger
	config      config.FireworksConfig
}

func (f *FireworksHandlerMethods) getAPIURLByFeature(feature codygateway.Feature) string {
	if feature == codygateway.FeatureChatCompletions {
		return "https://api.fireworks.ai/inference/v1/chat/completions"
	} else {
		return "https://api.fireworks.ai/inference/v1/completions"
	}
}

func (f *FireworksHandlerMethods) validateRequest(_ context.Context, _ log.Logger, _ codygateway.Feature, _ fireworksRequest) error {
	// TODO[#61278]: Add missing request validation for all LLM providers in Cody Gateway.
	return nil
}

func (f *FireworksHandlerMethods) shouldFlagRequest(_ context.Context, _ log.Logger, _ fireworksRequest) (*flaggingResult, error) {
	// TODO[#61278]: Add missing request validation for all LLM providers in Cody Gateway.
	return nil, nil
}

func (f *FireworksHandlerMethods) transformBody(body *fireworksRequest, _ string) {
	// We don't want to let users generate multiple responses, as this would
	// mess with rate limit counting.
	if body.N > 1 {
		body.N = 1
	}

	body.Model = pickStarCoderModel(body.Model, f.config)
}

func (f *FireworksHandlerMethods) getRequestMetadata(body fireworksRequest) (model string, additionalMetadata map[string]any) {
	return body.Model, map[string]any{"stream": body.Stream}
}

func (f *FireworksHandlerMethods) transformRequest(r *http.Request) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+f.config.AccessToken)
}

func (f *FireworksHandlerMethods) parseResponseAndUsage(logger log.Logger, reqBody fireworksRequest, r io.Reader) (promptUsage, completionUsage usageStats) {
	// First, extract prompt usage details from the request.
	promptUsage.characters = len(reqBody.Prompt)

	// Try to parse the request we saw, if it was non-streaming, we can simply parse
	// it as JSON.
	if !reqBody.Stream {
		var res fireworksResponse
		if err := json.NewDecoder(r).Decode(&res); err != nil {
			logger.Error("failed to parse fireworks response as JSON", log.Error(err))
			return promptUsage, completionUsage
		}

		promptUsage.tokens = res.Usage.PromptTokens
		completionUsage.tokens = res.Usage.CompletionTokens
		if len(res.Choices) > 0 {
			// TODO: Later, we should look at the usage field.
			completionUsage.characters = len(res.Choices[0].Text)
		}
		return promptUsage, completionUsage
	}

	// Otherwise, we have to parse the event stream.
	//
	// TODO: Does fireworks streaming include usage data?
	// Unclear in the API currently: https://readme.fireworks.ai/reference/createcompletion
	// For now, just count character usage, and set token counts to
	// -1 as sentinel values.
	promptUsage.tokens = -1
	completionUsage.tokens = -1

	dec := fireworks.NewDecoder(r)
	// Consume all the messages, but we only care about the last completion data.
	for dec.Scan() {
		data := dec.Data()

		// Gracefully skip over any data that isn't JSON-like.
		if !bytes.HasPrefix(data, []byte("{")) {
			continue
		}

		var event fireworksResponse
		if err := json.Unmarshal(data, &event); err != nil {
			logger.Error("failed to decode event payload", log.Error(err), log.String("body", string(data)))
			continue
		}

		if len(event.Choices) > 0 {
			completionUsage.characters += len(event.Choices[0].Text)
		}
		// These are only included in the last message, so we're not worried about overwriting
		if event.Usage.PromptTokens > 0 {
			promptUsage.tokens = event.Usage.PromptTokens
		}
		if event.Usage.CompletionTokens > 0 {
			completionUsage.tokens = event.Usage.CompletionTokens
		}
	}
	if err := dec.Err(); err != nil {
		logger.Error("failed to decode Fireworks streaming response", log.Error(err))
	}
	if completionUsage.tokens == -1 || promptUsage.tokens == -1 {
		logger.Warn("did not extract token counts from Fireworks streaming response", log.Int("prompt-tokens", promptUsage.tokens), log.Int("completion-tokens", completionUsage.tokens))
	}

	return promptUsage, completionUsage
}

func pickStarCoderModel(model string, config config.FireworksConfig) string {
	if model == "starcoder" {
		// Enterprise virtual model string
		model = pickModelBasedOnTrafficSplit(config.StarcoderEnterpriseSingleTenantPercent, fireworks.Starcoder16bSingleTenant, fireworks.Starcoder16b)
	} else if model == "starcoder-16b" || model == "starcoder-7b" {
		// PLG virtual model strings
		multiTenantModel := fireworks.Starcoder16b
		if model == "starcoder-7b" {
			multiTenantModel = fireworks.Starcoder7b
		}
		model = pickModelBasedOnTrafficSplit(config.StarcoderCommunitySingleTenantPercent, fireworks.Starcoder16bSingleTenant, multiTenantModel)
	}

	// PLG virtual model strings
	if model == "starcoder2-15b" {
		model = fireworks.StarcoderTwo15b
	}
	if model == "starcoder2-7b" {
		model = fireworks.StarcoderTwo7b
	}

	return model
}

// Picks a model based on a specific percentage split. If the percent value is 0, the
// zeroPercentModel is always picked. If the value is 100, the hundredPercentModel is always picked.
func pickModelBasedOnTrafficSplit(percentage int, hundredPercentModel string, zeroPercentModel string) string {
	// Create a value inside the range of [0, 100).
	roll := rand.Intn(100)

	// Check if the roll is within the target percentage:
	//
	// - If the percentage is `0`, the roll will never be smaller than percentage
	// - If the percentage is `100`, the roll will always be smaller than percentage
	// - Otherwise, e.g. for a percentage of `30`, the roll will have exactly 30 out of 100 possible
	//   draws (since it will be < only if it is within the range [0, 30))
	if roll < percentage {
		return hundredPercentModel
	} else {
		return zeroPercentModel
	}
}
