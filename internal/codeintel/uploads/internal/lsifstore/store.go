package lsifstore

import (
	"context"
	"time"

	"github.com/sourcegraph/scip/bindings/go/scip"

	codeintelshared "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type Store interface {
	WithTransaction(ctx context.Context, f func(s Store) error) error

	// Insert
	InsertMetadata(ctx context.Context, uploadID int, meta ProcessedMetadata) error
	NewPreciseSCIPWriter(ctx context.Context, uploadID int) (SCIPWriter, error)
	NewSyntacticSCIPWriter(ctx context.Context, uploadID int) (SCIPWriter, error)

	// Reconciliation and cleanup
	IDsWithMeta(ctx context.Context, ids []int) ([]int, error)
	ReconcileCandidates(ctx context.Context, batchSize int) ([]int, error)
	ReconcileCandidatesWithTime(ctx context.Context, batchSize int, now time.Time) (_ []int, err error)
	DeleteLsifDataByUploadIds(ctx context.Context, bundleIDs ...int) (err error)
	DeleteAbandonedSchemaVersionsRecords(ctx context.Context) (_ int, err error)
	DeleteUnreferencedDocuments(ctx context.Context, batchSize int, maxAge time.Duration, now time.Time) (numScanned, numDeleted int, err error)
}

type SCIPWriter interface {
	InsertDocument(ctx context.Context, path string, scipDocument *scip.Document) error
	Flush(ctx context.Context) (uint32, error)
}

type store struct {
	db         *basestore.Store
	operations *operations
}

func New(observationCtx *observation.Context, db codeintelshared.CodeIntelDB) Store {
	return newInternal(observationCtx, db)
}

func newInternal(observationCtx *observation.Context, db codeintelshared.CodeIntelDB) *store {
	return &store{
		db:         basestore.NewWithHandle(db.Handle()),
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
		db:         tx,
		operations: s.operations,
	}, nil
}

func (s *store) Done(err error) error {
	return s.db.Done(err)
}
