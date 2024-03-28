package store

import (
	"context"
	"time"

	logger "github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/shared"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type Store interface {
	WithTransaction(ctx context.Context, f func(tx Store) error) error

	// Inference configuration
	GetInferenceScript(ctx context.Context) (string, error)
	SetInferenceScript(ctx context.Context, script string) error

	// Repository configuration
	RepositoryExceptions(ctx context.Context, repositoryID int) (canSchedule, canInfer bool, _ error)
	SetRepositoryExceptions(ctx context.Context, repositoryID int, canSchedule, canInfer bool) error
	GetIndexConfigurationByRepositoryID(ctx context.Context, repositoryID int) (shared.IndexConfiguration, bool, error)
	UpdateIndexConfigurationByRepositoryID(ctx context.Context, repositoryID int, data []byte) error

	// Coverage summaries
	TopRepositoriesToConfigure(ctx context.Context, limit int) ([]uploadsshared.RepositoryWithCount, error)
	RepositoryIDsWithConfiguration(ctx context.Context, offset, limit int) ([]uploadsshared.RepositoryWithAvailableIndexers, int, error)
	GetLastIndexScanForRepository(ctx context.Context, repositoryID int) (*time.Time, error)
	SetConfigurationSummary(ctx context.Context, repositoryID int, numEvents int, availableIndexers map[string]uploadsshared.AvailableIndexer) error
	TruncateConfigurationSummary(ctx context.Context, numRecordsToRetain int) error

	// Scheduler
	GetQueuedRepoRev(ctx context.Context, batchSize int) ([]RepoRev, error)
	MarkRepoRevsAsProcessed(ctx context.Context, ids []int) error

	// Enqueuer
	IsQueued(ctx context.Context, repositoryID int, commit string) (bool, error)
	IsQueuedRootIndexer(ctx context.Context, repositoryID int, commit string, root string, indexer string) (bool, error)
	InsertIndexes(ctx context.Context, indexes []uploadsshared.Index) ([]uploadsshared.Index, error)

	// Dependency indexing
	InsertDependencyIndexingJob(ctx context.Context, uploadID int, externalServiceKind string, syncTime time.Time) (int, error)
	QueueRepoRev(ctx context.Context, repositoryID int, commit string) error
}

type RepoRev struct {
	ID           int
	RepositoryID int
	Rev          string
}

type store struct {
	db         *basestore.Store
	logger     logger.Logger
	operations *operations
}

func New(observationCtx *observation.Context, db database.DB) Store {
	return &store{
		db:         basestore.NewWithHandle(db.Handle()),
		logger:     logger.Scoped("autoindexing.store"),
		operations: newOperations(observationCtx),
	}
}

func (s *store) WithTransaction(ctx context.Context, f func(s Store) error) error {
	return s.withTransaction(ctx, func(s *store) error { return f(s) })
}

func (s *store) withTransaction(ctx context.Context, f func(s *store) error) error {
	return basestore.InTransaction[*store](ctx, s, f)
}

func (s *store) Transact(ctx context.Context) (*store, error) {
	tx, err := s.db.Transact(ctx)
	if err != nil {
		return nil, err
	}

	return &store{
		logger:     s.logger,
		db:         tx,
		operations: s.operations,
	}, nil
}

func (s *store) Done(err error) error {
	return s.db.Done(err)
}
