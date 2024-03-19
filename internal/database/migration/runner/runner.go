package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database/migration/definition"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/schemas"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/shared"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type RunnerFactoryWithSchemas func(schemaNames []string, schemas []*schemas.Schema) (*Runner, error)

type Runner struct {
	logger             log.Logger
	storeFactoryCaches map[string]*storeFactoryCache
	schemas            []*schemas.Schema
}

type StoreFactory func(ctx context.Context) (Store, error)

func NewRunner(logger log.Logger, storeFactories map[string]StoreFactory) *Runner {
	return NewRunnerWithSchemas(logger, storeFactories, schemas.Schemas)
}

func NewRunnerWithSchemas(logger log.Logger, storeFactories map[string]StoreFactory, schemas []*schemas.Schema) *Runner {
	storeFactoryCaches := make(map[string]*storeFactoryCache, len(storeFactories))
	for name, factory := range storeFactories {
		storeFactoryCaches[name] = &storeFactoryCache{factory: factory}
	}

	return &Runner{
		logger:             logger,
		storeFactoryCaches: storeFactoryCaches,
		schemas:            schemas,
	}
}

type storeFactoryCache struct {
	sync.Mutex
	factory StoreFactory
	store   Store
}

func (fc *storeFactoryCache) get(ctx context.Context) (Store, error) {
	fc.Lock()
	defer fc.Unlock()

	if fc.store != nil {
		return fc.store, nil
	}

	store, err := fc.factory(ctx)
	if err != nil {
		return nil, err
	}

	fc.store = store
	return store, nil
}

// Store returns the store associated with the given schema.
func (r *Runner) Store(ctx context.Context, schemaName string) (Store, error) {
	if factoryCache, ok := r.storeFactoryCaches[schemaName]; ok {
		return factoryCache.get(ctx)
	}

	return nil, errors.Newf("unknown schema %q", schemaName)
}

type schemaContext struct {
	logger               log.Logger
	schema               *schemas.Schema
	store                Store
	initialSchemaVersion schemaVersion
}

type schemaVersion struct {
	appliedVersions []int
	pendingVersions []int
	failedVersions  []int
}

type visitFunc func(ctx context.Context, schemaContext schemaContext) error

// forEachSchema invokes the given function once for each schema in the given list, with
// store instances initialized for each given schema name. Each function invocation occurs
// concurrently. Errors from each invocation are collected and returned. An error from one
// goroutine will not cancel the progress of another.
func (r *Runner) forEachSchema(ctx context.Context, schemaNames []string, visitor visitFunc) error {
	// Create map of relevant schemas keyed by name
	schemaMap, err := r.prepareSchemas(schemaNames)
	if err != nil {
		return err
	}

	// Create map of migration stores keyed by name
	storeMap, err := r.prepareStores(ctx, schemaNames)
	if err != nil {
		return err
	}

	// Create map of versions keyed by name
	versionMap, err := r.fetchVersions(ctx, storeMap)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errorCh := make(chan error, len(schemaNames))

	for _, schemaName := range schemaNames {
		wg.Add(1)

		go func() {
			defer wg.Done()

			errorCh <- visitor(ctx, schemaContext{
				logger:               r.logger,
				schema:               schemaMap[schemaName],
				store:                storeMap[schemaName],
				initialSchemaVersion: versionMap[schemaName],
			})
		}()
	}

	wg.Wait()
	close(errorCh)

	var errs error
	for err := range errorCh {
		if err != nil {
			errs = errors.Append(errs, err)
		}
	}

	return errs
}

func (r *Runner) prepareSchemas(schemaNames []string) (map[string]*schemas.Schema, error) {
	schemaMap := make(map[string]*schemas.Schema, len(schemaNames))

	for _, targetSchemaName := range schemaNames {
		for _, schema := range r.schemas {
			if schema.Name == targetSchemaName {
				schemaMap[schema.Name] = schema
				break
			}
		}
	}

	// Ensure that all supplied schema names are valid
	for _, schemaName := range schemaNames {
		if _, ok := schemaMap[schemaName]; !ok {
			return nil, errors.Newf("unknown schema %q", schemaName)
		}
	}

	return schemaMap, nil
}

func (r *Runner) prepareStores(ctx context.Context, schemaNames []string) (map[string]Store, error) {
	storeMap := make(map[string]Store, len(schemaNames))

	for _, schemaName := range schemaNames {
		store, err := r.Store(ctx, schemaName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to establish database connection for schema %q", schemaName)
		}

		storeMap[schemaName] = store
	}

	return storeMap, nil
}

func (r *Runner) fetchVersions(ctx context.Context, storeMap map[string]Store) (map[string]schemaVersion, error) {
	versions := make(map[string]schemaVersion, len(storeMap))

	for schemaName, store := range storeMap {
		schemaVersion, err := r.fetchVersion(ctx, schemaName, store)
		if err != nil {
			return nil, err
		}

		versions[schemaName] = schemaVersion
	}

	return versions, nil
}

func (r *Runner) fetchVersion(ctx context.Context, schemaName string, store Store) (schemaVersion, error) {
	appliedVersions, pendingVersions, failedVersions, err := store.Versions(ctx)
	if err != nil {
		return schemaVersion{}, errors.Wrapf(err, "failed to fetch version for schema %q", schemaName)
	}

	return schemaVersion{
		appliedVersions,
		pendingVersions,
		failedVersions,
	}, nil
}

type lockedVersionCallback func(
	schemaVersion schemaVersion,
	byState definitionsByState,
	earlyUnlock unlockFunc,
) error

type unlockFunc func(err error) error

// withLockedSchemaState attempts to take an advisory lock, then re-checks the version of the
// database. The resulting schema state is passed to the given function. The advisory lock
// will be released on function exit, but the callback may explicitly release the lock earlier.
//
// If the ignoreSingleDirtyLog flag is set to true, then the callback will be invoked if there is
// a single dirty migration log, and it's the next migration that would be applied with respect to
// the given schema context. This is meant to enable a short development loop where the user can
// re-apply the `up` command without having to create a dummy migration log to proceed.
//
// If the ignoreSinglePendingLog flag is set to true, then the callback will be invoked if there is
// a single pending migration log, and it's the next migration that would be applied with respect to
// the given schema context. This is meant to be used in the upgrade process, where an interrupted
// migrator command will appear as a concurrent upgrade attempt.
//
// This method returns a true-valued flag if it should be re-invoked by the caller.
func (r *Runner) withLockedSchemaState(
	ctx context.Context,
	schemaContext schemaContext,
	definitions []definition.Definition,
	ignoreSingleDirtyLog bool,
	ignoreSinglePendingLog bool,
	f lockedVersionCallback,
) (retry bool, _ error) {
	// Take an advisory lock to determine if there are any migrator instances currently
	// running queries unrelated to non-concurrent index creation. This will block until
	// we are able to gain the lock.
	unlock, err := r.pollLock(ctx, schemaContext)
	if err != nil {
		return false, err
	} else {
		defer func() { err = unlock(err) }()
	}

	// Re-fetch the current schema of the database now that we hold the lock. This may differ
	// from our original assumption if another migrator is running concurrently.
	schemaVersion, err := r.fetchVersion(ctx, schemaContext.schema.Name, schemaContext.store)
	if err != nil {
		return false, err
	}

	// Filter out any unlisted migrations (most likely future upgrades) and group them by status.
	byState := groupByState(schemaVersion, definitions)

	r.logger.Info(
		"Checked current schema state",
		log.String("schema", schemaContext.schema.Name),
		log.Ints("appliedVersions", extractIDs(byState.applied)),
		log.Ints("pendingVersions", extractIDs(byState.pending)),
		log.Ints("failedVersions", extractIDs(byState.failed)),
	)

	// Detect failed migrations, and determine if we need to wait longer for concurrent migrator
	// instances to finish their current work.
	if retry, err := validateSchemaState(
		ctx,
		schemaContext,
		definitions,
		byState,
		ignoreSingleDirtyLog,
		ignoreSinglePendingLog,
	); err != nil {
		return false, err
	} else if retry {
		// An index is currently being created. We return true here to flag to the caller that
		// we should wait a small time, then be re-invoked. We don't want to take any action
		// here while the other proceses is working.
		return true, nil
	}

	// Invoke the callback with the current schema state
	return false, f(schemaVersion, byState, unlock)
}

const (
	lockPollInterval = time.Second
	lockPollLogRatio = 5
)

// pollLock will attempt to acquire a session-level advisory lock while the given context has not
// been canceled. The caller must eventually invoke the unlock function on successful acquisition
// of the lock.
func (r *Runner) pollLock(ctx context.Context, schemaContext schemaContext) (unlock func(err error) error, _ error) {
	numWaits := 0
	logger := r.logger.With(log.String("schema", schemaContext.schema.Name))

	for {
		if acquired, unlock, err := schemaContext.store.TryLock(ctx); err != nil {
			return nil, err
		} else if acquired {
			logger.Info("Acquired schema migration lock")

			var logOnce sync.Once

			loggedUnlock := func(err error) error {
				logOnce.Do(func() {
					logger.Info("Released schema migration lock")
				})

				return unlock(err)
			}

			return loggedUnlock, nil
		}

		if numWaits%lockPollLogRatio == 0 {
			logger.Info("Schema migration lock is currently held - will re-attempt to acquire lock")
		}

		if err := wait(ctx, lockPollInterval); err != nil {
			return nil, err
		}

		numWaits++
	}
}

type definitionsByState struct {
	applied []definition.Definition
	pending []definition.Definition
	failed  []definition.Definition
}

// groupByState returns the the given definitions grouped by their status (applied, pending, failed) as
// indicated by the current schema.
func groupByState(schemaVersion schemaVersion, definitions []definition.Definition) definitionsByState {
	appliedVersionsMap := intSet(schemaVersion.appliedVersions)
	failedVersionsMap := intSet(schemaVersion.failedVersions)
	pendingVersionsMap := intSet(schemaVersion.pendingVersions)

	states := definitionsByState{}
	for _, def := range definitions {
		if _, ok := appliedVersionsMap[def.ID]; ok {
			states.applied = append(states.applied, def)
		}
		if _, ok := pendingVersionsMap[def.ID]; ok {
			states.pending = append(states.pending, def)
		}
		if _, ok := failedVersionsMap[def.ID]; ok {
			states.failed = append(states.failed, def)
		}
	}

	return states
}

// validateSchemaState inspects the given definitions grouped by state and determines if the schema
// state should be re-queried (when `retry` is true). This function returns an error if the database
// is in a dirty state (contains failed migrations or pending migrations without a backing query).
func validateSchemaState(
	ctx context.Context,
	schemaContext schemaContext,
	definitions []definition.Definition,
	byState definitionsByState,
	ignoreSingleDirtyLog bool,
	ignoreSinglePendingLog bool,
) (retry bool, _ error) {
	if ignoreSingleDirtyLog && len(byState.failed) == 1 {
		appliedVersionMap := intSet(extractIDs(byState.applied))
		for _, def := range definitions {
			if _, ok := appliedVersionMap[definitions[0].ID]; ok {
				continue
			}

			if byState.failed[0].ID == def.ID {
				schemaContext.logger.Warn("Attempting to re-try migration that previously failed")
				return false, nil
			}
		}
	}

	if ignoreSinglePendingLog && len(byState.pending) == 1 {
		schemaContext.logger.Warn("Ignoring a pending migration")
		return false, nil
	}

	if len(byState.failed) > 0 {
		// Explicit failures require administrator intervention
		return false, newDirtySchemaError(schemaContext.schema.Name, byState.failed)
	}

	if len(byState.pending) > 0 {
		// We are currently holding the lock, so any migrations that are "pending" are either
		// dead and the migrator instance has died before finishing the operation, or they're
		// active concurrent index creation operations. We'll partition this set into those two
		// groups and determine what to do.
		if pendingDefinitions, failedDefinitions, err := partitionPendingMigrations(ctx, schemaContext, byState.pending); err != nil {
			return false, err
		} else if len(failedDefinitions) > 0 {
			// Explicit failures require administrator intervention
			return false, newDirtySchemaError(schemaContext.schema.Name, failedDefinitions)
		} else if len(pendingDefinitions) > 0 {
			for _, definitionWithStatus := range pendingDefinitions {
				logIndexStatus(
					schemaContext,
					definitionWithStatus.definition.IndexMetadata.TableName,
					definitionWithStatus.definition.IndexMetadata.IndexName,
					definitionWithStatus.indexStatus,
					true,
				)
			}

			return true, nil
		}
	}

	return false, nil
}

type definitionWithStatus struct {
	definition  definition.Definition
	indexStatus shared.IndexStatus
}

// partitionPendingMigrations partitions the given migrations into two sets: the set of pending
// migration definitions, which includes migrations with visible and active create index operation
// running in the database, and the set of filed migration definitions, which includes migrations
// which are marked as pending but do not appear as active.
//
// This function assumes that the migration advisory lock is held.
func partitionPendingMigrations(
	ctx context.Context,
	schemaContext schemaContext,
	definitions []definition.Definition,
) (pendingDefinitions []definitionWithStatus, failedDefinitions []definition.Definition, _ error) {
	for _, def := range definitions {
		if def.IsCreateIndexConcurrently {
			tableName := def.IndexMetadata.TableName
			indexName := def.IndexMetadata.IndexName

			if indexStatus, ok, err := schemaContext.store.IndexStatus(ctx, tableName, indexName); err != nil {
				return nil, nil, errors.Wrapf(err, "failed to check creation status of index %q.%q", tableName, indexName)
			} else if ok && indexStatus.Phase != nil {
				pendingDefinitions = append(pendingDefinitions, definitionWithStatus{def, indexStatus})
				continue
			}
		}

		failedDefinitions = append(failedDefinitions, def)
	}

	return pendingDefinitions, failedDefinitions, nil
}

// getAndLogIndexStatus calls IndexStatus on the given store and returns the results. The result
// is logged to the package-level logger.
func getAndLogIndexStatus(ctx context.Context, schemaContext schemaContext, tableName, indexName string) (shared.IndexStatus, bool, error) {
	indexStatus, exists, err := schemaContext.store.IndexStatus(ctx, tableName, indexName)
	if err != nil {
		return shared.IndexStatus{}, false, errors.Wrap(err, "failed to query state of index")
	}

	logIndexStatus(schemaContext, tableName, indexName, indexStatus, exists)
	return indexStatus, exists, nil
}

// logIndexStatus logs the result of IndexStatus to the package-level logger.
func logIndexStatus(schemaContext schemaContext, tableName, indexName string, indexStatus shared.IndexStatus, exists bool) {
	schemaContext.logger.Info(
		"Checked progress of index creation",
		log.Object("result",
			log.String("schema", schemaContext.schema.Name),
			log.String("tableName", tableName),
			log.String("indexName", indexName),
			log.Bool("exists", exists),
			log.Bool("isValid", indexStatus.IsValid),
			renderIndexStatus(indexStatus),
		),
	)
}

// renderIndexStatus returns a slice of interface pairs describing the given index status for use in a
// call to logger. If the index is currently being created, the progress of the create operation will be
// summarized.
func renderIndexStatus(progress shared.IndexStatus) log.Field {
	if progress.Phase == nil {
		return log.Object("index status", log.Bool("in-progress", false))
	}

	index := -1
	for i, phase := range shared.CreateIndexConcurrentlyPhases {
		if phase == *progress.Phase {
			index = i
			break
		}
	}

	return log.Object(
		"index status",
		log.Bool("in-progress", true),
		log.String("phase", *progress.Phase),
		log.String("phases", fmt.Sprintf("%d of %d", index, len(shared.CreateIndexConcurrentlyPhases))),
		log.String("lockers", fmt.Sprintf("%d of %d", progress.LockersDone, progress.LockersTotal)),
		log.String("blocks", fmt.Sprintf("%d of %d", progress.BlocksDone, progress.BlocksTotal)),
		log.String("tuples", fmt.Sprintf("%d of %d", progress.TuplesDone, progress.TuplesTotal)),
	)
}
