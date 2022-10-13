package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/types"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"

	"github.com/sourcegraph/log/logtest"
	edb "github.com/sourcegraph/sourcegraph/enterprise/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func Test_SchedulerStartsAndStops(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t))

	routines := NewScheduler(ctx, insightsDB, &observation.TestContext).Routines()
	goroutine.MonitorBackgroundRoutines(ctx, routines...)
}

func Test_SchedulerMovesBackfillFromNewToProcessing(t *testing.T) {
	logger := logtest.Scoped(t)
	// ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	// defer cancel()
	ctx := context.Background()
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t))
	bfs := newBackfillStore(insightsDB)
	insightsStore := store.NewInsightStore(insightsDB)

	scheduler := NewScheduler(ctx, insightsDB, &observation.TestContext)

	series, err := insightsStore.CreateSeries(ctx, types.InsightSeries{
		SeriesID:            "series1",
		Query:               "asdf",
		SampleIntervalUnit:  string(types.Month),
		SampleIntervalValue: 1,
		GenerationMethod:    types.Search,
	})
	require.NoError(t, err)

	backfill, err := bfs.NewBackfill(ctx, series)
	require.NoError(t, err)

	err = scheduler.EnqueueBackfill(ctx, backfill)
	require.NoError(t, err)

	dequeue, found, err := scheduler.newBackfillStore.Dequeue(ctx, "test", nil)
	require.NoError(t, err)
	if !found {
		t.Fatal(errors.New("no queued record found"))
	}
	job, _ := dequeue.(*BaseJob)
	require.Equal(t, backfill.Id, job.backfillId)

	modified, err := backfill.SetScope(ctx, bfs, []int32{1, 5, 7}, 100)
	require.NoError(t, err)
	require.Equal(t, 1, modified.repoIteratorId)

	err = scheduler.newBackfillStore.Requeue(ctx, dequeue.RecordID(), time.Time{})
	require.NoError(t, err)
	// now the record should only show up to the in progress handler

	dequeue, found, err = scheduler.newBackfillStore.Dequeue(ctx, "test", nil)
	require.NoError(t, err)
	if found {
		t.Fatal(errors.New("found record that should not be visible to the new backfill store"))
	}

	// now ensure the in progress handler _can_ pick it up
	dequeue, found, err = scheduler.inProgressStore.Dequeue(ctx, "test", nil)
	require.NoError(t, err)
	if !found {
		t.Fatal(errors.New("no queued record found"))
	}
	job, _ = dequeue.(*BaseJob)
	require.Equal(t, backfill.Id, job.backfillId)
}
