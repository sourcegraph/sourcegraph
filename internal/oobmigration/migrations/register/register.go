package register

import (
	"context"
	"database/sql"
	"time"

	"github.com/derision-test/glock"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	connections "github.com/sourcegraph/sourcegraph/internal/database/connections/live"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	internalinsights "github.com/sourcegraph/sourcegraph/internal/insights"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/batches"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/codehosts"
	lsifMigrations "github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/codeintel/lsif"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/iam"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/insights"
	insightsBackfiller "github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/insights/backfillv2"
	insightsrecordingtimes "github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/insights/recording_times"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func RegisterOSSMigrators(ctx context.Context, db database.DB, runner *oobmigration.Runner) error {
	defaultKeyring := keyring.Default()

	return registerOSSMigrators(runner, false, migratorDependencies{
		store:   basestore.NewWithHandle(db.Handle()),
		keyring: &defaultKeyring,
	})
}

func RegisterOSSMigratorsUsingConfAndStoreFactory(
	ctx context.Context,
	db database.DB,
	runner *oobmigration.Runner,
	conf conftypes.UnifiedQuerier,
	_ migrations.StoreFactory,
) error {
	keys, err := keyring.NewRing(ctx, conf.SiteConfig().EncryptionKeys)
	if err != nil {
		return err
	}
	if keys == nil {
		keys = &keyring.Ring{}
	}

	return registerOSSMigrators(runner, true, migratorDependencies{
		store:   basestore.NewWithHandle(db.Handle()),
		keyring: keys,
	})
}

type migratorDependencies struct {
	store   *basestore.Store
	keyring *keyring.Ring
}

func registerOSSMigrators(runner *oobmigration.Runner, noDelay bool, deps migratorDependencies) error {
	return RegisterAll(runner, noDelay, []TaggedMigrator{
		batches.NewExternalServiceWebhookMigratorWithDB(deps.store, deps.keyring.ExternalServiceKey, 50),
		batches.NewUserRoleAssignmentMigrator(deps.store, 250),
		codehosts.NewMigratorWithDB(deps.store, deps.keyring.ExternalServiceKey),
	})
}

type TaggedMigrator interface {
	oobmigration.Migrator
	ID() int
	Interval() time.Duration
}

func RegisterAll(runner *oobmigration.Runner, noDelay bool, migrators []TaggedMigrator) error {
	for _, migrator := range migrators {
		options := oobmigration.MigratorOptions{Interval: migrator.Interval()}
		if noDelay {
			options.Interval = time.Nanosecond
		}

		if err := runner.Register(migrator.ID(), migrator, options); err != nil {
			return err
		}
	}

	return nil
}

func RegisterEnterpriseMigrators(ctx context.Context, db database.DB, runner *oobmigration.Runner) error {
	codeIntelDB, err := initCodeintelDB(&observation.TestContext)
	if err != nil {
		return err
	}

	var insightsStore *basestore.Store
	if internalinsights.IsEnabled() {
		insightsDB, err := initCodeInsightsDB(&observation.TestContext)
		if err != nil {
			return err
		}

		insightsStore = basestore.NewWithHandle(basestore.NewHandleWithDB(log.NoOp(), insightsDB, sql.TxOptions{}))
	}

	defaultKeyring := keyring.Default()

	return registerEnterpriseMigrators(runner, false, dependencies{
		store:          basestore.NewWithHandle(db.Handle()),
		codeIntelStore: basestore.NewWithHandle(basestore.NewHandleWithDB(log.NoOp(), codeIntelDB, sql.TxOptions{})),
		insightsStore:  insightsStore,
		keyring:        &defaultKeyring,
	})
}

func RegisterEnterpriseMigratorsUsingConfAndStoreFactory(
	ctx context.Context,
	db database.DB,
	runner *oobmigration.Runner,
	conf conftypes.UnifiedQuerier,
	storeFactory migrations.StoreFactory,
) error {
	codeIntelStore, err := storeFactory.Store(ctx, "codeintel")
	if err != nil {
		return err
	}
	insightsStore, err := storeFactory.Store(ctx, "codeinsights")
	if err != nil {
		return err
	}

	keys, err := keyring.NewRing(ctx, conf.SiteConfig().EncryptionKeys)
	if err != nil {
		return err
	}
	if keys == nil {
		keys = &keyring.Ring{}
	}

	return registerEnterpriseMigrators(runner, true, dependencies{
		store:          basestore.NewWithHandle(db.Handle()),
		codeIntelStore: codeIntelStore,
		insightsStore:  insightsStore,
		keyring:        keys,
	})
}

type dependencies struct {
	store          *basestore.Store
	codeIntelStore *basestore.Store
	insightsStore  *basestore.Store
	keyring        *keyring.Ring
}

func registerEnterpriseMigrators(runner *oobmigration.Runner, noDelay bool, deps dependencies) error {
	migrators := []TaggedMigrator{
		iam.NewSubscriptionAccountNumberMigrator(deps.store, 500),
		iam.NewLicenseKeyFieldsMigrator(deps.store, 500),
		iam.NewUnifiedPermissionsMigrator(deps.store),
		batches.NewSSHMigratorWithDB(deps.store, deps.keyring.BatchChangesCredentialKey, 5),
		batches.NewExternalForkNameMigrator(deps.store, 500),
		batches.NewEmptySpecIDMigrator(deps.store),
		lsifMigrations.NewDiagnosticsCountMigrator(deps.codeIntelStore, 1000, 0),
		lsifMigrations.NewDefinitionLocationsCountMigrator(deps.codeIntelStore, 1000, 0),
		lsifMigrations.NewReferencesLocationsCountMigrator(deps.codeIntelStore, 1000, 0),
		lsifMigrations.NewDocumentColumnSplitMigrator(deps.codeIntelStore, 100, 0),
		lsifMigrations.NewSCIPMigrator(deps.store, deps.codeIntelStore),
	}
	if deps.insightsStore != nil {
		migrators = append(migrators,
			insights.NewMigrator(deps.store, deps.insightsStore),
			insightsrecordingtimes.NewRecordingTimesMigrator(deps.insightsStore, 500),
			insightsBackfiller.NewMigrator(deps.insightsStore, glock.NewRealClock(), 10),
		)
	}
	return RegisterAll(runner, noDelay, migrators)
}

func initCodeintelDB(observationCtx *observation.Context) (*sql.DB, error) {
	dsn := conf.GetServiceConnectionValueAndRestartOnChange(func(serviceConnections conftypes.ServiceConnections) string {
		return serviceConnections.CodeIntelPostgresDSN
	})

	db, err := connections.EnsureNewCodeIntelDB(observationCtx, dsn, "oobmigration")
	if err != nil {
		return nil, errors.Errorf("failed to connect to codeintel database: %s", err)
	}

	return db, nil
}

func initCodeInsightsDB(observationCtx *observation.Context) (*sql.DB, error) {
	dsn := conf.GetServiceConnectionValueAndRestartOnChange(func(serviceConnections conftypes.ServiceConnections) string {
		return serviceConnections.CodeInsightsDSN
	})

	db, err := connections.EnsureNewCodeInsightsDB(observationCtx, dsn, "oobmigration")
	if err != nil {
		return nil, errors.Errorf("failed to connect to codeinsights database: %s", err)
	}

	return db, nil
}
