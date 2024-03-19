package backfillv2

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/derision-test/glock"
	"github.com/hexops/autogold/v2"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/log/logtest"

	edb "github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/insights/timeseries"
	"github.com/sourcegraph/sourcegraph/internal/insights/types"
)

type SeriesValidate struct {
	SeriesID           string
	CreatedAt          string
	NextRecordingAfter string
	NextSnapshotAfter  string
	BackfillQueuedAt   string
	JustInTime         bool
	NeedsMigration     bool
	BackfillState      string
}

const validateSeriesSql = `
	SELECT  s.series_id, s.created_at, s.next_recording_after, s.next_snapshot_after, s.backfill_queued_at, s.just_in_time, s.needs_migration, isb.state
	FROM insight_series s
		LEFT JOIN insight_series_backfill isb on s.id = isb.series_id`

func scanValidateSeries(rows *sql.Rows, queryErr error) (_ []SeriesValidate, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	timeFmt := "2006-01-02 15:04:05"

	var createdAt, nextRecordingAfter, nextSnapshotAfter time.Time
	var backfillQueuedAt *time.Time
	var backfillState *string

	results := make([]SeriesValidate, 0)
	for rows.Next() {
		var temp SeriesValidate
		if err := rows.Scan(
			&temp.SeriesID,
			&createdAt,
			&nextRecordingAfter,
			&nextSnapshotAfter,
			&backfillQueuedAt,
			&temp.JustInTime,
			&temp.NeedsMigration,
			&backfillState,
		); err != nil {
			return nil, err
		}
		temp.CreatedAt = createdAt.Format(timeFmt)
		temp.NextRecordingAfter = nextRecordingAfter.Format(timeFmt)
		temp.NextSnapshotAfter = nextSnapshotAfter.Format(timeFmt)
		if backfillQueuedAt != nil {
			tmp := backfillQueuedAt.Format(timeFmt)
			temp.BackfillQueuedAt = tmp
		}
		if backfillState != nil {
			temp.BackfillState = *backfillState
		}

		results = append(results, temp)
	}
	return results, nil
}

func getResults(ctx context.Context, store basestore.ShareableStore) (map[string]SeriesValidate, error) {
	series, err := scanValidateSeries(store.Handle().QueryContext(ctx, validateSeriesSql))
	if err != nil {
		return nil, err
	}
	m := make(map[string]SeriesValidate, len(series))
	for _, s := range series {
		m[s.SeriesID] = s
	}
	return m, nil
}

type InsightSeries struct {
	ID                         int
	SeriesID                   string
	Query                      string
	CreatedAt                  time.Time
	OldestHistoricalAt         time.Time
	LastRecordedAt             time.Time
	NextRecordingAfter         time.Time
	LastSnapshotAt             time.Time
	NextSnapshotAfter          time.Time
	BackfillQueuedAt           *time.Time
	Enabled                    bool
	Repositories               []string
	SampleIntervalUnit         string
	SampleIntervalValue        int
	GeneratedFromCaptureGroups bool
	JustInTime                 bool
	GenerationMethod           string
	GroupBy                    *string
}

func createSeries(ctx context.Context, store basestore.ShareableStore, series InsightSeries, clock glock.Clock) (InsightSeries, error) {
	if series.CreatedAt.IsZero() {
		series.CreatedAt = clock.Now()
	}
	interval := timeseries.TimeInterval{
		Unit:  types.IntervalUnit(series.SampleIntervalUnit),
		Value: series.SampleIntervalValue,
	}
	if !interval.IsValid() {
		interval = timeseries.DefaultInterval
	}
	series.NextSnapshotAfter = series.CreatedAt.Truncate(time.Minute).AddDate(0, 0, 1)
	series.NextRecordingAfter = series.CreatedAt.AddDate(0, 0, 2)
	q := sqlf.Sprintf(createInsightSeriesSql,
		series.SeriesID,
		series.Query,
		series.CreatedAt,
		series.OldestHistoricalAt,
		series.LastRecordedAt,
		series.NextRecordingAfter,
		series.LastSnapshotAt,
		series.NextSnapshotAfter,
		pq.Array(series.Repositories),
		series.SampleIntervalUnit,
		series.SampleIntervalValue,
		series.GeneratedFromCaptureGroups,
		series.JustInTime,
		series.GenerationMethod,
		series.GroupBy,
		series.JustInTime && series.GenerationMethod != "language-stats", // marking needs migration
		series.BackfillQueuedAt,
	)

	row := store.Handle().QueryRowContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
	var id int
	err := row.Scan(&id)
	if err != nil {
		return InsightSeries{}, err
	}
	series.ID = id
	series.Enabled = true
	return series, nil
}

const createInsightSeriesSql = `
INSERT INTO insight_series (series_id, query, created_at, oldest_historical_at, last_recorded_at,
                            next_recording_after, last_snapshot_at, next_snapshot_after, repositories,
							sample_interval_unit, sample_interval_value, generated_from_capture_groups,
							just_in_time, generation_method, group_by, needs_migration, backfill_queued_at)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
RETURNING id;`

func makeBackfill(t *testing.T, ctx context.Context, store basestore.ShareableStore) makeBackfillFunc {
	return func(series InsightSeries, state string) error {
		q := sqlf.Sprintf("INSERT INTO insight_series_backfill (series_id, state) VALUES(%s, %s)", series.ID, state)
		_, err := store.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
		if err != nil {
			t.Fail()
			return err
		}
		return err
	}
}

/*
Test cases for migrator

+------+---------------+------------+-----------------+--------------+------------+-----------------+-------------------------------------------------------------------------------------------------------+
| Case | Insight Type  | Created At | Backfill Queued | Just In Time | Repo Scope | Backfill Exists |                                           Expected outcome                                            |
+------+---------------+------------+-----------------+--------------+------------+-----------------+-------------------------------------------------------------------------------------------------------+
| a    | Search        | Recent     | null            | false        | all        | false           | Backfill 'new', Job Created, Series - BackfillQueuedAt=now                                            |
| b    | Search        | Recent     | null            | false        | named      | false           | Backfill 'new', Job Created, Series - BackfillQueuedAt=now                                            |
| c    | Search        | Recent     | Recent          | false        | all        | false           | Backfill 'completed'                                                                                  |
| d    | Search        | Recent     | Recent          | false        | named      | false           | Backfill 'completed'                                                                                  |
| e    | Search        | Year ago   | Year Ago        | false        | all        | false           | Backfill 'completed'                                                                                  |
| f    | Search        | Recent     | null            | True         | named      | false           | Backfill 'new', Job Created, Series:CreatedAt=now JIT=false NeedsMigration=false BackfillQueuedAt=now |
| g    | Search        | Year ago   | null            | true         | named      | false           | Backfill 'new', Job Created, Series:CreatedAt=now JIT=false NeedsMigration=false BackfillQueuedAt=now |
| h    | Capture Group | Year ago   | null            | true         | named      | false           | Backfill 'new', Job Created, Series:CreatedAt=now JIT=false NeedsMigration=false BackfillQueuedAt=now |
| i    | Capture Group | Recent     | Recent          | false        | named      | false           | Backfill 'completed'                                                                                  |
| j    | Lang Stats    | Recent     | null            | true         | named      | false           | no change                                                                                             |
| k    | Group By      | Recent     | Recent          | false        | named      | false           | no change                                                                                             |
| l    | Group By      | Year Ago   | Year Ago        | false        | named      | false           | no change                                                                                             |
| m    | Search        | Recent     | Recent          | false        | all        | true            | no change                                                                                             |
+------+---------------+------------+-----------------+--------------+------------+-----------------+-------------------------------------------------------------------------------------------------------+
*/

type testCase struct {
	name   string
	series InsightSeries
	want   autogold.Value
}

type (
	makeSeriesFunc   func(id string, createdAt time.Time, backfillQueuedAt *time.Time, jit bool, repos []string, generationMethod string, captureGroup bool, groupBy *string) InsightSeries
	makeBackfillFunc func(series InsightSeries, state string) error
)

func makeNewSeries(t *testing.T, ctx context.Context, store basestore.ShareableStore, clock glock.Clock) func(id string, createdAt time.Time, backfillQueuedAt *time.Time, jit bool, repos []string, generationMethod string, captureGroup bool, groupBy *string) InsightSeries {
	return func(id string, createdAt time.Time, backfillQueuedAt *time.Time, jit bool, repos []string, generationMethod string, captureGroup bool, groupBy *string) InsightSeries {
		s := InsightSeries{
			SeriesID:                   id,
			Query:                      "sample",
			CreatedAt:                  createdAt,
			BackfillQueuedAt:           backfillQueuedAt,
			SampleIntervalUnit:         "DAY",
			SampleIntervalValue:        2,
			JustInTime:                 jit,
			Repositories:               repos,
			GenerationMethod:           generationMethod,
			GeneratedFromCaptureGroups: captureGroup,
			GroupBy:                    groupBy,
		}
		series, err := createSeries(ctx, store, s, clock)
		if err != nil {
			t.Fail()
			return InsightSeries{}
		}
		return series
	}
}

func newSearchSeries(ms makeSeriesFunc, id string, createdAt time.Time, backfillQueuedAt *time.Time, jit bool, repos []string) InsightSeries {
	return ms(id, createdAt, backfillQueuedAt, jit, repos, "search", false, nil)
}

func newSearchSeriesWithBackfill(ms makeSeriesFunc, mb makeBackfillFunc, id string, createdAt time.Time, backfillQueuedAt *time.Time, jit bool, repos []string, backfillState string) InsightSeries {
	s := ms(id, createdAt, backfillQueuedAt, jit, repos, "search", false, nil)
	_ = mb(s, backfillState)
	return s
}

func newCGSeries(ms makeSeriesFunc, id string, createdAt time.Time, backfillQueuedAt *time.Time, jit bool, repos []string) InsightSeries {
	return ms(id, createdAt, backfillQueuedAt, jit, repos, "search-compute", true, nil)
}

func newGroupBySeries(ms makeSeriesFunc, id string, createdAt time.Time, backfillQueuedAt *time.Time, jit bool, repo string) InsightSeries {
	gb := "repo"
	return ms(id, createdAt, backfillQueuedAt, jit, []string{repo}, "mapping-compute", true, &gb)
}

func newLangStats(ms makeSeriesFunc, id string, createdAt time.Time, backfillQueuedAt *time.Time, repo string) InsightSeries {
	return ms(id, createdAt, backfillQueuedAt, true, []string{repo}, "language-stats", false, nil)
}

func TestBackfillV2Migrator(t *testing.T) {
	t.Setenv("DISABLE_CODE_INSIGHTS", "")

	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)
	clock := glock.NewMockClockAt(time.Date(2022, time.April, 15, 1, 0, 0, 0, time.UTC))
	store := basestore.NewWithHandle(db.Handle())
	migrator := NewMigrator(store, clock, 1)

	ms := makeNewSeries(t, ctx, store, clock)
	mb := makeBackfill(t, ctx, store)

	now := clock.Now()
	recent := clock.Now().AddDate(0, 0, -10)
	yearAgo := clock.Now().AddDate(-1, 0, 0)
	cases := []testCase{
		{
			name:   "Not backfilled all repos search insight",
			series: newSearchSeries(ms, "a", now, nil, false, nil),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "a", CreatedAt: "2022-04-15 01:00:00",
				NextRecordingAfter: "2022-04-17 01:00:00",
				NextSnapshotAfter:  "2022-04-16 01:00:00",
				BackfillQueuedAt:   "2022-04-15 01:00:00",
				BackfillState:      "new",
			}),
		},
		{
			name:   "Not backfilled named repos search insight",
			series: newSearchSeries(ms, "b", now, nil, false, []string{"repoA", "repoB"}),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "b", CreatedAt: "2022-04-15 01:00:00",
				NextRecordingAfter: "2022-04-17 01:00:00",
				NextSnapshotAfter:  "2022-04-16 01:00:00",
				BackfillQueuedAt:   "2022-04-15 01:00:00",
				BackfillState:      "new",
			}),
		},
		{
			name:   "Recent Backfilled all repos search insight",
			series: newSearchSeries(ms, "c", recent, &recent, false, nil),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "c", CreatedAt: "2022-04-05 01:00:00",
				NextRecordingAfter: "2022-04-07 01:00:00",
				NextSnapshotAfter:  "2022-04-06 01:00:00",
				BackfillQueuedAt:   "2022-04-05 01:00:00",
				BackfillState:      "completed",
			}),
		},
		{
			name:   "Recent Backfilled named repos search insight",
			series: newSearchSeries(ms, "d", recent, &recent, false, []string{"repoA", "repoB"}),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "d", CreatedAt: "2022-04-05 01:00:00",
				NextRecordingAfter: "2022-04-07 01:00:00",
				NextSnapshotAfter:  "2022-04-06 01:00:00",
				BackfillQueuedAt:   "2022-04-05 01:00:00",
				BackfillState:      "completed",
			}),
		},
		{
			name:   "Older Backfilled all repos search insight",
			series: newSearchSeries(ms, "e", yearAgo, &yearAgo, false, nil),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "e", CreatedAt: "2021-04-15 01:00:00",
				NextRecordingAfter: "2021-04-17 01:00:00",
				NextSnapshotAfter:  "2021-04-16 01:00:00",
				BackfillQueuedAt:   "2021-04-15 01:00:00",
				BackfillState:      "completed",
			}),
		},
		{
			name:   "Recent JIT search insight",
			series: newSearchSeries(ms, "f", recent, nil, true, []string{"repoA", "repoB"}),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "f", CreatedAt: "2022-04-15 01:00:00",
				NextRecordingAfter: "2022-04-17 01:00:00",
				NextSnapshotAfter:  "2022-04-16 00:00:00",
				BackfillQueuedAt:   "2022-04-15 01:00:00",
				BackfillState:      "new",
			}),
		},
		{
			name:   "Older JIT search insight",
			series: newSearchSeries(ms, "g", yearAgo, nil, true, []string{"repoA", "repoB"}),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "g", CreatedAt: "2022-04-15 01:00:00",
				NextRecordingAfter: "2022-04-17 01:00:00",
				NextSnapshotAfter:  "2022-04-16 00:00:00",
				BackfillQueuedAt:   "2022-04-15 01:00:00",
				BackfillState:      "new",
			}),
		},
		{
			name:   "Older JIT capture group insight",
			series: newCGSeries(ms, "h", yearAgo, nil, true, []string{"repoA", "repoB"}),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "h", CreatedAt: "2022-04-15 01:00:00",
				NextRecordingAfter: "2022-04-17 01:00:00",
				NextSnapshotAfter:  "2022-04-16 00:00:00",
				BackfillQueuedAt:   "2022-04-15 01:00:00",
				BackfillState:      "new",
			}),
		},
		{
			name:   "Recent backfilled capture group insight",
			series: newCGSeries(ms, "i", recent, &recent, false, []string{"repoA", "repoB"}),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "i", CreatedAt: "2022-04-05 01:00:00",
				NextRecordingAfter: "2022-04-07 01:00:00",
				NextSnapshotAfter:  "2022-04-06 01:00:00",
				BackfillQueuedAt:   "2022-04-05 01:00:00",
				BackfillState:      "completed",
			}),
		},
		{
			name:   "Recent search insight with new backfill completed",
			series: newSearchSeriesWithBackfill(ms, mb, "m", recent, &recent, false, nil, "complete"),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "m", CreatedAt: "2022-04-05 01:00:00",
				NextRecordingAfter: "2022-04-07 01:00:00",
				NextSnapshotAfter:  "2022-04-06 01:00:00",
				BackfillQueuedAt:   "2022-04-05 01:00:00",
				BackfillState:      "complete",
			}),
		},
		{
			name:   "Recent search insight with new backfill new",
			series: newSearchSeriesWithBackfill(ms, mb, "n", recent, &recent, false, nil, "new"),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "n", CreatedAt: "2022-04-05 01:00:00",
				NextRecordingAfter: "2022-04-07 01:00:00",
				NextSnapshotAfter:  "2022-04-06 01:00:00",
				BackfillQueuedAt:   "2022-04-05 01:00:00",
				BackfillState:      "new",
			}),
		},
	}
	caesNoMigrate := []testCase{
		{
			name:   "Recent Lang Stats insight",
			series: newLangStats(ms, "j", recent, nil, "repoA"),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "j", CreatedAt: "2022-04-05 01:00:00",
				NextRecordingAfter: "2022-04-07 01:00:00",
				NextSnapshotAfter:  "2022-04-06 01:00:00",
				JustInTime:         true,
			}),
		},
		{
			name:   "Recent Group By insight",
			series: newGroupBySeries(ms, "k", recent, &recent, false, "repoA"),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "k", CreatedAt: "2022-04-05 01:00:00",
				NextRecordingAfter: "2022-04-07 01:00:00",
				NextSnapshotAfter:  "2022-04-06 01:00:00",
				BackfillQueuedAt:   "2022-04-05 01:00:00",
			}),
		},
		{
			name:   "Older Group By insight",
			series: newGroupBySeries(ms, "l", yearAgo, &yearAgo, false, "repoA"),
			want: autogold.Expect(SeriesValidate{
				SeriesID: "l", CreatedAt: "2021-04-15 01:00:00",
				NextRecordingAfter: "2021-04-17 01:00:00",
				NextSnapshotAfter:  "2021-04-16 01:00:00",
				BackfillQueuedAt:   "2021-04-15 01:00:00",
			}),
		},
	}

	assertProgress := func(expectedProgress float64, applyReverse bool) {
		if progress, err := migrator.Progress(context.Background(), applyReverse); err != nil {
			t.Fatalf("unexpected error querying progress: %s", err)
		} else if progress != expectedProgress {
			t.Errorf("unexpected progress. want=%.2f have=%.2f", expectedProgress, progress)
		}
	}
	done := float64(2) // there are 2 series that already have backfill records
	assertProgress(done/float64(len(cases)), false)
	for range len(cases) {
		err := migrator.Up(ctx)
		assert.NoError(t, err, "unexpected error migrating up")
	}
	// check finished
	assertProgress(1, false)
	results, err := getResults(ctx, store)
	assert.NoError(t, err)

	totalCases := append(cases, caesNoMigrate...)
	for _, c := range totalCases {
		t.Run(c.name, func(t *testing.T) {
			got := results[c.series.SeriesID]
			c.want.Equal(t, got)
		})
	}
}

func TestBackfillV2MigratorNoInsights(t *testing.T) {
	t.Setenv("DISABLE_CODE_INSIGHTS", "true")
	logger := logtest.Scoped(t)
	db := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)
	clock := glock.NewMockClockAt(time.Date(2022, time.April, 15, 1, 0, 0, 0, time.UTC))
	store := basestore.NewWithHandle(db.Handle())
	migrator := NewMigrator(store, clock, 1)

	assertProgress := func(expectedProgress float64, applyReverse bool) {
		if progress, err := migrator.Progress(context.Background(), applyReverse); err != nil {
			t.Fatalf("unexpected error querying progress: %s", err)
		} else if progress != expectedProgress {
			t.Errorf("unexpected progress. want=%.2f have=%.2f", expectedProgress, progress)
		}
	}
	// make a single series that would be migrated
	ms := makeNewSeries(t, context.Background(), store, clock)
	newSearchSeries(ms, "a", clock.Now(), nil, false, nil)

	// ensure that since insights is disabled it says it's done
	assertProgress(1, false)
}
