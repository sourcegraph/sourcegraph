package types

import (
	"testing"

	"github.com/hexops/autogold/v2"
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
