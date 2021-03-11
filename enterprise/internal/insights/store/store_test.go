package store

import (
	"context"
	"testing"
	"time"

	"github.com/hexops/autogold"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func TestSeriesPoints(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()

	ctx := context.Background()
	clock := timeutil.Now
	timescale, cleanup := dbtesting.TimescaleDB(t)
	defer cleanup()
	store := NewWithClock(timescale, clock)

	// Confirm we get no results initially.
	points, err := store.SeriesPoints(ctx, SeriesPointsOpts{})
	if err != nil {
		t.Fatal(err)
	}
	autogold.Want("SeriesPoints", []SeriesPoint{}).Equal(t, points)

	// Insert some fake data.
	_, err = timescale.Exec(`
INSERT INTO repo_names(name) VALUES ('github.com/gorilla/mux-original');
INSERT INTO repo_names(name) VALUES ('github.com/gorilla/mux-renamed');
INSERT INTO metadata(metadata) VALUES ('{"hello": "world", "languages": ["Go", "Python", "Java"]}');
SELECT setseed(0.5);
INSERT INTO series_points(
    time,
	series_id,
    value,
    metadata_id,
    repo_id,
    repo_name_id,
    original_repo_name_id)
SELECT time,
    'somehash',
    random()*80 - 40,
    (SELECT id FROM metadata WHERE metadata = '{"hello": "world", "languages": ["Go", "Python", "Java"]}'),
    2,
    (SELECT id FROM repo_names WHERE name = 'github.com/gorilla/mux-renamed'),
    (SELECT id FROM repo_names WHERE name = 'github.com/gorilla/mux-original')
	FROM generate_series(TIMESTAMP '2020-01-01 00:00:00', TIMESTAMP '2020-06-01 00:00:00', INTERVAL '240 min') AS time;
`)
	if err != nil {
		t.Fatal(err)
	}

	time := func(s string) *time.Time {
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
		autogold.Want("SeriesPoints(2).len", int(12)).Equal(t, len(points))
		autogold.Want("SeriesPoints(2)[len()-1].String()", `SeriesPoint{Time: "2019-12-19 00:00:00 +0000 UTC", Value: 38.50014526394119, Metadata: {"hello": "world", "languages": ["Go", "Python", "Java"]}}`).Equal(t, points[len(points)-1].String())
		autogold.Want("SeriesPoints(2)[0].String()", `SeriesPoint{Time: "2020-06-01 00:00:00 +0000 UTC", Value: -37.8750440811433, Metadata: {"hello": "world", "languages": ["Go", "Python", "Java"]}}`).Equal(t, points[0].String())
	})

	t.Run("subset of data", func(t *testing.T) {
		// Confirm we can get a subset of data points.
		points, err = store.SeriesPoints(ctx, SeriesPointsOpts{
			From: time("2020-03-01T00:00:00Z"),
			To:   time("2020-06-01T00:00:00Z"),
		})
		if err != nil {
			t.Fatal(err)
		}
		autogold.Want("SeriesPoints(3).len", int(8)).Equal(t, len(points))
		autogold.Want("SeriesPoints(3)[0].String()", `SeriesPoint{Time: "2020-06-01 00:00:00 +0000 UTC", Value: -37.8750440811433, Metadata: {"hello": "world", "languages": ["Go", "Python", "Java"]}}`).Equal(t, points[0].String())
		autogold.Want("SeriesPoints(3)[len()-1].String()", `SeriesPoint{Time: "2020-02-17 00:00:00 +0000 UTC", Value: 36.186608083675935, Metadata: {"hello": "world", "languages": ["Go", "Python", "Java"]}}`).Equal(t, points[len(points)-1].String())
	})

	t.Run("latest 3 points", func(t *testing.T) {
		// Confirm we can get a subset of data points.
		points, err = store.SeriesPoints(ctx, SeriesPointsOpts{
			Limit: 3,
		})
		if err != nil {
			t.Fatal(err)
		}
		autogold.Want("SeriesPoints(4).len", int(3)).Equal(t, len(points))
		autogold.Want("SeriesPoints(4)[0].String()", `SeriesPoint{Time: "2020-06-01 00:00:00 +0000 UTC", Value: -37.8750440811433, Metadata: {"hello": "world", "languages": ["Go", "Python", "Java"]}}`).Equal(t, points[0].String())
		autogold.Want("SeriesPoints(4)[1].String()", `SeriesPoint{Time: "2020-05-17 00:00:00 +0000 UTC", Value: 39.775432350908204, Metadata: {"hello": "world", "languages": ["Go", "Python", "Java"]}}`).Equal(t, points[1].String())
		autogold.Want("SeriesPoints(4)[2].String()", `SeriesPoint{Time: "2020-05-02 00:00:00 +0000 UTC", Value: 39.61012571588327, Metadata: {"hello": "world", "languages": ["Go", "Python", "Java"]}}`).Equal(t, points[2].String())
	})

}

func TestDistinctSeriesWithData(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()

	ctx := context.Background()
	clock := timeutil.Now
	timescale, cleanup := dbtesting.TimescaleDB(t)
	defer cleanup()
	store := NewWithClock(timescale, clock)

	time := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	optionalString := func(v string) *string { return &v }
	optionalRepoID := func(v api.RepoID) *api.RepoID { return &v }

	// Record some duplicate data points.
	for _, record := range []RecordSeriesPointArgs{
		{
			SeriesID: "one",
			Point:    SeriesPoint{Time: time("2020-03-01T00:00:00Z"), Value: 1.1},
			RepoName: optionalString("repo1"),
			RepoID:   optionalRepoID(3),
			Metadata: map[string]interface{}{"some": "data"},
		},
		{
			SeriesID: "two",
			Point:    SeriesPoint{Time: time("2020-03-02T00:00:00Z"), Value: 2.2},
			Metadata: []interface{}{"some", "data", "two"},
		},
		{
			SeriesID: "two",
			Point:    SeriesPoint{Time: time("2020-03-02T00:01:00Z"), Value: 2.2},
			Metadata: []interface{}{"some", "data", "two"},
		},
		{
			SeriesID: "three",
			Point:    SeriesPoint{Time: time("2020-03-03T00:00:00Z"), Value: 3.3},
			Metadata: nil,
		},
		{
			SeriesID: "three",
			Point:    SeriesPoint{Time: time("2020-03-03T00:01:00Z"), Value: 3.3},
			Metadata: nil,
		},
	} {
		if err := store.RecordSeriesPoint(ctx, record); err != nil {
			t.Fatal(err)
		}
	}

	// Which series on 02-29 have data?
	withData, err := store.DistinctSeriesWithData(ctx, time("2020-02-29T00:00:00Z"), time("2020-02-29T23:59:59Z"))
	if err != nil {
		t.Fatal(err)
	}
	autogold.Want("first", []string{}).Equal(t, withData)

	// Which series on 03-01 have data?
	withData, err = store.DistinctSeriesWithData(ctx, time("2020-03-01T00:00:00Z"), time("2020-03-01T23:59:59Z"))
	if err != nil {
		t.Fatal(err)
	}
	autogold.Want("second", []string{"one"}).Equal(t, withData)

	// Which series from 03-01 to 03-04 have data?
	withData, err = store.DistinctSeriesWithData(ctx, time("2020-03-01T00:00:00Z"), time("2020-03-04T23:59:59Z"))
	if err != nil {
		t.Fatal(err)
	}
	autogold.Want("third", []string{"one", "three", "two"}).Equal(t, withData)
}

func TestRecordSeriesPoints(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()

	ctx := context.Background()
	clock := timeutil.Now
	timescale, cleanup := dbtesting.TimescaleDB(t)
	defer cleanup()
	store := NewWithClock(timescale, clock)

	time := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	optionalString := func(v string) *string { return &v }
	optionalRepoID := func(v api.RepoID) *api.RepoID { return &v }

	// Record some data points.
	for _, record := range []RecordSeriesPointArgs{
		{
			SeriesID: "one",
			Point:    SeriesPoint{Time: time("2020-03-01T00:00:00Z"), Value: 1.1},
			RepoName: optionalString("repo1"),
			RepoID:   optionalRepoID(3),
			Metadata: map[string]interface{}{"some": "data"},
		},
		{
			SeriesID: "two",
			Point:    SeriesPoint{Time: time("2020-03-02T00:00:00Z"), Value: 2.2},
			Metadata: []interface{}{"some", "data", "two"},
		},
		{
			SeriesID: "no metadata",
			Point:    SeriesPoint{Time: time("2020-03-03T00:00:00Z"), Value: 3.3},
			Metadata: nil,
		},
	} {
		if err := store.RecordSeriesPoint(ctx, record); err != nil {
			t.Fatal(err)
		}
	}

	// Confirm we get the expected data back.
	points, err := store.SeriesPoints(ctx, SeriesPointsOpts{})
	if err != nil {
		t.Fatal(err)
	}
	autogold.Want("len(points)", int(3)).Equal(t, len(points))
	autogold.Want("points[0].String()", `SeriesPoint{Time: "2020-03-03 00:00:00 +0000 UTC", Value: 3.3, Metadata: }`).Equal(t, points[0].String())
	autogold.Want("points[1].String()", `SeriesPoint{Time: "2020-02-17 00:00:00 +0000 UTC", Value: 2.2, Metadata: ["some", "data", "two"]}`).Equal(t, points[1].String())
	autogold.Want("points[2].String()", `SeriesPoint{Time: "2020-02-17 00:00:00 +0000 UTC", Value: 1.1, Metadata: {"some": "data"}}`).Equal(t, points[2].String())

	// Confirm the data point with repository name got recorded correctly.
	// TODO(slimsag): future: once we support querying by repo ID/names, add tests to ensure that information is inserted properly here.
}
