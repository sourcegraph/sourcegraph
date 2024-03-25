package types

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"

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
	Prompt            string    `json:"prompt"`
	Messages          []Message `json:"messages"`
	MaxTokensToSample int       `json:"maxTokensToSample,omitempty"`
	Temperature       float32   `json:"temperature,omitempty"`
	StopSequences     []string  `json:"stopSequences,omitempty"`
	TopK              int       `json:"topK,omitempty"`
	TopP              float32   `json:"topP,omitempty"`
	Model             string    `json:"model,omitempty"`
	Stream            *bool     `json:"stream,omitempty"`
	Logprobs          *uint8    `json:"logprobs"`
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
	Completion string    `json:"completion"`
	StopReason string    `json:"stopReason"`
	Logprobs   *Logprobs `json:"logprobs,omitempty"`
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
)

type CompletionsClient interface {
	// Stream executions a completions request, streaming results to the callback.
	// Callers should check for ErrStatusNotOK and handle the error appropriately.
	Stream(context.Context, CompletionsFeature, CompletionsVersion, CompletionRequestParameters, SendCompletionEvent) error
	// Complete executions a completions request until done. Callers should check
	// for ErrStatusNotOK and handle the error appropriately.
	Complete(context.Context, CompletionsFeature, CompletionsVersion, CompletionRequestParameters) (*CompletionResponse, error)
}
