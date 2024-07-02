package clientconfig

import (
	"context"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	_ "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth"
	_ "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/registry"
	"github.com/sourcegraph/sourcegraph/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
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

	// Register the initial singletonConfigService.
	initialConfigSvc := service{
		db:     db,
		logger: log.Scoped("clientconfig"),
	}
	singletonConfigService = &initialConfigSvc
	return nil
}
