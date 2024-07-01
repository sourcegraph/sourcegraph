package modelconfig

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// Service is the system-wide component for obtaining the set of
// LLM models the current Sourcegraph instance is configured to use.
//
// You can obtain the global, package-level instance of this interface by
// calling the Get() method. It is safe for concurrent reads.
//
// Updating the system's model configuration is done by updating the Site
// Configuration. The implementation of the Service will listen
// for configuration changes and update the ModelConfiguration object in-
// memory as appropriate.
type Service interface {
	// Get returns the current model configuration for this Sourcegraph instance.
	// Callers should not modify the returned data, and treat it as if it were
	// immutable.
	Get() (*types.ModelConfiguration, error)
}

// Global instance of the model config service. We don't initialize via
// a sync.Once because we ensure Init cannot be called twice, and assume it
// will not be called concurrently.
var singletonConfigService *service

// Get returns the singleton ModelConfigService.
//
// This requires that the Init function has been called before hand, which
// is typically done on application startup.
func Get() Service {
	if singletonConfigService == nil {
		panic("ModelConfigService not initialized. Init not called.")
	}
	return singletonConfigService
}

// service implements the Service interface, and exposes a thread-safe `set` method
// for updating the current configuration.
type service struct {
	// currentConfig is the "source of truth" for this Sg instance's model configuration.
	currentConfig   *types.ModelConfiguration
	currentConfigMu sync.RWMutex
}

func (svc *service) Get() (*types.ModelConfiguration, error) {
	svc.currentConfigMu.RLock()
	defer svc.currentConfigMu.RUnlock()

	// Create a deep copy of the current configuration, so callers can operate on
	// older versions without worrying about data races or other types of errors.
	cfgCopy, err := deepCopy(svc.currentConfig)
	if err != nil {
		return nil, err
	}
	return cfgCopy, nil
}

// set updates the set.currentConfig to the supplied value. It is assumped that
// the Service will "own" the pointer, and the caller will no longer modify it.
func (svc *service) set(newConfig *types.ModelConfiguration) {
	// Block until the lock is available.
	svc.currentConfigMu.Lock()
	defer svc.currentConfigMu.Unlock()

	svc.currentConfig = newConfig
}

// deepCopy returns a deep copy of the entire ModelConfiguration data structure.
func deepCopy(source *types.ModelConfiguration) (*types.ModelConfiguration, error) {
	// Rather than manage all the boiler plage by hand, or resorting to reflection
	// we just round-trip the configuration data through JSON.
	//
	// This means that ALL fields in the types package MUST be exported, since
	// unexported fields will silently be dropped by JSON marshalling. But that's
	// not a problem in-practice.
	bytes, err := json.Marshal(source)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling source config")
	}

	var cfgCopy types.ModelConfiguration
	if err = json.Unmarshal(bytes, &cfgCopy); err != nil {
		return nil, errors.Wrap(err, "unmarshalling source config")
	}

	return &cfgCopy, nil
}

func convertLegacyCompletionsConfig(completionsCfg *schema.Completions) *types.SiteModelConfiguration {
	if completionsCfg == nil {
		return nil
	}

	chatModelRef := fmt.Sprintf("%s/unknown/%s", completionsCfg.Provider, completionsCfg.ChatModel)
	fastModelRef := fmt.Sprintf("%s/unknown/%s", completionsCfg.Provider, completionsCfg.FastChatModel)
	autocompleteModelRef := fmt.Sprintf("%s/unknown/%s", completionsCfg.Provider, completionsCfg.CompletionModel)

	baseConfig := types.SiteModelConfiguration{
		// Don't use any Sourcegraph-supplied model information.
		SourcegraphModelConfig: nil,

		ProviderOverrides: []types.ProviderOverride{
			{
				ID:               types.ProviderID(completionsCfg.Provider),
				ClientSideConfig: nil,
				ServerSideConfig: &types.ServerSideProviderConfig{
					GenericProvider: &types.GenericProviderConfig{
						AccessToken: completionsCfg.AccessToken,
						Endpoint:    completionsCfg.Endpoint,
					},
				},

				DefaultModelConfig: &types.DefaultModelConfig{
					Capabilities: []types.ModelCapability{
						types.ModelCapabilityAutocomplete,
						types.ModelCapabilityChat,
					},
					Category: types.ModelCategoryBalanced,
					Status:   types.ModelStatusStable,
					Tier:     types.ModelTierEnterprise,
				},
			},
		},

		ModelOverrides: []types.ModelOverride{
			{
				ModelRef:    types.ModelRef(chatModelRef),
				DisplayName: completionsCfg.ChatModel,
				ModelName:   completionsCfg.ChatModel,

				ContextWindow: types.ContextWindow{
					MaxInputTokens:  completionsCfg.ChatModelMaxTokens,
					MaxOutputTokens: 4000,
				},
			},
			{
				ModelRef:    types.ModelRef(fastModelRef),
				DisplayName: completionsCfg.FastChatModel,
				ModelName:   completionsCfg.FastChatModel,

				ContextWindow: types.ContextWindow{
					MaxInputTokens:  completionsCfg.FastChatModelMaxTokens,
					MaxOutputTokens: 4000,
				},
			},
			{
				ModelRef:    types.ModelRef(autocompleteModelRef),
				DisplayName: completionsCfg.CompletionModel,
				ModelName:   completionsCfg.CompletionModel,

				ContextWindow: types.ContextWindow{
					MaxInputTokens:  completionsCfg.CompletionModelMaxTokens,
					MaxOutputTokens: 4000,
				},
			},
		},

		DefaultModels: &types.DefaultModels{
			Chat:           types.ModelRef(completionsCfg.ChatModel),
			CodeCompletion: types.ModelRef(completionsCfg.CompletionModel),
			FastChat:       types.ModelRef(completionsCfg.FastChatModel),
		},
	}
	return &baseConfig
}
