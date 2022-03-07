package background

import (
	"context"
	"database/sql"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"

	"github.com/cockroachdb/errors"
	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/store"

	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
)

// NewInsightsDataPrunerJob will periodically delete recorded data series that have been marked `deleted`.
func NewInsightsDataPrunerJob(ctx context.Context, postgres dbutil.DB, insightsdb dbutil.DB) goroutine.BackgroundRoutine {
	interval := time.Minute * 60
	pruneInterval := time.Microsecond * -1 // records must be soft deleted at least this long

	return goroutine.NewPeriodicGoroutine(ctx, interval,
		goroutine.NewHandlerWithErrorMessage("insights_data_prune", func(ctx context.Context) (err error) {
			return performPurge(ctx, postgres, insightsdb, time.Now().Add(pruneInterval))
		}))
}

func performPurge(ctx context.Context, postgres dbutil.DB, insightsdb dbutil.DB, deletedBefore time.Time) (err error) {
	insightStore := store.NewInsightStore(insightsdb)
	timeseriesStore := store.New(insightsdb, store.NewInsightPermissionStore(postgres))

	// select the series that need to be pruned. These will be the series that are currently flagged
	// as "soft deleted" for a given amount of time.
	seriesIds, err := insightStore.GetSoftDeletedSeries(ctx, deletedBefore)
	if err != nil {
		return errors.Wrap(err, "GetSoftDeletedSeries")
	}

	for _, id := range seriesIds {
		log15.Info("pruning insight series", "seriesId", id)
		err := deleteQueuedRecords(ctx, postgres, id)
		if err != nil {
			return errors.Wrap(err, "deleteQueuedRecords")
		}
		// We will always delete the series definition last, such that any possible partial state
		// the series definition will always be referencable and therefore can be re-attempted. This operation
		// isn't across the same database currently, so there isn't a single transaction across all the
		// tables.
		tx, err := timeseriesStore.Transact(ctx)
		if err != nil {
			return err
		}
		err = tx.Delete(ctx, id)
		if err != nil {
			return errors.Wrap(err, "Delete")
		}

		insightStoreTx := insightStore.With(tx)
		err = insightStoreTx.HardDeleteSeries(ctx, id)
		if err != nil {
			return err
		}

		err = tx.Done(err)
	}

	return err
}

func deleteQueuedRecords(ctx context.Context, postgres dbutil.DB, seriesId string) error {
	queueStore := basestore.NewWithDB(postgres, sql.TxOptions{})
	return queueStore.Exec(ctx, sqlf.Sprintf(deleteQueuedForSeries, seriesId))
}

const deleteQueuedForSeries = `
delete from insights_query_runner_jobs where series_id = %s;
`
