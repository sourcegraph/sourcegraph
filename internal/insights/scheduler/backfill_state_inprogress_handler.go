package scheduler

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/derision-test/glock"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/insights/background/queryrunner"
	"github.com/sourcegraph/sourcegraph/internal/insights/pipeline"
	"github.com/sourcegraph/sourcegraph/internal/insights/scheduler/iterator"
	"github.com/sourcegraph/sourcegraph/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/insights/timeseries"
	itypes "github.com/sourcegraph/sourcegraph/internal/insights/types"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	defaultInterruptSeconds    = 60
	inProgressPollingInterval  = time.Second * 5
	defaultErrorThresholdFloor = 50
)

func makeInProgressWorker(ctx context.Context, config JobMonitorConfig) (*workerutil.Worker[*BaseJob], *dbworker.Resetter[*BaseJob], dbworkerstore.Store[*BaseJob]) {
	db := config.InsightsDB
	backfillStore := NewBackfillStore(db)

	name := "backfill_in_progress_worker"

	workerStore := dbworkerstore.New(config.ObservationCtx, db.Handle(), dbworkerstore.Options[*BaseJob]{
		Name:              fmt.Sprintf("%s_store", name),
		TableName:         "insights_background_jobs",
		ViewName:          "insights_jobs_backfill_in_progress",
		ColumnExpressions: baseJobColumns,
		Scan:              dbworkerstore.BuildWorkerScan(scanBaseJob),
		OrderByExpression: sqlf.Sprintf("estimated_cost, backfill_id"),
		MaxNumResets:      100,
		StalledMaxAge:     time.Second * 30,
		RetryAfter:        time.Second * 30,
		MaxNumRetries:     3,
	})

	task := &inProgressHandler{
		workerStore:        workerStore,
		backfillStore:      backfillStore,
		seriesReadComplete: store.NewInsightStore(db),
		insightsStore:      config.InsightStore,
		backfillRunner:     config.BackfillRunner,
		repoStore:          config.RepoStore,
		clock:              glock.NewRealClock(),
		config:             newHandlerConfig,
	}

	worker := dbworker.NewWorker(ctx, workerStore, workerutil.Handler[*BaseJob](task), workerutil.WorkerOptions{
		Name:              name,
		Description:       "generates and runs searches to backfill a code insight",
		NumHandlers:       1,
		Interval:          inProgressPollingInterval,
		HeartbeatInterval: 15 * time.Second,
		Metrics:           workerutil.NewMetrics(config.ObservationCtx, name),
	})

	resetter := dbworker.NewResetter(log.Scoped("resetter"), workerStore, dbworker.ResetterOptions{
		Name:     fmt.Sprintf("%s_resetter", name),
		Interval: time.Second * 20,
		Metrics:  dbworker.NewResetterMetrics(config.ObservationCtx, name),
	})

	return worker, resetter, workerStore
}

type inProgressHandler struct {
	workerStore        dbworkerstore.Store[*BaseJob]
	backfillStore      *BackfillStore
	seriesReadComplete SeriesReadBackfillComplete
	repoStore          database.RepoStore
	insightsStore      store.Interface
	backfillRunner     pipeline.Backfiller
	config             func() handlerConfig

	clock glock.Clock
}

type handlerConfig struct {
	interruptAfter      time.Duration
	errorThresholdFloor int
	pageSize            int
	repoConcurrency     int
}

func newHandlerConfig() handlerConfig {
	return handlerConfig{interruptAfter: getInterruptAfter(), errorThresholdFloor: getErrorThresholdFloor(), pageSize: getPageSize(), repoConcurrency: getRepoConcurrency()}
}

var _ workerutil.Handler[*BaseJob] = &inProgressHandler{}

func (h *inProgressHandler) Handle(ctx context.Context, logger log.Logger, job *BaseJob) error {
	ctx = actor.WithInternalActor(ctx)

	execution, err := h.load(ctx, logger, job.backfillId)
	if err != nil {
		return err
	}
	execution.config = h.config()

	logger.Info("insights backfill progress handler loaded",
		log.Int("recordId", job.RecordID()),
		log.Int("jobNumFailures", job.NumFailures),
		log.Int("seriesId", execution.series.ID),
		log.String("seriesUniqueId", execution.series.SeriesID),
		log.Int("backfillId", execution.backfill.Id),
		log.Int("repoTotalCount", execution.itr.TotalCount),
		log.Float64("percentComplete", execution.itr.PercentComplete),
		log.Int("erroredRepos", execution.itr.ErroredRepos()),
		log.Int("totalErrors", execution.itr.TotalErrors()))

	interrupt, err := h.doExecution(ctx, execution)
	if err != nil {
		return err
	}
	if interrupt {
		return h.doInterrupt(ctx, job)
	}
	return nil
}

type nextNFunc func(pageSize int, config iterator.IterationConfig) ([]api.RepoID, bool, iterator.FinishNFunc)

func (h *inProgressHandler) doExecution(ctx context.Context, execution *backfillExecution) (interrupt bool, err error) {
	config := h.config()

	timeExpired := h.clock.After(config.interruptAfter)

	itrConfig := iterator.IterationConfig{
		MaxFailures: 3,
		OnTerminal: func(ctx context.Context, tx *basestore.Store, repoId int32, terminalErr error) error {
			reason := translateIncompleteReasons(terminalErr)
			execution.logger.Debug("insights backfill incomplete repo writing all datapoints",
				execution.logFields(
					log.Int32("repoId", repoId),
					log.String("reason", string(reason)))...)

			id := int(repoId)
			for _, frame := range execution.sampleTimes {
				tss := h.insightsStore.WithOther(tx)
				if err := tss.AddIncompleteDatapoint(ctx, store.AddIncompleteDatapointInput{
					SeriesID: execution.series.ID,
					RepoID:   &id,
					Reason:   reason,
					Time:     frame,
				}); err != nil {
					return errors.Wrap(err, "AddIncompleteDatapoint")
				}
			}
			return nil
		},
	}

	itrLoop := func(pageSize, concurrency int, nextFunc nextNFunc) (interrupted bool, _ error) {
		mu := sync.Mutex{}
		for {
			repoIds, more, finish := nextFunc(pageSize, itrConfig)
			if !more {
				break
			}
			select {
			case <-timeExpired:
				return true, nil
			default:
				p := pool.New().WithContext(ctx).WithMaxGoroutines(concurrency)
				repoErrors := map[int32]error{}
				startPage := time.Now()
				for i := range len(repoIds) {
					repoId := repoIds[i]
					p.Go(func(ctx context.Context) error {
						repo, repoErr := h.repoStore.Get(ctx, repoId)
						if repoErr != nil {
							// If the repo is not found it was deleted and will return no results
							// no need to error here which will add an alert to the insight
							if errors.Is(repoErr, &database.RepoNotFoundErr{ID: repoId}) {
								return nil
							}
							mu.Lock()
							repoErrors[int32(repoId)] = errors.Wrap(repoErr, "InProgressHandler.repoStore.Get")
							mu.Unlock()
							return nil
						}
						execution.logger.Debug("doing iteration work", log.Int("repo_id", int(repoId)))
						runErr := h.backfillRunner.Run(ctx, pipeline.BackfillRequest{Series: execution.series, Repo: &types.MinimalRepo{ID: repo.ID, Name: repo.Name}, SampleTimes: execution.sampleTimes})
						if runErr != nil {
							execution.logger.Error("error during backfill execution", execution.logFields(log.Error(runErr))...)
							mu.Lock()
							repoErrors[int32(repoId)] = runErr
							mu.Unlock()
							return nil
						}
						return nil
					})

				}
				// The groups functions don't return errors so not checking for them
				p.Wait()
				execution.logger.Debug("page complete", log.Duration("page duration", time.Since(startPage)), log.Int("page size", pageSize), log.Int("number repos", len(repoIds)))
				err = finish(ctx, h.backfillStore.Store, repoErrors)
				if err != nil {
					return false, err
				}
				if execution.exceedsErrorThreshold() {
					err = h.disableBackfill(ctx, execution)
					if err != nil {
						return false, errors.Wrap(err, "disableBackfill")
					}
				}
			}
		}
		return false, nil
	}

	execution.logger.Debug("starting primary loop", log.Int("seriesId", execution.series.ID), log.Int("backfillId", execution.backfill.Id))
	if interrupted, err := itrLoop(config.pageSize, config.repoConcurrency, execution.itr.NextPageWithFinish); err != nil {
		return false, errors.Wrap(err, "InProgressHandler.PrimaryLoop")
	} else if interrupted {
		execution.logger.Info("interrupted insight series backfill", execution.logFields(log.Duration("interruptAfter", config.interruptAfter))...)
		return true, nil
	}

	execution.logger.Debug("starting retry loop", log.Int("seriesId", execution.series.ID), log.Int("backfillId", execution.backfill.Id))
	if interrupted, err := itrLoop(1, 1, retryAdapter(execution.itr.NextRetryWithFinish)); err != nil {
		return false, errors.Wrap(err, "InProgressHandler.RetryLoop")
	} else if interrupted {
		execution.logger.Info("interrupted insight series backfill retry", execution.logFields(log.Duration("interruptAfter", config.interruptAfter))...)
		return true, nil
	}

	if !execution.itr.HasMore() && !execution.itr.HasErrors() {
		return false, h.finish(ctx, execution)
	} else {
		// in this state we have some errors that will need reprocessing, we will place this job back in queue
		return true, nil
	}
}

func retryAdapter(next func(config iterator.IterationConfig) (api.RepoID, bool, iterator.FinishFunc)) nextNFunc {
	return func(pageSize int, config iterator.IterationConfig) ([]api.RepoID, bool, iterator.FinishNFunc) {
		repoId, more, finish := next(config)
		return []api.RepoID{repoId}, more, func(ctx context.Context, store *basestore.Store, maybeErr map[int32]error) error {
			repoErr := maybeErr[int32(repoId)]
			return finish(ctx, store, repoErr)
		}
	}

}

func (h *inProgressHandler) finish(ctx context.Context, ex *backfillExecution) (err error) {
	tx, err := h.backfillStore.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()
	bfs := h.backfillStore.With(tx)

	err = ex.itr.MarkComplete(ctx, tx.Store)
	if err != nil {
		return errors.Wrap(err, "iterator.MarkComplete")
	}
	err = h.seriesReadComplete.SetSeriesBackfillComplete(ctx, ex.series.SeriesID, ex.itr.CompletedAt)
	if err != nil {
		return err
	}
	err = ex.backfill.SetCompleted(ctx, bfs)
	if err != nil {
		return errors.Wrap(err, "backfill.SetCompleted")
	}
	ex.logger.Info("backfill set to completed state", ex.logFields()...)
	return nil
}

func (h *inProgressHandler) disableBackfill(ctx context.Context, ex *backfillExecution) (err error) {
	tx, err := h.backfillStore.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()
	bfs := h.backfillStore.With(tx)

	// fail the backfill, this should help prevent out of control jobs from consuming all of the resources
	if err = ex.backfill.SetFailed(ctx, bfs); err != nil {
		return errors.Wrap(err, "SetFailed")
	}
	if err = ex.itr.MarkComplete(ctx, tx.Store); err != nil {
		return errors.Wrap(err, "itr.MarkComplete")
	}
	for _, frame := range ex.sampleTimes {
		tss := h.insightsStore.WithOther(tx)
		if err = tss.AddIncompleteDatapoint(ctx, store.AddIncompleteDatapointInput{
			SeriesID: ex.series.ID,
			Reason:   store.ReasonExceedsErrorLimit,
			Time:     frame,
		}); err != nil {
			return errors.Wrap(err, "SetFailed.AddIncompleteDatapoint")
		}
	}
	ex.logger.Info("backfill disabled due to exceeding error threshold", ex.logFields(log.Int("threshold", ex.getThreshold()))...)
	return nil
}

func (h *inProgressHandler) load(ctx context.Context, logger log.Logger, backfillId int) (*backfillExecution, error) {
	backfillJob, err := h.backfillStore.LoadBackfill(ctx, backfillId)
	if err != nil {
		return nil, errors.Wrap(err, "loadBackfill")
	}
	series, err := h.seriesReadComplete.GetDataSeriesByID(ctx, backfillJob.SeriesId)
	if err != nil {
		return nil, errors.Wrap(err, "GetDataSeriesByID")
	}

	itr, err := backfillJob.repoIterator(ctx, h.backfillStore)
	if err != nil {
		return nil, errors.Wrap(err, "repoIterator")
	}

	sampleTimes := timeseries.BuildSampleTimes(12, timeseries.TimeInterval{
		Unit:  itypes.IntervalUnit(series.SampleIntervalUnit),
		Value: series.SampleIntervalValue,
	}, series.CreatedAt.Truncate(time.Minute))

	return &backfillExecution{
		series:      series,
		backfill:    backfillJob,
		itr:         itr,
		logger:      logger,
		sampleTimes: sampleTimes,
	}, nil
}

type backfillExecution struct {
	series      *itypes.InsightSeries
	backfill    *SeriesBackfill
	itr         *iterator.PersistentRepoIterator
	logger      log.Logger
	sampleTimes []time.Time
	config      handlerConfig
}

func (b *backfillExecution) logFields(extra ...log.Field) []log.Field {
	fields := []log.Field{
		log.Int("seriesId", b.series.ID),
		log.String("seriesUniqueId", b.series.SeriesID),
		log.Int("backfillId", b.backfill.Id),
		log.Duration("totalDuration", b.itr.RuntimeDuration),
		log.Int("repoTotalCount", b.itr.TotalCount),
		log.Int("errorCount", b.itr.TotalErrors()),
		log.Float64("percentComplete", b.itr.PercentComplete),
		log.Int("erroredRepos", b.itr.ErroredRepos()),
	}
	fields = append(fields, extra...)
	return fields
}

func (h *inProgressHandler) doInterrupt(ctx context.Context, job *BaseJob) error {
	return h.workerStore.Requeue(ctx, job.ID, h.clock.Now().Add(inProgressPollingInterval))
}

func getInterruptAfter() time.Duration {
	val := conf.Get().InsightsBackfillInterruptAfter
	if val != 0 {
		return time.Duration(val) * time.Second
	}
	return time.Duration(defaultInterruptSeconds) * time.Second
}

func getPageSize() int {
	val := conf.Get().InsightsBackfillRepositoryGroupSize
	if val > 0 {
		return int(math.Min(float64(val), 100))
	}
	return 10
}

func getRepoConcurrency() int {
	val := conf.Get().InsightsBackfillRepositoryConcurrency
	if val > 0 {
		return int(math.Min(float64(val), 10))
	}
	return 3
}

func getErrorThresholdFloor() int {
	return defaultErrorThresholdFloor
}

func translateIncompleteReasons(err error) store.IncompleteReason {
	if errors.Is(err, queryrunner.SearchTimeoutError) {
		return store.ReasonTimeout
	}
	return store.ReasonGeneric
}

func (b *backfillExecution) exceedsErrorThreshold() bool {
	return b.itr.TotalErrors() > calculateErrorThreshold(.05, b.config.errorThresholdFloor, b.itr.TotalCount)
}

func (b *backfillExecution) getThreshold() int {
	return calculateErrorThreshold(.05, b.config.errorThresholdFloor, b.itr.TotalCount)
}

func calculateErrorThreshold(percent float64, floor int, cardinality int) int {
	scaled := int(float64(cardinality) * percent)
	if scaled <= floor {
		return floor
	}
	return scaled
}
