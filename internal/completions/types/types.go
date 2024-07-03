package types

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const HUMAN_MESSAGE_SPEAKER = "human"
const ASSISTANT_MESSAGE_SPEAKER = "assistant"
const SYSTEM_MESSAGE_SPEAKER = "system"

type Message struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

func (m Message) IsValidSpeaker() bool {
	return m.Speaker == HUMAN_MESSAGE_SPEAKER || m.Speaker == ASSISTANT_MESSAGE_SPEAKER
}

func (m Message) GetPrompt(humanPromptPrefix, assistantPromptPrefix string) (string, error) {
	var prefix string
	switch m.Speaker {
	case HUMAN_MESSAGE_SPEAKER:
		prefix = humanPromptPrefix
	case ASSISTANT_MESSAGE_SPEAKER:
		prefix = assistantPromptPrefix
	default:
		return "", errors.Newf("expected message speaker to be 'human' or 'assistant', got %s", m.Speaker)
	}

	if len(m.Text) == 0 {
		// Important: no trailing space (affects output quality)
		return prefix, nil
	}
	return fmt.Sprintf("%s %s", prefix, m.Text), nil
}

type CodyCompletionRequestParameters struct {
	CompletionRequestParameters

	// When Fast is true, then it is used as a hint to prefer a model
	// that is faster (but probably "dumber").
	Fast bool
}

type CompletionRequestParameters struct {
	// Prompt exists only for backwards compatibility. Do not use it in new
	// implementations. It will be removed once we are reasonably sure 99%
	// of VSCode extension installations are upgraded to a new Cody version.
	Prompt                                       string    `json:"prompt"`
	Messages                                     []Message `json:"messages"`
	MaxTokensToSample                            int       `json:"maxTokensToSample,omitempty"`
	Temperature                                  float32   `json:"temperature,omitempty"`
	StopSequences                                []string  `json:"stopSequences,omitempty"`
	TopK                                         int       `json:"topK,omitempty"`
	TopP                                         float32   `json:"topP,omitempty"`
	Model                                        string    `json:"model,omitempty"`
	Stream                                       *bool     `json:"stream,omitempty"`
	Logprobs                                     *uint8    `json:"logprobs"`
	User                                         string    `json:"user,omitempty"`
	AzureChatModel                               string    `json:"azureChatModel,omitempty"`
	AzureCompletionModel                         string    `json:"azureCompletionModel,omitempty"`
	AzureUseDeprecatedCompletionsAPIForOldModels bool      `json:"azureUseDeprecatedCompletionsAPIForOldModels,omitempty"`
}

// IsStream returns whether a streaming response is requested. For backwards
// compatibility reasons, we are using a pointer to a bool instead of a bool
// to default to true in case the value is not explicity provided.
func (p CompletionRequestParameters) IsStream(feature CompletionsFeature) bool {
	if p.Stream == nil {
		return defaultStreamMode(feature)
	}
	return *p.Stream
}

func defaultStreamMode(feature CompletionsFeature) bool {
	switch feature {
	case CompletionsFeatureChat:
		return true
	case CompletionsFeatureCode:
		return false
	default:
		// Safeguard, should be never reached.
		return true
	}
}

func (p *CompletionRequestParameters) Attrs(feature CompletionsFeature) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("promptLength", len(p.Prompt)),
		attribute.Int("numMessages", len(p.Messages)),
		attribute.Int("maxTokensToSample", p.MaxTokensToSample),
		attribute.Float64("temperature", float64(p.Temperature)),
		attribute.Int("topK", p.TopK),
		attribute.Float64("topP", float64(p.TopP)),
		attribute.String("model", p.Model),
		attribute.Bool("stream", p.IsStream(feature)),
	}
}

type CompletionResponse struct {
	// Completion is the full completion string.
	// In the V2 API, this field is empty for streaming responses.
	// Use DeltaText instead.
	Completion string `json:"completion,omitempty"`
	// DeltaText is the incremental text that was added to the prompt.
	// It is only present in the V2 API. If set, don't use Completion.
	DeltaText  string    `json:"deltaText,omitempty"`
	StopReason string    `json:"stopReason,omitempty"`
	Logprobs   *Logprobs `json:"logprobs,omitempty"`
}

// CompletionResponseBuilder is used to build a completion response in the right format.
// It is version aware and renders the output in the right format depending on the version.
type CompletionResponseBuilder interface {
	// NextMessage should be called for each message that is returned by the model.
	// text is expected to be incremental over the last invocation of NextMessage.
	NextMessage(text string, logprobs *Logprobs) CompletionResponse
	// Stop should be called when the model has finished producing a response.
	// A stopReason can be given to indicate why the generation stopped.
	Stop(stopReason string) CompletionResponse
}

// NewCompletionResponseBuilder returns a new CompletionResponseBuilder for the given version.
func NewCompletionResponseBuilder(version CompletionsVersion) CompletionResponseBuilder {
	return &completionResponseBuilder{version: version}
}

type completionResponseBuilder struct {
	version         CompletionsVersion
	totalCompletion string
}

func (b *completionResponseBuilder) NextMessage(text string, logprobs *Logprobs) CompletionResponse {
	if b.version == CompletionsV2 {
		return CompletionResponse{
			DeltaText: text,
			Logprobs:  logprobs,
		}
	}

	b.totalCompletion += text

	return CompletionResponse{
		Completion: b.totalCompletion,
		Logprobs:   logprobs,
	}
}

func (b *completionResponseBuilder) Stop(stopReason string) CompletionResponse {
	return CompletionResponse{
		// In < V2, the completion is also set on the stop reason message.
		Completion: b.totalCompletion,
		StopReason: stopReason,
	}
}

type Logprobs struct {
	Tokens        []string             `json:"tokens"`
	TokenLogprobs []float32            `json:"token_logprobs"`
	TopLogprobs   []map[string]float32 `json:"top_logprobs"`
	TextOffset    []int32              `json:"text_offset"`
}

// Append concatenates the additional logprobs to the original ones
// and returns a reference to the mutated original logprobs. Note
// this mutates the receiver. If the receiver is nil, a shallow
// copy of additional is returned.
//
// Intended usage: original = original.Append(additional)
func (original *Logprobs) Append(additional *Logprobs) *Logprobs {
	if original == nil {
		if additional == nil {
			return nil
		} else {
			newLogprobs := *additional
			return &newLogprobs
		}
	}
	if additional == nil {
		return original
	}

	original.Tokens = append(original.Tokens, additional.Tokens...)
	original.TokenLogprobs = append(original.TokenLogprobs, additional.TokenLogprobs...)
	original.TopLogprobs = append(original.TopLogprobs, additional.TopLogprobs...)
	original.TextOffset = append(original.TextOffset, additional.TextOffset...)
	return original
}

type SendCompletionEvent func(event CompletionResponse) error

type CompletionsFeature string

const (
	CompletionsFeatureChat CompletionsFeature = "chat_completions"
	CompletionsFeatureCode CompletionsFeature = "code_completions"
)

func (b CompletionsFeature) IsValid() bool {
	switch b {
	case CompletionsFeatureChat,
		CompletionsFeatureCode:
		return true
	}
	return false
}

// ID returns a numeric ID representing the feature for analytics purposes.
func (b CompletionsFeature) ID() int {
	switch b {
	case CompletionsFeatureChat:
		return 1
	case CompletionsFeatureCode:
		return 2
	default:
		return -1
	}
}

type CompletionsVersion int

const (
	CompletionsVersionLegacy CompletionsVersion = 0
	CompletionsV1            CompletionsVersion = 1
	// CompletionsV2 extends CompletionsV1 by support for delta completions events.
	// The response will only contain newly generated characters, not characters that
	// were already sent.
	CompletionsV2 CompletionsVersion = 2
)

// CodyClientName represents the name of a client in URL query parameters.
type CodyClientName string

const (
	CodyClientWeb       CodyClientName = "web"
	CodyClientVscode    CodyClientName = "vscode"
	CodyClientJetbrains CodyClientName = "jetbrains"
)

type CompletionRequest struct {
	Feature    CompletionsFeature
	Version    CompletionsVersion
	Parameters CompletionRequestParameters
}

type CompletionsClient interface {
	// Stream executions a completions request, streaming results to the callback.
	// Callers should check for ErrStatusNotOK and handle the error appropriately.
	Stream(context.Context, log.Logger, CompletionRequest, SendCompletionEvent) error
	// Complete executions a completions request until done. Callers should check
	// for ErrStatusNotOK and handle the error appropriately.
	Complete(context.Context, log.Logger, CompletionRequest) (*CompletionResponse, error)
}

func ConvertFromLegacyMessages(messages []Message) []Message {
	filteredMessages := make([]Message, 0)
	skipNext := false
	for i, message := range messages {
		if skipNext {
			skipNext = false
			continue
		}

		// 1. If the first message is "system prompt like" convert it to an actual system prompt
		//
		// Note: The prefix we scan for here is used in the current chat prompts for VS Code and the
		//       old Web UI prompt.
		if i == 0 && strings.HasPrefix(message.Text, "You are Cody, an AI") {
			message.Speaker = SYSTEM_MESSAGE_SPEAKER
			skipNext = true
		}

		if i == len(messages)-1 && message.Speaker == ASSISTANT_MESSAGE_SPEAKER {
			// 2. If the last message is from an `assistant` with no or empty `text`, omit it
			if message.Text == "" {
				continue
			}

			// 3. Final assistant content cannot end with trailing whitespace
			message.Text = strings.TrimRight(message.Text, " \t\n\r")

		}

		// 4. If there is any assistant message in the middle of the messages without a `text`, omit
		//    both the empty assistant message as well as the unanswered question from the `user`

		// Don't apply this to the human message before the last message (it should always be included)
		if i >= len(messages)-2 {
			filteredMessages = append(filteredMessages, message)
			continue
		}
		// If the next message is an assistant message with no or empty `content`, omit the current and
		// the next one
		nextMessage := messages[i+1]
		if (nextMessage.Speaker == ASSISTANT_MESSAGE_SPEAKER && nextMessage.Text == "") ||
			(message.Speaker == ASSISTANT_MESSAGE_SPEAKER && message.Text == "") {
			continue
		}
		filteredMessages = append(filteredMessages, message)
	}

	return filteredMessages
}
