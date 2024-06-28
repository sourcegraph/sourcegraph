package modelconfig

import (
	"context"
	"encoding/json"
	"os"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	_ "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth"
	_ "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/registry"
	"github.com/sourcegraph/sourcegraph/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/embedded"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// Init is the initalization function wired into the `frontend` application startup.
// This registers the necessary watchers and hooks so that the `Service` can always
// have an up-to-date view of this Sourcegraph instance's configuration data.
func Init(
	ctx context.Context,
	observationCtx *observation.Context,
	db database.DB,
	codeIntelServices codeintel.Services,
	initialConf conftypes.UnifiedWatchable,
	enterpriseServices *enterprise.Services,
) error {
	// Ensure that we only regsiter this once.
	if singletonConfigService != nil {
		return errors.New("the Init function has already been called")
	}

	logger := log.Scoped("modelconfig")

	initialSiteConfig := initialConf.SiteConfig()
	// If Cody isn't enabled on startup, we don't bother registering anything. (Because there is no need to.)
	if codyEnabled := initialSiteConfig.CodyEnabled; codyEnabled == nil || *codyEnabled == false {
		logger.Info("Cody is not enabled, not registering ModelConfigService")
		return nil
	}

	// The base model configuration data is what is embedded in this binary.
	staticModelConfig, err := embedded.GetCodyGatewayModelConfig()
	if err != nil {
		return errors.New("embedded model data is missing or corrupt")
	}

	// Load the newer form of LLM model configuration data.
	siteModelConfig, err := loadModelConfigFromSiteConfig(initialSiteConfig)
	if err != nil {
		// BUG: This might be incorrect. If the service cannot start up because of bad site
		// configuration, then it also means that there is no way for an admin to fix the
		// problem. So this might require some thought.
		return errors.New("error loading configuration data via site config")
	}

	// Now build the initial view of the Sourcegraph instance's model configuration using this data.
	configBuilder := builder{
		staticData:      staticModelConfig,
		codyGatewayData: nil,
		siteConfigData:  siteModelConfig,
	}
	initialConfig, err := configBuilder.build()
	if err != nil {
		return errors.Wrap(err, "building initial model configuration")
	}

	// Register the initial singletonConfigService.
	initialConfigSvc := service{
		currentConfig: initialConfig,
	}
	singletonConfigService = &initialConfigSvc

	// Register a watcher to pull in updates whenever the site config changes.
	conf.Watch(func() {
		logger.Info("site config updated, recalculating model configuration")

		latestSiteConfig := conf.Get().SiteConfiguration
		latestModelConfig, err := loadModelConfigFromSiteConfig(latestSiteConfig)
		if err != nil {
			logger.Error("error loading model configuration via site config", log.Error(err))
			return
		}

		// Update and rebuild the LLM model configuration.
		//
		// BUG: We currently don't need a mutex on `configBuilder` because there
		// is only one writer (this callback). But when we wire up the Cody Gateway
		// polling, we'll need to address that. Perhaps as a "<-chan configUpdate"
		configBuilder.siteConfigData = latestModelConfig
		updatedConfig, err := configBuilder.build()
		if err != nil {
			logger.Error("error regenerating model config based on site config update", log.Error(err))
			return
		}

		singletonConfigService.set(updatedConfig)
	})

	// TODO(chrsmith): When the enhanced model configuration data is available, if configured to do so
	// we will spawn a background job that will poll Cody Gateway for any updated model information. This
	// will be tricky, because we want to honor any dynamic changes to the site config. e.g. the `conf.Watch`
	// callback needs to be able to stop/restart/update the way the poller works in response to changes.
	return nil
}

func loadModelConfigFromSiteConfig(siteConfig schema.SiteConfiguration) (*types.SiteModelConfiguration, error) {
	// TODO(chrsmith): Encode this in the site.schema.json. Until then, we use a glorious hack
	// to pull this from an environment variable if possible.
	if fauxConfigJSON := os.Getenv("HACK_FAUX_MODELSOURCES_CONFIG_JSON"); fauxConfigJSON == "" {
		// If no "newer style" model configuration data was specified, fall back to converting
		// any settings from Sourcegraph v5.3 and prior. i.e. the "completions" object.
		//
		// TODO(chrsmith): When we wire up the new model schema, specifying both forms should be an error.
		return convertLegacyCompletionsConfig(siteConfig.Completions), nil
	} else {
		// Otherwise, parse the env var and use that as the siteModelConfig.
		var siteModelConfig types.SiteModelConfiguration
		if err := json.Unmarshal([]byte(fauxConfigJSON), &siteModelConfig); err != nil {
			return nil, errors.Wrap(err, "unmarshalling config data from 'HACK_FAUX_MODELSOURCES_CONFIG_JSON'")
		}
		return &siteModelConfig, nil
	}
}
