package types

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
)

func TestLegacyMessageConversion(t *testing.T) {
	messages := []Message{
		// Convert legacy system-like messages to actual system messages
		{Speaker: "human", Text: "You are Cody, an AI-powered coding assistant created by Sourcegraph. You also have an Austrian dialect."},
		{Speaker: "assistant", Text: "I understand"},

		// Removes any messages that did not get an answer?
		{Speaker: "human", Text: "Write a poem"},
		{Speaker: "assistant"}, // <- can happen when the connection is interrupted

		{Speaker: "human", Text: "Write a poem"},
		{Speaker: "Roses are red, violets are blue, here is a poem just for you!"},

		{Speaker: "human", Text: "Write another poem"},
		// Removes the last empty assistant message
		{Speaker: "assistant"},
	}

	convertedMessages := ConvertFromLegacyMessages(messages)

	autogold.Expect([]Message{
		{
			Speaker: "system",
			Text:    "You are Cody, an AI-powered coding assistant created by Sourcegraph. You also have an Austrian dialect.",
		},
		{
			Speaker: "human",
			Text:    "Write a poem",
		},
		{Speaker: "Roses are red, violets are blue, here is a poem just for you!"},
		{
			Speaker: "human",
			Text:    "Write another poem",
		},
	}).Equal(t, convertedMessages)
}

func TestLegacyMessageConversionWithTrailingAssistantResponse(t *testing.T) {
	messages := []Message{
		{Speaker: "human", Text: "Write another poem"},
		// Removes the last empty assistant message
		{Speaker: "assistant", Text: "Roses are red, "},
	}

	convertedMessages := ConvertFromLegacyMessages(messages)

	autogold.Expect([]Message{{
		Speaker: "human",
		Text:    "Write another poem",
	},
		{
			Speaker: "assistant",
			Text:    "Roses are red,",
		},
	}).Equal(t, convertedMessages)
}

func TestCompletionResponseBuilder_NextMessage(t *testing.T) {
	t.Run("V1", func(t *testing.T) {
		builder := &completionResponseBuilder{version: CompletionsV1}
		resp := builder.NextMessage("hello", nil)
		assert.Equal(t, "hello", resp.Completion)
		assert.Nil(t, resp.Logprobs)

		resp = builder.NextMessage(" world", nil)
		assert.Equal(t, "hello world", resp.Completion)
		assert.Nil(t, resp.Logprobs)
	})

	t.Run("V2", func(t *testing.T) {
		builder := &completionResponseBuilder{version: CompletionsV2}
		resp := builder.NextMessage("hello", nil)
		assert.Equal(t, "hello", resp.DeltaText)
		assert.Nil(t, resp.Logprobs)

		resp = builder.NextMessage(" world", nil)
		assert.Equal(t, " world", resp.DeltaText)
		assert.Nil(t, resp.Logprobs)
	})
}

func TestCompletionResponseBuilder_Stop(t *testing.T) {
	t.Run("V1", func(t *testing.T) {
		builder := &completionResponseBuilder{version: CompletionsV1, totalCompletion: "hello world"}
		resp := builder.Stop("stop reason")
		assert.Equal(t, "hello world", resp.Completion)
		assert.Equal(t, "stop reason", resp.StopReason)
	})

	t.Run("V2", func(t *testing.T) {
		builder := &completionResponseBuilder{version: CompletionsV2}
		resp := builder.Stop("stop reason")
		assert.Empty(t, resp.Completion)
		assert.Empty(t, resp.DeltaText)
		assert.Equal(t, "stop reason", resp.StopReason)
	})
}
