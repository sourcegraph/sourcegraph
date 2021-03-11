package background

import (
	"time"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
)

// RegisterMigrations registers all currently implemented out of band migrations
// by batch changes with the migration runner.
func RegisterMigrations(cstore *store.Store, outOfBandMigrationRunner *oobmigration.Runner) error {
	migrations := map[int]oobmigration.Migrator{
		BatchChangesSSHMigrationID: &sshMigrator{store: cstore},
	}

	for id, migrator := range migrations {
		if err := outOfBandMigrationRunner.Register(id, migrator, oobmigration.MigratorOptions{Interval: 5 * time.Second}); err != nil {
			return err
		}
	}

	return nil
}
