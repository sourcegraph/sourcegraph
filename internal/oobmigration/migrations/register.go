package migrations

import (
	"context"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/batches"
)

func RegisterOSSMigrators(ctx context.Context, db database.DB, runner *oobmigration.Runner) error {
	keyring := keyring.Default()

	return registerOSSMigrators(runner, migratorDependencies{
		store:   basestore.NewWithHandle(db.Handle()),
		keyring: &keyring,
	})
}

type StoreFactory interface {
	Store(ctx context.Context, schemaName string) (*basestore.Store, error)
}

func RegisterOSSMigratorsUsingConfAndStoreFactory(
	ctx context.Context,
	db database.DB,
	runner *oobmigration.Runner,
	conf conftypes.UnifiedQuerier,
	storeFactory StoreFactory,
) error {
	keys, err := keyring.NewRing(ctx, conf.SiteConfig().EncryptionKeys)
	if err != nil {
		return err
	}
	if keys == nil {
		keys = &keyring.Ring{}
	}

	return registerOSSMigrators(runner, migratorDependencies{
		store:   basestore.NewWithHandle(db.Handle()),
		keyring: keys,
	})
}

type migratorDependencies struct {
	store   *basestore.Store
	keyring *keyring.Ring
}

func registerOSSMigrators(runner *oobmigration.Runner, deps migratorDependencies) error {
	return RegisterAll(runner, []TaggedMigrator{
		batches.NewExternalServiceWebhookMigratorWithDB(deps.store, deps.keyring.ExternalServiceKey, 50),
	})
}

type TaggedMigrator interface {
	oobmigration.Migrator
	ID() int
	Interval() time.Duration
}

func RegisterAll(runner *oobmigration.Runner, migrators []TaggedMigrator) error {
	for _, migrator := range migrators {
		if err := runner.Register(
			migrator.ID(),
			migrator,
			oobmigration.MigratorOptions{Interval: migrator.Interval()},
		); err != nil {
			return err
		}
	}

	return nil
}
