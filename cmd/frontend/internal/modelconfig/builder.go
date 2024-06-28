package modelconfig

import (
	"fmt"
	"slices"

	"github.com/sourcegraph/sourcegraph/internal/modelconfig"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// builder implements the logic for constructing the Sourcegraph instance's
// LLM model configuration data, based on various configuration settings and available
// data.
type builder struct {
	// staticData is what is embedded into this binary, known at build-time.
	staticData *types.ModelConfiguration

	// codyGatewayData is what we have recently obtained by checking Cody Gateway
	// for any recent updates.
	//
	// TODO(chrsmith): This aspect is not yet implemented, and this field will
	// always be nil.
	codyGatewayData *types.ModelConfiguration

	// siteConfigData is the data that is defined in the site configuration.
	// This is in a slightly different format to be more expressive than what
	// is provided by Cody Gateway or embedded in the binary.
	siteConfigData *types.SiteModelConfiguration
}

// build merges all of the model configuration data together, presenting it in
// its final form to be consumed by the Sourcegraph instance and passed onto its
// clients.
func (b *builder) build() (*types.ModelConfiguration, error) {
	if b.staticData == nil {
		return nil, errors.New("no static data available")
	}
	baseConfig := b.staticData

	// If we have newer data from Cody Gateway, use that instead of what is
	// baked into our codebase.
	if b.codyGatewayData != nil {
		baseConfig = b.codyGatewayData
	}

	// Interpret site configuration.

	// If no site configuration data is supplied, then just use Sourcegraph
	// supplied data.
	if b.siteConfigData == nil {
		return deepCopy(baseConfig)
	}

	// But if we are using site config data, ensure it is valid before appying.
	if vErr := modelconfig.ValidateSiteConfig(b.siteConfigData); vErr != nil {
		return nil, errors.Wrap(vErr, "invalid site configuration")
	}
	outConfig, err := applySiteConfig(baseConfig, b.siteConfigData)
	if err != nil {
		return nil, errors.Wrap(err, "applying site config")
	}

	return outConfig, nil
}

// applySiteConfig returns the LLM Model configuration after applying the Sourcegraph admin supplied site config overrides.
// Will mutate the provided `baseConfig` in-place, and return the same value.
func applySiteConfig(baseConfig *types.ModelConfiguration, siteConfig *types.SiteModelConfiguration) (*types.ModelConfiguration, error) {
	if baseConfig == nil || siteConfig == nil {
		return nil, errors.New("baseConfig or siteConfig nil")
	}

	// If the admin has explicitly disabled the Sourcegraph-supplied data, zero out the base config.
	sgModelConfig := siteConfig.SourcegraphModelConfig
	if sgModelConfig == nil {
		baseConfig = &types.ModelConfiguration{
			Revision:      "-",
			SchemaVersion: types.CurrentModelSchemaVersion,

			// No Models or Providers.
			Providers: nil,
			Models:    nil,

			// Don't provide any DefaultModels either.
			//
			// These instead need to come from the siteConfig, or be inferred from
			// the models available.
			DefaultModels: types.DefaultModels{},
		}
	} else {
		// Apply any model filters from the base configuration.
		if modelFilters := sgModelConfig.ModelFilters; modelFilters != nil {
			var filteredModels []types.Model
			for _, baseConfigModel := range baseConfig.Models {
				// Status filter.
				if modelFilters.StatusFilter != nil {
					if !slices.Contains(modelFilters.StatusFilter, string(baseConfigModel.Category)) {
						continue
					}
				}

				// Allow list. If not specified, include all models.
				// Otherwise, only include those that match.
				if len(modelFilters.Allow) > 0 {
					if !filterListMatches(baseConfigModel.ModelRef, modelFilters.Allow) {
						continue
					}
				}
				// Deny list. Exclude all matches.
				if len(modelFilters.Deny) > 0 {
					if filterListMatches(baseConfigModel.ModelRef, modelFilters.Deny) {
						continue
					}
				}

				filteredModels = append(filteredModels, baseConfigModel)
			}

			// Replace the base config models with the filtered set.
			baseConfig.Models = filteredModels
		}
	}

	// Apply any ProviderOverrides from the site configuration to the baseConfig object.
	providerOverrideLookup := map[types.ProviderID]*types.ProviderOverride{}
	for i := range siteConfig.ProviderOverrides {
		providerOverride := &siteConfig.ProviderOverrides[i]
		providerOverrideLookup[providerOverride.ID] = providerOverride

		// Lookup the provider this configuration is for.
		var providerToOverride *types.Provider
		for i := range baseConfig.Providers {
			if baseConfig.Providers[i].ID == providerOverride.ID {
				providerToOverride = &baseConfig.Providers[i]
				break
			}
		}

		// The site configuration has an override for a provider that
		// isn't in the base config. So it must entirely come from the
		// site configuration.
		if providerToOverride == nil {
			providerToOverride = &types.Provider{
				ID:               providerOverride.ID,
				DisplayName:      fmt.Sprintf("Provider %q", providerOverride.ID),
				ServerSideConfig: providerOverride.ServerSideConfig,
			}
			baseConfig.Providers = append(baseConfig.Providers, *providerToOverride)
		}

		// Blow away the client/server-side config. We don't bother merging it
		// since we don't expect any Sourcegraph-supplied configuration data to
		// contain client or server-side specific configuration
		providerToOverride.ClientSideConfig = providerOverride.ClientSideConfig
		providerToOverride.ServerSideConfig = providerOverride.ServerSideConfig
	}

	// Apply Model Overrides. Since we need to apply any ProviderOverride.DefaultModelConfig,
	// we just build a lookup and add any entries to the baseConfig.Models. So we can actually
	// set their fields later.
	modelOverrideLookup := map[types.ModelRef]*types.ModelOverride{}
	for i := range siteConfig.ModelOverrides {
		modelOverride := &siteConfig.ModelOverrides[i]
		modelOverrideLookup[modelOverride.ModelRef] = modelOverride
	}

	// Now loop through all baseConfig models, and apply the override or provider
	// defaults.
	for i := range baseConfig.Models {
		mod := &baseConfig.Models[i]

		// If this model is associated with one of the ProviderOverrides, then fetch
		// its DefaultModelConfig.
		var providerDefaultModelConfig *types.DefaultModelConfig
		modelProviderID := mod.ModelRef.ProviderID()
		if providerOverride := providerOverrideLookup[modelProviderID]; providerOverride != nil {
			providerDefaultModelConfig = providerOverride.DefaultModelConfig
		}

		// Apply the Provider's DefaultModelConfig, if applicable.
		err := modelconfig.ApplyDefaultModelConfiguration(mod, providerDefaultModelConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "applying provider default model config (%q)", modelProviderID)
		}

		// If a ModelOverride exists for this particular model, then apply it.
		if modelOverride := modelOverrideLookup[mod.ModelRef]; modelOverride != nil {
			if err = modelconfig.ApplyModelOverride(mod, *modelOverride); err != nil {
				return nil, errors.Wrapf(err, "applying model override (%q)", mod.ModelRef)
			}

			// Remove the key from the modelOverrideLookup, see below.
			delete(modelOverrideLookup, mod.ModelRef)
		}
	}

	// If there are remaining keys in `modelOverrideLookup` means that the are for a ModelRef that
	// wasn't found in the base configuration. So in that case we add those as "entirely new" models
	// to the base config.
	for _, modelOverride := range modelOverrideLookup {
		newModelRef := modelOverride.ModelRef
		newModel := &types.Model{
			ModelRef: newModelRef,
			// This isn't to provide a "default" so much as it is just to
			// ensure it isn't completely invalid.
			ContextWindow: types.ContextWindow{
				MaxInputTokens:  4_000,
				MaxOutputTokens: 4_000,
			},
		}

		// Apply provider default config if applicable.
		var providerDefaultModelConfig *types.DefaultModelConfig
		modelProviderID := newModelRef.ProviderID()
		if providerOverride := providerOverrideLookup[modelProviderID]; providerOverride != nil {
			providerDefaultModelConfig = providerOverride.DefaultModelConfig
		}

		// Apply the Provider's DefaultModelConfig, if applicable.
		err := modelconfig.ApplyDefaultModelConfiguration(newModel, providerDefaultModelConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "applying default provider config (%q)", modelProviderID)
		}

		// Apply the ModelOverride.
		if err = modelconfig.ApplyModelOverride(newModel, *modelOverride); err != nil {
			return nil, errors.Wrapf(err, "applying model override (%q)", newModelRef)
		}

		baseConfig.Models = append(baseConfig.Models, *newModel)
	}

	// Use the DefaultModels from the site config. Otherwise, we need to pick something randomly
	// to ensure they are at least defined.
	if siteConfig.DefaultModels != nil {
		baseConfig.DefaultModels.Chat = siteConfig.DefaultModels.Chat
		baseConfig.DefaultModels.CodeCompletion = siteConfig.DefaultModels.CodeCompletion
		baseConfig.DefaultModels.FastChat = siteConfig.DefaultModels.FastChat
	} else {
		getModelMatchingCategory := func(wantCategories ...types.ModelCategory) *types.ModelRef {
			for _, model := range baseConfig.Models {
				for _, wantCategory := range wantCategories {
					if model.Category == wantCategory {
						return &model.ModelRef
					}
				}
			}
			return nil
		}
		// Infer the default models to used based on category. This is probably not going to lead to great
		// results. But :shrug: it's better than just crash looping because the config is under-specified.
		if baseConfig.DefaultModels.Chat == "" {
			validModel := getModelMatchingCategory(types.ModelCategoryAccuracy, types.ModelCategoryBalanced)
			if validModel == nil {
				return nil, errors.New("no suitable model found for Chat")
			}
			baseConfig.DefaultModels.Chat = *validModel
		}
		if baseConfig.DefaultModels.FastChat == "" {
			validModel := getModelMatchingCategory(types.ModelCategorySpeed, types.ModelCategoryBalanced)
			if validModel == nil {
				return nil, errors.New("no suitable model found for FastChat")
			}
			baseConfig.DefaultModels.FastChat = *validModel
		}
		if baseConfig.DefaultModels.CodeCompletion == "" {
			validModel := getModelMatchingCategory(types.ModelCategorySpeed, types.ModelCategoryBalanced)
			if validModel == nil {
				return nil, errors.New("no suitable model found for Chat")
			}
			baseConfig.DefaultModels.CodeCompletion = *validModel
		}
	}

	// Validate the resulting configuration.
	if err := modelconfig.ValidateModelConfig(baseConfig); err != nil {
		return nil, errors.Wrap(err, "result of application was invalid configuration")
	}
	return baseConfig, nil
}
