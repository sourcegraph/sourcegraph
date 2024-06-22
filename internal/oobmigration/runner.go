package oobmigration

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/derision-test/glock"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Runner correlates out-of-band migration records in the database with a migrator instance,
// and will run each migration that has no yet completed: either reached 100% in the forward
// direction or 0% in the reverse direction.
type Runner struct {
	store         storeIface
	logger        log.Logger
	refreshTicker glock.Ticker
	operations    *operations
	migrators     map[int]migratorAndOption
	ctx           context.Context    // root context passed to the handler
	cancel        context.CancelFunc // cancels the root context
	finished      chan struct{}      // signals that Start has finished
}

type migratorAndOption struct {
	Migrator
	migratorOptions
}

func NewRunnerWithDB(observationCtx *observation.Context, db database.DB, refreshInterval time.Duration) *Runner {
	return NewRunner(observationCtx, NewStoreWithDB(db), refreshInterval)
}

func NewRunner(observationCtx *observation.Context, store *Store, refreshInterval time.Duration) *Runner {
	return newRunner(observationCtx, &storeShim{store}, glock.NewRealTicker(refreshInterval))
}

func newRunner(observationCtx *observation.Context, store storeIface, refreshTicker glock.Ticker) *Runner {
	// IMPORTANT: actor.WithInternalActor prevents issues caused by
	// database-level authz checks: migration tasks should always be
	// privileged.
	ctx, cancel := context.WithCancel(actor.WithInternalActor(context.Background()))

	return &Runner{
		store:         store,
		logger:        observationCtx.Logger.Scoped("oobmigration"),
		refreshTicker: refreshTicker,
		operations:    newOperations(observationCtx),
		migrators:     map[int]migratorAndOption{},
		ctx:           ctx,
		cancel:        cancel,
		finished:      make(chan struct{}),
	}
}

// MigratorOptions configures the behavior of a registered migrator.
type MigratorOptions struct {
	// Interval specifies the time between invocations of an active migration.
	Interval time.Duration

	// ticker mocks periodic behavior for tests.
	ticker glock.Ticker
}

func (r *Runner) SynchronizeMetadata(ctx context.Context) error {
	return r.store.SynchronizeMetadata(ctx)
}

// Register correlates the given migrator with the given migration identifier. An error is
// returned if a migrator is already associated with this migration.
func (r *Runner) Register(id int, migrator Migrator, options MigratorOptions) error {
	if _, ok := r.migrators[id]; ok {
		return errors.Newf("migrator %d already registered", id)
	}

	if options.Interval == 0 {
		options.Interval = time.Second
	}
	if options.ticker == nil {
		options.ticker = glock.NewRealTicker(options.Interval)
	}

	r.migrators[id] = migratorAndOption{migrator, migratorOptions{
		ticker: options.ticker,
	}}
	return nil
}

type migrationStatusError struct {
	id               int
	expectedProgress float64
	actualProgress   float64
}

func newMigrationStatusError(id int, expectedProgress, actualProgress float64) error {
	return migrationStatusError{
		id:               id,
		expectedProgress: expectedProgress,
		actualProgress:   actualProgress,
	}
}

func (e migrationStatusError) Error() string {
	return fmt.Sprintf("migration %d expected to be at %.2f%% (at %.2f%%)", e.id, e.expectedProgress*100, e.actualProgress*100)
}

// Validate checks the migration records present in the database (including their progress) and returns
// an error if there are unfinished migrations relative to the given version. Specifically, it is illegal
// for a Sourcegraph instance to start up with a migration that has one of the following properties:
//
// - A migration with progress != 0 is introduced _after_ the given version
// - A migration with progress != 1 is deprecated _on or before_ the given version
//
// This error is used to block startup of the application with an informative message indicating that
// the site admin must either (1) run the previous version of Sourcegraph longer to allow the unfinished
// migrations to complete in the case of a premature upgrade, or (2) run a standalone migration utility
// to rewind changes on an unmoving database in the case of a premature downgrade.
func (r *Runner) Validate(ctx context.Context, currentVersion, firstVersion Version) error {
	migrations, err := r.store.List(ctx)
	if err != nil {
		return err
	}

	errs := make([]error, 0, len(migrations))
	for _, migration := range migrations {
		currentVersionCmpIntroduced := CompareVersions(currentVersion, migration.Introduced)
		if currentVersionCmpIntroduced == VersionOrderBefore {
			migration.ApplyReverse = true
			progress, err := updateProgress(ctx, r.store, &migration, r.migrators[migration.ID].Migrator)
			if err != nil {
				errs = append(errs, errors.Newf("failed to get progress status for migration %d", migration.ID))
				continue
			}
			if progress != 0 {
				// Unfinished rollback: currentVersion before introduced version and progress > 0
				errs = append(errs, newMigrationStatusError(migration.ID, 0, migration.Progress))
			}
		}

		if migration.Deprecated == nil {
			continue
		}

		firstVersionCmpDeprecated := CompareVersions(firstVersion, *migration.Deprecated)
		if firstVersionCmpDeprecated != VersionOrderBefore {
			// Edge case: sourcegraph instance booted on or after deprecation version
			continue
		}

		currentVersionCmpDeprecated := CompareVersions(currentVersion, *migration.Deprecated)
		if currentVersionCmpDeprecated != VersionOrderBefore {
			migration.ApplyReverse = false
			progress, err := updateProgress(ctx, r.store, &migration, r.migrators[migration.ID].Migrator)
			if err != nil {
				errs = append(errs, errors.Newf("failed to get progress status for migration %d", migration.ID))
				continue
			}
			if progress != 1 {
				// Unfinished migration: currentVersion on or after deprecated version, progress < 1
				errs = append(errs, newMigrationStatusError(migration.ID, 1, migration.Progress))
			}
		}
	}

	return wrapMigrationErrors(errs...)
}

func wrapMigrationErrors(errs ...error) error {
	if len(errs) == 0 {
		return nil
	}

	descriptions := make([]string, 0, len(errs))
	for _, err := range errs {
		descriptions = append(descriptions, fmt.Sprintf("  - %s\n", err))
	}
	sort.Strings(descriptions)

	return errors.Errorf(
		"Unfinished migrations. Please revert Sourcegraph to the previous version and wait for the following migrations to complete.\n\n%s\n",
		strings.Join(descriptions, "\n"),
	)
}

// UpdateDirection sets the direction for each of the given migrations atomically.
func (r *Runner) UpdateDirection(ctx context.Context, ids []int, applyReverse bool) (err error) {
	tx, err := r.store.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	for _, id := range ids {
		if err := tx.UpdateDirection(ctx, id, applyReverse); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) Name() string {
	return "oobmigrationRunner"
}

// Start runs registered migrators on a loop until they complete. This method will periodically
// re-read from the database in order to refresh its current view of the migrations.
func (r *Runner) Start(currentVersion, firstVersion Version) {
	r.startInternal(func(migration Migration) bool {
		if CompareVersions(currentVersion, migration.Introduced) == VersionOrderBefore {
			// current version before migration introduction
			return false
		}

		if migration.Deprecated != nil && CompareVersions(firstVersion, *migration.Deprecated) == VersionOrderAfter {
			// instance initialized after migration deprecation
			return false
		}

		// migration not yet deprecated or current version is before deprecated version
		return migration.Deprecated == nil || CompareVersions(currentVersion, *migration.Deprecated) == VersionOrderBefore
	})
}

// StartPartial runs registered migrators matching one of the given identifiers on a loop until
// they complete. This method will periodically re-read from the database in order to refresh its
// current view of the migrations. When the given set of identifiers is empty, all migrations in
// the database with a registered migrator will be considered active.
func (r *Runner) StartPartial(ids []int) {
	idMap := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		idMap[id] = struct{}{}
	}

	r.startInternal(func(m Migration) bool {
		_, ok := idMap[m.ID]
		return ok
	})
}

func (r *Runner) startInternal(shouldRunMigration func(m Migration) bool) {
	defer close(r.finished)

	ctx := r.ctx
	var wg sync.WaitGroup
	migrationProcesses := map[int]chan Migration{}

	// Periodically read the complete set of out-of-band migrations from the database
	for migrations := range r.listMigrations(ctx) {
		for i := range migrations {
			migration := migrations[i]
			migrator, ok := r.migrators[migration.ID]
			if !ok {
				continue
			}

			if !shouldRunMigration(migration) {
				continue
			}

			// Ensure we have a migration routine running for this migration
			r.ensureProcessorIsRunning(&wg, migrationProcesses, migration.ID, func(ch <-chan Migration) {
				runMigrator(ctx, r.store, migrator.Migrator, ch, migrator.migratorOptions, r.logger, r.operations)
			})

			// Send the new migration to the processor routine. This loop guarantees
			// that either (1) the routine can immediately write the new value into the
			// free buffer slot, in which case we immediately break; (2) the routine
			// cannot immediately write because the buffer slot is full with a migration
			// value that is comparatively out of date.
			//
			// In this second case we'll read from the channel to free the buffer slot
			// of the old value, then write our new value there.
			//
			// Note: This loop breaks after two iterations (at most).
		loop:
			for {
				select {
				case migrationProcesses[migration.ID] <- migrations[i]:
					break loop
				case <-migrationProcesses[migration.ID]:
				}
			}
		}
	}

	// Unblock all processor routines
	for _, ch := range migrationProcesses {
		close(ch)
	}

	// Wait for processor routines to finish
	wg.Wait()
}

// listMigrations returns a channel that will asynchronously receive the full list of out-of-band
// migrations that exist in the database. This channel will receive a value periodically as long
// as the given context is active.
func (r *Runner) listMigrations(ctx context.Context) <-chan []Migration {
	ch := make(chan []Migration)

	go func() {
		defer close(ch)

		for {
			migrations, err := r.store.List(ctx)
			if err != nil {
				if !errors.Is(err, ctx.Err()) {
					r.logger.Error("Failed to list out-of-band migrations", log.Error(err))
				}
			} else {
				select {
				case ch <- migrations:
				case <-ctx.Done():
					return
				}
			}

			select {
			case <-r.refreshTicker.Chan():
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

// ensureProcessorIsRunning ensures that there is a non-nil channel at m[id]. If this key
// is not set, a new channel is created and stored in this key. The channel is then passed
// to runMigrator in a goroutine.
//
// This method logs the execution of the migration processor in the given wait group.
func (r *Runner) ensureProcessorIsRunning(wg *sync.WaitGroup, m map[int]chan Migration, id int, runMigrator func(<-chan Migration)) {
	if _, ok := m[id]; ok {
		return
	}

	wg.Add(1)
	ch := make(chan Migration, 1)
	m[id] = ch

	go func() {
		runMigrator(ch)
		wg.Done()
	}()
}

// Stop will cancel the context used in Start, then blocks until Start has returned.
func (r *Runner) Stop(context.Context) error {
	r.cancel()
	<-r.finished
	return nil
}

type migratorOptions struct {
	ticker glock.Ticker
}

// runMigrator runs the given migrator function periodically (on each read from ticker)
// while the migration is not complete. We will periodically (on each read from migrations)
// update our current view of the migration progress and (more importantly) its direction.
func runMigrator(ctx context.Context, store storeIface, migrator Migrator, migrations <-chan Migration, options migratorOptions, logger log.Logger, operations *operations) {
	// Get initial migration. This channel will close when the context
	// is canceled, so we don't need to do any more complex select here.
	migration, ok := <-migrations
	if !ok {
		return
	}

	// We're just starting up - refresh our progress before migrating
	if _, err := updateProgress(ctx, store, &migration, migrator); err != nil {
		if !errors.Is(err, ctx.Err()) {
			logger.Error("Failed to determine migration progress", log.Error(err), log.Int("migrationID", migration.ID))
		}
	}

	for {
		select {
		case migration, ok = <-migrations:
			if !ok {
				return
			}

			// We just got a new version of the migration from the database. We need to check
			// the actual progress based on the migrator in case the progress as stored in the
			// migrations table has been de-synchronized from the actual progress.
			if _, err := updateProgress(ctx, store, &migration, migrator); err != nil {
				if !errors.Is(err, ctx.Err()) {
					logger.Error("Failed to determine migration progress", log.Error(err), log.Int("migrationID", migration.ID))
				}
			}

		case <-options.ticker.Chan():
			if !migration.Complete() {
				// Run the migration only if there's something left to do
				if err := runMigrationFunction(ctx, store, &migration, migrator, logger, operations); err != nil {
					if !errors.Is(err, ctx.Err()) {
						logger.Error("Failed migration action", log.Error(err), log.Int("migrationID", migration.ID))
					}
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

// runMigrationFunction invokes the Up or Down method on the given migrator depending on the migration
// direction. If an error occurs, it will be associated in the database with the migration record.
// Regardless of the success of the migration function, the progress function on the migrator will be
// invoked and the progress written to the database.
func runMigrationFunction(ctx context.Context, store storeIface, migration *Migration, migrator Migrator, logger log.Logger, operations *operations) error {
	migrationFunc := runMigrationUp
	if migration.ApplyReverse {
		migrationFunc = runMigrationDown
	}

	if migrationErr := migrationFunc(ctx, migration, migrator, logger, operations); migrationErr != nil {
		if !errors.Is(migrationErr, ctx.Err()) {
			logger.Error("Failed to perform migration", log.Error(migrationErr), log.Int("migrationID", migration.ID))
		}

		// Migration resulted in an error. All we'll do here is add this error to the migration's error
		// message list. Unless _that_ write to the database fails, we'll continue along the happy path
		// in order to update the migration, which could have made additional progress before failing.

		if err := store.AddError(ctx, migration.ID, migrationErr.Error()); err != nil {
			return err
		}
	}

	_, err := updateProgress(ctx, store, migration, migrator)
	return err
}

// updateProgress invokes the Progress method on the given migrator, updates the Progress field of the
// given migration record, and updates the record in the database.
func updateProgress(ctx context.Context, store storeIface, migration *Migration, migrator Migrator) (float64, error) {
	progress, err := migrator.Progress(ctx, migration.ApplyReverse)
	if err != nil {
		return 0, err
	}

	if err := store.UpdateProgress(ctx, migration.ID, progress); err != nil {
		return 0, err
	}

	migration.Progress = progress
	return progress, nil
}

func runMigrationUp(ctx context.Context, migration *Migration, migrator Migrator, logger log.Logger, operations *operations) (err error) {
	ctx, _, endObservation := operations.upForMigration(migration.ID).With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("migrationID", migration.ID),
	}})
	defer endObservation(1, observation.Args{})

	logger.Debug("Running up migration", log.Int("migrationID", migration.ID))
	return migrator.Up(ctx)
}

func runMigrationDown(ctx context.Context, migration *Migration, migrator Migrator, logger log.Logger, operations *operations) (err error) {
	ctx, _, endObservation := operations.downForMigration(migration.ID).With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("migrationID", migration.ID),
	}})
	defer endObservation(1, observation.Args{})

	logger.Debug("Running down migration", log.Int("migrationID", migration.ID))
	return migrator.Down(ctx)
}
