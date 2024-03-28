package scheduler

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/inference"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/store"
	policiesshared "github.com/sourcegraph/sourcegraph/internal/codeintel/policies/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/reposcheduler"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type indexSchedulerJob struct {
	repoSchedulingSvc reposcheduler.RepositorySchedulingService
	policiesSvc       PoliciesService
	policyMatcher     PolicyMatcher
	indexEnqueuer     IndexEnqueuer
	repoStore         database.RepoStore
}

var m = new(metrics.SingletonREDMetrics)

func NewScheduler(
	observationCtx *observation.Context,
	repoSchedulingSvc reposcheduler.RepositorySchedulingService,
	policiesSvc PoliciesService,
	policyMatcher PolicyMatcher,
	indexEnqueuer IndexEnqueuer,
	repoStore database.RepoStore,
	config *Config,
) goroutine.BackgroundRoutine {
	job := indexSchedulerJob{
		repoSchedulingSvc: repoSchedulingSvc,
		policiesSvc:       policiesSvc,
		policyMatcher:     policyMatcher,
		indexEnqueuer:     indexEnqueuer,
		repoStore:         repoStore,
	}

	redMetrics := m.Get(func() *metrics.REDMetrics {
		return metrics.NewREDMetrics(
			observationCtx.Registerer,
			"codeintel_autoindexing_background",
			metrics.WithLabels("op"),
			metrics.WithCountHelp("Total number of method invocations."),
		)
	})

	return goroutine.NewPeriodicGoroutine(
		actor.WithInternalActor(context.Background()),
		goroutine.HandlerFunc(func(ctx context.Context) error {
			return job.handleScheduler(ctx, config.RepositoryProcessDelay, config.RepositoryBatchSize, config.PolicyBatchSize, config.InferenceConcurrency)
		}),
		goroutine.WithName("codeintel.autoindexing-background-scheduler"),
		goroutine.WithDescription("schedule autoindexing jobs in the background using defined or inferred configurations"),
		goroutine.WithInterval(config.SchedulerInterval),
		goroutine.WithOperation(observationCtx.Operation(observation.Op{
			Name:              "codeintel.indexing.HandleIndexSchedule",
			MetricLabelValues: []string{"HandleIndexSchedule"},
			Metrics:           redMetrics,
			ErrorFilter: func(err error) observation.ErrorFilterBehaviour {
				if errors.As(err, &inference.LimitError{}) {
					return observation.EmitForNone
				}
				return observation.EmitForDefault
			},
		})),
	)
}

// For mocking in tests
var autoIndexingEnabled = conf.CodeIntelAutoIndexingEnabled

func (b indexSchedulerJob) handleScheduler(
	ctx context.Context,
	repositoryProcessDelay time.Duration,
	repositoryBatchSize int,
	policyBatchSize int,
	inferenceConcurrency int,
) error {
	if !autoIndexingEnabled() {
		return nil
	}

	var repositoryMatchLimit *int
	if val := conf.CodeIntelAutoIndexingPolicyRepositoryMatchLimit(); val != -1 {
		repositoryMatchLimit = &val
	}

	// Get the batch of repositories that we'll handle in this invocation of the periodic goroutine. This
	// set should contain repositories that have yet to be updated, or that have been updated least recently.
	// This allows us to update every repository reliably, even if it takes a long time to process through
	// the backlog.
	repositories, err := b.repoSchedulingSvc.GetRepositoriesForIndexScan(
		ctx,
		reposcheduler.NewBatchOptions(
			repositoryProcessDelay,
			conf.CodeIntelAutoIndexingAllowGlobalPolicies(),
			repositoryMatchLimit,
			repositoryBatchSize),
		time.Now(),
	)
	if err != nil {
		return errors.Wrap(err, "uploadSvc.GetRepositoriesForIndexScan")
	}
	if len(repositories) == 0 {
		// All repositories updated recently enough
		return nil
	}

	now := timeutil.Now()

	// In parallel enqueue all the repos.
	var (
		sema  = semaphore.NewWeighted(int64(inferenceConcurrency))
		errs  error
		errMu sync.Mutex
	)

	for _, repository := range repositories {
		if err := sema.Acquire(ctx, 1); err != nil {
			return err
		}
		go func() {
			defer sema.Release(1)
			if repositoryErr := b.handleRepository(ctx, repository.ID, policyBatchSize, now); repositoryErr != nil {
				if !errors.As(err, &inference.LimitError{}) {
					errMu.Lock()
					errs = errors.Append(errs, repositoryErr)
					errMu.Unlock()
				}
			}
		}()
	}

	if err := sema.Acquire(ctx, int64(inferenceConcurrency)); err != nil {
		return errors.Wrap(err, "acquiring semaphore")
	}

	return errs
}

func (b indexSchedulerJob) handleRepository(ctx context.Context, repositoryID, policyBatchSize int, now time.Time) error {
	repo, err := b.repoStore.Get(ctx, api.RepoID(repositoryID))
	if err != nil {
		return err
	}

	var (
		t        = true
		offset   = 0
		repoName = repo.Name
	)

	for {
		// Retrieve the set of configuration policies that affect indexing for this repository.
		policies, totalCount, err := b.policiesSvc.GetConfigurationPolicies(ctx, policiesshared.GetConfigurationPoliciesOptions{
			RepositoryID: repositoryID,
			ForIndexing:  &t,
			Limit:        policyBatchSize,
			Offset:       offset,
		})
		if err != nil {
			return errors.Wrap(err, "policySvc.GetConfigurationPolicies")
		}
		offset += len(policies)

		// Get the set of commits within this repository that match an indexing policy
		commitMap, err := b.policyMatcher.CommitsDescribedByPolicy(ctx, repositoryID, repoName, policies, now)
		if err != nil {
			return errors.Wrap(err, "policies.CommitsDescribedByPolicy")
		}

		for commit, policyMatches := range commitMap {
			if len(policyMatches) == 0 {
				continue
			}

			// Attempt to queue an index if one does not exist for each of the matching commits
			if _, err := b.indexEnqueuer.QueueIndexes(ctx, repositoryID, commit, "", false, false); err != nil {
				if errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
					continue
				}

				return errors.Wrap(err, "indexEnqueuer.QueueIndexes")
			}
		}

		if len(policies) == 0 || offset >= totalCount {
			return nil
		}
	}
}

func NewOnDemandScheduler(s store.Store, indexEnqueuer IndexEnqueuer, config *Config) goroutine.BackgroundRoutine {
	return goroutine.NewPeriodicGoroutine(
		actor.WithInternalActor(context.Background()),
		goroutine.HandlerFunc(func(ctx context.Context) error {
			if !autoIndexingEnabled() {
				return nil
			}

			return s.WithTransaction(ctx, func(tx store.Store) error {
				repoRevs, err := tx.GetQueuedRepoRev(ctx, config.OnDemandBatchsize)
				if err != nil {
					return err
				}

				ids := make([]int, 0, len(repoRevs))
				for _, repoRev := range repoRevs {
					if _, err := indexEnqueuer.QueueIndexes(ctx, repoRev.RepositoryID, repoRev.Rev, "", false, false); err != nil {
						return err
					}

					ids = append(ids, repoRev.ID)
				}

				return tx.MarkRepoRevsAsProcessed(ctx, ids)
			})
		}),
		goroutine.WithName("codeintel.autoindexing-ondemand-scheduler"),
		goroutine.WithDescription("schedule autoindexing jobs for explicitly requested repo+revhash combinations"),
		goroutine.WithInterval(config.OnDemandSchedulerInterval),
	)
}
