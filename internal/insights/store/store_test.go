package store

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hexops/autogold/v2"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	edb "github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/insights/types"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestSeriesPoints(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	logger := logtest.Scoped(t)
	ctx := context.Background()
	clock := timeutil.Now
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)

	postgres := database.NewDB(logger, dbtest.NewDB(t))
	permStore := NewInsightPermissionStore(postgres)
	store := NewWithClock(insightsDB, permStore, clock)

	// Confirm we get no results initially.
	points, err := store.SeriesPoints(ctx, SeriesPointsOpts{})
	if err != nil {
		t.Fatal(err)
	}
	autogold.Expect([]SeriesPoint{}).Equal(t, points)

	// Insert some fake data.
	_, err = insightsDB.ExecContext(context.Background(), `
INSERT INTO repo_names(name) VALUES ('github.com/gorilla/mux-original');
INSERT INTO repo_names(name) VALUES ('github.com/gorilla/mux-renamed');
SELECT setseed(0.5);
INSERT INTO series_points(
    time,
	series_id,
    value,
    repo_id,
    repo_name_id,
    original_repo_name_id)
SELECT time,
    'somehash',
    random()*80 - 40,
    2,
    (SELECT id FROM repo_names WHERE name = 'github.com/gorilla/mux-renamed'),
    (SELECT id FROM repo_names WHERE name = 'github.com/gorilla/mux-original')
	FROM GENERATE_SERIES(CURRENT_TIMESTAMP::date - INTERVAL '30 weeks', CURRENT_TIMESTAMP::date, '2 weeks') AS time;
`)
	if err != nil {
		t.Fatal(err)
	}

	parseTime := func(s string) *time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return &v
	}

	t.Run("all data points", func(t *testing.T) {
		// Confirm we get all data points.
		points, err = store.SeriesPoints(ctx, SeriesPointsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		t.Log(points)
		autogold.Expect(16).Equal(t, len(points))
	})

	t.Run("subset of data", func(t *testing.T) {
		// Confirm we can get a subset of data points.
		points, err = store.SeriesPoints(ctx, SeriesPointsOpts{
			From: parseTime("2020-03-01T00:00:00Z"),
			To:   parseTime("2020-06-01T00:00:00Z"),
		})
		if err != nil {
			t.Fatal(err)
		}
		autogold.Expect(0).Equal(t, len(points))
	})

	t.Run("latest 3 points", func(t *testing.T) {
		// Confirm we can get a subset of data points.
		points, err = store.SeriesPoints(ctx, SeriesPointsOpts{
			Limit: 3,
		})
		if err != nil {
			t.Fatal(err)
		}
		autogold.Expect(3).Equal(t, len(points))
	})

	t.Run("include list", func(t *testing.T) {
		points, err = store.SeriesPoints(ctx, SeriesPointsOpts{Included: []api.RepoID{2}})
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(16, len(points)); diff != "" {
			t.Errorf("unexpected results from include list: %v", diff)
		}
	})
	t.Run("exclude list", func(t *testing.T) {
		points, err = store.SeriesPoints(ctx, SeriesPointsOpts{Excluded: []api.RepoID{2}})
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(0, len(points)); diff != "" {
			t.Errorf("unexpected results from include list: %v", diff)
		}
	})
}

func TestCountData(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	ctx := context.Background()
	clock := timeutil.Now
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)
	postgres := database.NewDB(logger, dbtest.NewDB(t))
	permStore := NewInsightPermissionStore(postgres)
	store := NewWithClock(insightsDB, permStore, clock)

	timeValue := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	timePtr := func(s string) *time.Time {
		return pointers.Ptr(timeValue(s))
	}
	optionalString := func(v string) *string { return &v }
	optionalRepoID := func(v api.RepoID) *api.RepoID { return &v }

	// Record some duplicate data points.
	records := []RecordSeriesPointArgs{
		{
			SeriesID:    "one",
			Point:       SeriesPoint{Time: timeValue("2020-03-01T00:00:00Z"), Value: 1.1},
			RepoName:    optionalString("repo1"),
			RepoID:      optionalRepoID(3),
			PersistMode: RecordMode,
		},
		{
			SeriesID:    "two",
			Point:       SeriesPoint{Time: timeValue("2020-03-02T00:00:00Z"), Value: 2.2},
			PersistMode: RecordMode,
		},
		{
			SeriesID:    "two",
			Point:       SeriesPoint{Time: timeValue("2020-03-02T00:01:00Z"), Value: 2.2},
			PersistMode: RecordMode,
		},
		{
			SeriesID:    "three",
			Point:       SeriesPoint{Time: timeValue("2020-03-03T00:00:00Z"), Value: 3.3},
			PersistMode: RecordMode,
		},
		{
			SeriesID:    "three",
			Point:       SeriesPoint{Time: timeValue("2020-03-03T00:01:00Z"), Value: 3.3},
			PersistMode: RecordMode,
		},
	}
	if err := store.RecordSeriesPoints(ctx, records); err != nil {
		t.Fatal(err)
	}

	// How many data points on 02-29?
	numDataPoints, err := store.CountData(ctx, CountDataOpts{
		From: timePtr("2020-02-29T00:00:00Z"),
		To:   timePtr("2020-02-29T23:59:59Z"),
	})
	if err != nil {
		t.Fatal(err)
	}
	autogold.Expect(0).Equal(t, numDataPoints)

	// How many data points on 03-01?
	numDataPoints, err = store.CountData(ctx, CountDataOpts{
		From: timePtr("2020-03-01T00:00:00Z"),
		To:   timePtr("2020-03-01T23:59:59Z"),
	})
	if err != nil {
		t.Fatal(err)
	}
	autogold.Expect(1).Equal(t, numDataPoints)

	// How many data points from 03-01 to 03-04?
	numDataPoints, err = store.CountData(ctx, CountDataOpts{
		From: timePtr("2020-03-01T00:00:00Z"),
		To:   timePtr("2020-03-04T23:59:59Z"),
	})
	if err != nil {
		t.Fatal(err)
	}
	autogold.Expect(5).Equal(t, numDataPoints)
}

func TestRecordSeriesPoints(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	ctx := context.Background()
	clock := timeutil.Now
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)
	postgres := database.NewDB(logger, dbtest.NewDB(t))
	permStore := NewInsightPermissionStore(postgres)
	store := NewWithClock(insightsDB, permStore, clock)

	// First test it does not error with no records.
	if err := store.RecordSeriesPoints(ctx, []RecordSeriesPointArgs{}); err != nil {
		t.Fatal(err)
	}

	optionalString := func(v string) *string { return &v }
	optionalRepoID := func(v api.RepoID) *api.RepoID { return &v }

	current := time.Date(2021, time.September, 10, 10, 0, 0, 0, time.UTC)

	records := []RecordSeriesPointArgs{
		{
			SeriesID:    "one",
			Point:       SeriesPoint{Time: current, Value: 1.1},
			RepoName:    optionalString("repo1"),
			RepoID:      optionalRepoID(3),
			PersistMode: RecordMode,
		},
		{
			SeriesID:    "one",
			Point:       SeriesPoint{Time: current.Add(-time.Hour * 24 * 14), Value: 2.2},
			RepoName:    optionalString("repo1"),
			RepoID:      optionalRepoID(3),
			PersistMode: RecordMode,
		},
		{
			SeriesID:    "one",
			Point:       SeriesPoint{Time: current.Add(-time.Hour * 24 * 28), Value: 3.3},
			RepoName:    optionalString("repo1"),
			RepoID:      optionalRepoID(3),
			PersistMode: SnapshotMode,
		},
		{
			SeriesID:    "one",
			Point:       SeriesPoint{Time: current.Add(-time.Hour * 24 * 42), Value: 3.3},
			RepoName:    optionalString("repo1"),
			RepoID:      optionalRepoID(3),
			PersistMode: SnapshotMode,
		},
	}
	if err := store.RecordSeriesPoints(ctx, records); err != nil {
		t.Fatal(err)
	}

	want := []SeriesPoint{
		{
			SeriesID: "one",
			Time:     current.Add(-time.Hour * 24 * 42),
			Value:    3.3,
		},
		{
			SeriesID: "one",
			Time:     current.Add(-time.Hour * 24 * 28),
			Value:    3.3,
		},
		{
			SeriesID: "one",
			Time:     current.Add(-time.Hour * 24 * 14),
			Value:    2.2,
		},
		{
			SeriesID: "one",
			Time:     current,
			Value:    1.1,
		},
	}

	// Confirm we get the expected data back.
	points, err := store.SeriesPoints(ctx, SeriesPointsOpts{})
	if err != nil {
		t.Fatal(err)
	}
	stringify := func(points []SeriesPoint) []string {
		s := []string{}
		for _, point := range points {
			s = append(s, point.String())
		}
		return s
	}
	autogold.Expect(stringify(want)).Equal(t, stringify(points))
}

func TestRecordSeriesPointsSnapshotOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	ctx := context.Background()
	clock := timeutil.Now
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)
	postgres := database.NewDB(logger, dbtest.NewDB(t))
	permStore := NewInsightPermissionStore(postgres)
	store := NewWithClock(insightsDB, permStore, clock)

	optionalString := func(v string) *string { return &v }
	optionalRepoID := func(v api.RepoID) *api.RepoID { return &v }

	current := time.Date(2021, time.September, 10, 10, 0, 0, 0, time.UTC)

	records := []RecordSeriesPointArgs{
		{
			SeriesID:    "one",
			Point:       SeriesPoint{Time: current, Value: 1.1},
			RepoName:    optionalString("repo1"),
			RepoID:      optionalRepoID(3),
			PersistMode: SnapshotMode,
		},
	}
	if err := store.RecordSeriesPoints(ctx, records); err != nil {
		t.Fatal(err)
	}

	// check snapshots table has a row
	row := store.QueryRow(ctx, sqlf.Sprintf("select count(*) from %s", sqlf.Sprintf(snapshotsTable)))
	if row.Err() != nil {
		t.Fatal(row.Err())
	}

	want := 1
	var got int
	err := row.Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected count from snapshots table (want/got): %v", diff)
	}

	// check recordings table has no rows
	row = store.QueryRow(ctx, sqlf.Sprintf("select count(*) from %s", sqlf.Sprintf(recordingTable)))
	if row.Err() != nil {
		t.Fatal(row.Err())
	}

	want = 0
	err = row.Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected count from recordings table (want/got): %v", diff)
	}
}

func TestRecordSeriesPointsRecordingOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	ctx := context.Background()
	clock := timeutil.Now
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)
	postgres := database.NewDB(logger, dbtest.NewDB(t))
	permStore := NewInsightPermissionStore(postgres)
	store := NewWithClock(insightsDB, permStore, clock)

	optionalString := func(v string) *string { return &v }
	optionalRepoID := func(v api.RepoID) *api.RepoID { return &v }

	current := time.Date(2021, time.September, 10, 10, 0, 0, 0, time.UTC)

	records := []RecordSeriesPointArgs{
		{
			SeriesID:    "one",
			Point:       SeriesPoint{Time: current, Value: 1.1},
			RepoName:    optionalString("repo1"),
			RepoID:      optionalRepoID(3),
			PersistMode: RecordMode,
		},
	}
	if err := store.RecordSeriesPoints(ctx, records); err != nil {
		t.Fatal(err)
	}

	// check snapshots table has a row
	row := store.QueryRow(ctx, sqlf.Sprintf("select count(*) from %s", sqlf.Sprintf(snapshotsTable)))
	if row.Err() != nil {
		t.Fatal(row.Err())
	}

	want := 0
	var got int
	err := row.Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected count from snapshots table (want/got): %v", diff)
	}

	// check recordings table has no rows
	row = store.QueryRow(ctx, sqlf.Sprintf("select count(*) from %s", sqlf.Sprintf(recordingTable)))
	if row.Err() != nil {
		t.Fatal(row.Err())
	}

	want = 1
	err = row.Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected count from recordings table (want/got): %v", diff)
	}
}

func TestDeleteSnapshots(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	ctx := context.Background()
	clock := timeutil.Now
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)
	postgres := database.NewDB(logger, dbtest.NewDB(t))
	permStore := NewInsightPermissionStore(postgres)
	insightStore := NewInsightStore(insightsDB)
	store := NewWithClock(insightsDB, permStore, clock)

	optionalString := func(v string) *string { return &v }
	optionalRepoID := func(v api.RepoID) *api.RepoID { return &v }

	current := time.Date(2021, time.September, 10, 10, 0, 0, 0, time.UTC)
	seriesID := "one"

	series := types.InsightSeries{
		SeriesID:           seriesID,
		Query:              "query-1",
		OldestHistoricalAt: current.Add(-time.Hour * 24 * 365),
		LastRecordedAt:     current.Add(-time.Hour * 24 * 365),
		NextRecordingAfter: current,
		LastSnapshotAt:     current,
		NextSnapshotAfter:  current,
		Enabled:            true,
		SampleIntervalUnit: string(types.Month),
		GenerationMethod:   types.Search,
	}
	series, err := insightStore.CreateSeries(ctx, series)
	if err != nil {
		t.Fatal(err)
	}
	if series.ID != 1 {
		t.Errorf("expected first series to have id 1")
	}
	records := []RecordSeriesPointArgs{
		{
			SeriesID:    seriesID,
			Point:       SeriesPoint{Time: current, Value: 1.1},
			RepoName:    optionalString("repo1"),
			RepoID:      optionalRepoID(3),
			PersistMode: SnapshotMode,
		},
		{
			SeriesID:    seriesID,
			Point:       SeriesPoint{Time: current.Add(time.Hour), Value: 1.1}, // offsetting the time by an hour so that the point is not deduplicated
			RepoName:    optionalString("repo1"),
			RepoID:      optionalRepoID(3),
			PersistMode: RecordMode,
		},
	}
	recordingTimes := types.InsightSeriesRecordingTimes{
		InsightSeriesID: 1,
		RecordingTimes:  []types.RecordingTime{{Timestamp: current, Snapshot: true}, {Timestamp: current.Add(time.Hour), Snapshot: false}},
	}
	if err := store.RecordSeriesPointsAndRecordingTimes(ctx, records, recordingTimes); err != nil {
		t.Fatal(err)
	}

	// first check that we have one recording and one snapshot
	points, err := store.SeriesPoints(ctx, SeriesPointsOpts{SeriesID: &seriesID})
	if err != nil {
		t.Fatal(err)
	}
	got := len(points)
	want := 2
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected count of series points prior to deleting snapshots (want/got): %v", diff)
	}
	err = store.DeleteSnapshots(ctx, &series)
	if err != nil {
		t.Fatal(err)
	}
	// now verify that the remaining point is the recording
	points, err = store.SeriesPoints(ctx, SeriesPointsOpts{SeriesID: &seriesID})
	if err != nil {
		t.Fatal(err)
	}
	got = len(points)
	want = 1
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected count of series points after deleting snapshots (want/got): %v", diff)
	}
	autogold.ExpectFile(t, points, autogold.ExportedOnly())

	gotRecordingTimes, err := store.GetInsightSeriesRecordingTimes(ctx, 1, SeriesPointsOpts{})
	if err != nil {
		t.Fatal(err)
	}
	wantRecordingTimes := types.InsightSeriesRecordingTimes{InsightSeriesID: 1, RecordingTimes: []types.RecordingTime{{Timestamp: current.Add(time.Hour)}}}
	autogold.Expect(gotRecordingTimes).Equal(t, wantRecordingTimes)
}

func TestValues(t *testing.T) {
	ids := []api.RepoID{1, 2, 3, 4, 5, 6}
	got := values(ids)
	want := "VALUES (1),(2),(3),(4),(5),(6)"

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected values string: %v", diff)
	}
}

func TestDelete(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	now := time.Date(2021, 12, 1, 0, 0, 0, 0, time.UTC)

	logger := logtest.Scoped(t)
	ctx := context.Background()
	clock := timeutil.Now
	insightsdb := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)

	repoName := "reallygreatrepo"
	repoId := api.RepoID(5)

	postgres := database.NewDB(logger, dbtest.NewDB(t))
	permStore := NewInsightPermissionStore(postgres)
	timeseriesStore := NewWithClock(insightsdb, permStore, clock)

	err := timeseriesStore.RecordSeriesPoints(ctx, []RecordSeriesPointArgs{
		{
			SeriesID: "series1",
			Point: SeriesPoint{
				SeriesID: "series1",
				Time:     now,
				Value:    50,
			},
			RepoName:    &repoName,
			RepoID:      &repoId,
			PersistMode: RecordMode,
		},
		{
			SeriesID: "series1",
			Point: SeriesPoint{
				SeriesID: "series1",
				Time:     now,
				Value:    50,
			},
			RepoName:    &repoName,
			RepoID:      &repoId,
			PersistMode: SnapshotMode,
		},
		{
			SeriesID: "series2",
			Point: SeriesPoint{
				SeriesID: "series2",
				Time:     now,
				Value:    25,
			},
			RepoName:    &repoName,
			RepoID:      &repoId,
			PersistMode: RecordMode,
		},
		{
			SeriesID: "series2",
			Point: SeriesPoint{
				SeriesID: "series2",
				Time:     now,
				Value:    25,
			},
			RepoName:    &repoName,
			RepoID:      &repoId,
			PersistMode: SnapshotMode,
		},
	})
	if err != nil {
		t.Error(err)
	}

	err = timeseriesStore.Delete(ctx, "series1")
	if err != nil {
		t.Fatal(err)
	}

	getCountForSeries := func(ctx context.Context, timeseriesStore *Store, mode PersistMode, seriesId string) int {
		table, err := getTableForPersistMode(mode)
		if err != nil {
			t.Fatal(err)
		}
		q := sqlf.Sprintf("select count(*) from %s where series_id = %s;", sqlf.Sprintf(table), seriesId)
		val, err := basestore.ScanInt(timeseriesStore.QueryRow(ctx, q))
		if err != nil {
			t.Fatal(err)
		}
		return val
	}

	if getCountForSeries(ctx, timeseriesStore, RecordMode, "series1") != 0 {
		t.Errorf("expected 0 count for series1 in record table")
	}
	if getCountForSeries(ctx, timeseriesStore, SnapshotMode, "series1") != 0 {
		t.Errorf("expected 0 count for series1 in snapshot table")
	}
	if getCountForSeries(ctx, timeseriesStore, RecordMode, "series2") != 1 {
		t.Errorf("expected 1 count for series2 in record table")
	}
	if getCountForSeries(ctx, timeseriesStore, SnapshotMode, "series2") != 1 {
		t.Errorf("expected 1 count for series2 in snapshot table")
	}
}

func getTableForPersistMode(mode PersistMode) (string, error) {
	switch mode {
	case RecordMode:
		return recordingTable, nil
	case SnapshotMode:
		return snapshotsTable, nil
	default:
		return "", errors.Newf("unsupported insights series point persist mode: %v", mode)
	}
}

func TestInsightSeriesRecordingTimes(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	now := time.Date(2021, 12, 1, 0, 0, 0, 0, time.UTC)

	logger := logtest.Scoped(t)
	ctx := context.Background()
	clock := timeutil.Now
	insightsdb := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)

	postgres := database.NewDB(logger, dbtest.NewDB(t))
	permStore := NewInsightPermissionStore(postgres)
	insightStore := NewInsightStore(insightsdb)
	timeseriesStore := NewWithClock(insightsdb, permStore, clock)

	series := types.InsightSeries{
		SeriesID:           "series1",
		Query:              "query-1",
		OldestHistoricalAt: now.Add(-time.Hour * 24 * 365),
		LastRecordedAt:     now.Add(-time.Hour * 24 * 365),
		NextRecordingAfter: now,
		LastSnapshotAt:     now,
		NextSnapshotAfter:  now,
		Enabled:            true,
		SampleIntervalUnit: string(types.Month),
		GenerationMethod:   types.Search,
	}
	got, err := insightStore.CreateSeries(ctx, series)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 1 {
		t.Errorf("expected first series to have id 1")
	}
	series.SeriesID = "series2" // copy to make a new one
	got, err = insightStore.CreateSeries(ctx, series)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 2 {
		t.Errorf("expected second series to have id 2")
	}

	makeRecordings := func(times []time.Time, snapshot bool) []types.RecordingTime {
		recordings := make([]types.RecordingTime, 0, len(times))
		for _, t := range times {
			recordings = append(recordings, types.RecordingTime{Snapshot: snapshot, Timestamp: t})
		}
		return recordings
	}

	series1Times := []time.Time{now, now.AddDate(0, 1, 0)}
	series2Times := []time.Time{now, now.AddDate(0, 1, 1), now.AddDate(0, -1, 1)}
	series1 := types.InsightSeriesRecordingTimes{
		InsightSeriesID: 1,
		RecordingTimes:  makeRecordings(series1Times, false),
	}
	series2 := types.InsightSeriesRecordingTimes{
		InsightSeriesID: 2,
		RecordingTimes:  makeRecordings(series2Times, false),
	}

	err = timeseriesStore.SetInsightSeriesRecordingTimes(ctx, []types.InsightSeriesRecordingTimes{
		series1,
		series2,
	})
	if err != nil {
		t.Fatal(err)
	}

	stringifyTimes := func(times []time.Time) string {
		s := []string{}
		for _, t := range times {
			s = append(s, t.String())
		}
		sort.Strings(s)
		return strings.Join(s, " ")
	}

	oldTime := now.AddDate(-1, 1, 1)
	afterNow := now.AddDate(0, 0, 1)

	testCases := []struct {
		name     string
		insert   *types.InsightSeriesRecordingTimes
		getFor   int
		getFrom  *time.Time
		getTo    *time.Time
		getAfter *time.Time
		want     autogold.Value
	}{
		{
			name:   "get all recording times for series1",
			getFor: 1,
			want:   autogold.Expect(stringifyTimes(series1Times)),
		},
		{
			name:   "duplicates are not inserted",
			insert: &types.InsightSeriesRecordingTimes{InsightSeriesID: 1, RecordingTimes: makeRecordings([]time.Time{now}, true)},
			getFor: 1,
			want:   autogold.Expect(stringifyTimes(series1Times)),
		},
		{
			name:   "UTC is always used",
			insert: &types.InsightSeriesRecordingTimes{InsightSeriesID: 2, RecordingTimes: makeRecordings([]time.Time{now.Local()}, true)},
			getFor: 2,
			want:   autogold.Expect(stringifyTimes(series2Times)),
		},
		{
			name:    "gets subset of series 2 recording times",
			getFor:  2,
			getFrom: &now,
			want:    autogold.Expect(stringifyTimes(series2Times[:2])),
		},
		{
			name:   "gets subset of series 1 recording times",
			getFor: 1,
			getTo:  &now,
			want:   autogold.Expect(stringifyTimes(series1Times[:1])),
		},
		{
			name:    "gets subset from and to",
			getFor:  2,
			getFrom: &oldTime,
			getTo:   &afterNow,
			want:    autogold.Expect(stringifyTimes(append(series2Times[:1], series2Times[2]))),
		},
		{
			name:     "gets all times after",
			getFor:   1,
			getAfter: &now,
			want:     autogold.Expect(stringifyTimes(series1Times[1:])),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.insert != nil {
				if err := timeseriesStore.SetInsightSeriesRecordingTimes(ctx, []types.InsightSeriesRecordingTimes{*tc.insert}); err != nil {
					t.Fatal(err)
				}
			}
			got, err := timeseriesStore.GetInsightSeriesRecordingTimes(ctx, tc.getFor, SeriesPointsOpts{From: tc.getFrom, To: tc.getTo, After: tc.getAfter})
			if err != nil {
				t.Fatal(err)
			}
			recordingTimes := []time.Time{}
			for _, recording := range got.RecordingTimes {
				recordingTimes = append(recordingTimes, recording.Timestamp)
			}
			tc.want.Equal(t, stringifyTimes(recordingTimes))
		})
	}
}

func Test_coalesceZeroValues(t *testing.T) {
	stringify := func(points []SeriesPoint) []string {
		s := []string{}
		for _, point := range points {
			s = append(s, point.String())
		}
		// Sort for determinism.
		sort.Strings(s)
		return s
	}
	testTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	generateTimes := func(n int) []time.Time {
		times := []time.Time{}
		for i := range n {
			times = append(times, testTime.AddDate(0, 0, i))
		}
		return times
	}
	capture := func(s string) *string {
		return &s
	}
	makeRecordingTimes := func(times []time.Time) []types.RecordingTime {
		recordingTimes := make([]types.RecordingTime, len(times))
		for i, t := range times {
			recordingTimes[i] = types.RecordingTime{Timestamp: t}
		}
		return recordingTimes
	}

	testCases := []struct {
		name           string
		points         map[string]*SeriesPoint
		recordingTimes []time.Time
		captureValues  map[string]struct{}
		want           autogold.Value
	}{
		{
			"empty returns empty",
			nil,
			nil,
			nil,
			autogold.Expect([]string{}),
		},
		{
			"empty recording times returns empty",
			map[string]*SeriesPoint{
				"2020-01-01 00:00:00 +0000 UTC": {"seriesID", testTime, 12, nil},
			},
			[]time.Time{},
			map[string]struct{}{"": {}},
			autogold.Expect([]string{}),
		},
		{
			"augment one data point",
			map[string]*SeriesPoint{
				"2020-01-01 00:00:00 +0000 UTC": {"seriesID", testTime, 1, nil},
			},
			generateTimes(2),
			map[string]struct{}{"": {}},
			autogold.Expect([]string{
				`SeriesPoint{Time: "2020-01-01 00:00:00 +0000 UTC", Value: 1}`,
				`SeriesPoint{Time: "2020-01-02 00:00:00 +0000 UTC", Value: 0}`,
			}),
		},
		{
			"augment capture data points",
			map[string]*SeriesPoint{
				"2020-01-01 00:00:00 +0000 UTCone":   {"1", testTime, 1, capture("one")},
				"2020-01-01 00:00:00 +0000 UTCtwo":   {"1", testTime, 2, capture("two")},
				"2020-01-01 00:00:00 +0000 UTCthree": {"1", testTime, 3, capture("three")},
				"2020-01-02 00:00:00 +0000 UTCone":   {"1", testTime.AddDate(0, 0, 1), 1, capture("one")},
			},
			generateTimes(2),
			map[string]struct{}{"one": {}, "two": {}, "three": {}},
			autogold.Expect([]string{
				`SeriesPoint{Time: "2020-01-01 00:00:00 +0000 UTC", Capture: "one", Value: 1}`,
				`SeriesPoint{Time: "2020-01-01 00:00:00 +0000 UTC", Capture: "three", Value: 3}`,
				`SeriesPoint{Time: "2020-01-01 00:00:00 +0000 UTC", Capture: "two", Value: 2}`,
				`SeriesPoint{Time: "2020-01-02 00:00:00 +0000 UTC", Capture: "one", Value: 1}`,
				`SeriesPoint{Time: "2020-01-02 00:00:00 +0000 UTC", Capture: "three", Value: 0}`,
				`SeriesPoint{Time: "2020-01-02 00:00:00 +0000 UTC", Capture: "two", Value: 0}`,
			}),
		},
		{
			"augment data point in the past",
			map[string]*SeriesPoint{
				"2020-01-01 00:00:00 +0000 UTC": {"1", testTime, 11, nil},
				"2020-01-02 00:00:00 +0000 UTC": {"1", testTime.AddDate(0, 0, 1), 22, nil},
			},
			append([]time.Time{testTime.AddDate(0, 0, -1)}, generateTimes(2)...),
			map[string]struct{}{"": {}},
			autogold.Expect([]string{
				`SeriesPoint{Time: "2019-12-31 00:00:00 +0000 UTC", Value: 0}`,
				`SeriesPoint{Time: "2020-01-01 00:00:00 +0000 UTC", Value: 11}`,
				`SeriesPoint{Time: "2020-01-02 00:00:00 +0000 UTC", Value: 22}`,
			}),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := coalesceZeroValues("1", tc.points, tc.captureValues, makeRecordingTimes(tc.recordingTimes))
			tc.want.Equal(t, stringify(got))
		})
	}
}

func TestGetOffsetNRecordingTime(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)
	mainDB := database.NewDB(logger, dbtest.NewDB(t))

	insightStore := NewInsightStore(insightsDB)
	seriesStore := New(insightsDB, NewInsightPermissionStore(mainDB))

	// create a series with id 1 to attach to recording times
	setupSeries(ctx, insightStore, t)

	// we want the 6th oldest sample time
	n := 6

	var expectedOldestTimestamp time.Time
	var expectedOldestTimestampExcludeSnapshot time.Time

	newTime := time.Now().Truncate(time.Hour)
	recordingTimes := types.InsightSeriesRecordingTimes{
		InsightSeriesID: 1,
		RecordingTimes: []types.RecordingTime{
			{newTime, true},
		},
	}
	for i := 1; i <= 11; i++ {
		newTime = newTime.Add(-1 * time.Hour)
		recordingTimes.RecordingTimes = append(recordingTimes.RecordingTimes, types.RecordingTime{
			Snapshot: false, Timestamp: newTime,
		})
		if i == n+1 {
			expectedOldestTimestampExcludeSnapshot = newTime
		}
		if i == n {
			expectedOldestTimestamp = newTime
		}
	}
	if err := seriesStore.SetInsightSeriesRecordingTimes(ctx, []types.InsightSeriesRecordingTimes{recordingTimes}); err != nil {
		t.Fatal(err)
	}

	t.Run("include snapshot timestamps", func(t *testing.T) {
		got, err := seriesStore.GetOffsetNRecordingTime(ctx, 1, n, false)
		if err != nil {
			t.Fatal(err)
		}
		if got.String() != expectedOldestTimestamp.String() {
			t.Errorf("expected timestamp %v got %v", expectedOldestTimestamp, got)
		}
	})
	t.Run("exclude snapshot timestamps", func(t *testing.T) {
		got, err := seriesStore.GetOffsetNRecordingTime(ctx, 1, n, true)
		if err != nil {
			t.Fatal(err)
		}
		if got.String() != expectedOldestTimestampExcludeSnapshot.String() {
			t.Errorf("expected timestamp %v got %v", expectedOldestTimestampExcludeSnapshot, got)
		}
	})
}

func TestGetAllDataForInsightViewId(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)

	permissionStore := NewMockInsightPermissionStore()
	// no repo restrictions by default
	permissionStore.GetUnauthorizedRepoIDsFunc.SetDefaultReturn(nil, nil)

	insightStore := NewInsightStore(insightsDB)
	seriesStore := New(insightsDB, permissionStore)

	// insert all view and series metadata
	view, err := insightStore.CreateView(ctx, types.InsightView{
		Title:            "my view",
		Description:      "my view description",
		UniqueID:         "1",
		PresentationType: types.Line,
	}, []InsightViewGrant{GlobalGrant()})
	if err != nil {
		t.Fatal(err)
	}
	series := setupSeries(ctx, insightStore, t)
	if series.SeriesID != "series1" {
		t.Fatal("series setup is incorrect, series id should be series1")
	}

	err = insightStore.AttachSeriesToView(ctx, series, view, types.InsightViewSeriesMetadata{
		Label:  "label",
		Stroke: "blue",
	})
	if err != nil {
		t.Fatal(err)
	}

	recordingTimes := types.InsightSeriesRecordingTimes{InsightSeriesID: series.ID}
	newTime := time.Now().Truncate(time.Hour)
	for i := 1; i <= 2; i++ {
		newTime = newTime.Add(time.Hour).UTC()
		recordingTimes.RecordingTimes = append(recordingTimes.RecordingTimes, types.RecordingTime{
			Snapshot: false, Timestamp: newTime,
		})
	}
	if err := seriesStore.SetInsightSeriesRecordingTimes(ctx, []types.InsightSeriesRecordingTimes{recordingTimes}); err != nil {
		t.Fatal(err)
	}

	t.Run("empty entries for no series points data", func(t *testing.T) {
		got, err := seriesStore.GetAllDataForInsightViewID(ctx, ExportOpts{InsightViewUniqueID: view.UniqueID})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(recordingTimes.RecordingTimes) {
			t.Fatalf("expected %d got %d series points for export", len(recordingTimes.RecordingTimes), len(got))
		}
		for i, rt := range recordingTimes.RecordingTimes {
			autogold.Expect(view.Title).Equal(t, got[i].InsightViewTitle)
			autogold.Expect(series.Query).Equal(t, got[i].SeriesQuery)
			autogold.Expect("label").Equal(t, got[i].SeriesLabel)
			autogold.Expect(0).Equal(t, got[i].Value)
			autogold.Expect(rt.Timestamp).Equal(t, got[i].RecordingTime.UTC())
			autogold.Expect(true).Equal(t, got[i].RepoName == nil && got[i].Capture == nil)
		}
	})

	// insert series point data
	_, err = insightsDB.ExecContext(context.Background(), `
INSERT INTO repo_names(name) VALUES ('github.com/gorilla/mux-original');
SELECT setseed(0.5);
INSERT INTO series_points(
	time,
	series_id,
	value,
	repo_id,
	repo_name_id,
	original_repo_name_id
)
SELECT recording_time,
    'series1',
    11,
    1111,
    (SELECT id FROM repo_names WHERE name = 'github.com/gorilla/mux-original'),
    (SELECT id FROM repo_names WHERE name = 'github.com/gorilla/mux-original')
	FROM insight_series_recording_times WHERE insight_series_id = 1;
`)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("only live data", func(t *testing.T) {
		got, err := seriesStore.GetAllDataForInsightViewID(ctx, ExportOpts{InsightViewUniqueID: view.UniqueID})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(recordingTimes.RecordingTimes) {
			t.Errorf("expected %d got %d series points for export", len(recordingTimes.RecordingTimes), len(got))
		}
		for _, sp := range got {
			repo := "github.com/gorilla/mux-original"
			var capture *string
			autogold.Expect(view.Title).Equal(t, sp.InsightViewTitle)
			autogold.Expect(series.Query).Equal(t, sp.SeriesQuery)
			autogold.Expect("label").Equal(t, sp.SeriesLabel)
			autogold.Expect(11).Equal(t, sp.Value)
			autogold.Expect(&repo).Equal(t, sp.RepoName)
			autogold.Expect(capture).Equal(t, sp.Capture)
		}
	})
	t.Run("respects repo permissions", func(t *testing.T) {
		permissionStore.GetUnauthorizedRepoIDsFunc.SetDefaultReturn([]api.RepoID{1111}, nil)
		defer func() {
			// cleanup
			permissionStore.GetUnauthorizedRepoIDsFunc.SetDefaultReturn(nil, nil)
		}()
		got, err := seriesStore.GetAllDataForInsightViewID(ctx, ExportOpts{InsightViewUniqueID: view.UniqueID})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("expected 0 results due to repo permissions, got %d", len(got))
		}
	})
	t.Run("respects include repo filter", func(t *testing.T) {
		// insert more series point data
		_, err = insightsDB.ExecContext(context.Background(), `
INSERT INTO repo_names(name) VALUES ('github.com/sourcegraph/sourcegraph');
SELECT setseed(0.5);
INSERT INTO series_points(
	time,
	series_id,
	value,
	repo_id,
	repo_name_id,
	original_repo_name_id
)
SELECT recording_time,
    'series1',
    22,
    2222,
    (SELECT id FROM repo_names WHERE name = 'github.com/sourcegraph/sourcegraph'),
    (SELECT id FROM repo_names WHERE name = 'github.com/sourcegraph/sourcegraph')
	FROM insight_series_recording_times WHERE insight_series_id = 1;
`)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			insightsDB.ExecContext(context.Background(), `DELETE FROM series_points WHERE repo_id = 2222`)
		}()
		got, err := seriesStore.GetAllDataForInsightViewID(ctx, ExportOpts{InsightViewUniqueID: view.UniqueID, ExcludeRepoRegex: []string{"gorilla"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 got %d series points for export", len(got))
		}
		for _, sp := range got {
			repo := "github.com/sourcegraph/sourcegraph"
			var capture *string
			autogold.Expect(view.Title).Equal(t, sp.InsightViewTitle)
			autogold.Expect(series.Query).Equal(t, sp.SeriesQuery)
			autogold.Expect("label").Equal(t, sp.SeriesLabel)
			autogold.Expect(22).Equal(t, sp.Value)
			autogold.Expect(&repo).Equal(t, sp.RepoName)
			autogold.Expect(capture).Equal(t, sp.Capture)
		}
	})
	t.Run("respects exclude repo filter", func(t *testing.T) {
		got, err := seriesStore.GetAllDataForInsightViewID(ctx, ExportOpts{InsightViewUniqueID: view.UniqueID, ExcludeRepoRegex: []string{"mux-original"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("expected 0 results due to filtering, got %d", len(got))
		}
	})
	t.Run("adds empty entry for no series points data", func(t *testing.T) {
		// add new recording time
		extraTime := newTime.Add(time.Hour).UTC()
		newRecordingTime := types.InsightSeriesRecordingTimes{InsightSeriesID: series.ID, RecordingTimes: []types.RecordingTime{
			{Timestamp: extraTime},
		}}
		if err := seriesStore.SetInsightSeriesRecordingTimes(ctx, []types.InsightSeriesRecordingTimes{newRecordingTime}); err != nil {
			t.Fatal(err)
		}
		got, err := seriesStore.GetAllDataForInsightViewID(ctx, ExportOpts{InsightViewUniqueID: view.UniqueID})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(recordingTimes.RecordingTimes)+1 {
			t.Fatalf("expected %d got %d series points for export", len(recordingTimes.RecordingTimes)+1, len(got))
		}
	})
}

func setupSeries(ctx context.Context, tx *InsightStore, t *testing.T) types.InsightSeries {
	now := time.Now()
	series := types.InsightSeries{
		SeriesID:           "series1",
		Query:              "query-1",
		OldestHistoricalAt: now.Add(-time.Hour * 24 * 365),
		LastRecordedAt:     now.Add(-time.Hour * 24 * 365),
		NextRecordingAfter: now,
		LastSnapshotAt:     now,
		NextSnapshotAfter:  now,
		Enabled:            true,
		SampleIntervalUnit: string(types.Month),
		GenerationMethod:   types.Search,
	}
	got, err := tx.CreateSeries(ctx, series)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 1 {
		t.Errorf("expected first series to have id 1")
	}
	return got
}
