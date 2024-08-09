package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/internal/actor"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/compute"
	"github.com/sourcegraph/sourcegraph/internal/insights/discovery"
	"github.com/sourcegraph/sourcegraph/internal/insights/priority"
	"github.com/sourcegraph/sourcegraph/internal/insights/query/querybuilder"
	"github.com/sourcegraph/sourcegraph/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/insights/timeseries"
	"github.com/sourcegraph/sourcegraph/internal/insights/types"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// newBackfillHandler - Handles backfill that are in the "new" state
// The new state is the initial state post creation of a series.  This handler is responsible only for determining the work
// that needs to be completed to backfill this series.  It then requeues the backfill record into "processing" to perform the actual backfill work.
type newBackfillHandler struct {
	workerStore     dbworkerstore.Store[*BaseJob]
	backfillStore   *BackfillStore
	seriesReader    SeriesReader
	repoIterator    discovery.SeriesRepoIterator
	costAnalyzer    priority.QueryAnalyzer
	timeseriesStore store.Interface
}

// makeNewBackfillWorker makes a new Worker, Resetter and Store to handle the queue of Backfill jobs that are in the state of "New"
func makeNewBackfillWorker(ctx context.Context, config JobMonitorConfig) (*workerutil.Worker[*BaseJob], *dbworker.Resetter[*BaseJob], dbworkerstore.Store[*BaseJob]) {
	insightsDB := config.InsightsDB
	backfillStore := NewBackfillStore(insightsDB)

	name := "backfill_new_backfill_worker"

	workerStore := dbworkerstore.New(config.ObservationCtx, insightsDB.Handle(), dbworkerstore.Options[*BaseJob]{
		Name:              fmt.Sprintf("%s_store", name),
		TableName:         "insights_background_jobs",
		ViewName:          "insights_jobs_backfill_new",
		ColumnExpressions: baseJobColumns,
		Scan:              dbworkerstore.BuildWorkerScan(scanBaseJob),
		OrderByExpression: sqlf.Sprintf("id"), // processes oldest records first
		MaxNumResets:      100,
		StalledMaxAge:     time.Second * 30,
		RetryAfter:        time.Second * 30,
		MaxNumRetries:     3,
	})

	task := newBackfillHandler{
		workerStore:     workerStore,
		backfillStore:   backfillStore,
		seriesReader:    store.NewInsightStore(insightsDB),
		repoIterator:    discovery.NewSeriesRepoIterator(config.AllRepoIterator, config.RepoStore, config.RepoQueryExecutor),
		costAnalyzer:    *config.CostAnalyzer,
		timeseriesStore: config.InsightStore,
	}

	worker := dbworker.NewWorker(ctx, workerStore, workerutil.Handler[*BaseJob](&task), workerutil.WorkerOptions{
		Name:              name,
		Description:       "determines the repos for a code insight and an approximate cost of the backfill",
		NumHandlers:       1,
		Interval:          5 * time.Second,
		HeartbeatInterval: 15 * time.Second,
		Metrics:           workerutil.NewMetrics(config.ObservationCtx, name),
	})

	resetter := dbworker.NewResetter(ctx, log.Scoped("BackfillNewResetter"), workerStore, dbworker.ResetterOptions{
		Name:     fmt.Sprintf("%s_resetter", name),
		Interval: time.Second * 20,
		Metrics:  dbworker.NewResetterMetrics(config.ObservationCtx, name),
	})

	return worker, resetter, workerStore
}

var _ workerutil.Handler[*BaseJob] = &newBackfillHandler{}

func (h *newBackfillHandler) Handle(ctx context.Context, logger log.Logger, job *BaseJob) (err error) {
	logger.Info("newBackfillHandler called", log.Int("recordId", job.RecordID()))

	// 🚨 SECURITY: we use the internal actor because all of the work is background work and not scoped to users
	ctx = actor.WithInternalActor(ctx)

	// setup transactions
	tx, err := h.backfillStore.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	// load backfill and series
	backfill, err := tx.LoadBackfill(ctx, job.backfillId)
	if err != nil {
		return errors.Wrap(err, "loadBackfill")
	}
	series, err := h.seriesReader.GetDataSeriesByID(ctx, backfill.SeriesId)
	if err != nil {
		return errors.Wrap(err, "GetDataSeriesByID")
	}

	// set backfill repo scope
	repoIds := []int32{}
	reposIterator, err := h.repoIterator.ForSeries(ctx, series)
	if err != nil {
		return errors.Wrap(err, "repoIterator.SeriesRepoIterator")
	}
	err = reposIterator.ForEach(ctx, func(repoName string, id api.RepoID) error {
		repoIds = append(repoIds, int32(id))
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "reposIterator.ForEach")
	}

	queryPlan, err := parseQuery(*series)
	if err != nil {
		return errors.Wrap(err, "parseQuery")
	}

	cost := h.costAnalyzer.Cost(&priority.QueryObject{
		Query:                queryPlan,
		NumberOfRepositories: int64(len(repoIds)),
	})

	backfill, err = backfill.SetScope(ctx, tx, repoIds, cost)
	if err != nil {
		return errors.Wrap(err, "backfill.SetScope")
	}

	sampleTimes := timeseries.BuildSampleTimes(12, timeseries.TimeInterval{
		Unit:  types.IntervalUnit(series.SampleIntervalUnit),
		Value: series.SampleIntervalValue,
	}, series.CreatedAt.Truncate(time.Minute))

	if err := h.timeseriesStore.SetInsightSeriesRecordingTimes(ctx, []types.InsightSeriesRecordingTimes{
		{
			InsightSeriesID: series.ID,
			RecordingTimes:  timeseries.MakeRecordingsFromTimes(sampleTimes, false),
		},
	}); err != nil {
		return errors.Wrap(err, "NewBackfillHandler.SetInsightSeriesRecordingTimes")
	}

	// update series state
	err = backfill.setState(ctx, tx, BackfillStateProcessing)
	if err != nil {
		return errors.Wrap(err, "backfill.setState")
	}

	// enqueue backfill for next step in processing
	err = enqueueBackfill(ctx, tx.Handle(), backfill)
	if err != nil {
		return errors.Wrap(err, "backfill.enqueueBackfill")
	}
	// We have to manually manipulate the queue record here to ensure that the new job is written in the same tx
	// that this job is marked complete. This is how we will ensure there is no desync if the mark complete operation
	// fails after we've already queued up a new job.
	_, err = h.workerStore.MarkComplete(ctx, job.RecordID(), dbworkerstore.MarkFinalOptions{})
	if err != nil {
		return errors.Wrap(err, "backfill.MarkComplete")
	}
	return err
}

func parseQuery(series types.InsightSeries) (query.Plan, error) {
	if series.GeneratedFromCaptureGroups {
		seriesQuery, err := compute.Parse(series.Query)
		if err != nil {
			return nil, errors.Wrap(err, "compute.Parse")
		}
		searchQuery, err := seriesQuery.ToSearchQuery()
		if err != nil {
			return nil, errors.Wrap(err, "ToSearchQuery")
		}
		plan, err := querybuilder.ParseQuery(searchQuery, "regexp")
		if err != nil {
			return nil, errors.Wrap(err, "ParseQuery")
		}
		return plan, nil
	}

	plan, err := querybuilder.ParseQuery(series.Query, "literal")
	if err != nil {
		return nil, errors.Wrap(err, "ParseQuery")
	}
	return plan, nil
}
