package fireworks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Single-tenant model identifiers
const Starcoder16bSingleTenant = "accounts/sourcegraph/models/starcoder-16b"

// Multi-tenant model identifiers
const Starcoder16b8bit = "accounts/fireworks/models/starcoder-16b-w8a16"
const Starcoder7b8bit = "accounts/fireworks/models/starcoder-7b-w8a16"
const Starcoder16b = "accounts/fireworks/models/starcoder-16b"
const Starcoder7b = "accounts/fireworks/models/starcoder-7b"
const StarcoderTwo15b = "accounts/sourcegraph/models/starcoder2-15b"
const StarcoderTwo7b = "accounts/sourcegraph/models/starcoder2-7b"
const Llama27bCode = "accounts/fireworks/models/llama-v2-7b-code"
const Llama213bCode = "accounts/fireworks/models/llama-v2-13b-code"
const Llama213bCodeInstruct = "accounts/fireworks/models/llama-v2-13b-code-instruct"
const Llama234bCodeInstruct = "accounts/fireworks/models/llama-v2-34b-code-instruct"
const Llama38bInstruct = "accounts/fireworks/models/llama-v3-8b-instruct"
const Llama370bInstruct = "accounts/fireworks/models/llama-v3-70b-instruct"
const Mistral7bInstruct = "accounts/fireworks/models/mistral-7b-instruct-4k"
const Mixtral8x7bInstruct = "accounts/fireworks/models/mixtral-8x7b-instruct"
const Mixtral8x22Instruct = "accounts/fireworks/models/mixtral-8x22b-instruct"
const DeepseekCoder1p3b = "accounts/sourcegraph/models/custom-deepseek-1p3b-base-hf-version"
const DeepseekCoder7b = "accounts/sourcegraph/models/deepseek-coder-7b-base"

const FineTunedFIMVariant1 = "fim-fine-tuned-model-variant-1"
const FineTunedFIMVariant2 = "fim-fine-tuned-model-variant-2"
const FineTunedFIMVariant3 = "fim-fine-tuned-model-variant-3"
const FineTunedFIMVariant4 = "fim-fine-tuned-model-variant-4"
const FineTunedFIMLangSpecificMixtral = "fim-lang-specific-model-mixtral"

const FineTunedMixtralTypescript = "accounts/sourcegraph/models/finetuned-fim-lang-typescript-model-mixtral-8x7b"
const FineTunedMixtralJavascript = "accounts/sourcegraph/models/finetuned-fim-lang-javascript-model-mixtral-8x7b"
const FineTunedMixtralPhp = "accounts/sourcegraph/models/finetuned-fim-lang-php-model-mixtral-8x7b"
const FineTunedMixtralPython = "accounts/sourcegraph/models/finetuned-fim-lang-python-model-mixtral-8x7b"
const FineTunedMixtralJsx = "accounts/sourcegraph/models/finetuned-fim-lang-jsx-model-mixtral-8x7b"
const FineTunedMixtralAll = "accounts/sourcegraph/models/finetuned-fim-lang-all-model-mixtral-8x7b"

var FineTunedMixtralModelVariants = []string{FineTunedMixtralTypescript, FineTunedMixtralJavascript, FineTunedMixtralPhp, FineTunedMixtralPython, FineTunedMixtralAll, FineTunedMixtralJsx, FineTunedFIMLangSpecificMixtral}

const FineTunedLlamaTypescript = "accounts/sourcegraph/models/lang-typescript-context-fim-meta-llama-3-8b-instruct-e-1"
const FineTunedLlamaJavascript = "accounts/sourcegraph/models/lang-javascript-context-fim-meta-llama-3-8b-instruct-e-1"
const FineTunedLlamaPhp = "accounts/sourcegraph/models/lang-php-context-fim-meta-llama-3-8b-instruct-e-1"
const FineTunedLlamaPython = "accounts/sourcegraph/models/lang-python-context-fim-meta-llama-3-8b-instruct-e-1"
const FineTunedLlamaAll = "accounts/sourcegraph/models/finetuned-fim-lang-all-model-meta-llama-3-8b"

var FineTunedLlamaModelVariants = []string{FineTunedLlamaTypescript, FineTunedLlamaJavascript, FineTunedLlamaPhp, FineTunedLlamaPython, FineTunedLlamaAll}

func NewClient(cli httpcli.Doer, endpoint, accessToken string) types.CompletionsClient {
	return &fireworksClient{
		cli:         cli,
		accessToken: accessToken,
		endpoint:    endpoint,
	}
}

type fireworksClient struct {
	cli         httpcli.Doer
	accessToken string
	endpoint    string
}

func (c *fireworksClient) Complete(
	ctx context.Context,
	logger log.Logger,
	request types.CompletionRequest) (*types.CompletionResponse, error) {
	feature := request.Feature
	requestParams := request.Parameters

	resp, err := c.makeRequest(ctx, feature, requestParams, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response fireworksResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if len(response.Choices) == 0 {
		// Empty response.
		return &types.CompletionResponse{}, nil
	}

	completion := ""
	if response.Choices[0].Text != "" {
		// The /completion endpoint returns a text field ...
		completion = response.Choices[0].Text
	} else if response.Choices[0].Message != nil {
		// ... whereas the /chat/completion endpoints returns this structure
		completion = response.Choices[0].Message.Content
	}

	return &types.CompletionResponse{
		Completion: completion,
		StopReason: response.Choices[0].FinishReason,
		Logprobs:   response.Choices[0].Logprobs,
	}, nil
}

func (c *fireworksClient) Stream(
	ctx context.Context,
	logger log.Logger,
	request types.CompletionRequest,
	sendEvent types.SendCompletionEvent) error {
	feature := request.Feature
	requestParams := request.Parameters
	logprobsInclude := uint8(0)
	requestParams.Logprobs = &logprobsInclude
	builder := types.NewCompletionResponseBuilder(request.Version)

	// HACK: Cody Gateway expects model names in <provider>/<model> format, but if we're connecting directly to
	// the Fireworks API, we need to strip the "fireworks" provider prefix
	if components := strings.Split(requestParams.Model, "/"); components[0] == "fireworks" {
		requestParams.Model = strings.Join(components[1:], "/")
	}

	resp, err := c.makeRequest(ctx, feature, requestParams, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	dec := NewDecoder(resp.Body)
	var accumulatedLogprobs *types.Logprobs
	for dec.Scan() {
		if ctx.Err() != nil && ctx.Err() == context.Canceled {
			return nil
		}

		data := dec.Data()
		// Gracefully skip over any data that isn't JSON-like.
		if !bytes.HasPrefix(data, []byte("{")) {
			continue
		}

		var event fireworksStreamingResponse
		if err := json.Unmarshal(data, &event); err != nil {
			return errors.Errorf("failed to decode event payload: %w - body: %s", err, string(data))
		}

		if len(event.Choices) > 0 {
			// The /completion endpoint returns a text field ...
			content := event.Choices[0].Text
			// ... whereas the /chat/completion endpoints returns this structure
			if event.Choices[0].Delta != nil {
				content = event.Choices[0].Delta.Content
			}
			accumulatedLogprobs = accumulatedLogprobs.Append(event.Choices[0].Logprobs)
			ev := builder.NextMessage(content, accumulatedLogprobs)
			err = sendEvent(ev)
			if err != nil {
				return err
			}

			if event.Choices[0].FinishReason != "" {
				err = sendEvent(builder.Stop(event.Choices[0].FinishReason))
				if err != nil {
					return err
				}
			}
		}
	}

	return dec.Err()
}

func (c *fireworksClient) makeRequest(ctx context.Context, feature types.CompletionsFeature, requestParams types.CompletionRequestParameters, stream bool) (*http.Response, error) {
	if requestParams.TopP < 0 {
		requestParams.TopP = 0
	}

	var reqBody []byte
	var err error
	var endpoint string

	if feature == types.CompletionsFeatureCode {
		// For compatibility reasons with other models, we expect to find the prompt
		// in the first and only message
		prompt, promptErr := getPrompt(requestParams.Messages)
		if promptErr != nil {
			return nil, promptErr
		}

		payload := fireworksRequest{
			Model:       requestParams.Model,
			Temperature: requestParams.Temperature,
			TopP:        requestParams.TopP,
			N:           1,
			Stream:      stream,
			MaxTokens:   int32(requestParams.MaxTokensToSample),
			Stop:        requestParams.StopSequences,
			Echo:        false,
			Prompt:      prompt,
			Logprobs:    requestParams.Logprobs,
		}

		reqBody, err = json.Marshal(payload)
		endpoint = c.endpoint
	} else {
		payload := fireworksChatRequest{
			Model:       requestParams.Model,
			Temperature: requestParams.Temperature,
			TopP:        requestParams.TopP,
			N:           1,
			Stream:      stream,
			MaxTokens:   int32(requestParams.MaxTokensToSample),
			Stop:        requestParams.StopSequences,
		}
		for _, m := range requestParams.Messages {
			var role string
			switch m.Speaker {
			case types.HUMAN_MESSAGE_SPEAKER:
				role = "user"
			case types.ASSISTANT_MESSAGE_SPEAKER:
				role = "assistant"
			default:
				role = strings.ToLower(role)
			}
			payload.Messages = append(payload.Messages, message{
				Role:    role,
				Content: m.Text,
			})
			// HACK: Replace the ending part of the endpint from `/completions` to `/chat/completions`
			//
			// This is _only_ used when running the Fireworks API directly from the SG instance
			// (without Cody Gateway) and is neccessary because every client can only have one
			// endpoint configured at the moment. If the request is routed to Cody Gateway, the
			// endpoint will not have `inference/v1/completions` in the URL
			endpoint = strings.Replace(c.endpoint, "/inference/v1/completions", "/inference/v1/chat/completions", 1)
		}

		reqBody, err = json.Marshal(payload)
	}

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewErrStatusNotOK("Fireworks", resp)
	}

	return resp, nil
}

// fireworksRequest captures fields from https://readme.fireworks.ai/reference/createcompletion
type fireworksRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	MaxTokens   int32    `json:"max_tokens,omitempty"`
	Temperature float32  `json:"temperature,omitempty"`
	TopP        float32  `json:"top_p,omitempty"`
	N           int32    `json:"n,omitempty"`
	Stream      bool     `json:"stream,omitempty"`
	Echo        bool     `json:"echo,omitempty"`
	Stop        []string `json:"stop,omitempty"`
	Logprobs    *uint8   `json:"logprobs,omitempty"`
}

// fireworksChatRequest captures fields from https://readme.fireworks.ai/reference/createchatcompletion
type fireworksChatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	MaxTokens   int32     `json:"max_tokens,omitempty"`
	Temperature float32   `json:"temperature,omitempty"`
	TopP        float32   `json:"top_p,omitempty"`
	N           int32     `json:"n,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response for a non-streaming request.
// It differs from the streaming response in the Choices list.
// This response uses the Message field whereas the streaming response uses the Delta field.
// https://readme.fireworks.ai/reference/createchatcompletion
type fireworksResponse struct {
	Choices []struct {
		Text    string `json:"text"`
		Message *struct {
			Content string `json:"content"`
		} `json:"message"`
		Index        int             `json:"index"`
		FinishReason string          `json:"finish_reason"`
		Logprobs     *types.Logprobs `json:"logprobs"`
	} `json:"choices"`
	Usage fireworksUsage `json:"usage"`
}

type fireworksStreamingResponse struct {
	Choices []struct {
		Text  string `json:"text"`
		Delta *struct {
			Content string `json:"content"`
		} `json:"delta"`
		Index        int             `json:"index"`
		FinishReason string          `json:"finish_reason"`
		Logprobs     *types.Logprobs `json:"logprobs"`
	} `json:"choices"`
	Usage fireworksUsage `json:"usage"`
}

type fireworksUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}
