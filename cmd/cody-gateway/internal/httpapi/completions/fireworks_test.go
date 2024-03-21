package completions

import (
	"strings"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/shared/config"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/fireworks"
)

func TestFireworksRequestGetTokenCount(t *testing.T) {
	logger := logtest.Scoped(t)

	t.Run("streaming", func(t *testing.T) {
		req := fireworksRequest{Stream: true}
		r := strings.NewReader(fireworksStreamingResponse)
		handler := &FireworksHandlerMethods{}
		promptUsage, completionUsage := handler.parseResponseAndUsage(logger, req, r)

		assert.Equal(t, 79, promptUsage.tokens)
		assert.Equal(t, 30, completionUsage.tokens)
	})

	t.Run("non-streaming", func(t *testing.T) {
		req := fireworksRequest{Stream: false}
		r := strings.NewReader(fireworksNonStreamingResponse)
		handler := &FireworksHandlerMethods{}
		promptUsage, completionUsage := handler.parseResponseAndUsage(logger, req, r)

		assert.Equal(t, 79, promptUsage.tokens)
		assert.Equal(t, 30, completionUsage.tokens)
	})
}

var fireworksStreamingResponse = `
data: {"id":"cmpl-448a6127ca074189b4e011ec","object":"chat.completion.chunk","created":1704368645,"model":"accounts/fireworks/models/mixtral-8x7b-instruct","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}],"usage":null}

data: {"id":"cmpl-448a6127ca074189b4e011ec","object":"chat.completion.chunk","created":1704368645,"model":"accounts/fireworks/models/mixtral-8x7b-instruct","choices":[{"index":0,"delta":{"content":"I am a helpful AI assistant"},"finish_reason":null}],"usage":null}

data: {"id":"cmpl-448a6127ca074189b4e011ec","object":"chat.completion.chunk","created":1704368645,"model":"accounts/fireworks/models/mixtral-8x7b-instruct","choices":[{"index":0,"delta":{"content":" and I don't"},"finish_reason":null}],"usage":null}

data: {"id":"cmpl-448a6127ca074189b4e011ec","object":"chat.completion.chunk","created":1704368645,"model":"accounts/fireworks/models/mixtral-8x7b-instruct","choices":[{"index":0,"delta":{"content":" have a personal name."},"finish_reason":null}],"usage":null}

data: {"id":"cmpl-448a6127ca074189b4e011ec","object":"chat.completion.chunk","created":1704368645,"model":"accounts/fireworks/models/mixtral-8x7b-instruct","choices":[{"index":0,"delta":{"content":" That's why I"},"finish_reason":null}],"usage":null}

data: {"id":"cmpl-448a6127ca074189b4e011ec","object":"chat.completion.chunk","created":1704368645,"model":"accounts/fireworks/models/mixtral-8x7b-instruct","choices":[{"index":0,"delta":{"content":" introduced myself as Cody"},"finish_reason":null}],"usage":null}

data: {"id":"cmpl-448a6127ca074189b4e011ec","object":"chat.completion.chunk","created":1704368645,"model":"accounts/fireworks/models/mixtral-8x7b-instruct","choices":[{"index":0,"delta":{"content":", to make it"},"finish_reason":"length"}],"usage":{"prompt_tokens":79,"total_tokens":109,"completion_tokens":30}}

data: [DONE]
`

var fireworksNonStreamingResponse = `{"id":"cmpl-a890423291fa6d7de7b8d8af","object":"chat.completion","created":1704368780,"model":"accounts/fireworks/models/mixtral-8x7b-instruct","choices":[{"index":0,"message":{"role":"assistant","content":"I don't have a \"real\" name, as I am an artificial intelligence and don't have a physical body or personal identity. I"},"finish_reason":"length"}],"usage":{"prompt_tokens":79,"total_tokens":109,"completion_tokens":30}}`

func TestFireworksStarCoderModelPicking(t *testing.T) {
	t.Run("returns single-tenant instance when ST rollout is set to 100%", func(t *testing.T) {
		assert.Equal(t, fireworks.Starcoder16bSingleTenant, pickStarCoderModel("starcoder", config.FireworksConfig{StarcoderEnterpriseSingleTenantPercent: 100}))
		assert.Equal(t, fireworks.Starcoder16bSingleTenant, pickStarCoderModel("starcoder-16b", config.FireworksConfig{StarcoderCommunitySingleTenantPercent: 100}))
		assert.Equal(t, fireworks.Starcoder16bSingleTenant, pickStarCoderModel("starcoder-7b", config.FireworksConfig{StarcoderCommunitySingleTenantPercent: 100}))
	})

	t.Run("returns unquantized multi-tenant instances when ST rollout is set to 0%", func(t *testing.T) {
		assert.Equal(t, fireworks.Starcoder16b, pickStarCoderModel("starcoder", config.FireworksConfig{StarcoderEnterpriseSingleTenantPercent: 0}))
		assert.Equal(t, fireworks.Starcoder16b, pickStarCoderModel("starcoder-16b", config.FireworksConfig{StarcoderCommunitySingleTenantPercent: 0}))
		assert.Equal(t, fireworks.Starcoder7b, pickStarCoderModel("starcoder-7b", config.FireworksConfig{StarcoderCommunitySingleTenantPercent: 0}))
	})

	t.Run("returns starcoder2 instances when starcoder2 virtual model string is used", func(t *testing.T) {
		assert.Equal(t, fireworks.StarcoderTwo7b, pickStarCoderModel("starcoder2-7b", config.FireworksConfig{StarcoderCommunitySingleTenantPercent: 0}))
		assert.Equal(t, fireworks.StarcoderTwo15b, pickStarCoderModel("starcoder2-15b", config.FireworksConfig{StarcoderCommunitySingleTenantPercent: 0}))
	})
}
