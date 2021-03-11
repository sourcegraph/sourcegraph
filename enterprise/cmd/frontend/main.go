// Command frontend contains the enterprise frontend implementation.
//
// It wraps the open source frontend command and merely injects a few
// proprietary things into it via e.g. blank/underscore imports in this file
// which register side effects with the frontend package.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/shared"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/authz"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/codemonitors"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/executor"
	licensing "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/licensing/init"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"

	_ "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth"
	enterpriseGraphQL "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/graphqlbackend"
	_ "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/registry"
)

func main() {
	shared.Main(enterpriseSetupHook)
}

var initFunctions = map[string]func(ctx context.Context, db dbutil.DB, outOfBandMigrationRunner *oobmigration.Runner, enterpriseServices *enterprise.Services) error{
	"authz":        authz.Init,
	"licensing":    licensing.Init,
	"executor":     executor.Init,
	"codeintel":    codeintel.Init,
	"insights":     insights.Init,
	"batches":      batches.InitFrontend,
	"codemonitors": codemonitors.Init,
}

func enterpriseSetupHook(db dbutil.DB, outOfBandMigrationRunner *oobmigration.Runner) enterprise.Services {
	debug, _ := strconv.ParseBool(os.Getenv("DEBUG"))
	if debug {
		log.Println("enterprise edition")
	}

	enterpriseGraphQL.InitDotcom(db)

	ctx := context.Background()
	enterpriseServices := enterprise.DefaultServices()

	for name, fn := range initFunctions {
		if err := fn(ctx, db, outOfBandMigrationRunner, &enterpriseServices); err != nil {
			log.Fatal(fmt.Sprintf("failed to initialize %s: %s", name, err))
		}
	}

	return enterpriseServices
}
