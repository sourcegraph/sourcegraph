package orgrepos

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/orgrepos/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
)

func Init(ctx context.Context, db dbutil.DB, _ conftypes.UnifiedWatchable, _outOfBandMigrationRunner *oobmigration.Runner, enterpriseServices *enterprise.Services, observationContext *observation.Context) error {
	enterpriseServices.OrgRepositoryResolver = resolvers.NewResolver(database.NewDB(db))
	return nil
}
