package openai

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
)

type mockDoer struct {
	do func(*http.Request) (*http.Response, error)
}

func (c *mockDoer) Do(r *http.Request) (*http.Response, error) {
	return c.do(r)
}

func TestErrStatusNotOK(t *testing.T) {
	mockClient := NewClient(&mockDoer{
		func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(bytes.NewReader([]byte("oh no, please slow down!"))),
			}, nil
		},
	}, "", "")

	t.Run("Complete", func(t *testing.T) {
		resp, err := mockClient.Complete(context.Background(), types.CompletionsFeatureChat, types.CompletionsVersionLegacy, types.CompletionRequestParameters{})
		require.Error(t, err)
		assert.Nil(t, resp)

		autogold.Expect("OpenAI: unexpected status code 429: oh no, please slow down!").Equal(t, err.Error())
		_, ok := types.IsErrStatusNotOK(err)
		assert.True(t, ok)
	})

	t.Run("Stream", func(t *testing.T) {
		err := mockClient.Stream(context.Background(), types.CompletionsFeatureChat, types.CompletionsVersionLegacy, types.CompletionRequestParameters{}, func(event types.CompletionResponse) error { return nil })
		require.Error(t, err)

		autogold.Expect("OpenAI: unexpected status code 429: oh no, please slow down!").Equal(t, err.Error())
		_, ok := types.IsErrStatusNotOK(err)
		assert.True(t, ok)
	})
}
