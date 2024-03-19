package store

import (
	"context"
	"time"

	logger "github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

type Store interface {
	WithTransaction(ctx context.Context, f func(s Store) error) error
	Handle() *basestore.Store

	// Upload records
	GetUploads(ctx context.Context, opts shared.GetUploadsOptions) ([]shared.Upload, int, error)
	GetUploadByID(ctx context.Context, id int) (shared.Upload, bool, error)
	GetCompletedUploadsByIDs(ctx context.Context, ids []int) ([]shared.CompletedUpload, error)
	GetUploadsByIDs(ctx context.Context, ids ...int) ([]shared.Upload, error)
	GetUploadsByIDsAllowDeleted(ctx context.Context, ids ...int) ([]shared.Upload, error)
	GetUploadIDsWithReferences(ctx context.Context, orderedMonikers []precise.QualifiedMonikerData, ignoreIDs []int, repositoryID int, commit string, limit int, offset int, trace observation.TraceLogger) ([]int, int, int, error)
	GetVisibleUploadsMatchingMonikers(ctx context.Context, repositoryID int, commit string, orderedMonikers []precise.QualifiedMonikerData, limit, offset int) (shared.PackageReferenceScanner, int, error)
	GetCompletedUploadsWithDefinitionsForMonikers(ctx context.Context, monikers []precise.QualifiedMonikerData) ([]shared.CompletedUpload, error)
	GetAuditLogsForUpload(ctx context.Context, uploadID int) ([]shared.UploadLog, error)
	DeleteUploads(ctx context.Context, opts shared.DeleteUploadsOptions) error
	DeleteUploadByID(ctx context.Context, id int) (bool, error)
	ReindexUploads(ctx context.Context, opts shared.ReindexUploadsOptions) error
	ReindexUploadByID(ctx context.Context, id int) error

	// Index records
	GetIndexes(ctx context.Context, opts shared.GetIndexesOptions) ([]shared.Index, int, error)
	GetIndexByID(ctx context.Context, id int) (shared.Index, bool, error)
	GetIndexesByIDs(ctx context.Context, ids ...int) ([]shared.Index, error)
	DeleteIndexByID(ctx context.Context, id int) (bool, error)
	DeleteIndexes(ctx context.Context, opts shared.DeleteIndexesOptions) error
	ReindexIndexByID(ctx context.Context, id int) error
	ReindexIndexes(ctx context.Context, opts shared.ReindexIndexesOptions) error

	// Upload record insertion + processing
	InsertUpload(ctx context.Context, upload shared.Upload) (int, error)
	AddUploadPart(ctx context.Context, uploadID, partIndex int) error
	MarkQueued(ctx context.Context, id int, uploadSize *int64) error
	MarkFailed(ctx context.Context, id int, reason string) error
	DeleteOverlappingCompletedUploads(ctx context.Context, repositoryID int, commit, root, indexer string) error
	WorkerutilStore(observationCtx *observation.Context) dbworkerstore.Store[shared.Upload]

	// Dependencies
	ReferencesForUpload(ctx context.Context, uploadID int) (shared.PackageReferenceScanner, error)
	UpdatePackages(ctx context.Context, uploadID int, packages []precise.Package) error
	UpdatePackageReferences(ctx context.Context, uploadID int, references []precise.PackageReference) error

	// Summary
	GetIndexers(ctx context.Context, opts shared.GetIndexersOptions) ([]string, error)
	GetRecentUploadsSummary(ctx context.Context, repositoryID int) ([]shared.UploadsWithRepositoryNamespace, error)
	GetRecentIndexesSummary(ctx context.Context, repositoryID int) ([]shared.IndexesWithRepositoryNamespace, error)
	RepositoryIDsWithErrors(ctx context.Context, offset, limit int) ([]shared.RepositoryWithCount, int, error)
	NumRepositoriesWithCodeIntelligence(ctx context.Context) (int, error)

	// Commit graph
	SetRepositoryAsDirty(ctx context.Context, repositoryID int) error
	GetDirtyRepositories(ctx context.Context) ([]shared.DirtyRepository, error)
	UpdateUploadsVisibleToCommits(ctx context.Context, repositoryID int, graph *gitdomain.CommitGraph, refDescriptions map[string][]gitdomain.RefDescription, maxAgeForNonStaleBranches, maxAgeForNonStaleTags time.Duration, dirtyToken int, now time.Time) error
	GetCommitsVisibleToUpload(ctx context.Context, uploadID, limit int, token *string) ([]string, *string, error)
	FindClosestCompletedUploads(ctx context.Context, repositoryID int, commit, path string, rootMustEnclosePath bool, indexer string) ([]shared.CompletedUpload, error)
	FindClosestCompletedUploadsFromGraphFragment(ctx context.Context, repositoryID int, commit, path string, rootMustEnclosePath bool, indexer string, commitGraph *gitdomain.CommitGraph) ([]shared.CompletedUpload, error)
	GetRepositoriesMaxStaleAge(ctx context.Context) (time.Duration, error)
	GetCommitGraphMetadata(ctx context.Context, repositoryID int) (stale bool, updatedAt *time.Time, _ error)

	// Expiration
	GetLastUploadRetentionScanForRepository(ctx context.Context, repositoryID int) (*time.Time, error)
	SetRepositoriesForRetentionScan(ctx context.Context, processDelay time.Duration, limit int) ([]int, error)
	UpdateUploadRetention(ctx context.Context, protectedIDs, expiredIDs []int) error
	SoftDeleteExpiredUploads(ctx context.Context, batchSize int) (int, int, error)
	SoftDeleteExpiredUploadsViaTraversal(ctx context.Context, maxTraversal int) (int, int, error)

	// Commit date
	GetOldestCommitDate(ctx context.Context, repositoryID int) (time.Time, bool, error)
	UpdateCommittedAt(ctx context.Context, repositoryID int, commit, commitDateString string) error
	SourcedCommitsWithoutCommittedAt(ctx context.Context, batchSize int) ([]SourcedCommits, error)

	// Cleanup
	HardDeleteUploadsByIDs(ctx context.Context, ids ...int) error
	DeleteUploadsStuckUploading(ctx context.Context, uploadedBefore time.Time) (int, int, error)
	DeleteUploadsWithoutRepository(ctx context.Context, now time.Time) (int, int, error)
	DeleteOldAuditLogs(ctx context.Context, maxAge time.Duration, now time.Time) (numRecordsScanned, numRecordsAltered int, _ error)
	ReconcileCandidates(ctx context.Context, batchSize int) ([]int, error)
	ProcessStaleSourcedCommits(ctx context.Context, minimumTimeSinceLastCheck time.Duration, commitResolverBatchSize int, commitResolverMaximumCommitLag time.Duration, shouldDelete func(ctx context.Context, repositoryID int, repositoryName, commit string) (bool, error)) (int, int, error)
	DeleteIndexesWithoutRepository(ctx context.Context, now time.Time) (int, int, error)
	ExpireFailedRecords(ctx context.Context, batchSize int, failedIndexMaxAge time.Duration, now time.Time) (int, int, error)
	ProcessSourcedCommits(ctx context.Context, minimumTimeSinceLastCheck time.Duration, commitResolverMaximumCommitLag time.Duration, limit int, f func(ctx context.Context, repositoryID int, repositoryName, commit string) (bool, error), now time.Time) (int, int, error)

	// Misc
	HasRepository(ctx context.Context, repositoryID int) (bool, error)
	HasCommit(ctx context.Context, repositoryID int, commit string) (bool, error)
	InsertDependencySyncingJob(ctx context.Context, uploadID int) (int, error)
}

type SourcedCommits struct {
	RepositoryID   int
	RepositoryName string
	Commits        []string
}

type store struct {
	logger     logger.Logger
	db         *basestore.Store
	operations *operations
}

func New(observationCtx *observation.Context, db database.DB) Store {
	return &store{
		logger:     logger.Scoped("uploads.store"),
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

func (s *store) Handle() *basestore.Store {
	return s.db
}
