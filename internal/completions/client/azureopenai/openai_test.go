package azureopenai

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/completions/tokenusage"
	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type mockAzureClient struct {
	getCompletionsStream     func(ctx context.Context, body azopenai.CompletionsOptions, options *azopenai.GetCompletionsStreamOptions) (azopenai.GetCompletionsStreamResponse, error)
	getCompletions           func(ctx context.Context, body azopenai.CompletionsOptions, options *azopenai.GetCompletionsOptions) (azopenai.GetCompletionsResponse, error)
	getChatCompletions       func(ctx context.Context, body azopenai.ChatCompletionsOptions, options *azopenai.GetChatCompletionsOptions) (azopenai.GetChatCompletionsResponse, error)
	getChatCompletionsStream func(ctx context.Context, body azopenai.ChatCompletionsOptions, options *azopenai.GetChatCompletionsStreamOptions) (azopenai.GetChatCompletionsStreamResponse, error)
}

func (c *mockAzureClient) GetCompletionsStream(ctx context.Context, body azopenai.CompletionsOptions, options *azopenai.GetCompletionsStreamOptions) (azopenai.GetCompletionsStreamResponse, error) {
	return c.getCompletionsStream(ctx, body, options)
}

func (c *mockAzureClient) GetCompletions(ctx context.Context, body azopenai.CompletionsOptions, options *azopenai.GetCompletionsOptions) (azopenai.GetCompletionsResponse, error) {
	return c.getCompletions(ctx, body, options)
}

func (c *mockAzureClient) GetChatCompletions(ctx context.Context, body azopenai.ChatCompletionsOptions, options *azopenai.GetChatCompletionsOptions) (azopenai.GetChatCompletionsResponse, error) {
	return c.getChatCompletions(ctx, body, options)
}

func (c *mockAzureClient) GetChatCompletionsStream(ctx context.Context, body azopenai.ChatCompletionsOptions, options *azopenai.GetChatCompletionsStreamOptions) (azopenai.GetChatCompletionsStreamResponse, error) {
	return c.getChatCompletionsStream(ctx, body, options)
}

func getNewMockAzureAPIClient(mock *mockAzureClient) func(accessToken, endpoint string) (CompletionsClient, error) {
	return func(accessToken, endpoint string) (CompletionsClient, error) {
		return mock, nil
	}
}

func azure429ResponseError() *azcore.ResponseError {
	return &azcore.ResponseError{
		StatusCode: http.StatusTooManyRequests,
		ErrorCode:  "429",
		RawResponse: &http.Response{
			Request:    &http.Request{Method: "GET", URL: &url.URL{}, Header: http.Header(map[string][]string{})},
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewReader([]byte("too many requests"))),
		},
	}
}

func TestErrStatusNotOK(t *testing.T) {
	getAzureAPIClient := getNewMockAzureAPIClient(&mockAzureClient{
		getCompletionsStream: func(ctx context.Context, body azopenai.CompletionsOptions, options *azopenai.GetCompletionsStreamOptions) (azopenai.GetCompletionsStreamResponse, error) {
			return azopenai.GetCompletionsStreamResponse{}, azure429ResponseError()
		},
		getCompletions: func(ctx context.Context, body azopenai.CompletionsOptions, options *azopenai.GetCompletionsOptions) (azopenai.GetCompletionsResponse, error) {
			return azopenai.GetCompletionsResponse{}, azure429ResponseError()
		},
		getChatCompletions: func(ctx context.Context, body azopenai.ChatCompletionsOptions, options *azopenai.GetChatCompletionsOptions) (azopenai.GetChatCompletionsResponse, error) {
			return azopenai.GetChatCompletionsResponse{}, azure429ResponseError()
		},
		getChatCompletionsStream: func(ctx context.Context, body azopenai.ChatCompletionsOptions, options *azopenai.GetChatCompletionsStreamOptions) (azopenai.GetChatCompletionsStreamResponse, error) {
			return azopenai.GetChatCompletionsStreamResponse{}, azure429ResponseError()
		},
	})
	tokenManager := tokenusage.NewManager()
	mockClient, _ := NewClient(getAzureAPIClient, "", "", *tokenManager)
	t.Run("Complete", func(t *testing.T) {
		logger := log.Scoped("completions")
		resp, err := mockClient.Complete(context.Background(), types.CompletionsFeatureChat, types.CompletionsVersionLegacy, types.CompletionRequestParameters{}, logger)
		require.Error(t, err)
		assert.Nil(t, resp)

		autogold.Expect("AzureOpenAI: unexpected status code 429: too many requests").Equal(t, err.Error())
		_, ok := types.IsErrStatusNotOK(err)
		assert.True(t, ok)
	})

	t.Run("Stream", func(t *testing.T) {
		logger := log.Scoped("completions")
		err := mockClient.Stream(context.Background(), types.CompletionsFeatureChat, types.CompletionsVersionLegacy, types.CompletionRequestParameters{}, func(event types.CompletionResponse) error { return nil }, logger)
		require.Error(t, err)

		autogold.Expect("AzureOpenAI: unexpected status code 429: too many requests").Equal(t, err.Error())
		_, ok := types.IsErrStatusNotOK(err)
		assert.True(t, ok)
	})
}

func TestGenericErr(t *testing.T) {
	getAzureAPIClient := getNewMockAzureAPIClient(&mockAzureClient{
		getCompletionsStream: func(ctx context.Context, body azopenai.CompletionsOptions, options *azopenai.GetCompletionsStreamOptions) (azopenai.GetCompletionsStreamResponse, error) {
			return azopenai.GetCompletionsStreamResponse{}, errors.New("error")
		},
		getCompletions: func(ctx context.Context, body azopenai.CompletionsOptions, options *azopenai.GetCompletionsOptions) (azopenai.GetCompletionsResponse, error) {
			return azopenai.GetCompletionsResponse{}, errors.New("error")
		},
		getChatCompletions: func(ctx context.Context, body azopenai.ChatCompletionsOptions, options *azopenai.GetChatCompletionsOptions) (azopenai.GetChatCompletionsResponse, error) {
			return azopenai.GetChatCompletionsResponse{}, errors.New("error")
		},
		getChatCompletionsStream: func(ctx context.Context, body azopenai.ChatCompletionsOptions, options *azopenai.GetChatCompletionsStreamOptions) (azopenai.GetChatCompletionsStreamResponse, error) {
			return azopenai.GetChatCompletionsStreamResponse{}, errors.New("error")
		},
	})
	tokenManager := tokenusage.NewManager()
	mockClient, _ := NewClient(getAzureAPIClient, "", "", *tokenManager)
	t.Run("Complete", func(t *testing.T) {
		logger := log.Scoped("completions")
		resp, err := mockClient.Complete(context.Background(), types.CompletionsFeatureChat, types.CompletionsVersionLegacy, types.CompletionRequestParameters{}, logger)
		require.Error(t, err)
		assert.Nil(t, resp)

		autogold.Expect("error").Equal(t, err.Error())
		_, ok := types.IsErrStatusNotOK(err)
		assert.False(t, ok)
	})

	t.Run("Stream", func(t *testing.T) {
		logger := log.Scoped("completions")
		err := mockClient.Stream(context.Background(), types.CompletionsFeatureChat, types.CompletionsVersionLegacy, types.CompletionRequestParameters{}, func(event types.CompletionResponse) error { return nil }, logger)
		require.Error(t, err)

		autogold.Expect("error").Equal(t, err.Error())
		_, ok := types.IsErrStatusNotOK(err)
		assert.False(t, ok)
	})
}
