package modelconfig

import (
	"fmt"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/embedded"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"

	modelconfigSDK "github.com/sourcegraph/sourcegraph/internal/modelconfig"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// verifyProviderByID will lookup the Provider from the ModelConfig and call the supplied function.
// If the provider isn't found, will report an error.
func verifyProviderByID(t *testing.T, cfg *types.ModelConfiguration, id string, fn func(t *testing.T, p types.Provider)) {
	t.Helper()
	t.Run(fmt.Sprintf("Provider=%s", id), func(t *testing.T) {
		for _, provider := range cfg.Providers {
			if provider.ID == types.ProviderID(id) {
				fn(t, provider)
				return
			}
		}

		t.Errorf("no provider with ID %q found", id)
	})
}

// verifyModelByMRef will lookup the Model from the ModelConfig and call the supplied function.
// If the model isn't found, will report an error.
func verifyModelByMRef(t *testing.T, cfg *types.ModelConfiguration, id string, fn func(t *testing.T, p types.Model)) {
	t.Helper()
	t.Run(fmt.Sprintf("Model=%s", id), func(t *testing.T) {
		var ranTest bool
		for _, model := range cfg.Models {
			if model.ModelRef == types.ModelRef(id) {
				if ranTest {
					t.Errorf("two models were found with the same ID %q", id)
				}
				fn(t, model)
				return
			}
		}

		t.Errorf("no model with ID %q found", id)
	})
}

func TestModelConfigBuilder(t *testing.T) {
	// Mock out the licensing check confirming Cody is enabled.
	initialMockCheck := licensing.MockCheckFeature
	licensing.MockCheckFeature = func(licensing.Feature) error {
		return nil // Don't fail when checking if Cody is enabled.
	}
	t.Cleanup(func() { licensing.MockCheckFeature = initialMockCheck })
	// Restore our mocked out site config.
	t.Cleanup(func() { conf.Mock(nil) })

	// setSiteCompletionsConfig sets site configuration data
	// to enable Cody Enterprise, and have the supplied completions
	// data.
	setSiteCompletionsConfig := func(userSuppliedCompConfig schema.Completions) *conftypes.CompletionsConfig {
		fauxSiteConfig := schema.SiteConfiguration{
			CodyEnabled:                  pointers.Ptr(true),
			CodyPermissions:              pointers.Ptr(false),
			CodyRestrictUsersFeatureFlag: pointers.Ptr(false),
			LicenseKey:                   "license-key",

			Completions: &userSuppliedCompConfig,
		}
		return conf.GetCompletionsConfig(fauxSiteConfig)
	}

	type testData struct {
		codyGatewayConfig     *types.ModelConfiguration
		SiteCompletionsConfig schema.Completions
	}

	// verifyCanBuildModelConfig builds the types.ModelConfiguration based on the supplied data.
	// Will abort the test if any invalid configuration data is produced.
	verifyCanBuildModelConfig := func(t *testing.T, testData testData) *types.ModelConfiguration {
		baseConfig, err := embedded.GetCodyGatewayModelConfig()
		require.NoError(t, err)
		require.NoError(t, modelconfigSDK.ValidateModelConfig(baseConfig), "baseConfig is invalid!")

		// First round-trip the completions config through the `conf` package,
		// so that various hard-coded defualts from `conf/computed.go` get applied.
		// (Otherwise we'd test combinations that would never be possible in prod.)
		completionsConfig := setSiteCompletionsConfig(testData.SiteCompletionsConfig)

		// TODO: We are assuming that the SiteModelConfiguration ONLY comes from the
		// "completions" section of the site config. In the future, we'll add a new
		// format in the site configuration so users can express the SiteModelConfiguration
		// data structure directly. (Since it is more flexible than the completions config
		// that is available today.)
		siteConfigData, err := convertCompletionsConfig(completionsConfig)
		require.NoError(t, err)

		b := builder{
			staticData:      baseConfig,
			codyGatewayData: testData.codyGatewayConfig,
			siteConfigData:  siteConfigData,
		}
		result, err := b.build()
		require.NoError(t, err)
		require.NoError(t, modelconfigSDK.ValidateModelConfig(result), "produced ModelConfiguration was invalid")

		return result
	}

	// Provide the bare minimum of site configuration data. This will enable Cody Pro
	// and return the conftypes.CompletionsConfig as a types.ModelConfiguration.
	t.Run("NoOverrides", func(t *testing.T) {
		cfg := verifyCanBuildModelConfig(t, testData{})
		assert.Equal(t, 2, len(cfg.Providers), "providers: %v", cfg.Providers)
		assert.Equal(t, 3, len(cfg.Models), "models: %v", cfg.Models)

		// Verify Providers are configured as expected.
		verifyProviderByID(t, cfg, "anthropic", func(t *testing.T, prov types.Provider) {
			require.NotNil(t, prov.ServerSideConfig)
			// Use the "sourcegraph" LLM API provider under the hood.
			require.NotNil(t, prov.ServerSideConfig.SourcegraphProvider)
			require.Nil(t, prov.ServerSideConfig.GenericProvider)
		})
		verifyProviderByID(t, cfg, "fireworks", func(t *testing.T, prov types.Provider) {
			require.NotNil(t, prov.ServerSideConfig)
			// Use the "sourcegraph" LLM API provider under the hood.
			require.NotNil(t, prov.ServerSideConfig.SourcegraphProvider)
			require.Nil(t, prov.ServerSideConfig.GenericProvider)
		})

		// Verify DefaultModels.
		assert.Equal(t, types.DefaultModels{
			Chat:           types.ModelRef("anthropic::unknown::claude-3-sonnet-20240229"),
			CodeCompletion: types.ModelRef("fireworks::unknown::starcoder"),
			FastChat:       types.ModelRef("anthropic::unknown::claude-3-haiku-20240307"),
		}, cfg.DefaultModels)

		// Verify Models are configured as expected.
		verifyModelByMRef(t, cfg, "anthropic::unknown::claude-3-haiku-20240307", func(t *testing.T, model types.Model) {
			// The MaxInputTokens can be specified via the site config.
			assert.Equal(t, 12_000, model.ContextWindow.MaxInputTokens)
			assert.Equal(t, 4_000, model.ContextWindow.MaxOutputTokens)
		})

		verifyModelByMRef(t, cfg, "anthropic::unknown::claude-3-sonnet-20240229", func(t *testing.T, model types.Model) {
			// The MaxInputTokens can be specified via the site config.
			assert.Equal(t, 12_000, model.ContextWindow.MaxInputTokens)
			assert.Equal(t, 4_000, model.ContextWindow.MaxOutputTokens)
		})

		verifyModelByMRef(t, cfg, "fireworks::unknown::starcoder", func(t *testing.T, model types.Model) {
			// The MaxInputTokens can be specified via the site config.
			assert.Equal(t, 9_000, model.ContextWindow.MaxInputTokens)
			assert.Equal(t, 4_000, model.ContextWindow.MaxOutputTokens)
		})
	})

	// t.Run("LegacyCompletionsConfig", func(t *testing.T) {
	// 	// Load the LLM model configuration expressed via the "completions" config.
	// 	siteConfig := convertLegacyCompletionsConfig(&schema.Completions{
	// 		Provider:        "aws-bedrock",
	// 		ChatModel:       "anthropic.claude-3-opus-20240229-v1:0",
	// 		CompletionModel: "anthropic.claude-instant-v1",
	// 		// FastChatMode is not set.

	// 		AccessToken: "secret",
	// 		Endpoint:    "https://example.com",
	// 	})

	// 	// BUG: Need to confirm the awkward syntax for AWS Bedrock, see:
	// 	// https://sourcegraph.com/docs/cody/clients/enable-cody-enterprise#use-amazon-bedrock-aws
	// 	// "completionModel": "anthropic.claude-instant-v1"

	// 	baseConfig, err := embedded.GetCodyGatewayModelConfig()
	// 	require.NoError(t, err)

	// 	outConfig, err := applySiteConfig(baseConfig, siteConfig)
	// 	require.NoError(t, err)

	// 	require.Equal(t, 1, len(outConfig.Providers))
	// 	provider := outConfig.Providers[0]
	// 	assert.EqualValues(t, "aws-bedrock", string(provider.ID))
	// 	assert.NotNil(t, provider.ServerSideConfig)

	// 	serverSideConfig := provider.ServerSideConfig
	// 	require.NotNil(t, serverSideConfig)
	// 	require.NotNil(t, serverSideConfig.GenericProvider)
	// 	assert.Equal(t, "secret", serverSideConfig.GenericProvider.AccessToken)
	// 	assert.Equal(t, "https://example.com", serverSideConfig.GenericProvider.Endpoint)

	// 	// Default models.
	// 	defModels := outConfig.DefaultModels
	// 	assert.EqualValues(t, "anthropic.claude-3-opus-20240229-v1:0", defModels.Chat)
	// 	assert.EqualValues(t, "", defModels.FastChat)
	// 	assert.EqualValues(t, "anthropic.claude-instant-v1", defModels.CodeCompletion)
	// })
}
