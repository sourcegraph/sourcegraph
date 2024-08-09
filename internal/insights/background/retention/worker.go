package retention

import (
	"context"
	"time"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var _ workerutil.Handler[*DataRetentionJob] = &dataRetentionHandler{}

type dataRetentionHandler struct {
	baseWorkerStore dbworkerstore.Store[*DataRetentionJob]
	insightsStore   *store.Store
}

func (h *dataRetentionHandler) Handle(ctx context.Context, logger log.Logger, record *DataRetentionJob) (err error) {
	doArchive := conf.ExperimentalFeatures().InsightsDataRetention
	// If the setting is not set we run retention by default.
	if doArchive != nil && !*doArchive {
		return nil
	}

	maximumSampleSize := getMaximumSampleSize(logger)

	// All the retention operations need to be completed in the same transaction
	tx, err := h.insightsStore.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	// We remove 1 off the maximum sample size so that we get the last timestamp that we want to keep data for.
	// We ignore snapshot timestamps. This is because if there are 10 record points and 1 snapshot point and a sample
	// size of 5 we don't want to keep 4 record points and the ephemeral snapshot point, but 5 record points.
	oldestRecordingTime, err := tx.GetOffsetNRecordingTime(ctx, record.InsightSeriesID, maximumSampleSize-1, true)
	if err != nil {
		return errors.Wrap(err, "GetOffsetNRecordingTime")
	}

	if oldestRecordingTime.IsZero() {
		// this series does not have any data beyond the max sample size
		logger.Debug("data retention procedure not needed", log.Int("seriesID", record.InsightSeriesID), log.Int("maxSampleSize", maximumSampleSize))
		return nil
	}

	if err := archiveOldRecordingTimes(ctx, tx, record.InsightSeriesID, oldestRecordingTime); err != nil {
		return errors.Wrap(err, "archiveOldRecordingTimes")
	}

	if err := archiveOldSeriesPoints(ctx, tx, record.SeriesID, oldestRecordingTime); err != nil {
		return errors.Wrap(err, "archiveOldSeriesPoints")
	}

	return nil
}

func getMaximumSampleSize(logger log.Logger) int {
	// Default should match what is shown in the schema not to be confusing
	maximumSampleSize := 30
	if configured := conf.Get().InsightsMaximumSampleSize; configured > 0 {
		maximumSampleSize = configured
	}
	if maximumSampleSize > 90 {
		logger.Info("code insights maximum sample size was set over allowed maximum, setting to 90", log.Int("disallowed maximum value", maximumSampleSize))
		maximumSampleSize = 90
	}
	return maximumSampleSize
}

// NewWorker returns a worker that will find what data to prune and separate for a series.
func NewWorker(ctx context.Context, logger log.Logger, workerStore dbworkerstore.Store[*DataRetentionJob], insightsStore *store.Store, metrics workerutil.WorkerObservability) *workerutil.Worker[*DataRetentionJob] {
	options := workerutil.WorkerOptions{
		Name:              "insights_data_retention_worker",
		Description:       "archives code insights data points over the maximum sample size",
		NumHandlers:       5,
		Interval:          30 * time.Minute,
		HeartbeatInterval: 15 * time.Second,
		Metrics:           metrics,
	}

	return dbworker.NewWorker[*DataRetentionJob](ctx, workerStore, &dataRetentionHandler{
		baseWorkerStore: workerStore,
		insightsStore:   insightsStore,
	}, options)
}

// NewResetter returns a resetter that will reset pending data retention jobs if they take too long
// to complete.
func NewResetter(ctx context.Context, logger log.Logger, workerStore dbworkerstore.Store[*DataRetentionJob], metrics dbworker.ResetterMetrics) *dbworker.Resetter[*DataRetentionJob] {
	options := dbworker.ResetterOptions{
		Name:     "insights_data_retention_worker_resetter",
		Interval: 1 * time.Minute,
		Metrics:  metrics,
	}
	return dbworker.NewResetter(ctx, logger, workerStore, options)
}

func CreateDBWorkerStore(observationCtx *observation.Context, store *basestore.Store) dbworkerstore.Store[*DataRetentionJob] {
	return dbworkerstore.New(observationCtx, store.Handle(), dbworkerstore.Options[*DataRetentionJob]{
		Name:              "insights_data_retention_worker_store",
		TableName:         "insights_data_retention_jobs",
		ColumnExpressions: dataRetentionJobColumns,
		Scan:              dbworkerstore.BuildWorkerScan(scanDataRetentionJob),
		OrderByExpression: sqlf.Sprintf("queued_at, id"),
		RetryAfter:        15 * time.Minute,
		MaxNumRetries:     5,
		MaxNumResets:      5,
		StalledMaxAge:     time.Second * 60,
	})
}

func EnqueueJob(ctx context.Context, workerBaseStore *basestore.Store, job *DataRetentionJob) (id int, err error) {
	tx, err := workerBaseStore.Transact(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { err = tx.Done(err) }()

	id, _, err = basestore.ScanFirstInt(tx.Query(
		ctx,
		sqlf.Sprintf(
			enqueueJobFmtStr,
			job.InsightSeriesID,
			job.SeriesID,
		),
	))
	if err != nil {
		return 0, err
	}
	job.ID = id
	return id, nil
}

const enqueueJobFmtStr = `
INSERT INTO insights_data_retention_jobs (series_id, series_id_string) VALUES (%s, %s)
RETURNING id
`

func archiveOldSeriesPoints(ctx context.Context, tx *store.Store, seriesID string, oldestTimestamp time.Time) error {
	return tx.Exec(ctx, sqlf.Sprintf(archiveOldSeriesPointsSql, seriesID, oldestTimestamp))
}

const archiveOldSeriesPointsSql = `
with moved_rows as (
	DELETE FROM series_points
	WHERE series_id = %s AND time < %s
	RETURNING *
)
INSERT INTO archived_series_points (series_id, time, value, repo_id, repo_name_id, original_repo_name_id, capture)
SELECT series_id, time, value, repo_id, repo_name_id, original_repo_name_id, capture from moved_rows
ON CONFLICT DO NOTHING
`

func archiveOldRecordingTimes(ctx context.Context, tx *store.Store, seriesID int, oldestTimestamp time.Time) error {
	return tx.Exec(ctx, sqlf.Sprintf(archiveOldRecordingTimesSql, seriesID, oldestTimestamp))
}

const archiveOldRecordingTimesSql = `
WITH moved_rows AS (
	DELETE FROM insight_series_recording_times
	WHERE insight_series_id = %s AND snapshot IS FALSE AND recording_time < %s
	RETURNING *
)
INSERT INTO archived_insight_series_recording_times
SELECT * FROM moved_rows
ON CONFLICT DO NOTHING
`
