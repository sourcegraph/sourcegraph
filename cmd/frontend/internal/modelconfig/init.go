package modelconfig

import (
	"context"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/embedded"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"

	modelconfigSDK "github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
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

	// We create the GraphQL resolver in this package, to avoid the circular dependency
	// between the graphqlbackend, enterprise, and this modelconfig package.
	enterpriseServices.ModelconfigResolver = newResolver(logger)

	// The base model configuration data is what is embedded in this binary.
	staticModelConfig, err := embedded.GetCodyGatewayModelConfig()
	if err != nil {
		logger.Error(
			"embedded model data is missing or corrupt and will not be used when determining available LLM models",
			log.Error(err))
		staticModelConfig = nil
	}

	// Try to load the admin-supplied configuration data. If this fails, e.g.
	// because the data is malformed, we just don't apply the configuration.
	// If the site configuration changes later, we'll retry loading and applying the
	// new configuration data.
	initialSiteConfig := initialConf.SiteConfig()
	siteModelConfig, err := maybeGetSiteModelConfiguration(logger, initialSiteConfig)
	if err != nil {
		logger.Error("error loading LLM model configuration data via site config on startup", log.Error(err))
		siteModelConfig = nil
	}

	// Now build the initial view of the Sourcegraph instance's model configuration using this data.
	// This configManager will mediate how async changes get translated into new updates to the
	// modelconfig.Service.
	configManager := &manager{
		logger: logger,
		builder: &builder{
			staticData:      staticModelConfig,
			codyGatewayData: nil, // NYI
			siteConfigData:  siteModelConfig,
		},
	}
	initialConfig, err := configManager.builder.build()
	if err != nil {
		// If somehow the configuration data is corrupted to the point we
		// weren't able to actually know what LLMs are supported, we still
		// want to allow the Sourcegraph instance to start up. And rely on
		// the site config validators to surface any errors to site admins
		// in the hope that they can address them.
		logger.Error("error building initial model configuration", log.Error(err))
		initialConfig = &types.ModelConfiguration{}
	}

	// Register the initial singletonConfigService.
	initialConfigSvc := service{
		currentConfig: initialConfig,
	}
	singletonConfigService = &initialConfigSvc

	// Register a watcher to pull in updates whenever the site config changes.
	conf.Watch(func() {
		logger.Info("applying site config changes to modelconfig")
		configManager.OnSiteConfigChange()
	})

	// Register a new site configuration validator, to surface any errors
	// for admin-supplied changes to LLM model configuration.
	conf.ContributeValidator(func(q conftypes.SiteConfigQuerier) conf.Problems {
		newSiteConfig := q.SiteConfig()

		// Unfortuantely we fail on the first error we encounter, rather than trying to
		// aggregate as many errors as we can find.
		_, err := configManager.applyNewSiteConfig(newSiteConfig)
		if err != nil {
			return conf.NewSiteProblems(err.Error())
		}
		return nil
	})

	return nil
}

// manager is responsible for keeping track of changes to the current Sourcegraph instance, and
// propagating model configuration changes. i.e. it wraps the builder type, providing hooks for
// when inputs to the configuration system changes, and then pushes the results to singletonConfigService.
type manager struct {
	logger  log.Logger
	builder *builder
}

// OnSiteConfigChange should be called whenever the Sourcegraph instance's site configuration changes.
// If there is an error while calculating the updated model configuration, no changes will be applied.
// (e.g. whatever the previous configuration data was will remain in-place.)
func (m *manager) OnSiteConfigChange() {
	latestSiteConfig := conf.Get().SiteConfiguration
	updatedConfig, err := m.applyNewSiteConfig(latestSiteConfig)
	if err != nil {
		// On error, just log and keep the current settings as-is.
		m.logger.Error("error applying site config changes", log.Error(err))
		return
	}

	// Expose the new configuration data from the Service.
	singletonConfigService.set(updatedConfig)
}

// applyNewSiteConfig attempts to merge the new site configuration data, with what is already
// available statically or from Cody Gateway.
func (m *manager) applyNewSiteConfig(latestSiteConfig schema.SiteConfiguration) (*modelconfigSDK.ModelConfiguration, error) {
	latestSiteModelConfiguration, err := maybeGetSiteModelConfiguration(m.logger, latestSiteConfig)
	if err != nil {
		// NOTE: If the site configuration data is somehow bad, we silently ignore
		// the changes. This is bad, because there is no signal to the Sourcegraph
		// admin that their configuration is invalid. Hopefully we can put the necessary
		// validation logic inside the site configuration validation checks, so
		// that they will be prevented from saving invalid config data in the first
		// place. But we need to always account for bogus/corrupted config data.
		return nil, errors.Wrap(err, "loading site configuration data")
	}

	// Update and rebuild the LLM model configuration.
	//
	// NOTE: We currently don't need a mutex on `configBuilder` because there
	// is only one writer (this callback). But when we wire up the Cody Gateway
	// polling, we'll need to address that.
	m.builder.siteConfigData = latestSiteModelConfiguration
	updatedConfig, err := m.builder.build()
	if err != nil {
		m.logger.Error("error calculating modelconfig", log.Error(err))
		return nil, err
	}

	return updatedConfig, nil
}
