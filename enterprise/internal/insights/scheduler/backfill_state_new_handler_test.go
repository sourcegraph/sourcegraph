package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/derision-test/glock"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	edb "github.com/sourcegraph/sourcegraph/enterprise/internal/database"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/discovery"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	itypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func Test_MovesBackfillFromNewToProcessing(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t))
	repos := database.NewMockRepoStore()
	repos.ListFunc.SetDefaultReturn([]*itypes.Repo{{ID: 1, Name: "repo1"}, {ID: 2, Name: "repo2"}}, nil)
	now := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := glock.NewMockClockAt(now)
	bfs := newBackfillStoreWithClock(insightsDB, clock)
	insightsStore := store.NewInsightStore(insightsDB)
	config := JobMonitorConfig{
		InsightsDB: insightsDB,
		RepoStore:  repos,
		ObsContext: &observation.TestContext,
	}
	monitor := NewBackgroundJobMonitor(ctx, config)

	series, err := insightsStore.CreateSeries(ctx, types.InsightSeries{
		SeriesID:            "series1",
		Query:               "asdf",
		SampleIntervalUnit:  string(types.Month),
		Repositories:        []string{"repo1", "repo2"},
		SampleIntervalValue: 1,
		GenerationMethod:    types.Search,
	})
	require.NoError(t, err)

	backfill, err := bfs.NewBackfill(ctx, series)
	require.NoError(t, err)

	err = enqueueBackfill(ctx, bfs.Handle(), backfill)
	require.NoError(t, err)

	dequeue, found, err := monitor.newBackfillStore.Dequeue(ctx, "test", nil)
	handler := newBackfillHandler{
		workerStore:   monitor.newBackfillStore,
		backfillStore: bfs,
		seriesReader:  store.NewInsightStore(insightsDB),
		repoIterator:  discovery.NewSeriesRepoIterator(nil, repos),
	}
	err = handler.Handle(ctx, logger, dequeue)
	require.NoError(t, err)

	dequeue, found, err = monitor.newBackfillStore.Dequeue(ctx, "test", nil)
	require.NoError(t, err)
	if found {
		t.Fatal(errors.New("found record that should not be visible to the new backfill store"))
	}

	// now ensure the in progress handler _can_ pick it up
	dequeue, found, err = monitor.inProgressStore.Dequeue(ctx, "test", nil)
	require.NoError(t, err)
	if !found {
		t.Fatal(errors.New("no queued record found"))
	}
	job, _ := dequeue.(*BaseJob)
	require.Equal(t, backfill.Id, job.backfillId)
}
