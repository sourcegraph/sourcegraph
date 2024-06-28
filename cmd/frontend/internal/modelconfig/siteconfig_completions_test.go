package modelconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestConvertCompletionsConfig(t *testing.T) {
	// Mock out the licensing check confirming Cody is enabled.
	initialMockCheck := licensing.MockCheckFeature
	licensing.MockCheckFeature = func(licensing.Feature) error {
		return nil // Don't fail when checking if Cody is enabled.
	}
	t.Cleanup(func() { licensing.MockCheckFeature = initialMockCheck })
	// Restore our mocked out site config.
	t.Cleanup(func() { conf.Mock(nil) })

	// loadCompletionsConfig sets the supplied completions configuration data
	// into the site config, and then loads it. This extra step ensures that
	// the default values are set as applicable in the returned object.
	// as well as the necessary checks for enabling Cody Pro.
	loadCompletionsConfig := func(userSuppliedCompConfig schema.Completions) *conftypes.CompletionsConfig {
		fauxSiteConfig := schema.SiteConfiguration{
			CodyEnabled:                  pointers.Ptr(true),
			CodyPermissions:              pointers.Ptr(false),
			CodyRestrictUsersFeatureFlag: pointers.Ptr(false),
			LicenseKey:                   "license-key",

			Completions: &userSuppliedCompConfig,
		}
		return conf.GetCompletionsConfig(fauxSiteConfig)
	}
	t.Run("Default", func(t *testing.T) {
		compConfig := loadCompletionsConfig(schema.Completions{
			Provider: "sourcegraph",
		})
		require.NotNil(t, compConfig)

		siteModelConfig, err := convertCompletionsConfig(compConfig)
		require.NoError(t, err)

		assert.Nil(t, siteModelConfig.SourcegraphModelConfig)
		require.NotNil(t, siteModelConfig.ProviderOverrides)
		require.NotNil(t, siteModelConfig.ModelOverrides)

		// ProviderOverrides. Because the default models are from different providers, we stub out
		// three different ProviderOverrides. However, all of these are configured to use the
		// "Sourcegraph API Provider".
		require.Equal(t, 2, len(siteModelConfig.ProviderOverrides))
		assert.EqualValues(t, "anthropic", siteModelConfig.ProviderOverrides[0].ID)
		assert.EqualValues(t, "fireworks", siteModelConfig.ProviderOverrides[1].ID)

		for _, providerOverride := range siteModelConfig.ProviderOverrides {
			// Stock model configuration.
			defModelCfg := providerOverride.DefaultModelConfig
			require.NotNil(t, defModelCfg)
			assert.Equal(t, types.ModelTierEnterprise, defModelCfg.Tier)

			require.Nil(t, providerOverride.ClientSideConfig)

			ssConfig := providerOverride.ServerSideConfig
			require.NotNil(t, ssConfig)
			require.NotNil(t, ssConfig.SourcegraphProvider)

			sgAPIProviderConfig := ssConfig.SourcegraphProvider
			require.NotNil(t, sgAPIProviderConfig)
			assert.Equal(t, "https://cody-gateway.sourcegraph.com", sgAPIProviderConfig.Endpoint)
			assert.NotEmpty(t, sgAPIProviderConfig.AccessToken) // Based on the license key.
		}

		// ModelOverrides
		require.Equal(t, 3, len(siteModelConfig.ModelOverrides))
		assert.EqualValues(t, "anthropic::unknown::claude-3-haiku-20240307", siteModelConfig.ModelOverrides[0].ModelRef)
		assert.EqualValues(t, "anthropic::unknown::claude-3-sonnet-20240229", siteModelConfig.ModelOverrides[1].ModelRef)
		assert.EqualValues(t, "fireworks::unknown::starcoder", siteModelConfig.ModelOverrides[2].ModelRef)

		// DefaultModels
		require.NotNil(t, siteModelConfig.DefaultModels)
		assert.EqualValues(t, "anthropic::unknown::claude-3-haiku-20240307", siteModelConfig.DefaultModels.FastChat)
		assert.EqualValues(t, "anthropic::unknown::claude-3-sonnet-20240229", siteModelConfig.DefaultModels.Chat)
		assert.EqualValues(t, "fireworks::unknown::starcoder", siteModelConfig.DefaultModels.CodeCompletion)
	})

	t.Run("OpenAI", func(t *testing.T) {
		compConfig := loadCompletionsConfig(schema.Completions{
			Provider:        "openai",
			ChatModel:       "gpt-4",
			FastChatModel:   "gpt-3.5-turbo",
			CompletionModel: "gpt-3.5-turbo-instruct",
			AccessToken:     "byok-key",
		})
		require.NotNil(t, compConfig)

		siteModelConfig, err := convertCompletionsConfig(compConfig)
		require.NoError(t, err)

		assert.Nil(t, siteModelConfig.SourcegraphModelConfig)
		require.NotNil(t, siteModelConfig.ProviderOverrides)
		require.NotNil(t, siteModelConfig.ModelOverrides)

		// ProviderOverrides. Default to using "sourcegraph" and Cody Gateway.
		require.Equal(t, 1, len(siteModelConfig.ProviderOverrides))
		providerOverride := siteModelConfig.ProviderOverrides[0]
		assert.EqualValues(t, "openai", providerOverride.ID)
		require.NotNil(t, providerOverride.ServerSideConfig)

		genericProviderConfig := providerOverride.ServerSideConfig.GenericProvider
		require.NotNil(t, genericProviderConfig)
		assert.Equal(t, "https://api.openai.com", genericProviderConfig.Endpoint)
		assert.NotEmpty(t, "byok-key", genericProviderConfig.AccessToken)

		// ModelOverrides
		require.Equal(t, 3, len(siteModelConfig.ModelOverrides))

		// DefaultModels
		require.NotNil(t, siteModelConfig.DefaultModels)
		assert.EqualValues(t, "openai::unknown::gpt-4", siteModelConfig.DefaultModels.Chat)
		assert.EqualValues(t, "openai::unknown::gpt-3.5-turbo", siteModelConfig.DefaultModels.FastChat)
		assert.EqualValues(t, "openai::unknown::gpt-3.5-turbo-instruct", siteModelConfig.DefaultModels.CodeCompletion)
	})

	t.Run("AWSBedrock", func(t *testing.T) {
		t.Run("OnDemandThoughput", func(t *testing.T) {
			compConfig := loadCompletionsConfig(schema.Completions{
				Provider:        "aws-bedrock",
				ChatModel:       "anthropic.claude-3-opus-20240229-v1:0",
				CompletionModel: "anthropic.claude-instant-v1",
				// FastChatModel not set, left to default.
				AccessToken: "", // Leave blank to pick up ambient AWS creds.
				Endpoint:    "us-west-2",
			})
			require.NotNil(t, compConfig)

			siteModelConfig, err := convertCompletionsConfig(compConfig)
			require.NoError(t, err)

			assert.Nil(t, siteModelConfig.SourcegraphModelConfig)
			require.NotNil(t, siteModelConfig.ProviderOverrides)
			require.NotNil(t, siteModelConfig.ModelOverrides)

			// The ID of the ProviderOverride is "anthropic", to match the models referenced.
			// However, the API Provider, i.e. the ProviderOverride's server-side configuration
			// will define how to _use_ this provider, which will be via AWS Bedrock.
			require.Equal(t, 1, len(siteModelConfig.ProviderOverrides))
			providerOverride := siteModelConfig.ProviderOverrides[0]
			assert.EqualValues(t, "anthropic", providerOverride.ID)
			require.NotNil(t, providerOverride.ServerSideConfig)

			awsBedrockConfig := providerOverride.ServerSideConfig.AWSBedrock
			require.NotNil(t, awsBedrockConfig)
			assert.Equal(t, "us-west-2", awsBedrockConfig.Endpoint)
			assert.Equal(t, "", awsBedrockConfig.AccessToken)

			// ModelOverrides
			require.Equal(t, 2, len(siteModelConfig.ModelOverrides))
			{
				m := siteModelConfig.ModelOverrides[1]
				assert.EqualValues(t, "anthropic::unknown::anthropic.claude-instant-v1", m.ModelRef)
				require.Nil(t, m.ServerSideConfig)
			}
			{
				m := siteModelConfig.ModelOverrides[0]
				assert.EqualValues(t, "anthropic::unknown::anthropic.claude-3-opus-20240229-v1_0", m.ModelRef)
				require.Nil(t, m.ServerSideConfig)
			}

			// DefaultModels
			require.NotNil(t, siteModelConfig.DefaultModels)
			assert.EqualValues(t, "anthropic::unknown::anthropic.claude-3-opus-20240229-v1_0", siteModelConfig.DefaultModels.Chat)
			assert.EqualValues(t, "anthropic::unknown::anthropic.claude-instant-v1", siteModelConfig.DefaultModels.FastChat)
			assert.EqualValues(t, "anthropic::unknown::anthropic.claude-instant-v1", siteModelConfig.DefaultModels.CodeCompletion)
		})

		t.Run("ProvisionedThroughput", func(t *testing.T) {
			compConfig := loadCompletionsConfig(schema.Completions{
				Provider:        "aws-bedrock",
				ChatModel:       "anthropic.claude-3-haiku-20240307-v1:0-100k/arn:aws:bedrock:us-west-2:012345678901:provisioned-model/abcdefghijkl",
				CompletionModel: "anthropic.claude-instant-v1",
				// FastChatModel not set, left to default.
				AccessToken: "access-key-id:secret-access-key:session-token",
				Endpoint:    "https://vpce-0000-00000.bedrock-runtime.us-west-2.vpce.amazonaws.com",
			})
			require.NotNil(t, compConfig)

			siteModelConfig, err := convertCompletionsConfig(compConfig)
			require.NoError(t, err)

			assert.Nil(t, siteModelConfig.SourcegraphModelConfig)
			require.NotNil(t, siteModelConfig.ProviderOverrides)
			require.NotNil(t, siteModelConfig.ModelOverrides)

			// ProviderOverrides.
			require.Equal(t, 1, len(siteModelConfig.ProviderOverrides))
			providerOverride := siteModelConfig.ProviderOverrides[0]
			assert.EqualValues(t, "anthropic", providerOverride.ID)
			require.NotNil(t, providerOverride.ServerSideConfig)

			awsBedrockConfig := providerOverride.ServerSideConfig.AWSBedrock
			require.NotNil(t, awsBedrockConfig)
			assert.Equal(t, "access-key-id:secret-access-key:session-token", awsBedrockConfig.AccessToken)
			assert.Equal(t, "https://vpce-0000-00000.bedrock-runtime.us-west-2.vpce.amazonaws.com", awsBedrockConfig.Endpoint)

			// ModelOverrides
			require.Equal(t, 2, len(siteModelConfig.ModelOverrides))

			chatModel := siteModelConfig.ModelOverrides[0]
			assert.EqualValues(t, "anthropic::unknown::anthropic.claude-3-haiku-20240307-v1_0-100k", chatModel.ModelRef)
			require.NotNil(t, chatModel.ServerSideConfig)
			assert.Equal(t, "arn:aws:bedrock:us-west-2:012345678901:provisioned-model/abcdefghijkl", chatModel.ServerSideConfig.AWSBedrockProvisionedThroughput.ARN)

			completionModel := siteModelConfig.ModelOverrides[1]
			assert.EqualValues(t, "anthropic::unknown::anthropic.claude-instant-v1", completionModel.ModelRef)
			assert.Nil(t, completionModel.ServerSideConfig)

			// DefaultModels. Note the that model was modified, such as stripping out the ARNM.
			require.NotNil(t, siteModelConfig.DefaultModels)
			assert.EqualValues(t, "anthropic::unknown::anthropic.claude-3-haiku-20240307-v1_0-100k", siteModelConfig.DefaultModels.Chat)
			assert.EqualValues(t, "anthropic::unknown::anthropic.claude-instant-v1", siteModelConfig.DefaultModels.FastChat)
			assert.EqualValues(t, "anthropic::unknown::anthropic.claude-instant-v1", siteModelConfig.DefaultModels.CodeCompletion)
		})
	})
}
