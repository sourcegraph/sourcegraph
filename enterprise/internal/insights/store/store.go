package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

// Interface is the interface describing a code insights store. See the Store struct
// for actual API usage.
type Interface interface {
	SeriesPoints(ctx context.Context, opts SeriesPointsOpts) ([]SeriesPoint, error)
	RecordSeriesPoint(ctx context.Context, v RecordSeriesPointArgs) error
	DistinctSeriesWithData(ctx context.Context, from, to time.Time) ([]string, error)
}

var _ Interface = &Store{}

// Store exposes methods to read and write code insights domain models from
// persistent storage.
type Store struct {
	*basestore.Store
	now func() time.Time
}

// New returns a new Store backed by the given Timescale db.
func New(db dbutil.DB) *Store {
	return NewWithClock(db, timeutil.Now)
}

// NewWithClock returns a new Store backed by the given db and
// clock for timestamps.
func NewWithClock(db dbutil.DB, clock func() time.Time) *Store {
	return &Store{Store: basestore.NewWithDB(db, sql.TxOptions{}), now: clock}
}

var _ basestore.ShareableStore = &Store{}

// Handle returns the underlying transactable database handle.
// Needed to implement the ShareableStore interface.
func (s *Store) Handle() *basestore.TransactableHandle { return s.Store.Handle() }

// With creates a new Store with the given basestore.Shareable store as the
// underlying basestore.Store.
// Needed to implement the basestore.Store interface
func (s *Store) With(other basestore.ShareableStore) *Store {
	return &Store{Store: s.Store.With(other), now: s.now}
}

var _ Interface = &Store{}

// SeriesPoint describes a single insights' series data point.
//
// Some fields that could be queried (series ID, repo ID/names) are omitted as they are primarily
// only useful for filtering the data you get back, and would inflate the data size considerably
// otherwise.
type SeriesPoint struct {
	// Time (always UTC).
	Time     time.Time
	Value    float64
	Metadata []byte
}

func (s *SeriesPoint) String() string {
	return fmt.Sprintf("SeriesPoint{Time: %q, Value: %v, Metadata: %s}", s.Time, s.Value, s.Metadata)
}

// SeriesPointsOpts describes options for querying insights' series data points.
type SeriesPointsOpts struct {
	// SeriesID is the unique series ID to query, if non-nil.
	SeriesID *string

	// TODO(slimsag): Add ability to filter based on repo ID, name, original name.
	// TODO(slimsag): Add ability to do limited filtering based on metadata.

	// Time ranges to query from/to, if non-nil, in UTC.
	From, To *time.Time

	// Limit is the number of data points to query, if non-zero.
	Limit int
}

// SeriesPoints queries data points over time for a specific insights' series.
func (s *Store) SeriesPoints(ctx context.Context, opts SeriesPointsOpts) ([]SeriesPoint, error) {
	points := make([]SeriesPoint, 0, opts.Limit)
	err := s.query(ctx, seriesPointsQuery(opts), func(sc scanner) error {
		var point SeriesPoint
		err := sc.Scan(
			&point.Time,
			&point.Value,
			&point.Metadata,
		)
		if err != nil {
			return err
		}
		points = append(points, point)
		return nil
	})
	return points, err
}

// Note that the series_points table may contain duplicate points, or points recorded at irregular
// intervals. In specific:
//
// 1. It may have multiple points recorded at the same exact point in time, e.g. with different
//    repo_id (datapoint recorded per repository), or only a single point recorded (datapoint
//    recorded globally.)
// 2. Rarely, it may contain duplicate data points. For example, when repo-updater is started the
//    initial jobs for recording insights will be enqueued, and then e.g. 12h later. If repo-updater
//    gets restarted multiple times, there may be many multiple nearly identical data points recorded
//    in a short period of time instead of at the 12h interval.
// 3. Data backfilling may not operate at the same interval, or same # of points per interval, and
//    thus the interval between data points may be irregular.
// 4. Searches may not complete at the same exact time, so even in a perfect world if the interval
//    should be 12h it may be off by a minute or so.
//
// Additionally, it is important to note that there may be data points associated with a repo OR not
// associated with a repo at all (global.)
//
// Because we want 1 point per N interval, and do not want to display duplicate points in the UI, we
// use a time_bucket() with an MAX() aggregation. This gives us one data point for some time interval,
// even if multiple were recorded in that timeframe.
//
// One goal of this query is to get e.g. the total number of search results (value) across all repos
// (or some subset selected by the WHERE clause.) In this case, you can imagine each repo having its
// results recorded at the 12h interval. There may be duplicate points. The subquery uses a time_bucket()
// and MAX() aggregation to get the "# of search results per unique repository", eliminating duplicate
// data points, and the top-level SUM() adds those together to get "# of search results across all
// repositories."
//
// Another goal of this query is to get e.g. "total # of services (value) deployed at our company",
// in which case `repo_id` and other repo fields will be NULL. The inner query still eliminates potential
// duplicate data points and the outer query in this case just SUMs one data point (as we don't have
// points per repository.)
var seriesPointsQueryFmtstr = `
-- source: enterprise/internal/insights/store/store.go:SeriesPoints
SELECT sub.time_bucket,
	SUM(sub.max),
	sub.metadata
FROM (
	SELECT time_bucket(INTERVAL '15 days', time) AS time_bucket,
		MAX(value),
		m.metadata,
		series_id,
		repo_id
	FROM series_points p
	LEFT JOIN metadata m ON p.metadata_id = m.id
	WHERE %s
	GROUP BY time_bucket, metadata, series_id, repo_id
	ORDER BY time_bucket DESC
) sub
GROUP BY time_bucket, metadata
ORDER BY time_bucket DESC
`

func seriesPointsQuery(opts SeriesPointsOpts) *sqlf.Query {
	preds := []*sqlf.Query{}

	if opts.SeriesID != nil {
		preds = append(preds, sqlf.Sprintf("series_id = %s", *opts.SeriesID))
	}
	if opts.From != nil {
		preds = append(preds, sqlf.Sprintf("time >= %s", *opts.From))
	}
	if opts.To != nil {
		preds = append(preds, sqlf.Sprintf("time <= %s", *opts.To))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}
	limitClause := ""
	if opts.Limit > 0 {
		limitClause = fmt.Sprintf("LIMIT %d", opts.Limit)
	}
	return sqlf.Sprintf(
		seriesPointsQueryFmtstr+limitClause,
		sqlf.Join(preds, "\n AND "),
	)
}

// DistinctSeriesWithData returns the distinct Series IDs that have at least one data point recorded
// in the given time range.
func (s *Store) DistinctSeriesWithData(ctx context.Context, from, to time.Time) ([]string, error) {
	query := sqlf.Sprintf(distinctSeriesWithDataFmtstr, from, to)
	var seriesIDs []string
	err := s.query(ctx, query, func(sc scanner) error {
		var seriesID string
		err := sc.Scan(&seriesID)
		if err != nil {
			return err
		}
		seriesIDs = append(seriesIDs, seriesID)
		return nil
	})
	return seriesIDs, err
}

const distinctSeriesWithDataFmtstr = `
SELECT DISTINCT series_id FROM series_points WHERE time >= %s AND time <= %s;
`

// RecordSeriesPointArgs describes arguments for the RecordSeriesPoint method.
type RecordSeriesPointArgs struct {
	// SeriesID is the unique series ID to query. It should describe the series of data uniquely,
	// but is not a DB table primary key ID.
	SeriesID string

	// Point is the actual data point recorded and at what time.
	Point SeriesPoint

	// Repository name and DB ID to associate with this data point, if any.
	//
	// Both must be specified if one is specified.
	RepoName *string
	RepoID   *api.RepoID

	// Metadata contains arbitrary JSON metadata to associate with the data point, if any.
	//
	// See the DB schema comments for intended use cases. This should generally be small,
	// low-cardinality data to avoid inflating the table.
	Metadata interface{}
}

// RecordSeriesPoint records a data point for the specfied series ID (which is a unique ID for the
// series, not a DB table primary key ID).
func (s *Store) RecordSeriesPoint(ctx context.Context, v RecordSeriesPointArgs) (err error) {
	// Start transaction.
	var txStore *basestore.Store
	txStore, err = s.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = txStore.Done(err) }()

	if (v.RepoName != nil && v.RepoID == nil) || (v.RepoID != nil && v.RepoName == nil) {
		return errors.New("RepoName and RepoID must be mutually specified")
	}

	// Upsert the repository name into a separate table, so we get a small ID we can reference
	// many times from the series_points table without storing the repo name multiple times.
	var repoNameID *int
	if v.RepoName != nil {
		repoNameIDValue, ok, err := basestore.ScanFirstInt(txStore.Query(ctx, sqlf.Sprintf(upsertRepoNameFmtStr, *v.RepoName, *v.RepoName)))
		if err != nil {
			return errors.Wrap(err, "upserting repo name ID")
		}
		if !ok {
			return errors.Wrap(err, "repo name ID not found (this should never happen)")
		}
		repoNameID = &repoNameIDValue
	}

	// Upsert the metadata into a separate table, so we get a small ID we can reference many times
	// from the series_points table without storing the metadata multiple times.
	var metadataID *int
	if v.Metadata != nil {
		jsonMetadata, err := json.Marshal(v.Metadata)
		if err != nil {
			return errors.Wrap(err, "upserting: encoding metadata")
		}
		metadataIDValue, ok, err := basestore.ScanFirstInt(txStore.Query(ctx, sqlf.Sprintf(upsertMetadataFmtStr, jsonMetadata, jsonMetadata)))
		if err != nil {
			return errors.Wrap(err, "upserting metadata ID")
		}
		if !ok {
			return errors.Wrap(err, "metadata ID not found (this should never happen)")
		}
		metadataID = &metadataIDValue
	}

	// Insert the actual data point.
	return txStore.Exec(ctx, sqlf.Sprintf(
		recordSeriesPointFmtstr,
		v.SeriesID,         // series_id
		v.Point.Time.UTC(), // time
		v.Point.Value,      // value
		metadataID,         // metadata_id
		v.RepoID,           // repo_id
		repoNameID,         // repo_name_id
		repoNameID,         // original_repo_name_id
	))
}

const upsertRepoNameFmtStr = `
-- source: enterprise/internal/insights/store/store.go:RecordSeriesPoint
WITH e AS(
	INSERT INTO repo_names(name)
	VALUES (%s)
	ON CONFLICT DO NOTHING
	RETURNING id
)
SELECT * FROM e
UNION
	SELECT id FROM repo_names WHERE name = %s;
`

const upsertMetadataFmtStr = `
-- source: enterprise/internal/insights/store/store.go:RecordSeriesPoint
WITH e AS(
    INSERT INTO metadata(metadata)
    VALUES (%s)
    ON CONFLICT DO NOTHING
    RETURNING id
)
SELECT * FROM e
UNION
	SELECT id FROM metadata WHERE metadata = %s;
`

const recordSeriesPointFmtstr = `
-- source: enterprise/internal/insights/store/store.go:RecordSeriesPoint
INSERT INTO series_points(
	series_id,
	time,
	value,
	metadata_id,
	repo_id,
	repo_name_id,
	original_repo_name_id)
VALUES (%s, %s, %s, %s, %s, %s, %s);
`

func (s *Store) query(ctx context.Context, q *sqlf.Query, sc scanFunc) error {
	rows, err := s.Store.Query(ctx, q)
	if err != nil {
		return err
	}
	return scanAll(rows, sc)
}

// scanner captures the Scan method of sql.Rows and sql.Row
type scanner interface {
	Scan(dst ...interface{}) error
}

// a scanFunc scans one or more rows from a scanner, returning
// the last id column scanned and the count of scanned rows.
type scanFunc func(scanner) (err error)

func scanAll(rows *sql.Rows, scan scanFunc) (err error) {
	defer func() { err = basestore.CloseRows(rows, err) }()
	for rows.Next() {
		if err = scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
