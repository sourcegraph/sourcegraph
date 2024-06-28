package modelconfig

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// getProviderConfiguration returns the API Provider configuration based on the supplied site configuration.
func getProviderConfiguration(siteConfig *conftypes.CompletionsConfig) *types.ServerSideProviderConfig {
	var serverSideConfig types.ServerSideProviderConfig
	switch siteConfig.Provider {
	case conftypes.CompletionsProviderNameAWSBedrock:
		serverSideConfig.AWSBedrock = &types.AWSBedrockProviderConfig{
			AccessToken: siteConfig.AccessToken,
			Endpoint:    siteConfig.Endpoint,
		}
	case conftypes.CompletionsProviderNameAzureOpenAI:
		serverSideConfig.AzureOpenAI = &types.AzureOpenAIProviderConfig{
			AccessToken: siteConfig.AccessToken,
			Endpoint:    siteConfig.Endpoint,
		}
	case conftypes.CompletionsProviderNameSourcegraph:
		serverSideConfig.SourcegraphProvider = &types.SourcegraphProviderConfig{
			AccessToken: siteConfig.AccessToken,
			Endpoint:    siteConfig.Endpoint,
		}

		// For all the other types of providers you can define in the site configuration, we
		// just use a generic config. Rather than creating one for Anthropic, Fireworks, Google, etc.
		// We'll add those when needed, when we expose the newer style configuration in the site-config.
	default:
		serverSideConfig.GenericProvider = &types.GenericProviderConfig{
			AccessToken: siteConfig.AccessToken,
			Endpoint:    siteConfig.Endpoint,
		}
	}

	return &serverSideConfig
}

// convertCompletionsConfig converts the supplied Completions configuration blob (the Cody Enterprise configuration data)
// into the newer types.SiteModelConfiguration structure.
//
// Assumes that the supplied completions object is valid, and contains all the required settings. e.g. the site admin
// can leave some things blank, but `conf/computed.go`'s `GetCompletionsConfig()` will fill the Endpoint and related
// fields with meaingful defaults.
func convertCompletionsConfig(completionsCfg *conftypes.CompletionsConfig) (*types.SiteModelConfiguration, error) {
	if completionsCfg == nil {
		return nil, nil
	}

	// Generic defaults.
	defaultModelConfig := types.DefaultModelConfig{
		Capabilities: []types.ModelCapability{
			types.ModelCapabilityAutocomplete,
			types.ModelCapabilityChat,
		},
		Category: types.ModelCategoryBalanced,
		Status:   types.ModelStatusStable,
		Tier:     types.ModelTierEnterprise,

		// IMPORTANT: The default model config contains an invalid
		// context window (0, 0). The ModelOverrides MUST set the
		// expected values.
	}

	// We build the SiteModelConfiguration "backwards". We look at the default models (chat, autocomplete,
	// fast chat) and then figure out the Model Providers and Model Overrides that are needed.
	//
	// The actual configuration data is just used in the ProviderOverride.ServerSideConfig settings.
	requiredProviders := map[string]*types.ProviderOverride{}
	requiredModels := map[types.ModelRef]*types.ModelOverride{}

	incorporateModel := func(modelIDFromConfig string) (types.ModelRef, error) {
		// Figure out the provider and model ID from the older-style format in the config.
		var (
			providerID string
			modelID    string

			modelServerSideConfig *types.ServerSideModelConfig
		)
		parts := strings.SplitN(modelIDFromConfig, "/", 2)
		switch len(parts) {
		case 1:
			providerID = string(completionsCfg.Provider)
			modelID = parts[0]
		case 2:
			providerID = parts[0]
			modelID = parts[1]
		default:
			return types.ModelRef(""), errors.Errorf("invalid model ID in config %q", modelIDFromConfig)
		}
		// Edge case, we support the user encoding an ARN in the model in the config.
		if completionsCfg.Provider == conftypes.CompletionsProviderNameAWSBedrock {
			bedrockModelRef := conftypes.NewBedrockModelRefFromModelID(modelIDFromConfig)
			providerID = "anthropic"
			// The model ID may contain colons, which we reject as part of the ModelID validation,
			// so we strip those out here.
			modelID = strings.ReplaceAll(bedrockModelRef.Model, ":", "_")

			if bedrockModelRef.ProvisionedCapacity != nil {
				modelServerSideConfig = &types.ServerSideModelConfig{
					AWSBedrockProvisionedThroughput: &types.AWSBedrockProvisionedThroughput{
						ARN: *bedrockModelRef.ProvisionedCapacity,
					},
				}
			}
		}

		// Create ProviderOverride if we haven't seen this provider before.
		// We need to remap the provider ID if it is referring to an API Provider and not
		// a Model Provider.
		effectiveProviderID := providerID
		if providerID == "aws-bedrock" {
			effectiveProviderID = "anthropic"
		}
		if providerID == "azure-ai" {
			effectiveProviderID = "openai"
		}

		if _, found := requiredProviders[providerID]; !found {
			providerOverride := types.ProviderOverride{
				ID:                 types.ProviderID(effectiveProviderID),
				DefaultModelConfig: &defaultModelConfig,
				ClientSideConfig:   nil,
				ServerSideConfig:   getProviderConfiguration(completionsCfg),
			}
			requiredProviders[effectiveProviderID] = &providerOverride
		}

		// Create the ModelOverride if we haven't seen this model before.
		rawModelRef := fmt.Sprintf("%s::unknown::%s", effectiveProviderID, modelID)
		modelRef := types.ModelRef(rawModelRef)
		if _, found := requiredModels[modelRef]; !found {
			modelOverride := types.ModelOverride{
				ModelRef:    types.ModelRef(modelRef),
				DisplayName: modelID,
				ModelName:   modelID,

				ServerSideConfig: modelServerSideConfig,
			}
			requiredModels[modelRef] = &modelOverride
		}

		return modelRef, nil
	}

	// Now loop through the models we need to enable, and as a side-effect we build out
	// the ProviderOverride and ModelOverride objects.
	chatModelRef, err := incorporateModel(completionsCfg.ChatModel)
	if err != nil {
		return nil, errors.Wrap(err, "inspecting chat model")
	}
	completionModelRef, err := incorporateModel(completionsCfg.CompletionModel)
	if err != nil {
		return nil, errors.Wrap(err, "inspecting completion model")
	}
	fastChatModelRef, err := incorporateModel(completionsCfg.FastChatModel)
	if err != nil {
		return nil, errors.Wrap(err, "inspecting fast chat model")
	}
	defaultModels := types.DefaultModels{
		Chat:           chatModelRef,
		CodeCompletion: completionModelRef,
		FastChat:       fastChatModelRef,
	}

	// BUG: Two default models (e.g. chat and fast chat) can share the same ModelRef
	// but have different max tokens. In this case, the "last write wins"
	requiredModels[chatModelRef].ContextWindow = types.ContextWindow{
		MaxInputTokens:  completionsCfg.ChatModelMaxTokens,
		MaxOutputTokens: 4_000,
	}
	requiredModels[completionModelRef].ContextWindow = types.ContextWindow{
		MaxInputTokens:  completionsCfg.CompletionModelMaxTokens,
		MaxOutputTokens: 4_000,
	}
	requiredModels[fastChatModelRef].ContextWindow = types.ContextWindow{
		MaxInputTokens:  completionsCfg.FastChatModelMaxTokens,
		MaxOutputTokens: 4_000,
	}

	// Now lineraize those maps.
	var providerOverrides []types.ProviderOverride
	for _, providerOverride := range requiredProviders {
		providerOverrides = append(providerOverrides, *providerOverride)
	}
	var modelOverrides []types.ModelOverride
	for _, modelOverride := range requiredModels {
		modelOverrides = append(modelOverrides, *modelOverride)
	}
	// Sort the slices so they are deterministic.
	slices.SortFunc(providerOverrides, func(x, y types.ProviderOverride) int {
		return strings.Compare(string(x.ID), string(y.ID))
	})
	slices.SortFunc(modelOverrides, func(x, y types.ModelOverride) int {
		return strings.Compare(string(x.ModelRef), string(y.ModelRef))
	})

	baseConfig := types.SiteModelConfiguration{
		// Don't use any Sourcegraph-supplied model information, as that would be a breaking change.
		// As Cody Enterprise, via the Completions config, ONLY allows you to specify one model per use-case.
		SourcegraphModelConfig: nil,

		ProviderOverrides: providerOverrides,
		ModelOverrides:    modelOverrides,

		DefaultModels: &defaultModels,
	}

	if err := modelconfig.ValidateSiteConfig(&baseConfig); err != nil {
		return nil, errors.Wrap(err, "site configuration is invalid")
	}

	return &baseConfig, nil
}
