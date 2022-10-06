package uploads

import (
	"context"
	"fmt"
	"time"

	"github.com/derision-test/glock"
	"github.com/opentracing/opentracing-go/log"
	logger "github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/types"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/lsifstore"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/store"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// var _ service = (*Service)(nil)

// TODO - remove or re-sync with implementation
type service interface {
	// Commits
	GetOldestCommitDate(ctx context.Context, repositoryID int) (time.Time, bool, error)
	GetCommitsVisibleToUpload(ctx context.Context, uploadID, limit int, token *string) (_ []string, nextToken *string, err error)
	GetStaleSourcedCommits(ctx context.Context, minimumTimeSinceLastCheck time.Duration, limit int, now time.Time) (_ []shared.SourcedCommits, err error)
	GetCommitGraphMetadata(ctx context.Context, repositoryID int) (stale bool, updatedAt *time.Time, err error)
	UpdateSourcedCommits(ctx context.Context, repositoryID int, commit string, now time.Time) (uploadsUpdated int, err error)
	DeleteSourcedCommits(ctx context.Context, repositoryID int, commit string, maximumCommitLag time.Duration, now time.Time) (uploadsUpdated int, uploadsDeleted int, err error)

	// Repositories
	GetRepoName(ctx context.Context, repositoryID int) (_ string, err error)
	GetRepositoriesForIndexScan(ctx context.Context, table, column string, processDelay time.Duration, allowGlobalPolicies bool, repositoryMatchLimit *int, limit int, now time.Time) (_ []int, err error)
	GetRepositoriesMaxStaleAge(ctx context.Context) (_ time.Duration, err error)
	GetDirtyRepositories(ctx context.Context) (_ map[int]int, err error)
	SetRepositoryAsDirty(ctx context.Context, repositoryID int) (err error)
	SetRepositoriesForRetentionScan(ctx context.Context, processDelay time.Duration, limit int) (_ []int, err error)

	// Uploads
	GetUploads(ctx context.Context, opts types.GetUploadsOptions) (uploads []types.Upload, totalCount int, err error)
	GetUploadByID(ctx context.Context, id int) (_ types.Upload, _ bool, err error)
	GetUploadsByIDs(ctx context.Context, ids ...int) (_ []types.Upload, err error)
	GetUploadIDsWithReferences(ctx context.Context, orderedMonikers []precise.QualifiedMonikerData, ignoreIDs []int, repositoryID int, commit string, limit int, offset int) (ids []int, recordsScanned int, totalCount int, err error)
	GetUploadDocumentsForPath(ctx context.Context, bundleID int, pathPattern string) ([]string, int, error)
	GetRecentUploadsSummary(ctx context.Context, repositoryID int) (upload []shared.UploadsWithRepositoryNamespace, err error)
	GetLastUploadRetentionScanForRepository(ctx context.Context, repositoryID int) (_ *time.Time, err error)
	UpdateUploadsVisibleToCommits(ctx context.Context, repositoryID int, graph *gitdomain.CommitGraph, refDescriptions map[string][]gitdomain.RefDescription, maxAgeForNonStaleBranches, maxAgeForNonStaleTags time.Duration, dirtyToken int, now time.Time) error
	UpdateUploadRetention(ctx context.Context, protectedIDs, expiredIDs []int) (err error)
	UpdateUploadsReferenceCounts(ctx context.Context, ids []int, dependencyUpdateType shared.DependencyReferenceCountUpdateType) (updated int, err error)
	SoftDeleteExpiredUploads(ctx context.Context) (count int, err error)
	HardDeleteExpiredUploads(ctx context.Context) (count int, err error)
	DeleteUploadsStuckUploading(ctx context.Context, uploadedBefore time.Time) (_ int, err error)
	DeleteUploadsWithoutRepository(ctx context.Context, now time.Time) (_ map[int]int, err error)
	DeleteUploadByID(ctx context.Context, id int) (_ bool, err error)
	InferClosestUploads(ctx context.Context, repositoryID int, commit, path string, exactPath bool, indexer string) ([]types.Dump, error)

	// Dumps
	FindClosestDumps(ctx context.Context, repositoryID int, commit, path string, rootMustEnclosePath bool, indexer string) (_ []types.Dump, err error)
	FindClosestDumpsFromGraphFragment(ctx context.Context, repositoryID int, commit, path string, rootMustEnclosePath bool, indexer string, commitGraph *gitdomain.CommitGraph) (_ []types.Dump, err error)
	GetDumpsWithDefinitionsForMonikers(ctx context.Context, monikers []precise.QualifiedMonikerData) (_ []types.Dump, err error)
	GetDumpsByIDs(ctx context.Context, ids []int) (_ []types.Dump, err error)

	// Packages
	UpdatePackages(ctx context.Context, dumpID int, packages []precise.Package) (err error)

	// References
	UpdatePackageReferences(ctx context.Context, dumpID int, references []precise.PackageReference) (err error)
	ReferencesForUpload(ctx context.Context, uploadID int) (_ shared.PackageReferenceScanner, err error)

	// Audit Logs
	GetAuditLogsForUpload(ctx context.Context, uploadID int) (_ []types.UploadLog, err error)
	DeleteOldAuditLogs(ctx context.Context, maxAge time.Duration, now time.Time) (count int, err error)

	// Tags
	GetListTags(ctx context.Context, repo api.RepoName, commitObjs ...string) (_ []*gitdomain.Tag, err error)
}

type Service struct {
	store             store.Store
	repoStore         RepoStore
	workerutilStore   dbworkerstore.Store
	lsifstore         lsifstore.LsifStore
	gitserverClient   GitserverClient
	policySvc         PolicyService
	autoIndexingSvc   AutoIndexingService
	expirationMetrics *expirationMetrics
	resetterMetrics   *resetterMetrics
	janitorMetrics    *janitorMetrics
	policyMatcher     PolicyMatcher
	locker            Locker
	logger            logger.Logger
	operations        *operations
	clock             glock.Clock
}

func newService(
	store store.Store,
	repoStore RepoStore,
	lsifstore lsifstore.LsifStore,
	gsc GitserverClient,
	policySvc PolicyService,
	autoindexingSvc AutoIndexingService,
	policyMatcher PolicyMatcher,
	locker Locker,
	observationContext *observation.Context,
) *Service {
	workerutilStore := store.WorkerutilStore(observationContext)

	// TODO - move this to metric reporter?
	dbworker.InitPrometheusMetric(observationContext, workerutilStore, "codeintel", "upload", nil)

	return &Service{
		store:             store,
		repoStore:         repoStore,
		workerutilStore:   workerutilStore,
		lsifstore:         lsifstore,
		gitserverClient:   gsc,
		policySvc:         policySvc,
		autoIndexingSvc:   autoindexingSvc,
		expirationMetrics: newExpirationMetrics(observationContext),
		resetterMetrics:   newResetterMetrics(observationContext),
		janitorMetrics:    newJanitorMetrics(observationContext),
		policyMatcher:     policyMatcher,
		locker:            locker,
		logger:            observationContext.Logger,
		operations:        newOperations(observationContext),
		clock:             glock.NewRealClock(),
	}
}

func (s *Service) GetCommitGraphMetadata(ctx context.Context, repositoryID int) (stale bool, updatedAt *time.Time, err error) {
	ctx, _, endObservation := s.operations.getCommitGraphMetadata.With(ctx, &err, observation.Args{LogFields: []log.Field{log.Int("repositoryID", repositoryID)}})
	defer endObservation(1, observation.Args{})

	return s.store.GetCommitGraphMetadata(ctx, repositoryID)
}

func (s *Service) GetCommitsVisibleToUpload(ctx context.Context, uploadID, limit int, token *string) (_ []string, nextToken *string, err error) {
	ctx, _, endObservation := s.operations.getCommitsVisibleToUpload.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	return s.store.GetCommitsVisibleToUpload(ctx, uploadID, limit, token)
}

func (s *Service) GetRepoName(ctx context.Context, repositoryID int) (_ string, err error) {
	ctx, _, endObservation := s.operations.getRepoName.With(ctx, &err, observation.Args{LogFields: []log.Field{log.Int("repositoryID", repositoryID)}})
	defer endObservation(1, observation.Args{})

	return s.store.RepoName(ctx, repositoryID)
}

func (s *Service) GetRepositoriesForIndexScan(ctx context.Context, table, column string, processDelay time.Duration, allowGlobalPolicies bool, repositoryMatchLimit *int, limit int, now time.Time) (_ []int, err error) {
	ctx, _, endObservation := s.operations.getRepositoriesForIndexScan.With(ctx, &err, observation.Args{
		LogFields: []log.Field{
			log.String("table", table),
			log.String("column", column),
			log.Int("processDelay in ms", int(processDelay.Milliseconds())),
			log.Bool("allowGlobalPolicies", allowGlobalPolicies),
			log.Int("limit", limit),
			log.String("now", now.String()),
		},
	})
	defer endObservation(1, observation.Args{})

	return s.store.GetRepositoriesForIndexScan(ctx, table, column, processDelay, allowGlobalPolicies, repositoryMatchLimit, limit, now)
}

func (s *Service) GetUploads(ctx context.Context, opts types.GetUploadsOptions) (uploads []types.Upload, totalCount int, err error) {
	ctx, _, endObservation := s.operations.getUploads.With(ctx, &err, observation.Args{
		LogFields: []log.Field{log.Int("repositoryID", opts.RepositoryID), log.String("state", opts.State), log.String("term", opts.Term)},
	})
	defer endObservation(1, observation.Args{})

	return s.store.GetUploads(ctx, opts)
}

// TODO: Not being used in the resolver layer
func (s *Service) GetUploadByID(ctx context.Context, id int) (_ types.Upload, _ bool, err error) {
	ctx, _, endObservation := s.operations.getUploadByID.With(ctx, &err, observation.Args{LogFields: []log.Field{log.Int("id", id)}})
	defer endObservation(1, observation.Args{})

	return s.store.GetUploadByID(ctx, id)
}

func (s *Service) GetUploadsByIDs(ctx context.Context, ids ...int) (_ []types.Upload, err error) {
	ctx, _, endObservation := s.operations.getUploadsByIDs.With(ctx, &err, observation.Args{LogFields: []log.Field{log.String("ids", fmt.Sprintf("%v", ids))}})
	defer endObservation(1, observation.Args{})

	return s.store.GetUploadsByIDs(ctx, ids...)
}

func (s *Service) GetUploadIDsWithReferences(ctx context.Context, orderedMonikers []precise.QualifiedMonikerData, ignoreIDs []int, repositoryID int, commit string, limit int, offset int) (ids []int, recordsScanned int, totalCount int, err error) {
	ctx, trace, endObservation := s.operations.getVisibleUploadsMatchingMonikers.With(ctx, &err, observation.Args{
		LogFields: []log.Field{
			log.Int("repositoryID", repositoryID),
			log.String("commit", commit),
			log.Int("limit", limit),
			log.Int("offset", offset),
			log.String("orderedMonikers", fmt.Sprintf("%v", orderedMonikers)),
			log.String("ignoreIDs", fmt.Sprintf("%v", ignoreIDs)),
		},
	})
	defer endObservation(1, observation.Args{})

	return s.store.GetUploadIDsWithReferences(ctx, orderedMonikers, ignoreIDs, repositoryID, commit, limit, offset, trace)
}

func (s *Service) DeleteUploadByID(ctx context.Context, id int) (_ bool, err error) {
	ctx, _, endObservation := s.operations.deleteUploadByID.With(ctx, &err, observation.Args{LogFields: []log.Field{log.Int("id", id)}})
	defer endObservation(1, observation.Args{})

	return s.store.DeleteUploadByID(ctx, id)
}

func (s *Service) DeleteUploads(ctx context.Context, opts types.DeleteUploadsOptions) (err error) {
	ctx, _, endObservation := s.operations.deleteUploadByID.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	return s.store.DeleteUploads(ctx, opts)
}

// numAncestors is the number of ancestors to query from gitserver when trying to find the closest
// ancestor we have data for. Setting this value too low (relative to a repository's commit rate)
// will cause requests for an unknown commit return too few results; setting this value too high
// will raise the latency of requests for an unknown commit.
//
// TODO(efritz) - make adjustable via site configuration
const numAncestors = 100

// inferClosestUploads will return the set of visible uploads for the given commit. If this commit is
// newer than our last refresh of the lsif_nearest_uploads table for this repository, then we will mark
// the repository as dirty and quickly approximate the correct set of visible uploads.
//
// Because updating the entire commit graph is a blocking, expensive, and lock-guarded process, we  want
// to only do that in the background and do something chearp in latency-sensitive paths. To construct an
// approximate result, we query gitserver for a (relatively small) set of ancestors for the given commit,
// correlate that with the upload data we have for those commits, and re-run the visibility algorithm over
// the graph. This will not always produce the full set of visible commits - some responses may not contain
// all results while a subsequent request made after the lsif_nearest_uploads has been updated to include
// this commit will.
func (s *Service) InferClosestUploads(ctx context.Context, repositoryID int, commit, path string, exactPath bool, indexer string) (_ []types.Dump, err error) {
	ctx, _, endObservation := s.operations.inferClosestUploads.With(ctx, &err, observation.Args{
		LogFields: []log.Field{log.Int("repositoryID", repositoryID), log.String("commit", commit), log.String("path", path), log.Bool("exactPath", exactPath), log.String("indexer", indexer)},
	})
	defer endObservation(1, observation.Args{})

	// The parameters exactPath and rootMustEnclosePath align here: if we're looking for dumps
	// that can answer queries for a directory (e.g. diagnostics), we want any dump that happens
	// to intersect the target directory. If we're looking for dumps that can answer queries for
	// a single file, then we need a dump with a root that properly encloses that file.
	if dumps, err := s.store.FindClosestDumps(ctx, repositoryID, commit, path, exactPath, indexer); err != nil {
		return nil, errors.Wrap(err, "store.FindClosestDumps")
	} else if len(dumps) != 0 {
		return dumps, nil
	}

	// Repository has no LSIF data at all
	if repositoryExists, err := s.store.HasRepository(ctx, repositoryID); err != nil {
		return nil, errors.Wrap(err, "dbstore.HasRepository")
	} else if !repositoryExists {
		return nil, nil
	}

	// Commit is known and the empty dumps list explicitly means nothing is visible
	if commitExists, err := s.store.HasCommit(ctx, repositoryID, commit); err != nil {
		return nil, errors.Wrap(err, "dbstore.HasCommit")
	} else if commitExists {
		return nil, nil
	}

	// Otherwise, the repository has LSIF data but we don't know about the commit. This commit
	// is probably newer than our last upload. Pull back a portion of the updated commit graph
	// and try to link it with what we have in the database. Then mark the repository's commit
	// graph as dirty so it's updated for subsequent requests.

	graph, err := s.gitserverClient.CommitGraph(ctx, repositoryID, gitserver.CommitGraphOptions{
		Commit: commit,
		Limit:  numAncestors,
	})
	if err != nil {
		return nil, errors.Wrap(err, "gitserverClient.CommitGraph")
	}

	dumps, err := s.store.FindClosestDumpsFromGraphFragment(ctx, repositoryID, commit, path, exactPath, indexer, graph)
	if err != nil {
		return nil, errors.Wrap(err, "dbstore.FindClosestDumpsFromGraphFragment")
	}

	if err := s.store.SetRepositoryAsDirty(ctx, repositoryID); err != nil {
		return nil, errors.Wrap(err, "dbstore.MarkRepositoryAsDirty")
	}

	return dumps, nil
}

func (s *Service) GetDumpsWithDefinitionsForMonikers(ctx context.Context, monikers []precise.QualifiedMonikerData) (_ []types.Dump, err error) {
	ctx, _, endObservation := s.operations.getDumpsWithDefinitionsForMonikers.With(ctx, &err, observation.Args{
		LogFields: []log.Field{log.String("monikers", fmt.Sprintf("%v", monikers))},
	})
	defer endObservation(1, observation.Args{})

	return s.store.GetDumpsWithDefinitionsForMonikers(ctx, monikers)
}

func (s *Service) GetDumpsByIDs(ctx context.Context, ids []int) (_ []types.Dump, err error) {
	ctx, _, endObservation := s.operations.getDumpsByIDs.With(ctx, &err, observation.Args{
		LogFields: []log.Field{log.Int("total_ids", len(ids)), log.String("ids", fmt.Sprintf("%v", ids))},
	})
	defer endObservation(1, observation.Args{})

	return s.store.GetDumpsByIDs(ctx, ids)
}

func (s *Service) ReferencesForUpload(ctx context.Context, uploadID int) (_ shared.PackageReferenceScanner, err error) {
	ctx, _, endObservation := s.operations.referencesForUpload.With(ctx, &err, observation.Args{
		LogFields: []log.Field{log.Int("uploadID", uploadID)},
	})
	defer endObservation(1, observation.Args{})

	return s.store.ReferencesForUpload(ctx, uploadID)
}

func (s *Service) GetAuditLogsForUpload(ctx context.Context, uploadID int) (_ []types.UploadLog, err error) {
	ctx, _, endObservation := s.operations.getAuditLogsForUpload.With(ctx, &err, observation.Args{
		LogFields: []log.Field{log.Int("uploadID", uploadID)},
	})
	defer endObservation(1, observation.Args{})

	return s.store.GetAuditLogsForUpload(ctx, uploadID)
}

func (s *Service) GetUploadDocumentsForPath(ctx context.Context, bundleID int, pathPattern string) (_ []string, _ int, err error) {
	ctx, _, endObservation := s.operations.getUploadDocumentsForPath.With(ctx, &err, observation.Args{
		LogFields: []log.Field{log.Int("bundleID", bundleID), log.String("pathPattern", pathPattern)},
	})
	defer endObservation(1, observation.Args{})

	return s.lsifstore.GetUploadDocumentsForPath(ctx, bundleID, pathPattern)
}

func (s *Service) GetRecentUploadsSummary(ctx context.Context, repositoryID int) (upload []shared.UploadsWithRepositoryNamespace, err error) {
	ctx, _, endObservation := s.operations.getRecentUploadsSummary.With(ctx, &err, observation.Args{
		LogFields: []log.Field{log.Int("repositoryID", repositoryID)},
	})
	defer endObservation(1, observation.Args{})

	return s.store.GetRecentUploadsSummary(ctx, repositoryID)
}

func (s *Service) GetLastUploadRetentionScanForRepository(ctx context.Context, repositoryID int) (_ *time.Time, err error) {
	ctx, _, endObservation := s.operations.getLastUploadRetentionScanForRepository.With(ctx, &err, observation.Args{
		LogFields: []log.Field{log.Int("repositoryID", repositoryID)},
	})
	defer endObservation(1, observation.Args{})

	return s.store.GetLastUploadRetentionScanForRepository(ctx, repositoryID)
}

func (s *Service) GetListTags(ctx context.Context, repo api.RepoName, commitObjs ...string) (_ []*gitdomain.Tag, err error) {
	ctx, _, endObservation := s.operations.getListTags.With(ctx, &err, observation.Args{
		LogFields: []log.Field{log.String("repo", string(repo)), log.String("commitObjs", fmt.Sprintf("%v", commitObjs))},
	})
	defer endObservation(1, observation.Args{})

	return s.gitserverClient.ListTags(ctx, repo, commitObjs...)
}

// NOTE: Used by autoindexing (for some reason?)
func (s *Service) GetDirtyRepositories(ctx context.Context) (_ map[int]int, err error) {
	ctx, _, endObservation := s.operations.getDirtyRepositories.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	return s.store.GetDirtyRepositories(ctx)
}
