package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/completions/tokenusage"
	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Anthropic model: https://docs.anthropic.com/en/docs/about-claude/models
const Claude3Haiku = "claude-3-haiku-20240307"
const Claude3Sonnet = "claude-3-sonnet-20240229"
const Claude3Opus = "claude-3-opus-20240229"
const Claude35Sonnet = "claude-3-5-sonnet-20240620"

func NewClient(cli httpcli.Doer, apiURL, accessToken string, viaGateway bool, tokenManager tokenusage.Manager) types.CompletionsClient {

	return &anthropicClient{
		cli:          cli,
		accessToken:  accessToken,
		apiURL:       apiURL,
		viaGateway:   viaGateway,
		tokenManager: tokenManager,
	}
}

const (
	clientID = "sourcegraph/1.0"
)

type anthropicClient struct {
	cli          httpcli.Doer
	accessToken  string
	apiURL       string
	viaGateway   bool
	tokenManager tokenusage.Manager
}

func (a *anthropicClient) Complete(
	ctx context.Context,
	logger log.Logger,
	request types.CompletionRequest) (*types.CompletionResponse, error) {

	feature := request.Feature
	version := request.Version
	requestParams := request.Parameters

	resp, err := a.makeRequest(ctx, requestParams, version, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response anthropicNonStreamingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	completion := ""
	for _, content := range response.Content {
		completion += content.Text
	}

	err = a.tokenManager.UpdateTokenCountsFromModelUsage(response.Usage.InputTokens, response.Usage.OutputTokens, "anthropic/"+requestParams.Model, string(feature), tokenusage.Anthropic)
	if err != nil {
		return nil, err
	}

	return &types.CompletionResponse{
		Completion: completion,
		StopReason: response.StopReason,
	}, nil
}

func (a *anthropicClient) Stream(
	ctx context.Context,
	logger log.Logger,
	request types.CompletionRequest,
	sendEvent types.SendCompletionEvent) error {

	feature := request.Feature
	builder := types.NewCompletionResponseBuilder(request.Version)
	requestParams := request.Parameters

	resp, err := a.makeRequest(ctx, requestParams, request.Version, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	dec := NewDecoder(resp.Body)
	var inputPromptTokens int
	for dec.Scan() {
		if ctx.Err() != nil && ctx.Err() == context.Canceled {
			return nil
		}

		data := dec.Data()
		// Gracefully skip over any data that isn't JSON-like. Anthropic's API sometimes sends
		// non-documented data over the stream, like timestamps.
		if !bytes.HasPrefix(data, []byte("{")) {
			continue
		}

		var event anthropicStreamingResponse
		if err := json.Unmarshal(data, &event); err != nil {
			return errors.Errorf("failed to decode event payload: %w - body: %s", err, string(data))
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil && event.Message.Usage != nil {
				inputPromptTokens = event.Message.Usage.InputTokens
			}
			continue
		case "content_block_delta":
			if event.Delta != nil {
				ev := builder.NextMessage(event.Delta.Text, nil)
				err = sendEvent(ev)
				if err != nil {
					return err
				}
			}
		case "message_delta":
			if event.Delta != nil {
				err = a.tokenManager.UpdateTokenCountsFromModelUsage(inputPromptTokens, event.Usage.OutputTokens, "anthropic/"+requestParams.Model, string(feature), tokenusage.Anthropic)
				if err != nil {
					logger.Warn("Failed to count tokens with the token manager %w ", log.Error(err))
				}
				err = sendEvent(builder.Stop(event.Delta.StopReason))
				if err != nil {
					return err
				}
			}
		default:
			continue
		}
	}
	return dec.Err()
}

func (a *anthropicClient) makeRequest(ctx context.Context, requestParams types.CompletionRequestParameters, version types.CompletionsVersion, stream bool) (*http.Response, error) {
	convertedMessages := requestParams.Messages
	stopSequences := removeWhitespaceOnlySequences(requestParams.StopSequences)
	if version == types.CompletionsVersionLegacy {
		convertedMessages = types.ConvertFromLegacyMessages(convertedMessages)
	}
	var payload any
	messages, err := toAnthropicMessages(convertedMessages)
	if err != nil {
		return nil, err
	}
	messagesPayload := anthropicRequestParameters{
		Messages:      messages,
		Stream:        stream,
		StopSequences: stopSequences,
		Model:         pinModel(requestParams.Model),
		Temperature:   requestParams.Temperature,
		MaxTokens:     requestParams.MaxTokensToSample,
		TopP:          requestParams.TopP,
		TopK:          requestParams.TopK,
	}

	if !a.viaGateway {
		// Convert the eventual first message from `system` to a top-level system prompt
		messagesPayload.System = "" // prevent the upstream API from setting this
		if len(messagesPayload.Messages) > 0 && messagesPayload.Messages[0].Role == types.SYSTEM_MESSAGE_SPEAKER {
			messagesPayload.System = messagesPayload.Messages[0].Content[0].Text
			messagesPayload.Messages = messagesPayload.Messages[1:]
		}
	}

	payload = messagesPayload

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	// Mimic headers set by the official Anthropic client:
	// https://sourcegraph.com/github.com/anthropics/anthropic-sdk-typescript@493075d70f50f1568a276ed0cb177e297f5fef9f/-/blob/src/index.ts
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Client", clientID)
	req.Header.Set("X-API-Key", a.accessToken)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.cli.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewErrStatusNotOK("Anthropic", resp)
	}

	return resp, nil
}

type anthropicRequestParameters struct {
	Messages      []anthropicMessage `json:"messages,omitempty"`
	Model         string             `json:"model"`
	Temperature   float32            `json:"temperature,omitempty"`
	TopP          float32            `json:"top_p,omitempty"`
	TopK          int                `json:"top_k,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	MaxTokens     int                `json:"max_tokens,omitempty"`

	// These are not accepted from the client an instead are only used to talk to the upstream LLM
	// APIs directly (these do NOT need to be set when talking to Cody Gateway)
	System string `json:"system,omitempty"`
}

type anthropicMessage struct {
	Role    string                    `json:"role"` // "user", "assistant", or "system" (only allowed for the first message)
	Content []anthropicMessageContent `json:"content"`
}

type anthropicMessageContent struct {
	Type string `json:"type"` // "text" or "image" (not yet supported)
	Text string `json:"text"`
}

type anthropicNonStreamingResponse struct {
	Content    []anthropicMessageContent      `json:"content"`
	Usage      anthropicMessagesResponseUsage `json:"usage"`
	StopReason string                         `json:"stop_reason"`
}

// AnthropicMessagesStreamingResponse captures all relevant-to-us fields from each relevant SSE event from https://docs.anthropic.com/claude/reference/messages_post.
type anthropicStreamingResponse struct {
	Type         string                                `json:"type"`
	Delta        *anthropicStreamingResponseTextBucket `json:"delta"`
	ContentBlock *anthropicStreamingResponseTextBucket `json:"content_block"`
	Usage        *anthropicMessagesResponseUsage       `json:"usage"`
	Message      *anthropicStreamingResponseMessage    `json:"message"`
}

type anthropicStreamingResponseMessage struct {
	Usage *anthropicMessagesResponseUsage `json:"usage"`
}

type anthropicMessagesResponseUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicStreamingResponseTextBucket struct {
	Text       string `json:"text"`        // for event `content_block_delta`
	StopReason string `json:"stop_reason"` // for event `message_delta`
}

// The /stream API does not support unpinned models
func pinModel(model string) string {
	switch model {
	case "claude-instant-1",
		"claude-instant-v1":
		return "claude-instant-1.2"
	case "claude-2":
		return "claude-2.0"
	default:
		return model
	}
}
