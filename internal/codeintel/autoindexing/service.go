package autoindexing

import (
	"context"
	"time"

	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/enqueuer"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/inference"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/jobselector"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/store"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Service struct {
	store           store.Store
	repoStore       database.RepoStore
	gitserverClient gitserver.Client
	indexEnqueuer   *enqueuer.IndexEnqueuer
	jobSelector     *jobselector.JobSelector
	operations      *operations
}

func newService(
	observationCtx *observation.Context,
	store store.Store,
	inferenceSvc InferenceService,
	repoStore database.RepoStore,
	gitserverClient gitserver.Client,
) *Service {
	// NOTE - this should go up a level in init.go.
	// Not going to do this now so that we don't blow up all of the
	// tests (which have pretty good coverage of the whole service).
	// We should rewrite/transplant tests to the closest package that
	// provides that behavior and then mock the dependencies in the
	// glue packages.

	jobSelector := jobselector.NewJobSelector(
		store,
		repoStore,
		inferenceSvc,
		gitserverClient,
		log.Scoped("autoindexing job selector"),
	)

	indexEnqueuer := enqueuer.NewIndexEnqueuer(
		observationCtx,
		store,
		repoStore,
		gitserverClient,
		jobSelector,
	)

	return &Service{
		store:           store,
		repoStore:       repoStore,
		gitserverClient: gitserverClient,
		indexEnqueuer:   indexEnqueuer,
		jobSelector:     jobSelector,
		operations:      newOperations(observationCtx),
	}
}

func (s *Service) GetIndexConfigurationByRepositoryID(ctx context.Context, repositoryID int) (shared.IndexConfiguration, bool, error) {
	return s.store.GetIndexConfigurationByRepositoryID(ctx, repositoryID)
}

// InferIndexConfiguration looks at the repository contents at the latest commit on the default branch of the given
// repository and determines an index configuration that is likely to succeed.
func (s *Service) InferIndexConfiguration(ctx context.Context, repositoryID int, commit string, localOverrideScript string, bypassLimit bool) (_ *shared.InferenceResult, err error) {
	ctx, trace, endObservation := s.operations.inferIndexConfiguration.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", repositoryID),
	}})
	defer endObservation(1, observation.Args{})

	repo, err := s.repoStore.Get(ctx, api.RepoID(repositoryID))
	if err != nil {
		return nil, err
	}

	if commit == "" {
		_, commitSHA, err := s.gitserverClient.GetDefaultBranch(ctx, repo.Name, false)
		if err != nil {
			return nil, errors.Wrapf(err, "gitserver.GetDefaultBranch: error resolving HEAD for %d", repositoryID)
		}
		// If we're dealing with an empty repo, we can't infer anything.
		if commitSHA == "" {
			return nil, nil
		}
		commit = string(commitSHA)
	} else {
		// Verify that the commit exists.
		_, err := s.gitserverClient.GetCommit(ctx, repo.Name, api.CommitID(commit))
		if errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
			return nil, errors.Newf("revision %s not found for %d", commit, repositoryID)
		}
		if err != nil {
			return nil, errors.Wrapf(err, "gitserver.CommitExists: error checking %s for %d", commit, repositoryID)
		}
	}
	trace.AddEvent("found", attribute.String("commit", commit))

	return s.InferIndexJobsFromRepositoryStructure(ctx, repositoryID, commit, localOverrideScript, bypassLimit)
}

func (s *Service) UpdateIndexConfigurationByRepositoryID(ctx context.Context, repositoryID int, data []byte) error {
	return s.store.UpdateIndexConfigurationByRepositoryID(ctx, repositoryID, data)
}

func (s *Service) QueueRepoRev(ctx context.Context, repositoryID int, rev string) error {
	return s.store.QueueRepoRev(ctx, repositoryID, rev)
}

func (s *Service) SetInferenceScript(ctx context.Context, script string) error {
	return s.store.SetInferenceScript(ctx, script)
}

func (s *Service) GetInferenceScript(ctx context.Context) (string, error) {
	return s.store.GetInferenceScript(ctx)
}

func (s *Service) QueueIndexes(ctx context.Context, repositoryID int, rev, configuration string, force, bypassLimit bool) ([]uploadsshared.Index, error) {
	return s.indexEnqueuer.QueueIndexes(ctx, repositoryID, rev, configuration, force, bypassLimit)
}

func (s *Service) QueueIndexesForPackage(ctx context.Context, pkg dependencies.MinimialVersionedPackageRepo) error {
	return s.indexEnqueuer.QueueIndexesForPackage(ctx, pkg)
}

func (s *Service) InferIndexJobsFromRepositoryStructure(ctx context.Context, repositoryID int, commit string, localOverrideScript string, bypassLimit bool) (*shared.InferenceResult, error) {
	return s.jobSelector.InferIndexJobsFromRepositoryStructure(ctx, repositoryID, commit, localOverrideScript, bypassLimit)
}

func IsLimitError(err error) bool {
	return errors.As(err, &inference.LimitError{})
}

func (s *Service) RepositoryIDsWithConfiguration(ctx context.Context, offset, limit int) ([]uploadsshared.RepositoryWithAvailableIndexers, int, error) {
	return s.store.RepositoryIDsWithConfiguration(ctx, offset, limit)
}

func (s *Service) GetLastIndexScanForRepository(ctx context.Context, repositoryID int) (*time.Time, error) {
	return s.store.GetLastIndexScanForRepository(ctx, repositoryID)
}
