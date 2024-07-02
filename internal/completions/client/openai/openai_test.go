package openai

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/sourcegraph/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/completions/tokenusage"
	"github.com/sourcegraph/sourcegraph/internal/completions/types"
)

type mockDoer struct {
	do func(*http.Request) (*http.Response, error)
}

func (c *mockDoer) Do(r *http.Request) (*http.Response, error) {
	return c.do(r)
}

func TestErrStatusNotOK(t *testing.T) {
	tokenManager := tokenusage.NewManager()
	mockClient := NewClient(&mockDoer{
		func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(bytes.NewReader([]byte("oh no, please slow down!"))),
			}, nil
		},
	}, "", "", *tokenManager)

	compRequest := types.CompletionRequest{
		Feature:    types.CompletionsFeatureChat,
		Version:    types.CompletionsVersionLegacy,
		Parameters: types.CompletionRequestParameters{},
	}

	t.Run("Complete", func(t *testing.T) {
		logger := log.Scoped("completions")
		resp, err := mockClient.Complete(context.Background(), logger, compRequest)
		require.Error(t, err)
		assert.Nil(t, resp)

		autogold.Expect("OpenAI: unexpected status code 429: oh no, please slow down!").Equal(t, err.Error())
		_, ok := types.IsErrStatusNotOK(err)
		assert.True(t, ok)
	})

	t.Run("Stream", func(t *testing.T) {
		logger := log.Scoped("completions")
		responseMetadataCapture := types.NewResponseMetadataCapture(func(types.CompletionResponse) error { return nil })
		err := mockClient.Stream(context.Background(), logger, compRequest, &responseMetadataCapture)
		require.Error(t, err)

		autogold.Expect("OpenAI: unexpected status code 429: oh no, please slow down!").Equal(t, err.Error())
		_, ok := types.IsErrStatusNotOK(err)
		assert.True(t, ok)
	})
}
