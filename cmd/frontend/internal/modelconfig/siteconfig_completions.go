package modelconfig

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// defModelKind is an enum for the three types of models you can specify when using
// the "completions" site configuration.
type defModelKind int

const (
	defModelChatModel defModelKind = iota
	defModelCompletion
	defModelFastChatModel
)

func (k defModelKind) String() string {
	switch k {
	case defModelChatModel:
		return "chat"
	case defModelCompletion:
		return "completion"
	case defModelFastChatModel:
		return "fast-chat"
	default:
		return "unknown"
	}
}

// legacyModelRef is the data that is encoded in the "model" strings within the Completions site config.
// i.e. the potentially ambugious format we used prior to types.ModelRef.
type legacyModelRef struct {
	provider string
	model    string
	// If using AWS Bedrock, we support encoding the ARN in the model name in the
	// site config. See `conftypes.NewBedrockModelRefFromModelID` for more info.`
	awsBedrockConfig *types.ServerSideModelConfig
}

// parseLegacyModelRef takes a reference to a model from the site configuration in the "legacy format",
// and infers all the surrounding data. e.g. "claude-instant", "openai/gpt-4o".
func parseLegacyModelRef(
	completionsCfg *conftypes.CompletionsConfig, modelIDFromConfig string) (legacyModelRef, error) {
	var (
		providerID string
		modelID    string

		awsBedrockConfig *types.ServerSideModelConfig
	)
	// SplitN because in the AWS Bedrock case, there may be many slashes in the ARN.
	parts := strings.SplitN(modelIDFromConfig, "/", 2)
	switch len(parts) {
	case 1:
		providerID = string(completionsCfg.Provider)
		modelID = parts[0]
	case 2:
		providerID = parts[0]
		modelID = parts[1]
	default:
		return legacyModelRef{}, errors.Errorf("invalid model ID in config %q", modelIDFromConfig)
	}
	// Edge case, we support the user encoding an ARN in the model in the config.
	if completionsCfg.Provider == conftypes.CompletionsProviderNameAWSBedrock {
		bedrockModelRef := conftypes.NewBedrockModelRefFromModelID(modelIDFromConfig)
		providerID = "anthropic"
		// The model ID may contain colons, which we reject as part of the ModelID validation,
		// so we strip those out here.
		modelID = strings.ReplaceAll(bedrockModelRef.Model, ":", "_")

		if bedrockModelRef.ProvisionedCapacity != nil {
			awsBedrockConfig = &types.ServerSideModelConfig{
				AWSBedrockProvisionedThroughput: &types.AWSBedrockProvisionedThroughput{
					ARN: *bedrockModelRef.ProvisionedCapacity,
				},
			}
		}
	}

	ref := legacyModelRef{
		provider:         providerID,
		model:            modelID,
		awsBedrockConfig: awsBedrockConfig,
	}
	return ref, nil
}

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
// MAY leave some things blank, but `conf/computed.go`'s `GetCompletionsConfig()` will provide defaults for all the
// values. So even if the site admin didn't provide the `Endpoint` field, we expect it to be present.
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

	// extractModelConfigInfo will pull out all of the configuration data for the given kind of model described
	// in the site configuration. e.g. the provider and model-specific settings for the configured "fast-chat"
	// model.
	extractModelConfigInfo := func(
		kind defModelKind, modelIDFromConfig string) (*types.ProviderOverride, *types.ModelOverride, error) {
		legacyModelRef, err := parseLegacyModelRef(completionsCfg, modelIDFromConfig)
		if err != nil {
			return nil, nil, errors.Wrap(err, "parsing legacy model ref")
		}

		// Create ProviderOverride if we haven't seen this provider before.
		// We need to remap the provider ID if it is referring to an API Provider and not
		// a Model Provider.
		var effectiveProviderID string
		switch legacyModelRef.provider {
		case "aws-bedrock":
			effectiveProviderID = "anthropic"
		case "azure-ai":
			effectiveProviderID = "openai"
		default:
			effectiveProviderID = legacyModelRef.provider
		}

		// Create the ProviderOverride with the configuration data.
		providerOverride := types.ProviderOverride{
			ID:                 types.ProviderID(effectiveProviderID),
			DefaultModelConfig: &defaultModelConfig,
			ClientSideConfig:   nil,
			ServerSideConfig:   getProviderConfiguration(completionsCfg),
		}

		// Each type of model reference in the site config can have its max token sizes
		// configured independently.
		var maxInputTokens int
		switch kind {
		case defModelChatModel:
			maxInputTokens = completionsCfg.ChatModelMaxTokens
		case defModelCompletion:
			maxInputTokens = completionsCfg.CompletionModelMaxTokens
		case defModelFastChatModel:
			maxInputTokens = completionsCfg.FastChatModelMaxTokens
		}

		// Create the ModelOverride if we haven't seen this model before.
		rawModelRef := fmt.Sprintf("%s::unknown::%s", effectiveProviderID, legacyModelRef.model)
		modelRef := types.ModelRef(rawModelRef)
		modelOverride := types.ModelOverride{
			ModelRef:    types.ModelRef(modelRef),
			DisplayName: legacyModelRef.model,
			ModelName:   legacyModelRef.model,

			// BUG: The ModelConfiguration schema does not recognize "smart context", and
			// will likely have breaking changes when rolled out. Carefully read this thread
			// and internalize what needs to happe to faithfully reproduce the intent of
			// those settings:
			// https://sourcegraph.com/docs/cody/clients/enable-cody-enterprise#smart-context-window
			// https://sourcegraph.slack.com/archives/C04MSD3DP5L/p1718294914637509
			ContextWindow: types.ContextWindow{
				MaxInputTokens: maxInputTokens,
				// The default here is to match Cody Gateway, which will reject
				// requests to output more than Xk tokens.
				MaxOutputTokens: 4_000,
			},

			// Will only be non-nil when appropriate.
			ServerSideConfig: legacyModelRef.awsBedrockConfig,
		}

		return &providerOverride, &modelOverride, nil
	}

	// Load all the data for the chat, completions, and fast-chat models. It's very likely
	// that it is redundant, e.g. they all are using the same provider with the same
	// configuration settings.
	configuredProviders := map[defModelKind]*types.ProviderOverride{}
	configuredModels := map[defModelKind]*types.ModelOverride{}

	loadModel := func(kind defModelKind, model string) error {
		providerCfg, modelCfg, err := extractModelConfigInfo(kind, model)
		configuredProviders[kind] = providerCfg
		configuredModels[kind] = modelCfg
		return err
	}
	if err := loadModel(defModelChatModel, completionsCfg.ChatModel); err != nil {
		return nil, errors.Wrap(err, "inspecting chat model")
	}
	if err := loadModel(defModelCompletion, completionsCfg.CompletionModel); err != nil {
		return nil, errors.Wrap(err, "inspecting completion model")
	}
	if err := loadModel(defModelFastChatModel, completionsCfg.FastChatModel); err != nil {
		return nil, errors.Wrap(err, "inspecting fast chat model")
	}

	// Dedupe the provider information. We only allow you to specify a single "provider" in the
	// config, but may output multiple ProviderOverrides. (e.g. if you are using the "sourcegraph"
	// provider, and referencing models from Fireworks, OpenAI, and Anthropic, then we will create
	// 3x types.ProviderOverride objects. But with each one using Sourcegraph as the API provider.)
	maps.DeleteFunc(configuredProviders, func(currentKind defModelKind, currentProvider *types.ProviderOverride) bool {
		// Delete the current provider if there is another one with the same values
		// to be returned.
		for otherKind, otherProvider := range configuredProviders {
			if currentKind != otherKind && currentProvider.ID == otherProvider.ID {
				// Delete this ProviderOverride, as it is just redudnant.
				return true
			}
		}
		return false
	})

	// stableDefModelKindIter is a stable iterator we can use since "for range" over maps is non-deterministic.
	stableDefModelKindIter := []defModelKind{defModelChatModel, defModelCompletion, defModelFastChatModel}

	// Deduping models is more tricky. If two models are referring to the same ModelRef, then generally
	// we should delete one of them. HOWEVER, because the max tokens can be configured, we also need
	// to compare the ModelOverride's metadata as well. And if there IS a difference there, then we
	// need to UPDATE the ModelRef, to disambiguate the two. e.g. the ModelRef
	// "anthropic::unknown::claude-instant_chat" has a different ContextWindow setting than
	// "anthropic::unknown::claude-instant_completions".
	for _, modelKind := range stableDefModelKindIter {
		for _, otherModelKind := range stableDefModelKindIter {
			// Ignore comparing the same kind, e.g. checking if "chat" is different from "chat".
			if modelKind == otherModelKind {
				continue
			}

			modelOverride := configuredModels[modelKind]
			otherModelOverride := configuredModels[otherModelKind]

			// Ignore if pointing to the same thing, e.g. we've already deduped.
			// e.g. "chat" points to the same *ModelOverride as "fast-chat".
			if configuredModels[modelKind] == configuredModels[otherModelKind] {
				continue
			}
			// If this modelKind (e.g. "chat") is referring to the same LLM Model as
			// the other modelKind (e.g. "fast-chat") then we need to do something.
			if modelOverride.ModelRef == otherModelOverride.ModelRef {
				// If the context window sizes are the same, then just update the
				// pointer in configuredModels so both are pointing to the same object.
				if modelOverride.ContextWindow.MaxInputTokens == otherModelOverride.ContextWindow.MaxInputTokens {
					configuredModels[otherModelKind] = configuredModels[modelKind]
				} else {
					// If there is a configured difference between the two, then just
					// add a suffix to the ModelRef to disambiguate.
					configuredModels[modelKind].ModelRef =
						types.ModelRef(string(configuredModels[modelKind].ModelRef) + "_" + modelKind.String())
				}
			}
		}
	}

	defaultModels := types.DefaultModels{
		Chat:           configuredModels[defModelChatModel].ModelRef,
		CodeCompletion: configuredModels[defModelCompletion].ModelRef,
		FastChat:       configuredModels[defModelFastChatModel].ModelRef,
	}

	// Now lineraize those maps.
	var providerOverrides []types.ProviderOverride
	for _, providerOverride := range configuredProviders {
		providerOverrides = append(providerOverrides, *providerOverride)
	}
	var modelOverrides []types.ModelOverride
	for _, modelKind := range stableDefModelKindIter {
		modelOverride := configuredModels[modelKind]
		// Since two kinds of default models may be aliasing the same
		// *ModelOverride, check if it is unique first.
		var alreadyInList bool
		for _, existingModel := range modelOverrides {
			if existingModel.ModelRef == modelOverride.ModelRef {
				alreadyInList = true
				break
			}
		}
		if !alreadyInList {
			modelOverrides = append(modelOverrides, *modelOverride)
		}
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
