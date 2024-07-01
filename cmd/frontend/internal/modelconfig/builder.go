package modelconfig

import (
	"fmt"
	"slices"
	"strings"

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

	// If the old config data is provided, then ignore the baseConfig entirely.
	// Since otherwise it would be a breaking change to include all of the Cody LLM Models.

	// Interpret site configuration.

	// If no site configuration data is supplied, then just use Sourcegraph
	// supplied data.
	if b.siteConfigData == nil {
		return deepCopy(baseConfig)
	}
	// But if we are using site config data, ensure it is valid.
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
		}
	} else {
		// Apply any model filters.
		if modelFilters := sgModelConfig.ModelFilters; modelFilters != nil {
			var filteredModels []types.Model
			for _, baseConfigModel := range baseConfig.Models {
				// Category filter.
				if modelFilters.CategoryFilter != nil {
					if !slices.Contains(modelFilters.CategoryFilter, string(baseConfigModel.Category)) {
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
			baseConfig.Models = filteredModels
		}
	}

	for _, providerOverride := range siteConfig.ProviderOverrides {
		var providerToOverride *types.Provider
		// Lookup the provider this configuration is for.
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
		providerToOverride.ServerSideConfig = providerOverride.ServerSideConfig
	}

	// Model Overrides
	for _, modelOverride := range siteConfig.ModelOverrides {
		var modelToOverride *types.Model
		// Lookup the model this configuration is for.
		for i := range baseConfig.Models {
			if baseConfig.Models[i].ModelRef == modelOverride.ModelRef {
				modelToOverride = &baseConfig.Models[i]
				break
			}
		}

		// The site configuration has an override for a model that
		// isn't in the base config. So it must entirely come from the
		// site configuration.
		if modelToOverride == nil {
			modelToOverride = &types.Model{
				ModelRef: modelOverride.ModelRef,
			}

			// TODO: We need to apply DefaultModelConfig from the ProviderOverride,
			// if one is supplied.

			baseConfig.Models = append(baseConfig.Models, *modelToOverride)
		}
	}

	if siteConfig.DefaultModels != nil {
		baseConfig.DefaultModels = *siteConfig.DefaultModels
	}

	return baseConfig, nil
}

// filterListMatches returns whether or not any of the patterns match the supplied
// mref. Assumes the supplied patterns are well-formed. Any asterisks can only be
// in the first or last character of the pattern.
func filterListMatches(mref types.ModelRef, patterns []string) bool {
	s := string(mref)
	for _, pattern := range patterns {
		if pattern == "*" {
			return true
		}

		pLen := len(pattern)
		if pLen < 3 {
			continue // Invalid pattern.
		}
		hasLeadingAsterisk := pattern[0] == '*'
		hasTrailingAsterisk := pattern[pLen-1] == '*'

		// e.g. "*latest"
		if hasLeadingAsterisk && !hasTrailingAsterisk {
			if strings.HasSuffix(s, pattern[1:]) {
				return true
			}
		}
		// e.g. "openai::*"
		if !hasLeadingAsterisk && hasTrailingAsterisk {
			if strings.HasPrefix(s, pattern[:pLen-1]) {
				return true
			}
		}
		// e.g. "anthropic::2023-06-01::claude-3-sonnet"
		if !hasLeadingAsterisk && !hasTrailingAsterisk {
			if s == pattern {
				return true
			}
		}
		// e.g. "*gpt*"
		if hasLeadingAsterisk && hasTrailingAsterisk {
			if strings.Contains(s, pattern[1:pLen-1]) {
				return true
			}
		}
	}

	return false
}
