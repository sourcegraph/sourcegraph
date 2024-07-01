package modelconfig

import (
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/modelconfig/embedded"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/schema"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterListMatches(t *testing.T) {
	const geminiMRef = "google::v1::gemini-1.5-pro-latest"

	tests := []struct {
		MRef    string
		Pattern string
		Want    bool
	}{
		{
			MRef:    geminiMRef,
			Pattern: "google*",
			Want:    true,
		},
		{
			MRef:    geminiMRef,
			Pattern: "google::v1::*",
			Want:    true,
		},
		{
			MRef:    geminiMRef,
			Pattern: "*",
			Want:    true,
		},
		{
			MRef:    geminiMRef,
			Pattern: "*v1*",
			Want:    true,
		},
		{
			MRef:    geminiMRef,
			Pattern: "*::gemini-1.5-pro-latest",
			Want:    true,
		},
		{
			MRef:    geminiMRef,
			Pattern: "*::gpt-4o",
			Want:    false,
		},

		// Negative tests.
		{
			MRef:    geminiMRef,
			Pattern: "*::gpt-4o",
			Want:    false,
		},
		{
			MRef:    geminiMRef,
			Pattern: "*::v2::*",
			Want:    false,
		},
		{
			MRef:    geminiMRef,
			Pattern: "google::v1::gemini-1.5-pro", // Doesn't end with "-latest"
			Want:    false,
		},
		{
			MRef:    geminiMRef,
			Pattern: "anthropic*",
			Want:    false,
		},
	}

	for _, test := range tests {
		got := filterListMatches(types.ModelRef(test.MRef), []string{test.Pattern})
		assert.Equal(t, test.Want, got, "mref: %q\npattern: %q", test.MRef, test.Pattern)
	}
}

func TestModelConfigBuilder(t *testing.T) {
	t.Run("NoOverrides", func(t *testing.T) {
		baseConfig, err := embedded.GetCodyGatewayModelConfig()
		require.NoError(t, err)

		siteConfig := types.SiteModelConfiguration{
			// Keep non-nil, but leave everything else zeroed out.
			SourcegraphModelConfig: &types.SourcegraphModelConfig{},
			ProviderOverrides:      nil,
			ModelOverrides:         nil,
		}

		result, err := applySiteConfig(baseConfig, &siteConfig)
		require.NoError(t, err)

		// We need to refetch the base config, since baseConfig will be
		// modified as a side-effect of calling applySiteConfig.
		refetchedBaseConfig, err := embedded.GetCodyGatewayModelConfig()
		require.NoError(t, err)

		require.EqualValues(t, *refetchedBaseConfig, *result)
	})

	t.Run("LegacyCompletionsConfig", func(t *testing.T) {
		// Load the LLM model configuration expressed via the "completions" config.
		siteConfig := convertLegacyCompletionsConfig(&schema.Completions{
			Provider:        "aws-bedrock",
			ChatModel:       "anthropic.claude-3-opus-20240229-v1:0",
			CompletionModel: "anthropic.claude-instant-v1",
			// FastChatMode is not set.

			AccessToken: "secret",
			Endpoint:    "https://example.com",
		})

		// BUG: Need to confirm the awkward syntax for AWS Bedrock, see:
		// https://sourcegraph.com/docs/cody/clients/enable-cody-enterprise#use-amazon-bedrock-aws
		// "completionModel": "anthropic.claude-instant-v1"

		baseConfig, err := embedded.GetCodyGatewayModelConfig()
		require.NoError(t, err)

		outConfig, err := applySiteConfig(baseConfig, siteConfig)
		require.NoError(t, err)

		require.Equal(t, 1, len(outConfig.Providers))
		provider := outConfig.Providers[0]
		assert.EqualValues(t, "aws-bedrock", string(provider.ID))
		assert.NotNil(t, provider.ServerSideConfig)

		serverSideConfig := provider.ServerSideConfig
		require.NotNil(t, serverSideConfig)
		require.NotNil(t, serverSideConfig.GenericProvider)
		assert.Equal(t, "secret", serverSideConfig.GenericProvider.AccessToken)
		assert.Equal(t, "https://example.com", serverSideConfig.GenericProvider.Endpoint)

		// Default models.
		defModels := outConfig.DefaultModels
		assert.EqualValues(t, "anthropic.claude-3-opus-20240229-v1:0", defModels.Chat)
		assert.EqualValues(t, "", defModels.FastChat)
		assert.EqualValues(t, "anthropic.claude-instant-v1", defModels.CodeCompletion)
	})
}
