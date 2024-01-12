package runner

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database/migration/definition"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (r *Runner) Run(ctx context.Context, options Options) error {
	if !options.PrivilegedMode.Valid() {
		return errors.Newf("invalid privileged mode")
	}

	if options.PrivilegedMode == NoopPrivilegedMigrations && options.MatchPrivilegedHash == nil {
		return errors.Newf("privileged hash matcher was not supplied")
	}

	schemaNames := make([]string, 0, len(options.Operations))
	for _, operation := range options.Operations {
		schemaNames = append(schemaNames, operation.SchemaName)
	}

	operationMap := make(map[string]MigrationOperation, len(options.Operations))
	for _, operation := range options.Operations {
		operationMap[operation.SchemaName] = operation
	}
	if len(operationMap) != len(options.Operations) {
		return errors.Newf("multiple operations defined on the same schema")
	}

	numRoutines := 1
	if options.Parallel {
		numRoutines = len(schemaNames)
	}
	semaphore := make(chan struct{}, numRoutines)

	return r.forEachSchema(ctx, schemaNames, func(ctx context.Context, schemaContext schemaContext) error {
		schemaName := schemaContext.schema.Name

		// Block until we can write into this channel. This ensures that we only have at most
		// the same number of active goroutines as we have slots in the channel's buffer.
		semaphore <- struct{}{}
		defer func() { <-semaphore }()

		if err := r.runSchema(
			ctx,
			operationMap[schemaName],
			schemaContext,
			options.PrivilegedMode,
			options.MatchPrivilegedHash,
			options.IgnoreSingleDirtyLog,
			options.IgnoreSinglePendingLog,
		); err != nil {
			return errors.Wrapf(err, "failed to run migration for schema %q", schemaName)
		}

		return nil
	})
}

// runSchema applies (or unapplies) the set of migrations required to fulfill the given operation. This
// method will attempt to coordinate with other concurrently running instances and may block while
// attempting to acquire a lock. An error is returned only if user intervention is deemed a necessity,
// the "dirty database" condition, or on context cancellation.
func (r *Runner) runSchema(
	ctx context.Context,
	operation MigrationOperation,
	schemaContext schemaContext,
	privilegedMode PrivilegedMode,
	matchPrivilegedHash func(hash string) bool,
	ignoreSingleDirtyLog bool,
	ignoreSinglePendingLog bool,
) error {
	// First, rewrite operations into a smaller set of operations we'll handle below. This call converts
	// upgrade and revert operations into targeted up and down operations.
	operation, err := desugarOperation(schemaContext, operation)
	if err != nil {
		return err
	}

	gatherDefinitions := schemaContext.schema.Definitions.Up
	if operation.Type != MigrationOperationTypeTargetedUp {
		gatherDefinitions = schemaContext.schema.Definitions.Down
	}

	// Get the set of migrations that need to be applied or unapplied, depending on the migration direction.
	definitions, err := gatherDefinitions(schemaContext.initialSchemaVersion.appliedVersions, operation.TargetVersions)
	if err != nil {
		return err
	}

	// Filter out any unlisted migrations (most likely future upgrades) and group them by status.
	byState := groupByState(schemaContext.initialSchemaVersion, definitions)

	logger := r.logger.With(
		log.String("schema", schemaContext.schema.Name),
	)

	logger.Info("Checked current schema state",
		log.Ints("appliedVersions", extractIDs(byState.applied)),
		log.Ints("pendingVersions", extractIDs(byState.pending)),
		log.Ints("failedVersions", extractIDs(byState.failed)))

	// Before we commit to performing an upgrade (which takes locks), determine if there is anything to do
	// and early out if not. We'll no-op if there are no definitions with pending or failed attempts, and
	// all migrations are applied (when migrating up) or unapplied (when migrating down).

	if len(byState.pending)+len(byState.failed) == 0 {
		if operation.Type == MigrationOperationTypeTargetedUp && len(byState.applied) == len(definitions) {
			logger.Info("Schema is in the expected state")
			return nil
		}

		if operation.Type == MigrationOperationTypeTargetedDown && len(byState.applied) == 0 {
			logger.Info("Schema is in the expected state")
			return nil
		}
	}

	logger.Info("Schema is not in the expected state - applying migration delta",
		log.Ints("targetDefinitions", extractIDs(definitions)),
		log.Ints("appliedVersions", extractIDs(byState.applied)),
		log.Ints("pendingVersions", extractIDs(byState.pending)),
		log.Ints("failedVersions", extractIDs(byState.failed)),
	)

	for {
		// Attempt to apply as many migrations as possible. We do this iteratively in chunks as we are unable
		// to hold a consistent advisory lock in the presence of migrations utilizing concurrent index creation.
		// Therefore, some invocations of this method will return with a flag to request re-invocation under a
		// new lock.

		if retry, err := r.applyMigrations(
			ctx,
			operation,
			schemaContext,
			definitions,
			privilegedMode,
			matchPrivilegedHash,
			ignoreSingleDirtyLog,
			ignoreSinglePendingLog,
		); err != nil {
			return err
		} else if !retry {
			break
		}
	}

	logger.Info("Schema is in the expected state")
	return nil
}

// applyMigrations attempts to take an advisory lock, then re-checks the version of the database. If there are
// still migrations to apply from the given definitions, they are applied in-order. If not all definitions are
// applied, this method returns a true-valued flag indicating that it should be re-invoked to attempt to apply
// the remaining definitions.
func (r *Runner) applyMigrations(
	ctx context.Context,
	operation MigrationOperation,
	schemaContext schemaContext,
	definitions []definition.Definition,
	privilegedMode PrivilegedMode,
	matchPrivilegedHash func(hash string) bool,
	ignoreSingleDirtyLog bool,
	ignoreSinglePendingLog bool,
) (retry bool, _ error) {
	var droppedLock bool
	up := operation.Type == MigrationOperationTypeTargetedUp

	callback := func(schemaVersion schemaVersion, _ definitionsByState, earlyUnlock unlockFunc) error {
		// Filter the set of definitions we still need to apply given our new view of the schema
		definitions := filterAppliedDefinitions(schemaVersion, operation, definitions)
		if len(definitions) == 0 {
			// Stop retry loop
			return nil
		}

		r.logger.Info(
			"Applying migrations",
			log.String("schema", schemaContext.schema.Name),
			log.Bool("up", up),
			log.Int("count", len(definitions)),
		)

		// Print a warning message or block the application of privileged migrations, depending on the
		// flags specified by the user. A nil error value returned here indicates that application of
		// each migration file can proceed.

		if err := r.checkPrivilegedState(operation, schemaContext, definitions, privilegedMode, matchPrivilegedHash); err != nil {
			return err
		}

		for _, def := range definitions {
			if up && def.IsCreateIndexConcurrently {
				// Handle execution of `CREATE INDEX CONCURRENTLY` specially
				if unlocked, err := r.createIndexConcurrently(ctx, schemaContext, def, earlyUnlock); err != nil {
					return err
				} else if unlocked {
					// We've forfeited our lock, but want to continue applying the remaining migrations (if any).
					// Setting this value here sends us back to the caller to be re-invoked.
					droppedLock = true
					return nil
				}
			} else {
				// Apply all other types of migrations uniformly
				if err := r.applyMigration(ctx, schemaContext, operation, def, privilegedMode); err != nil {
					return err
				}
			}
		}

		// Stop retry loop
		return nil
	}

	if retry, err := r.withLockedSchemaState(
		ctx,
		schemaContext,
		definitions,
		ignoreSingleDirtyLog,
		ignoreSinglePendingLog,
		callback,
	); err != nil {
		return false, err
	} else if retry {
		// There are active index creation operations ongoing; wait a short time before requerying
		// the state of the migrations so we don't flood the database with constant queries to the
		// system catalog. We check here instead of in the caller because we don't want a delay when
		// we drop the lock to create an index concurrently (returning `droppedLock = true` below).
		return true, wait(ctx, indexPollInterval)
	}

	return droppedLock, nil
}

// checkPrivilegedState determines if we should fail-fast or print a warning about privileged migration
// behavior given the set of definitions to apply.
func (r *Runner) checkPrivilegedState(
	operation MigrationOperation,
	schemaContext schemaContext,
	definitions []definition.Definition,
	privilegedMode PrivilegedMode,
	matchPrivilegedHash func(hash string) bool,
) error {
	up := operation.Type == MigrationOperationTypeTargetedUp

	if privilegedMode == ApplyPrivilegedMigrations || (privilegedMode == RefusePrivilegedMigrations && !up) {
		// We will either apply all migrations, or we are downgrading and do not want to
		// fail-fast as the user is not expected to front-load the removal of extensions,
		// which could trivially break down migrations defined after the inclusion of the
		// extension. In the latter case, we want to fail only at the point where the down
		// migration can be safely applied.
		return nil
	}

	// Gather only the privileged definitions
	privilegedDefinitions := make([]definition.Definition, 0, len(definitions))
	for _, def := range definitions {
		if def.Privileged {
			privilegedDefinitions = append(privilegedDefinitions, def)
		}
	}
	if len(privilegedDefinitions) == 0 {
		// All migrations are unprivileged
		return nil
	}

	// Extract IDs from privileged definitions
	privilegedDefinitionIDs := make([]int, 0, len(privilegedDefinitions))
	for _, def := range privilegedDefinitions {
		privilegedDefinitionIDs = append(privilegedDefinitionIDs, def.ID)
	}

	if privilegedMode == RefusePrivilegedMigrations {
		// The condition at the top of this function ensures that we're migrating up. In
		// this case, we want to fail-fast and alert the user that they should run a set
		// of privileged migrations manually before proceeding.
		return newPrivilegedMigrationError(operation.SchemaName, privilegedDefinitionIDs...)
	}

	if privilegedMode == NoopPrivilegedMigrations {
		// The user has enabled a mode where we assume the contents of the privileged migrations
		// have already been applied, or in the down direction will be applied after this operation.

		if privilegedHash := hashDefinitionIDs(privilegedDefinitionIDs); !matchPrivilegedHash(privilegedHash) && up {
			// In order to ensure the user reads the following instructions for this operation, we
			// fail-fast equivalently to the -unprivileged-only case when a hash of the privileged
			// migrations to-be-applied is not also supplied.

			return errors.Newf(
				"refusing to apply a privileged migration: apply the following SQL and re-run with the added flag `-privileged-hash=%s` to continue.\n\n```\n%s\n```\n",
				privilegedHash,
				concatenateSQL(privilegedDefinitions, up),
			)
		}

		message := "The migrator assumes that the following SQL queries have already been applied. Failure to have done so may cause the following operation to fail."
		if !up {
			message = "The following SQL queries must be applied after the downgrade operation is complete."
		}

		r.logger.Warn(
			message,
			log.String("schema", schemaContext.schema.Name),
			log.String("sql", concatenateSQL(privilegedDefinitions, up)),
		)
	}

	return nil
}

// applyMigration applies the given migration in the direction indicated by the given operation.
func (r *Runner) applyMigration(
	ctx context.Context,
	schemaContext schemaContext,
	operation MigrationOperation,
	definition definition.Definition,
	privilegedMode PrivilegedMode,
) error {
	up := operation.Type == MigrationOperationTypeTargetedUp

	if definition.Privileged {
		if privilegedMode == RefusePrivilegedMigrations {
			return newPrivilegedMigrationError(operation.SchemaName, definition.ID)
		}

		if privilegedMode == NoopPrivilegedMigrations {
			noop := func() error {
				return nil
			}
			if err := schemaContext.store.WithMigrationLog(ctx, definition, up, noop); err != nil {
				return errors.Wrapf(err, "failed to create migration log %d", definition.ID)
			}

			r.logger.Warn(
				"Adding migrating log for privileged migration, but not applying its changes",
				log.String("schema", schemaContext.schema.Name),
				log.Int("migrationID", definition.ID),
				log.Bool("up", up),
			)

			return nil
		}
	}

	r.logger.Info(
		"Applying migration",
		log.String("schema", schemaContext.schema.Name),
		log.Int("migrationID", definition.ID),
		log.Bool("up", up),
	)

	applyMigration := func() (err error) {
		tx := schemaContext.store

		if !definition.IsCreateIndexConcurrently {
			tx, err = schemaContext.store.Transact(ctx)
			if err != nil {
				return err
			}
			defer func() { err = tx.Done(err) }()
		}

		if up {
			if err := tx.Up(ctx, definition); err != nil {
				return errors.Wrapf(err, "failed to apply migration %d:\n```\n%s\n```\n", definition.ID, definition.UpQuery.Query(sqlf.PostgresBindVar))
			}
		} else {
			if err := tx.Down(ctx, definition); err != nil {
				return errors.Wrapf(err, "failed to apply migration %d:\n```\n%s\n```\n", definition.ID, definition.DownQuery.Query(sqlf.PostgresBindVar))
			}
		}

		return nil
	}
	return schemaContext.store.WithMigrationLog(ctx, definition, up, applyMigration)
}

const indexPollInterval = time.Second * 5

// createIndexConcurrently deals with the special case of `CREATE INDEX CONCURRENTLY` migrations. We cannot
// hold an advisory lock during concurrent index creation without trivially deadlocking concurrent migrator
// instances (see `internal/database/migration/store/store_test.go:TestIndexStatus` for an example). Instead,
// we use Postgres system tables to determine the status of the index being created and re-issue the index
// creation command if it's missing or invalid.
//
// If the given `unlock` function is called then `unlocked` will be true on return. This allows the caller
// to maintain the lock in the case that the index already exists due to an out-of-band operation.
func (r *Runner) createIndexConcurrently(
	ctx context.Context,
	schemaContext schemaContext,
	definition definition.Definition,
	unlock func(err error) error,
) (unlocked bool, _ error) {
	tableName := definition.IndexMetadata.TableName
	indexName := definition.IndexMetadata.IndexName

pollIndexStatusLoop:
	for {
		// Query the current status of the target index
		indexStatus, exists, err := getAndLogIndexStatus(ctx, schemaContext, tableName, indexName)
		if err != nil {
			return false, errors.Wrap(err, "failed to query state of index")
		}

		if exists && indexStatus.IsValid {
			// Index exists and is valid; nothing to do. We'll return here, but we need to ensure
			// we add a migration log here before moving on.
			//
			// This was a particular problem when we would create an index concurrently on DotCom
			// ahead of a merge+rollout to confirm expected performance changes. When the migrator
			// runs, it sees a valid index and exits without adding a log. This causes the frontend
			// to fail as it's still missing proof that the index's migration was ran.
			//
			// This doesn't happen normally, where the migration log is missing AND the index does
			// not yet exist (or is invalid). This may have affected customers that have previously
			// downgraded.
			noop := func() error {
				return nil
			}
			if err := schemaContext.store.WithMigrationLog(ctx, definition, true, noop); err != nil {
				return false, errors.Wrapf(err, "failed to create migration log %d", definition.ID)
			}

			return unlocked, nil
		}

		if exists && indexStatus.Phase == nil {
			// Index is invalid but no creation operation is in-progress. We can try to repair this
			// state automatically by dropping the index and re-create it as if it never existed.
			// Assuming that the down migration drops the index created in the up direction, we'll
			// just apply that. We open a (hopefully) short-lived transaction here to drop the
			// existing index and write the migration log entry in the same shot.

			tx, err := schemaContext.store.Transact(ctx)
			if err != nil {
				return false, err
			}

			dropIndex := func() error {
				return tx.Down(ctx, definition)
			}
			if err := tx.WithMigrationLog(ctx, definition, false, dropIndex); err != nil {
				// Ensure we don't leak txn on error here
				return false, tx.Done(err)
			}

			// Close transaction immediately after use instead of deferring from in the loop
			if err := tx.Done(nil); err != nil {
				return false, err
			}
		}

		// Release advisory lock before attempting to create index or wait on the the index creation
		// operation. Concurrent index creation works in several distinct phases. One of those phases
		// requires that all existant transactions finish. If we hold an advisory lock in this session
		// that blocks another transaction, the index creation operation will deadlock and fail.
		//
		// Note that we assume idempotency on this unlock function.
		if err := unlock(nil); err != nil {
			return false, err
		}
		unlocked = true

		// Index is currently being created. Wait a small time and check the index status again. We don't
		// want to take any action here while the other proceses is working.
		if exists && indexStatus.Phase != nil {
			if err := wait(ctx, indexPollInterval); err != nil {
				return true, err
			}

			continue pollIndexStatusLoop
		}

		// Create the index. Ignore duplicate table/index already exists errors. This can happen if there
		// is a race between two migrator instances fighting to create the same index. Just silence the
		// error within the `createIndex` function (so we prevent a spurious migration log failure entry)
		// and set a flag indicating a to retry the operation. We retry instead of returning so that we
		// do not prematurely begin to apply the next migration, which may assume the successful creation
		// of the index.

		var (
			pgErr        *pgconn.PgError
			raceDetected bool

			errorFilter = func(err error) error {
				if err == nil {
					return err
				}
				if !errors.As(err, &pgErr) || pgErr.Code != "42P07" {
					return err
				}

				raceDetected = true
				return nil
			}
		)

		r.logger.Info(
			"Creating index concurrently",
			log.String("schema", schemaContext.schema.Name),
			log.Int("migrationID", definition.ID),
			log.String("tableName", tableName),
			log.String("indexName", indexName),
		)

		createIndex := func() error {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			go func() {
				for {
					if err := wait(ctx, indexPollInterval); err != nil {
						return
					}

					if _, _, err := getAndLogIndexStatus(ctx, schemaContext, tableName, indexName); err != nil {
						r.logger.Error("Failed to retrieve index status", log.Error(err))
					}
				}
			}()

			return errorFilter(schemaContext.store.Up(ctx, definition))
		}
		if err := schemaContext.store.WithMigrationLog(ctx, definition, true, createIndex); err != nil {
			return false, err
		} else if raceDetected {
			continue
		}

		return true, nil
	}
}

// filterAppliedDefinitions returns a subset of the given definition slice. A definition will be included
// in the resulting slice if we're migrating up and the migration is not applied, or if we're migrating down
// and the migration is applied.
//
// The resulting slice will have the same relative order as the input slice. This function does not alter
// the input slice.
func filterAppliedDefinitions(
	schemaVersion schemaVersion,
	operation MigrationOperation,
	definitions []definition.Definition,
) []definition.Definition {
	up := operation.Type == MigrationOperationTypeTargetedUp
	appliedVersionMap := intSet(schemaVersion.appliedVersions)

	filtered := make([]definition.Definition, 0, len(definitions))
	for _, def := range definitions {
		if _, ok := appliedVersionMap[def.ID]; ok == up {
			// Either
			// - needs to be applied and already applied, or
			// - needs to be unapplied and not currently applied.
			continue
		}

		filtered = append(filtered, def)
	}

	return filtered
}

// concatenateSQL renders and concatenates the query text of each of the given migration definitions,
// depending on the given migration direction. The output will wrap the concatenated SQL in a single
// transaction, and the source of each query will be identified via a SQL comment.
func concatenateSQL(definitions []definition.Definition, up bool) string {
	migrationContents := make([]string, 0, len(definitions))
	for _, def := range definitions {
		migrationContents = append(migrationContents, fmt.Sprintf("-- Migration %d\n%s\n", def.ID, strings.TrimSpace(renderQuery(def, up))))
	}

	return fmt.Sprintf("BEGIN;\n\n%s\nCOMMIT;\n", strings.Join(migrationContents, "\n"))
}

// renderQuery returns the string representation of the definition's SQL query.
func renderQuery(definition definition.Definition, up bool) string {
	query := definition.UpQuery
	if !up {
		query = definition.DownQuery
	}

	return query.Query(sqlf.PostgresBindVar)
}

// hashDefinitionIDs returns a deterministic hash of the given definition IDs.
func hashDefinitionIDs(ids []int) string {
	hasher := sha1.New()
	hasher.Write([]byte(strings.Join(intsToStrings(ids), ",")))
	return base64.StdEncoding.EncodeToString(hasher.Sum(nil))
}
