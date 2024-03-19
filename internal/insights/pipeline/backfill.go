package pipeline

import (
	"context"
	"sync"
	"time"

	"github.com/derision-test/glock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	internalGitserver "github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/insights/background/queryrunner"
	"github.com/sourcegraph/sourcegraph/internal/insights/compression"
	"github.com/sourcegraph/sourcegraph/internal/insights/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/insights/query/querybuilder"
	"github.com/sourcegraph/sourcegraph/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/insights/types"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	itypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type BackfillRequest struct {
	Series      *types.InsightSeries
	Repo        *itypes.MinimalRepo
	SampleTimes []time.Time
}

type requestContext struct {
	backfillRequest *BackfillRequest
}

type Backfiller interface {
	Run(ctx context.Context, request BackfillRequest) error
}

type GitCommitClient interface {
	FirstCommit(ctx context.Context, repoName api.RepoName) (*gitdomain.Commit, error)
	RecentCommits(ctx context.Context, repoName api.RepoName, target time.Time, revision string) ([]*gitdomain.Commit, error)
	GitserverClient() internalGitserver.Client
}

var _ GitCommitClient = (*gitserver.GitCommitClient)(nil)

type SearchJobGenerator func(ctx context.Context, req requestContext) (*requestContext, []*queryrunner.SearchJob, error)
type SearchRunner func(ctx context.Context, reqContext *requestContext, jobs []*queryrunner.SearchJob) (*requestContext, []store.RecordSeriesPointArgs, error)
type ResultsPersister func(ctx context.Context, reqContext *requestContext, points []store.RecordSeriesPointArgs) (*requestContext, error)

type BackfillerConfig struct {
	CommitClient    GitCommitClient
	CompressionPlan compression.DataFrameFilter
	SearchHandlers  map[types.GenerationMethod]queryrunner.InsightsHandler
	InsightStore    store.Interface

	SearchPlanWorkerLimit   int
	SearchRunnerWorkerLimit int
	SearchRateLimiter       *ratelimit.InstrumentedLimiter
	HistoricRateLimiter     *ratelimit.InstrumentedLimiter
}

func NewDefaultBackfiller(config BackfillerConfig) Backfiller {
	logger := log.Scoped("insightsBackfiller")
	searchJobGenerator := makeSearchJobsFunc(logger, config.CommitClient, config.CompressionPlan, config.SearchPlanWorkerLimit, config.HistoricRateLimiter)
	searchRunner := makeRunSearchFunc(config.SearchHandlers, config.SearchRunnerWorkerLimit, config.SearchRateLimiter)
	persister := makeSaveResultsFunc(logger, config.InsightStore)
	return newBackfiller(searchJobGenerator, searchRunner, persister, glock.NewRealClock())

}

func newBackfiller(jobGenerator SearchJobGenerator, searchRunner SearchRunner, resultsPersister ResultsPersister, clock glock.Clock) Backfiller {
	return &backfiller{
		searchJobGenerator: jobGenerator,
		searchRunner:       searchRunner,
		persister:          resultsPersister,
		clock:              clock,
	}

}

type backfiller struct {
	// dependencies
	searchJobGenerator SearchJobGenerator
	searchRunner       SearchRunner
	persister          ResultsPersister

	clock glock.Clock
}

var backfillMetrics = metrics.NewREDMetrics(prometheus.DefaultRegisterer, "insights_repo_backfill", metrics.WithLabels("step"))

func (b *backfiller) Run(ctx context.Context, req BackfillRequest) error {

	// setup
	startingReqContext := requestContext{backfillRequest: &req}
	start := b.clock.Now()

	step1ReqContext, searchJobs, jobErr := b.searchJobGenerator(ctx, startingReqContext)
	endGenerateJobs := b.clock.Now()
	backfillMetrics.Observe(endGenerateJobs.Sub(start).Seconds(), 1, &jobErr, "generate_jobs")
	if jobErr != nil {
		return jobErr
	}

	step2ReqContext, recordings, searchErr := b.searchRunner(ctx, step1ReqContext, searchJobs)
	endSearchRunner := b.clock.Now()
	backfillMetrics.Observe(endSearchRunner.Sub(endGenerateJobs).Seconds(), 1, &searchErr, "run_searches")
	if searchErr != nil {
		return searchErr
	}

	_, saveErr := b.persister(ctx, step2ReqContext, recordings)
	endPersister := b.clock.Now()
	backfillMetrics.Observe(endPersister.Sub(endSearchRunner).Seconds(), 1, &saveErr, "save_results")
	return saveErr
}

// Implementation of steps for Backfill process
var compressionSavingsMetric = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "src_insights_backfill_searches_per_frame",
	Help:    "the ratio of searches per frame for insights backfills",
	Buckets: prometheus.LinearBuckets(.1, .1, 10),
}, []string{"preempted"})

func makeSearchJobsFunc(logger log.Logger, commitClient GitCommitClient, compressionPlan compression.DataFrameFilter, searchJobWorkerLimit int, rateLimit *ratelimit.InstrumentedLimiter) SearchJobGenerator {
	return func(ctx context.Context, reqContext requestContext) (*requestContext, []*queryrunner.SearchJob, error) {
		numberOfSamples := len(reqContext.backfillRequest.SampleTimes)
		jobs := make([]*queryrunner.SearchJob, 0, numberOfSamples)
		if reqContext.backfillRequest == nil {
			return &reqContext, jobs, errors.New("backfill request provided")
		}
		req := reqContext.backfillRequest
		buildJob := makeHistoricalSearchJobFunc(logger, commitClient)
		logger.Debug("making search plan")
		// Find the first commit made to the repository on the default branch.
		firstHEADCommit, err := commitClient.FirstCommit(ctx, req.Repo.Name)
		if err != nil {
			if errors.Is(err, gitserver.EmptyRepoErr) {
				// This is fine it's empty there is no work to be done
				compressionSavingsMetric.
					With(prometheus.Labels{"preempted": "true"}).
					Observe(0)
				return &reqContext, jobs, nil
			}

			return &reqContext, jobs, err
		}
		// Rate limit starting compression
		err = rateLimit.Wait(ctx)
		if err != nil {
			return &reqContext, jobs, err
		}
		searchPlan := compressionPlan.Filter(ctx, req.SampleTimes, req.Repo.Name)
		ratio := 1.0
		if numberOfSamples > 0 {
			ratio = float64(len(searchPlan.Executions)) / float64(numberOfSamples)
		}
		compressionSavingsMetric.
			With(prometheus.Labels{"preempted": "false"}).
			Observe(ratio)
		mu := &sync.Mutex{}

		groupContext, groupCancel := context.WithCancel(ctx)
		defer groupCancel()
		p := pool.New().WithContext(groupContext).WithMaxGoroutines(searchJobWorkerLimit).WithCancelOnError()
		for i := len(searchPlan.Executions) - 1; i >= 0; i-- {
			execution := searchPlan.Executions[i]
			p.Go(func(ctx context.Context) error {
				// Build historical data for this unique timeframe+repo+series.
				err, job, _ := buildJob(ctx, &buildSeriesContext{
					execution:       execution,
					repoName:        req.Repo.Name,
					id:              req.Repo.ID,
					firstHEADCommit: firstHEADCommit,
					seriesID:        req.Series.SeriesID,
					series:          req.Series,
				})
				mu.Lock()
				defer mu.Unlock()
				if job != nil {
					jobs = append(jobs, job)
				}
				return err
			})
		}
		err = p.Wait()
		if err != nil {
			jobs = nil
		}
		return &reqContext, jobs, err
	}
}

type buildSeriesContext struct {
	// The timeframe we're building historical data for.
	execution compression.QueryExecution

	// The repository we're building historical data for.
	id       api.RepoID
	repoName api.RepoName

	// The first commit made in the repository on the default branch.
	firstHEADCommit *gitdomain.Commit

	// The series we're building historical data for.
	seriesID string
	series   *types.InsightSeries
}

type searchJobFunc func(ctx context.Context, bctx *buildSeriesContext) (err error, job *queryrunner.SearchJob, preempted []store.RecordSeriesPointArgs)

func makeHistoricalSearchJobFunc(logger log.Logger, commitClient GitCommitClient) searchJobFunc {
	return func(ctx context.Context, bctx *buildSeriesContext) (err error, job *queryrunner.SearchJob, preempted []store.RecordSeriesPointArgs) {
		logger.Debug("making search job")
		rawQuery := bctx.series.Query
		containsRepo, err := querybuilder.ContainsField(rawQuery, query.FieldRepo)
		if err != nil {
			return err, nil, nil
		}
		if containsRepo {
			// This maintains existing behavior that searches with a repo filter are ignored
			return nil, nil, nil
		}

		// Optimization: If the timeframe we're building data for starts (or ends) before the first commit in the
		// repository, then we know there are no results (the repository didn't have any commits at all
		// at that point in time.)
		repoName := string(bctx.repoName)
		if bctx.execution.RecordingTime.Before(bctx.firstHEADCommit.Author.Date) {
			return err, nil, nil
		}

		revision := bctx.execution.Revision
		if len(bctx.execution.Revision) == 0 {
			recentCommits, revErr := commitClient.RecentCommits(ctx, bctx.repoName, bctx.execution.RecordingTime, "")
			if revErr != nil {
				if errors.HasType(revErr, &gitdomain.RevisionNotFoundError{}) || gitdomain.IsRepoNotExist(revErr) {
					return // no error - repo may not be cloned yet (or not even pushed to code host yet)
				}
				err = errors.Append(err, errors.Wrap(revErr, "FindNearestCommit"))
				return
			}
			var nearestCommit *gitdomain.Commit
			if len(recentCommits) > 0 {
				nearestCommit = recentCommits[0]
			}
			if nearestCommit == nil {
				// a.statistics[bctx.seriesID].Errored += 1
				return // repository has no commits / is empty. Maybe not yet pushed to code host.
			}
			if nearestCommit.Committer == nil {
				// a.statistics[bctx.seriesID].Errored += 1
				return
			}
			revision = string(nearestCommit.ID)
		}

		// Construct the search query that will generate data for this repository and time (revision) tuple.
		var newQueryStr string
		modifiedQuery, err := querybuilder.SingleRepoQuery(querybuilder.BasicQuery(rawQuery), repoName, revision, querybuilder.CodeInsightsQueryDefaults(len(bctx.series.Repositories) == 0))
		if err != nil {
			err = errors.Append(err, errors.Wrap(err, "SingleRepoQuery"))
			return
		}
		newQueryStr = modifiedQuery.String()
		if bctx.series.GroupBy != nil {
			computeQuery, computeErr := querybuilder.ComputeInsightCommandQuery(modifiedQuery, querybuilder.MapType(*bctx.series.GroupBy), commitClient.GitserverClient())
			if computeErr != nil {
				err = errors.Append(err, errors.Wrap(err, "ComputeInsightCommandQuery"))
				return
			}
			newQueryStr = computeQuery.String()
		}

		job = &queryrunner.SearchJob{
			SeriesID:        bctx.seriesID,
			SearchQuery:     newQueryStr,
			RecordTime:      &bctx.execution.RecordingTime,
			PersistMode:     string(store.RecordMode),
			DependentFrames: bctx.execution.SharedRecordings,
		}
		return err, job, preempted
	}
}

func makeRunSearchFunc(searchHandlers map[types.GenerationMethod]queryrunner.InsightsHandler, searchWorkerLimit int, rateLimiter *ratelimit.InstrumentedLimiter) SearchRunner {
	return func(ctx context.Context, reqContext *requestContext, jobs []*queryrunner.SearchJob) (*requestContext, []store.RecordSeriesPointArgs, error) {
		points := make([]store.RecordSeriesPointArgs, 0, len(jobs))
		series := reqContext.backfillRequest.Series
		mu := &sync.Mutex{}
		groupContext, groupCancel := context.WithCancel(ctx)
		defer groupCancel()
		p := pool.New().WithContext(groupContext).WithMaxGoroutines(searchWorkerLimit).WithCancelOnError()
		for i := range len(jobs) {
			job := jobs[i]
			p.Go(func(ctx context.Context) error {
				h := searchHandlers[series.GenerationMethod]
				err := rateLimiter.Wait(ctx)
				if err != nil {
					return errors.Wrap(err, "rateLimiter.Wait")
				}
				searchPoints, err := h(ctx, job, series, *job.RecordTime)
				if err != nil {
					return err
				}
				mu.Lock()
				defer mu.Unlock()
				points = append(points, searchPoints...)
				return nil
			})
		}
		err := p.Wait()
		// don't return any points if they don't all succeed
		if err != nil {
			points = nil
		}
		return reqContext, points, err
	}
}

func makeSaveResultsFunc(logger log.Logger, insightStore store.Interface) ResultsPersister {
	return func(ctx context.Context, reqContext *requestContext, points []store.RecordSeriesPointArgs) (*requestContext, error) {
		if ctx.Err() != nil {
			return reqContext, ctx.Err()
		}
		logger.Debug("writing search results")
		err := insightStore.RecordSeriesPoints(ctx, points)
		return reqContext, err
	}

}
