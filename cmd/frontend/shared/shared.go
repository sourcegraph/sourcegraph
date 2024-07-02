// Package shared is the enterprise frontend program's shared main entrypoint.
//
// It lets the invoker of the OSS frontend shared entrypoint injects a few
// proprietary things into it via e.g. blank/underscore imports in this file
// which register side effects with the frontend package.
package shared

import (
	"context"
	"os"
	"strconv"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/authz"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/batches"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/clientconfig"
	codeintelinit "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/codemonitors"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/codycontext"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/completions"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/compute"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/contentlibrary"
	internalcontext "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/context"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/embeddings"
	executor "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/executorqueue"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/githubapp"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/guardrails"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/insights"
	licensing "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/licensing/init"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/notebooks"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/own"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/rbac"
	_ "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/registry"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/repos/webhooks"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/scim"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/searchcontexts"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/telemetry"
	"github.com/sourcegraph/sourcegraph/internal/codeintel"
	codeintelshared "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	connections "github.com/sourcegraph/sourcegraph/internal/database/connections/live"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/schema"
)

type EnterpriseInitializer = func(context.Context, *observation.Context, database.DB, codeintel.Services, conftypes.UnifiedWatchable, *enterprise.Services) error

var initFunctions = map[string]EnterpriseInitializer{
	"authz":          authz.Init,
	"batches":        batches.Init,
	"clientconfig":   clientconfig.Init,
	"codeintel":      codeintelinit.Init,
	"codemonitors":   codemonitors.Init,
	"completions":    completions.Init,
	"compute":        compute.Init,
	"dotcom":         dotcom.Init,
	"embeddings":     embeddings.Init,
	"context":        internalcontext.Init,
	"githubapp":      githubapp.Init,
	"guardrails":     guardrails.Init,
	"insights":       insights.Init,
	"licensing":      licensing.Init,
	"notebooks":      notebooks.Init,
	"own":            own.Init,
	"rbac":           rbac.Init,
	"repos.webhooks": webhooks.Init,
	"scim":           scim.Init,
	"searchcontexts": searchcontexts.Init,
	"contentLibrary": contentlibrary.Init,
	"search":         search.Init,
	"telemetry":      telemetry.Init,
	"codycontext":    codycontext.Init,
}

func EnterpriseSetupHook(db database.DB, conf conftypes.UnifiedWatchable) enterprise.Services {
	logger := log.Scoped("enterprise")
	debug, _ := strconv.ParseBool(os.Getenv("DEBUG"))
	if debug {
		logger.Debug("enterprise edition")
	}

	auth.Init(logger, db)

	ctx := context.Background()
	enterpriseServices := enterprise.DefaultServices()

	observationCtx := observation.NewContext(logger)

	codeIntelServices, err := codeintel.NewServices(codeintel.ServiceDependencies{
		DB:             db,
		CodeIntelDB:    mustInitializeCodeIntelDB(logger),
		ObservationCtx: observationCtx,
	})
	if err != nil {
		logger.Fatal("failed to initialize code intelligence", log.Error(err))
	}

	for name, fn := range initFunctions {
		if err := fn(ctx, observationCtx, db, codeIntelServices, conf, &enterpriseServices); err != nil {
			logger.Fatal("failed to initialize", log.String("name", name), log.Error(err))
		}
	}

	// Inititalize executor last, as we require code intel and batch changes services to be
	// already populated on the enterpriseServices object.
	if err := executor.Init(observationCtx, db, conf, &enterpriseServices); err != nil {
		logger.Fatal("failed to initialize executor", log.Error(err))
	}

	return enterpriseServices
}

func mustInitializeCodeIntelDB(logger log.Logger) codeintelshared.CodeIntelDB {
	dsn := conf.GetServiceConnectionValueAndRestartOnChange(func(serviceConnections conftypes.ServiceConnections) string {
		return serviceConnections.CodeIntelPostgresDSN
	})

	db, err := connections.EnsureNewCodeIntelDB(observation.NewContext(logger), dsn, "frontend")
	if err != nil {
		logger.Fatal("Failed to connect to codeintel database", log.Error(err))
	}

	return codeintelshared.NewCodeIntelDB(logger, db)
}

func switchableSiteConfig() conftypes.WatchableSiteConfig {
	confClient := conf.DefaultClient()
	switchable := &switchingSiteConfig{
		watchers:            make([]func(), 0),
		WatchableSiteConfig: &noopSiteConfig{},
	}
	switchable.WatchableSiteConfig.(*noopSiteConfig).switcher = switchable

	go func() {
		<-AutoUpgradeDone
		httpcli.Configure(confClient)
		switchable.WatchableSiteConfig = confClient
		for _, watcher := range switchable.watchers {
			confClient.Watch(watcher)
		}
		switchable.watchers = nil
	}()

	return switchable
}

type switchingSiteConfig struct {
	watchers []func()
	conftypes.WatchableSiteConfig
}

type noopSiteConfig struct {
	switcher *switchingSiteConfig
}

func (n *noopSiteConfig) SiteConfig() schema.SiteConfiguration {
	return schema.SiteConfiguration{}
}

func (n *noopSiteConfig) Watch(f func()) {
	f()
	n.switcher.watchers = append(n.switcher.watchers, f)
}
