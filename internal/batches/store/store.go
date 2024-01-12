package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/github_apps/store"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// SQLColumns is a slice of column names, that can be converted to a slice of
// *sqlf.Query.
type SQLColumns []string

// ToSqlf returns all the columns wrapped in a *sqlf.Query.
func (s SQLColumns) ToSqlf() []*sqlf.Query {
	columns := []*sqlf.Query{}
	for _, col := range s {
		columns = append(columns, sqlf.Sprintf(col))
	}
	return columns
}

// FmtStr returns a sqlf format string that can be concatenated to a query and
// contains as many `%s` as columns.
func (s SQLColumns) FmtStr() string {
	elems := make([]string, len(s))
	for i := range s {
		elems[i] = "%s"
	}
	return fmt.Sprintf("(%s)", strings.Join(elems, ", "))
}

// ErrNoResults is returned by Store method calls that found no results.
var ErrNoResults = errors.New("no results")

// RandomID generates a random ID to be used for identifiers in the database.
func RandomID() (string, error) {
	random, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return random.String(), nil
}

// Store exposes methods to read and write batches domain models
// from persistent storage.
type Store struct {
	*basestore.Store

	logger         log.Logger
	key            encryption.Key
	now            func() time.Time
	operations     *operations
	observationCtx *observation.Context
}

// New returns a new Store backed by the given database.
func New(db database.DB, observationCtx *observation.Context, key encryption.Key) *Store {
	return NewWithClock(db, observationCtx, key, timeutil.Now)
}

// NewWithClock returns a new Store backed by the given database and
// clock for timestamps.
func NewWithClock(db database.DB, observationCtx *observation.Context, key encryption.Key, clock func() time.Time) *Store {
	return &Store{
		logger:         observationCtx.Logger,
		Store:          basestore.NewWithHandle(db.Handle()),
		key:            key,
		now:            clock,
		operations:     newOperations(observationCtx),
		observationCtx: observationCtx,
	}
}

// observationCtx returns the observation context wrapped in this store.
func (s *Store) ObservationCtx() *observation.Context {
	return s.observationCtx
}

func (s *Store) GitHubAppsStore() store.GitHubAppsStore {
	return store.GitHubAppsWith(s.Store).WithEncryptionKey(keyring.Default().GitHubAppKey)
}

// DatabaseDB returns a database.DB with the same handle that this Store was
// instantiated with.
// It's here for legacy reason to pass the database.DB to a repos.Store while
// repos.Store doesn't accept a basestore.TransactableHandle yet.
func (s *Store) DatabaseDB() database.DB { return database.NewDBWith(s.logger, s) }

// Clock returns the clock used by the Store.
func (s *Store) Clock() func() time.Time { return s.now }

var _ basestore.ShareableStore = &Store{}

// With creates a new Store with the given basestore.Shareable store as the
// underlying basestore.Store.
// Needed to implement the basestore.Store interface
func (s *Store) With(other basestore.ShareableStore) *Store {
	return &Store{
		logger:         s.logger,
		Store:          s.Store.With(other),
		key:            s.key,
		operations:     s.operations,
		observationCtx: s.observationCtx,
		now:            s.now,
	}
}

// Transact creates a new transaction.
// It's required to implement this method and wrap the Transact method of the
// underlying basestore.Store.
func (s *Store) Transact(ctx context.Context) (*Store, error) {
	txBase, err := s.Store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	return &Store{
		logger:         s.logger,
		Store:          txBase,
		key:            s.key,
		operations:     s.operations,
		observationCtx: s.observationCtx,
		now:            s.now,
	}, nil
}

// Repos returns a database.RepoStore using the same connection as this store.
func (s *Store) Repos() database.RepoStore {
	return database.ReposWith(s.logger, s)
}

// ExternalServices returns a database.ExternalServiceStore using the same connection as this store.
func (s *Store) ExternalServices() database.ExternalServiceStore {
	return database.ExternalServicesWith(s.observationCtx.Logger, s)
}

// UserCredentials returns a database.UserCredentialsStore using the same connection as this store.
func (s *Store) UserCredentials() database.UserCredentialsStore {
	return database.UserCredentialsWith(s.logger, s, s.key)
}

func (s *Store) query(ctx context.Context, q *sqlf.Query, sc scanFunc) error {
	rows, err := s.Store.Query(ctx, q)
	if err != nil {
		return err
	}
	return scanAll(rows, sc)
}

func (s *Store) queryCount(ctx context.Context, q *sqlf.Query) (int, error) {
	count, ok, err := basestore.ScanFirstInt(s.Query(ctx, q))
	if err != nil || !ok {
		return count, err
	}
	return count, nil
}

type operations struct {
	createBatchChange      *observation.Operation
	upsertBatchChange      *observation.Operation
	updateBatchChange      *observation.Operation
	deleteBatchChange      *observation.Operation
	countBatchChanges      *observation.Operation
	getBatchChange         *observation.Operation
	getBatchChangeDiffStat *observation.Operation
	getRepoDiffStat        *observation.Operation
	listBatchChanges       *observation.Operation

	createBatchSpecExecution *observation.Operation
	getBatchSpecExecution    *observation.Operation
	cancelBatchSpecExecution *observation.Operation
	listBatchSpecExecutions  *observation.Operation

	createBatchSpec         *observation.Operation
	updateBatchSpec         *observation.Operation
	deleteBatchSpec         *observation.Operation
	countBatchSpecs         *observation.Operation
	getBatchSpec            *observation.Operation
	getBatchSpecDiffStat    *observation.Operation
	getNewestBatchSpec      *observation.Operation
	listBatchSpecs          *observation.Operation
	listBatchSpecRepoIDs    *observation.Operation
	deleteExpiredBatchSpecs *observation.Operation

	upsertBatchSpecWorkspaceFile *observation.Operation
	deleteBatchSpecWorkspaceFile *observation.Operation
	getBatchSpecWorkspaceFile    *observation.Operation
	listBatchSpecWorkspaceFiles  *observation.Operation
	countBatchSpecWorkspaceFiles *observation.Operation

	getBulkOperation        *observation.Operation
	listBulkOperations      *observation.Operation
	countBulkOperations     *observation.Operation
	listBulkOperationErrors *observation.Operation

	getChangesetEvent     *observation.Operation
	listChangesetEvents   *observation.Operation
	countChangesetEvents  *observation.Operation
	upsertChangesetEvents *observation.Operation

	createChangesetJob *observation.Operation
	getChangesetJob    *observation.Operation

	createChangesetSpec                      *observation.Operation
	updateChangesetSpecBatchSpecID           *observation.Operation
	deleteChangesetSpec                      *observation.Operation
	countChangesetSpecs                      *observation.Operation
	getChangesetSpec                         *observation.Operation
	listChangesetSpecs                       *observation.Operation
	deleteExpiredChangesetSpecs              *observation.Operation
	deleteUnattachedExpiredChangesetSpecs    *observation.Operation
	getRewirerMappings                       *observation.Operation
	listChangesetSpecsWithConflictingHeadRef *observation.Operation
	deleteChangesetSpecs                     *observation.Operation

	createChangeset                   *observation.Operation
	deleteChangeset                   *observation.Operation
	countChangesets                   *observation.Operation
	getChangeset                      *observation.Operation
	listChangesetSyncData             *observation.Operation
	listChangesets                    *observation.Operation
	enqueueChangeset                  *observation.Operation
	updateChangeset                   *observation.Operation
	updateChangesetBatchChanges       *observation.Operation
	updateChangesetUIPublicationState *observation.Operation
	updateChangesetCodeHostState      *observation.Operation
	updateChangesetCommitVerification *observation.Operation
	getChangesetExternalIDs           *observation.Operation
	cancelQueuedBatchChangeChangesets *observation.Operation
	enqueueChangesetsToClose          *observation.Operation
	getChangesetsStats                *observation.Operation
	getRepoChangesetsStats            *observation.Operation
	getGlobalChangesetsStats          *observation.Operation
	enqueueNextScheduledChangeset     *observation.Operation
	getChangesetPlaceInSchedulerQueue *observation.Operation
	cleanDetachedChangesets           *observation.Operation

	listCodeHosts         *observation.Operation
	getExternalServiceIDs *observation.Operation

	createSiteCredential *observation.Operation
	deleteSiteCredential *observation.Operation
	getSiteCredential    *observation.Operation
	listSiteCredentials  *observation.Operation
	updateSiteCredential *observation.Operation

	createBatchSpecWorkspace       *observation.Operation
	getBatchSpecWorkspace          *observation.Operation
	listBatchSpecWorkspaces        *observation.Operation
	countBatchSpecWorkspaces       *observation.Operation
	markSkippedBatchSpecWorkspaces *observation.Operation
	listRetryBatchSpecWorkspaces   *observation.Operation

	createBatchSpecWorkspaceExecutionJobs              *observation.Operation
	createBatchSpecWorkspaceExecutionJobsForWorkspaces *observation.Operation
	getBatchSpecWorkspaceExecutionJob                  *observation.Operation
	listBatchSpecWorkspaceExecutionJobs                *observation.Operation
	deleteBatchSpecWorkspaceExecutionJobs              *observation.Operation
	cancelBatchSpecWorkspaceExecutionJobs              *observation.Operation
	retryBatchSpecWorkspaceExecutionJobs               *observation.Operation
	disableBatchSpecWorkspaceExecutionCache            *observation.Operation

	createBatchSpecResolutionJob *observation.Operation
	getBatchSpecResolutionJob    *observation.Operation
	listBatchSpecResolutionJobs  *observation.Operation

	listBatchSpecExecutionCacheEntries     *observation.Operation
	markUsedBatchSpecExecutionCacheEntries *observation.Operation
	createBatchSpecExecutionCacheEntry     *observation.Operation
	cleanBatchSpecExecutionCacheEntries    *observation.Operation
}

var (
	singletonOperations *operations
	operationsOnce      sync.Once
)

// newOperations generates a singleton of the operations struct.
// TODO: We should create one per observationCtx.
func newOperations(observationCtx *observation.Context) *operations {
	operationsOnce.Do(func() {
		m := metrics.NewREDMetrics(
			observationCtx.Registerer,
			"batches_dbstore",
			metrics.WithLabels("op"),
			metrics.WithCountHelp("Total number of method invocations."),
		)

		op := func(name string) *observation.Operation {
			return observationCtx.Operation(observation.Op{
				Name:              fmt.Sprintf("batches.dbstore.%s", name),
				MetricLabelValues: []string{name},
				Metrics:           m,
				ErrorFilter: func(err error) observation.ErrorFilterBehaviour {
					if errors.Is(err, ErrNoResults) {
						return observation.EmitForNone
					}
					return observation.EmitForDefault
				},
			})
		}

		singletonOperations = &operations{
			createBatchChange:      op("CreateBatchChange"),
			upsertBatchChange:      op("UpsertBatchChange"),
			updateBatchChange:      op("UpdateBatchChange"),
			deleteBatchChange:      op("DeleteBatchChange"),
			countBatchChanges:      op("CountBatchChanges"),
			listBatchChanges:       op("ListBatchChanges"),
			getBatchChange:         op("GetBatchChange"),
			getBatchChangeDiffStat: op("GetBatchChangeDiffStat"),
			getRepoDiffStat:        op("GetRepoDiffStat"),

			createBatchSpecExecution: op("CreateBatchSpecExecution"),
			getBatchSpecExecution:    op("GetBatchSpecExecution"),
			cancelBatchSpecExecution: op("CancelBatchSpecExecution"),
			listBatchSpecExecutions:  op("ListBatchSpecExecutions"),

			createBatchSpec:         op("CreateBatchSpec"),
			updateBatchSpec:         op("UpdateBatchSpec"),
			deleteBatchSpec:         op("DeleteBatchSpec"),
			countBatchSpecs:         op("CountBatchSpecs"),
			getBatchSpec:            op("GetBatchSpec"),
			getBatchSpecDiffStat:    op("GetBatchSpecDiffStat"),
			getNewestBatchSpec:      op("GetNewestBatchSpec"),
			listBatchSpecs:          op("ListBatchSpecs"),
			listBatchSpecRepoIDs:    op("ListBatchSpecRepoIDs"),
			deleteExpiredBatchSpecs: op("DeleteExpiredBatchSpecs"),

			upsertBatchSpecWorkspaceFile: op("UpsertBatchSpecWorkspaceFile"),
			deleteBatchSpecWorkspaceFile: op("DeleteBatchSpecWorkspaceFile"),
			getBatchSpecWorkspaceFile:    op("GetBatchSpecWorkspaceFile"),
			listBatchSpecWorkspaceFiles:  op("ListBatchSpecWorkspaceFiles"),
			countBatchSpecWorkspaceFiles: op("CountBatchSpecWorkspaceFiles"),

			getBulkOperation:        op("GetBulkOperation"),
			listBulkOperations:      op("ListBulkOperations"),
			countBulkOperations:     op("CountBulkOperations"),
			listBulkOperationErrors: op("ListBulkOperationErrors"),

			getChangesetEvent:     op("GetChangesetEvent"),
			listChangesetEvents:   op("ListChangesetEvents"),
			countChangesetEvents:  op("CountChangesetEvents"),
			upsertChangesetEvents: op("UpsertChangesetEvents"),

			createChangesetJob: op("CreateChangesetJob"),
			getChangesetJob:    op("GetChangesetJob"),

			createChangesetSpec:                      op("CreateChangesetSpec"),
			updateChangesetSpecBatchSpecID:           op("UpdateChangesetSpecBatchSpecID"),
			deleteChangesetSpec:                      op("DeleteChangesetSpec"),
			countChangesetSpecs:                      op("CountChangesetSpecs"),
			getChangesetSpec:                         op("GetChangesetSpec"),
			listChangesetSpecs:                       op("ListChangesetSpecs"),
			deleteExpiredChangesetSpecs:              op("DeleteExpiredChangesetSpecs"),
			deleteUnattachedExpiredChangesetSpecs:    op("DeleteUnattachedExpiredChangesetSpecs"),
			deleteChangesetSpecs:                     op("DeleteChangesetSpecs"),
			getRewirerMappings:                       op("GetRewirerMappings"),
			listChangesetSpecsWithConflictingHeadRef: op("ListChangesetSpecsWithConflictingHeadRef"),

			createChangeset:                   op("CreateChangeset"),
			deleteChangeset:                   op("DeleteChangeset"),
			countChangesets:                   op("CountChangesets"),
			getChangeset:                      op("GetChangeset"),
			listChangesetSyncData:             op("ListChangesetSyncData"),
			listChangesets:                    op("ListChangesets"),
			enqueueChangeset:                  op("EnqueueChangeset"),
			updateChangeset:                   op("UpdateChangeset"),
			updateChangesetBatchChanges:       op("UpdateChangesetBatchChanges"),
			updateChangesetUIPublicationState: op("UpdateChangesetUIPublicationState"),
			updateChangesetCodeHostState:      op("UpdateChangesetCodeHostState"),
			updateChangesetCommitVerification: op("UpdateChangesetCommitVerification"),
			getChangesetExternalIDs:           op("GetChangesetExternalIDs"),
			cancelQueuedBatchChangeChangesets: op("CancelQueuedBatchChangeChangesets"),
			enqueueChangesetsToClose:          op("EnqueueChangesetsToClose"),
			getChangesetsStats:                op("GetChangesetsStats"),
			getRepoChangesetsStats:            op("GetRepoChangesetsStats"),
			getGlobalChangesetsStats:          op("GetGlobalChangesetsStats"),
			enqueueNextScheduledChangeset:     op("EnqueueNextScheduledChangeset"),
			getChangesetPlaceInSchedulerQueue: op("GetChangesetPlaceInSchedulerQueue"),
			cleanDetachedChangesets:           op("CleanDetachedChangesets"),

			listCodeHosts:         op("ListCodeHosts"),
			getExternalServiceIDs: op("GetExternalServiceIDs"),

			createSiteCredential: op("CreateSiteCredential"),
			deleteSiteCredential: op("DeleteSiteCredential"),
			getSiteCredential:    op("GetSiteCredential"),
			listSiteCredentials:  op("ListSiteCredentials"),
			updateSiteCredential: op("UpdateSiteCredential"),

			createBatchSpecWorkspace:       op("CreateBatchSpecWorkspace"),
			getBatchSpecWorkspace:          op("GetBatchSpecWorkspace"),
			listBatchSpecWorkspaces:        op("ListBatchSpecWorkspaces"),
			countBatchSpecWorkspaces:       op("CountBatchSpecWorkspaces"),
			markSkippedBatchSpecWorkspaces: op("MarkSkippedBatchSpecWorkspaces"),
			listRetryBatchSpecWorkspaces:   op("ListRetryBatchSpecWorkspaces"),

			createBatchSpecWorkspaceExecutionJobs:              op("CreateBatchSpecWorkspaceExecutionJobs"),
			createBatchSpecWorkspaceExecutionJobsForWorkspaces: op("CreateBatchSpecWorkspaceExecutionJobsForWorkspaces"),
			getBatchSpecWorkspaceExecutionJob:                  op("GetBatchSpecWorkspaceExecutionJob"),
			listBatchSpecWorkspaceExecutionJobs:                op("ListBatchSpecWorkspaceExecutionJobs"),
			deleteBatchSpecWorkspaceExecutionJobs:              op("DeleteBatchSpecWorkspaceExecutionJobs"),
			cancelBatchSpecWorkspaceExecutionJobs:              op("CancelBatchSpecWorkspaceExecutionJobs"),
			retryBatchSpecWorkspaceExecutionJobs:               op("RetryBatchSpecWorkspaceExecutionJobs"),
			disableBatchSpecWorkspaceExecutionCache:            op("DisableBatchSpecWorkspaceExecutionCache"),

			createBatchSpecResolutionJob: op("CreateBatchSpecResolutionJob"),
			getBatchSpecResolutionJob:    op("GetBatchSpecResolutionJob"),
			listBatchSpecResolutionJobs:  op("ListBatchSpecResolutionJobs"),

			listBatchSpecExecutionCacheEntries:     op("ListBatchSpecExecutionCacheEntries"),
			markUsedBatchSpecExecutionCacheEntries: op("MarkUsedBatchSpecExecutionCacheEntries"),
			createBatchSpecExecutionCacheEntry:     op("CreateBatchSpecExecutionCacheEntry"),

			cleanBatchSpecExecutionCacheEntries: op("CleanBatchSpecExecutionCacheEntries"),
		}
	})

	return singletonOperations
}

// a scanFunc scans one or more rows from a dbutil.Scanner, returning
// the last id column scanned and the count of scanned rows.
type scanFunc func(dbutil.Scanner) (err error)

func scanAll(rows *sql.Rows, scan scanFunc) (err error) {
	defer func() { err = basestore.CloseRows(rows, err) }()

	for rows.Next() {
		if err = scan(rows); err != nil {
			return err
		}
	}

	return rows.Err()
}

// buildRecordScanner converts a scan*() function as implemented in lots of
// places in this package into something we can use in
// `dbworker.BuildWorkerScan`.
func buildRecordScanner[T any](scan func(*T, dbutil.Scanner) error) func(dbutil.Scanner) (*T, error) {
	return func(s dbutil.Scanner) (*T, error) {
		var t T
		err := scan(&t, s)
		return &t, err
	}
}

func jsonbColumn(metadata any) (msg json.RawMessage, err error) {
	switch m := metadata.(type) {
	case nil:
		msg = json.RawMessage("{}")
	case string:
		msg = json.RawMessage(m)
	case []byte:
		msg = m
	case json.RawMessage:
		msg = m
	default:
		msg, err = json.MarshalIndent(m, "        ", "    ")
	}
	return
}

type LimitOpts struct {
	Limit int
}

func (o LimitOpts) DBLimit() int {
	if o.Limit == 0 {
		return o.Limit
	}
	// We always request one item more than actually requested, to determine the next ID for pagination.
	// The store should make sure to strip the last element in a result set, if len(rs) == o.DBLimit().
	return o.Limit + 1
}

func (o LimitOpts) ToDB() string {
	var limitClause string
	if o.Limit > 0 {
		limitClause = fmt.Sprintf("LIMIT %d", o.DBLimit())
	}
	return limitClause
}

func isUniqueConstraintViolation(err error, constraintName string) bool {
	var e *pgconn.PgError
	return errors.As(err, &e) && e.Code == "23505" && e.ConstraintName == constraintName
}
