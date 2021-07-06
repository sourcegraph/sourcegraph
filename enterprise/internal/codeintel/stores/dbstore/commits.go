package dbstore

import (
	"bytes"
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/opentracing/opentracing-go/log"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/commitgraph"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

// scanCommitGraphView scans a commit graph view from the return value of `*Store.query`.
func scanCommitGraphView(rows *sql.Rows, queryErr error) (_ *commitgraph.CommitGraphView, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	commitGraphView := commitgraph.NewCommitGraphView()

	for rows.Next() {
		var meta commitgraph.UploadMeta
		var commit, token string

		if err := rows.Scan(&meta.UploadID, &commit, &token, &meta.Distance); err != nil {
			return nil, err
		}

		commitGraphView.Add(meta, commit, token)
	}

	return commitGraphView, nil
}

// HasRepository determines if there is LSIF data for the given repository.
func (s *Store) HasRepository(ctx context.Context, repositoryID int) (_ bool, err error) {
	ctx, endObservation := s.operations.hasRepository.With(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("repositoryID", repositoryID),
	}})
	defer endObservation(1, observation.Args{})

	count, _, err := basestore.ScanFirstInt(s.Store.Query(ctx, sqlf.Sprintf(hasRepositoryQuery, repositoryID)))
	return count > 0, err
}

const hasRepositoryQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:HasRepository
SELECT COUNT(*) FROM lsif_uploads WHERE state != 'deleted' AND repository_id = %s LIMIT 1
`

// HasCommit determines if the given commit is known for the given repository.
func (s *Store) HasCommit(ctx context.Context, repositoryID int, commit string) (_ bool, err error) {
	ctx, endObservation := s.operations.hasCommit.With(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("repositoryID", repositoryID),
		log.String("commit", commit),
	}})
	defer endObservation(1, observation.Args{})

	count, _, err := basestore.ScanFirstInt(s.Store.Query(
		ctx,
		sqlf.Sprintf(
			hasCommitQuery,
			repositoryID, dbutil.CommitBytea(commit),
			repositoryID, dbutil.CommitBytea(commit),
		),
	))

	return count > 0, err
}

const hasCommitQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:HasCommit
SELECT
	(SELECT COUNT(*) FROM lsif_nearest_uploads WHERE repository_id = %s AND commit_bytea = %s) +
	(SELECT COUNT(*) FROM lsif_nearest_uploads_links WHERE repository_id = %s AND commit_bytea = %s)
`

// MarkRepositoryAsDirty marks the given repository's commit graph as out of date.
func (s *Store) MarkRepositoryAsDirty(ctx context.Context, repositoryID int) (err error) {
	ctx, endObservation := s.operations.markRepositoryAsDirty.With(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("repositoryID", repositoryID),
	}})
	defer endObservation(1, observation.Args{})

	return s.Store.Exec(ctx, sqlf.Sprintf(markRepositoryAsDirtyQuery, repositoryID))
}

const markRepositoryAsDirtyQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:MarkRepositoryAsDirty
INSERT INTO lsif_dirty_repositories (repository_id, dirty_token, update_token)
VALUES (%s, 1, 0)
ON CONFLICT (repository_id) DO UPDATE SET dirty_token = lsif_dirty_repositories.dirty_token + 1
`

func scanIntPairs(rows *sql.Rows, queryErr error) (_ map[int]int, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	values := map[int]int{}
	for rows.Next() {
		var value1 int
		var value2 int
		if err := rows.Scan(&value1, &value2); err != nil {
			return nil, err
		}

		values[value1] = value2
	}

	return values, nil
}

// DirtyRepositories returns a map from repository identifiers to a dirty token for each repository whose commit
// graph is out of date. This token should be passed to CalculateVisibleUploads in order to unmark the repository.
func (s *Store) DirtyRepositories(ctx context.Context) (_ map[int]int, err error) {
	ctx, traceLog, endObservation := s.operations.dirtyRepositories.WithAndLogger(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	repositories, err := scanIntPairs(s.Store.Query(ctx, sqlf.Sprintf(dirtyRepositoriesQuery)))
	if err != nil {
		return nil, err
	}
	traceLog(log.Int("numRepositories", len(repositories)))

	return repositories, nil
}

const dirtyRepositoriesQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:DirtyRepositories
SELECT lsif_dirty_repositories.repository_id, lsif_dirty_repositories.dirty_token
  FROM lsif_dirty_repositories
    INNER JOIN repo ON repo.id = lsif_dirty_repositories.repository_id
  WHERE dirty_token > update_token
    AND repo.deleted_at IS NULL
`

// CommitGraphMetadata returns whether or not the commit graph for the given repository is stale, along with the date of
// the most recent commit graph refresh for the given repository.
func (s *Store) CommitGraphMetadata(ctx context.Context, repositoryID int) (stale bool, updatedAt *time.Time, err error) {
	ctx, endObservation := s.operations.commitGraphMetadata.With(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("repositoryID", repositoryID),
	}})
	defer endObservation(1, observation.Args{})

	updateToken, dirtyToken, updatedAt, exists, err := scanCommitGraphMetadata(s.Store.Query(ctx, sqlf.Sprintf(commitGraphQuery, repositoryID)))
	if err != nil {
		return false, nil, err
	}
	if !exists {
		return false, nil, nil
	}

	return updateToken != dirtyToken, updatedAt, err
}

const commitGraphQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:CommitGraphMetadata
SELECT update_token, dirty_token, updated_at FROM lsif_dirty_repositories WHERE repository_id = %s LIMIT 1
`

// scanCommitGraphMetadata scans a a commit graph metadata row from the return value of `*Store.query`.
func scanCommitGraphMetadata(rows *sql.Rows, queryErr error) (updateToken, dirtyToken int, updatedAt *time.Time, _ bool, err error) {
	if queryErr != nil {
		return 0, 0, nil, false, queryErr
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	if rows.Next() {
		if err := rows.Scan(&updateToken, &dirtyToken, &updatedAt); err != nil {
			return 0, 0, nil, false, err
		}

		return updateToken, dirtyToken, updatedAt, true, nil
	}

	return 0, 0, nil, false, nil
}

// CalculateVisibleUploads uses the given commit graph and the tip of non-stale branches and tags to determine the
// set of LSIF uploads that are visible for each commit, and the set of uploads which are visible at the tip of a
// non-stale branch or tag. The decorated commit graph is serialized to Postgres for use by find closest dumps
// queries.
//
// If dirtyToken is supplied, the repository will be unmarked when the supplied token does matches the most recent
// token stored in the database, the flag will not be cleared as another request for update has come in since this
// token has been read.
func (s *Store) CalculateVisibleUploads(
	ctx context.Context,
	repositoryID int,
	commitGraph *gitserver.CommitGraph,
	refDescriptions map[string]gitserver.RefDescription,
	maxAgeForNonStaleBranches time.Duration,
	maxAgeForNonStaleTags time.Duration,
	dirtyToken int,
	now time.Time,
) (err error) {
	ctx, traceLog, endObservation := s.operations.calculateVisibleUploads.WithAndLogger(ctx, &err, observation.Args{
		LogFields: []log.Field{
			log.Int("repositoryID", repositoryID),
			log.Int("numCommitGraphKeys", len(commitGraph.Order())),
			log.Int("numRefDescriptions", len(refDescriptions)),
			log.Int("dirtyToken", dirtyToken),
		},
	})
	defer endObservation(1, observation.Args{})

	tx, err := s.transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	// Determine the retention policy for this repository
	maxAgeForNonStaleBranches, maxAgeForNonStaleTags, err = refineRetentionConfiguration(ctx, tx, repositoryID, maxAgeForNonStaleBranches, maxAgeForNonStaleTags)
	if err != nil {
		return err
	}
	traceLog(
		log.String("maxAgeForNonStaleBranches", maxAgeForNonStaleBranches.String()),
		log.String("maxAgeForNonStaleTags", maxAgeForNonStaleTags.String()),
	)

	// Pull all queryable upload metadata known to this repository so we can correlate
	// it with the current  commit graph.
	commitGraphView, err := scanCommitGraphView(tx.Store.Query(ctx, sqlf.Sprintf(calculateVisibleUploadsCommitGraphQuery, repositoryID)))
	if err != nil {
		return err
	}
	traceLog(
		log.Int("numCommitGraphViewMetaKeys", len(commitGraphView.Meta)),
		log.Int("numCommitGraphViewTokenKeys", len(commitGraphView.Tokens)),
	)

	// Determine which uploads are visible to which commits for this repository
	graph := commitgraph.NewGraph(commitGraph, commitGraphView)

	// Write the graph into temporary tables in Postgres
	if err := tx.writeVisibleUploads(ctx, sanitizeCommitInput(ctx, graph, refDescriptions, maxAgeForNonStaleBranches, maxAgeForNonStaleTags)); err != nil {
		return err
	}

	// Persist data to permenant table: t_lsif_nearest_uploads -> lsif_nearest_uploads
	if err := tx.persistNearestUploads(ctx, repositoryID); err != nil {
		return err
	}

	// Persist data to permenant table: t_lsif_nearest_uploads_links -> lsif_nearest_uploads_links
	if err := tx.persistNearestUploadsLinks(ctx, repositoryID); err != nil {
		return err
	}

	// Persist data to permenant table: t_lsif_uploads_visible_at_tip -> lsif_uploads_visible_at_tip
	if err := tx.persistUploadsVisibleAtTip(ctx, repositoryID); err != nil {
		return err
	}

	if dirtyToken != 0 {
		// If the user requests us to clear a dirty token, set the updated_token value to
		// the dirty token if it wouldn't decrease the value. Dirty repositories are determined
		// by having a non-equal dirty and update token, and we want the most recent upload
		// token to win this write.
		if err := tx.Store.Exec(ctx, sqlf.Sprintf(calculateVisibleUploadsDirtyRepositoryQuery, dirtyToken, now, repositoryID)); err != nil {
			return err
		}
	}

	return nil
}

const calculateVisibleUploadsCommitGraphQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:CalculateVisibleUploads
SELECT id, commit, md5(root || ':' || indexer) as token, 0 as distance FROM lsif_uploads WHERE state = 'completed' AND repository_id = %s
`

const calculateVisibleUploadsDirtyRepositoryQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:CalculateVisibleUploads
UPDATE lsif_dirty_repositories SET update_token = GREATEST(update_token, %s), updated_at = %s WHERE repository_id = %s
`

// refineRetentionConfiguration returns the maximum age for no-stale branches and tags, effectively, as configured
// for hte given repository. If there is no retention configuration for the given repository, the given default
// values are returned unchanged.
func refineRetentionConfiguration(ctx context.Context, store *Store, repositoryID int, maxAgeForNonStaleBranches, maxAgeForNonStaleTags time.Duration) (_, _ time.Duration, err error) {
	rows, err := store.Store.Query(ctx, sqlf.Sprintf(retentionConfigurationQuery, repositoryID))
	if err != nil {
		return 0, 0, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	for rows.Next() {
		var v1, v2 int
		if err := rows.Scan(&v1, &v2); err != nil {
			return 0, 0, err
		}

		maxAgeForNonStaleBranches = time.Second * time.Duration(v1)
		maxAgeForNonStaleTags = time.Second * time.Duration(v2)
	}

	return maxAgeForNonStaleBranches, maxAgeForNonStaleTags, nil
}

const retentionConfigurationQuery = `
SELECT max_age_for_non_stale_branches_seconds, max_age_for_non_stale_tags_seconds
FROM lsif_retention_configuration
WHERE repository_id = %s
`

// writeVisibleUploads serializes the given input into a the following set of temporary tables in the database.
//
//   - t_lsif_nearest_uploads        (mirroring lsif_nearest_uploads)
//   - t_lsif_nearest_uploads_links  (mirroring lsif_nearest_uploads_links)
//   - t_lsif_uploads_visible_at_tip (mirroring lsif_uploads_visible_at_tip)
//
// The data in these temporary tables can then be moved into a persisted/permanent table. We previously would perform a
// bulk delete of the records associated with a repository, then reinsert all of the data needed to be persisted. This
// caused massive table bloat on some instances. Storing into a temporary table and then inserting/updating/deleting
// records into the persisted table minimizes the number of tuples we need to touch and drastically reduces table bloat.
func (s *Store) writeVisibleUploads(ctx context.Context, sanitizedInput *sanitizedCommitInput) (err error) {
	ctx, traceLog, endObservation := s.operations.writeVisibleUploads.WithAndLogger(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	if err := s.createTemporaryNearestUploadsTables(ctx); err != nil {
		return err
	}

	// Insert the set of uploads that are visible from each commit for a given repository into a temporary table.
	nearestUploadsWriter := func() error {
		return batch.InsertValues(
			ctx,
			s.Handle().DB(),
			"t_lsif_nearest_uploads",
			[]string{"commit_bytea", "uploads"},
			sanitizedInput.nearestUploadsRowValues,
		)
	}

	// Insert the commits not inserted into the table above by adding links to a unique ancestor and their relative
	// distance in the graph into another temporary table. We use this as a cheap way to reconstruct the full data
	// set, which is multiplicative in the size of the commit graph AND the number of unique roots.
	nearestUploadsLinksWriter := func() error {
		return batch.InsertValues(
			ctx,
			s.Handle().DB(),
			"t_lsif_nearest_uploads_links",
			[]string{"commit_bytea", "ancestor_commit_bytea", "distance"},
			sanitizedInput.nearestUploadsLinksRowValues,
		)
	}

	// Insert the set of uploads visible from the tip of the default branch into a temporary table. These values are
	// used to determine which bundles for a repository we open during a global find references query.
	uploadsVisibleAtTipWriter := func() error {
		return batch.InsertValues(
			ctx,
			s.Handle().DB(),
			"t_lsif_uploads_visible_at_tip",
			[]string{"upload_id", "branch_or_tag_name", "is_default_branch"},
			sanitizedInput.uploadsVisibleAtTipRowValues,
		)
	}

	if err := goroutine.Parallel(nearestUploadsWriter, nearestUploadsLinksWriter, uploadsVisibleAtTipWriter); err != nil {
		return err
	}
	traceLog(
		log.Int("numNearestUploadsRecords", int(sanitizedInput.numNearestUploadsRecords)),
		log.Int("numNearestUploadsLinksRecords", int(sanitizedInput.numNearestUploadsLinksRecords)),
		log.Int("numUploadsVisibleAtTipRecords", int(sanitizedInput.numUploadsVisibleAtTipRecords)),
	)

	return nil
}

func (s *Store) createTemporaryNearestUploadsTables(ctx context.Context) error {
	temporaryTableQueries := []string{
		temporaryNearestUploadsTableQuery,
		temporaryNearestUploadsLinksTableQuery,
		temporaryUploadsVisibleAtTipTableQuery,
	}

	for _, temporaryTableQuery := range temporaryTableQueries {
		if err := s.Store.Exec(ctx, sqlf.Sprintf(temporaryTableQuery)); err != nil {
			return err
		}
	}

	return nil
}

const temporaryNearestUploadsTableQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:createTemporaryNearestUploadsTables
CREATE TEMPORARY TABLE t_lsif_nearest_uploads (
	commit_bytea bytea NOT NULL,
	uploads      jsonb NOT NULL
) ON COMMIT DROP
`

const temporaryNearestUploadsLinksTableQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:createTemporaryNearestUploadsTables
CREATE TEMPORARY TABLE t_lsif_nearest_uploads_links (
	commit_bytea          bytea NOT NULL,
	ancestor_commit_bytea bytea NOT NULL,
	distance              integer NOT NULL
) ON COMMIT DROP
`

const temporaryUploadsVisibleAtTipTableQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:createTemporaryNearestUploadsTables
CREATE TEMPORARY TABLE t_lsif_uploads_visible_at_tip (
	upload_id integer NOT NULL,
	branch_or_tag_name text NOT NULL,
	is_default_branch boolean NOT NULL
) ON COMMIT DROP
`

// persistNearestUploads modifies the lsif_nearest_uploads table so that it has same data
// as t_lsif_nearest_uploads for the given repository.
func (s *Store) persistNearestUploads(ctx context.Context, repositoryID int) (err error) {
	ctx, traceLog, endObservation := s.operations.persistNearestUploads.WithAndLogger(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	rowsInserted, rowsUpdated, rowsDeleted, err := s.bulkTransfer(
		ctx,
		sqlf.Sprintf(nearestUploadsInsertQuery, repositoryID, repositoryID),
		sqlf.Sprintf(nearestUploadsUpdateQuery, repositoryID),
		sqlf.Sprintf(nearestUploadsDeleteQuery, repositoryID),
	)
	if err != nil {
		return err
	}
	traceLog(
		log.Int("lsif_nearest_uploads.ins", rowsInserted),
		log.Int("lsif_nearest_uploads.upd", rowsUpdated),
		log.Int("lsif_nearest_uploads.del", rowsDeleted),
	)

	return nil
}

const nearestUploadsInsertQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:persistNearestUploads
INSERT INTO lsif_nearest_uploads
SELECT %s, source.commit_bytea, source.uploads
FROM t_lsif_nearest_uploads source
WHERE source.commit_bytea NOT IN (SELECT nu.commit_bytea FROM lsif_nearest_uploads nu WHERE nu.repository_id = %s)
`

const nearestUploadsUpdateQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:persistNearestUploads
UPDATE lsif_nearest_uploads nu
SET uploads = source.uploads
FROM t_lsif_nearest_uploads source
WHERE
	nu.repository_id = %s AND
	nu.commit_bytea = source.commit_bytea AND
	nu.uploads != source.uploads
`

const nearestUploadsDeleteQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:persistNearestUploads
DELETE FROM lsif_nearest_uploads nu
WHERE
	nu.repository_id = %s AND
	nu.commit_bytea NOT IN (SELECT source.commit_bytea FROM t_lsif_nearest_uploads source)
`

// persistNearestUploadsLinks modifies the lsif_nearest_uploads_links table so that it has same
// data as t_lsif_nearest_uploads_links for the given repository.
func (s *Store) persistNearestUploadsLinks(ctx context.Context, repositoryID int) (err error) {
	ctx, traceLog, endObservation := s.operations.persistNearestUploadsLinks.WithAndLogger(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	rowsInserted, rowsUpdated, rowsDeleted, err := s.bulkTransfer(
		ctx,
		sqlf.Sprintf(nearestUploadsLinksInsertQuery, repositoryID, repositoryID),
		sqlf.Sprintf(nearestUploadsLinksUpdateQuery, repositoryID),
		sqlf.Sprintf(nearestUploadsLinksDeleteQuery, repositoryID),
	)
	if err != nil {
		return err
	}
	traceLog(
		log.Int("lsif_nearest_uploads_links.ins", rowsInserted),
		log.Int("lsif_nearest_uploads_links.upd", rowsUpdated),
		log.Int("lsif_nearest_uploads_links.del", rowsDeleted),
	)

	return nil
}

const nearestUploadsLinksInsertQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:persistNearestUploadsLinks
INSERT INTO lsif_nearest_uploads_links
SELECT %s, source.commit_bytea, source.ancestor_commit_bytea, source.distance
FROM t_lsif_nearest_uploads_links source
WHERE source.commit_bytea NOT IN (SELECT nul.commit_bytea FROM lsif_nearest_uploads_links nul WHERE nul.repository_id = %s)
`

const nearestUploadsLinksUpdateQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:persistNearestUploadsLinks
UPDATE lsif_nearest_uploads_links nul
SET ancestor_commit_bytea = source.ancestor_commit_bytea, distance = source.distance
FROM t_lsif_nearest_uploads_links source
WHERE
	nul.repository_id = %s AND
	nul.commit_bytea = source.commit_bytea AND
	nul.ancestor_commit_bytea != source.ancestor_commit_bytea AND
	nul.distance != source.distance
`

const nearestUploadsLinksDeleteQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:persistNearestUploadsLinks
DELETE FROM lsif_nearest_uploads_links nul
WHERE
	nul.repository_id = %s AND
	nul.commit_bytea NOT IN (SELECT source.commit_bytea FROM t_lsif_nearest_uploads_links source)
`

// persistUploadsVisibleAtTip modifies the lsif_uploads_visible_at_tip table so that it has same
// data as t_lsif_uploads_visible_at_tip for the given repository.
func (s *Store) persistUploadsVisibleAtTip(ctx context.Context, repositoryID int) (err error) {
	ctx, traceLog, endObservation := s.operations.persistUploadsVisibleAtTip.WithAndLogger(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	rowsInserted, rowsUpdated, rowsDeleted, err := s.bulkTransfer(
		ctx,
		sqlf.Sprintf(uploadsVisibleAtTipInsertQuery, repositoryID, repositoryID),
		nil,
		sqlf.Sprintf(uploadsVisibleAtTipDeleteQuery, repositoryID),
	)
	if err != nil {
		return err
	}
	traceLog(
		log.Int("lsif_uploads_visible_at_tip.ins", rowsInserted),
		log.Int("lsif_uploads_visible_at_tip.upd", rowsUpdated),
		log.Int("lsif_uploads_visible_at_tip.del", rowsDeleted),
	)

	return nil
}

const uploadsVisibleAtTipInsertQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:persistUploadsVisibleAtTip
INSERT INTO lsif_uploads_visible_at_tip
SELECT %s, source.upload_id, source.branch_or_tag_name, source.is_default_branch
FROM t_lsif_uploads_visible_at_tip source
WHERE NOT EXISTS (
	SELECT 1
	FROM lsif_uploads_visible_at_tip vat
	WHERE
		vat.repository_id = %s AND
		vat.upload_id = source.upload_id AND
		vat.branch_or_tag_name = source.branch_or_tag_name AND
		vat.is_default_branch = source.is_default_branch
)
`

const uploadsVisibleAtTipDeleteQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:persistUploadsVisibleAtTip
DELETE FROM lsif_uploads_visible_at_tip vat
WHERE
	vat.repository_id = %s AND
	NOT EXISTS (
		SELECT 1
		FROM t_lsif_uploads_visible_at_tip source
		WHERE
			source.upload_id = vat.upload_id AND
			source.branch_or_tag_name = vat.branch_or_tag_name AND
			source.is_default_branch = vat.is_default_branch
	)
`

// bulkTransfer performs the given insert, update, and delete queries and returns the number of records
// touched by each. If any query is nil, the returned count will be zero.
func (s *Store) bulkTransfer(ctx context.Context, insertQuery, updateQuery, deleteQuery *sqlf.Query) (rowsInserted int, rowsUpdated int, rowsDeleted int, err error) {
	prepareQuery := func(query *sqlf.Query) *sqlf.Query {
		if query == nil {
			return sqlf.Sprintf("SELECT 0")
		}

		return sqlf.Sprintf("%s RETURNING 1", query)
	}

	rows, err := s.Store.Query(ctx, sqlf.Sprintf(bulkTransferQuery, prepareQuery(insertQuery), prepareQuery(updateQuery), prepareQuery(deleteQuery)))
	if err != nil {
		return 0, 0, 0, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	if rows.Next() {
		if err := rows.Scan(&rowsInserted, &rowsUpdated, &rowsDeleted); err != nil {
			return 0, 0, 0, err
		}

		return rowsInserted, rowsUpdated, rowsDeleted, nil
	}

	return 0, 0, 0, nil
}

const bulkTransferQuery = `
-- source: enterprise/internal/codeintel/stores/dbstore/commits.go:bulkTransfer
WITH
	ins AS (%s),
	upd AS (%s),
	del AS (%s)
SELECT
	(SELECT COUNT(*) FROM ins) AS num_ins,
	(SELECT COUNT(*) FROM upd) AS num_upd,
	(SELECT COUNT(*) FROM del) AS num_del
`

type uploadMetaListSerializer struct {
	buf     bytes.Buffer
	scratch []byte
}

func NewUploadMetaListSerializer() *uploadMetaListSerializer {
	return &uploadMetaListSerializer{
		scratch: make([]byte, 4),
	}
}

// Serialize returns a new byte slice with the given upload metadata values encoded
// as a JSON object (keys being the upload_id and values being the distance field).
//
// Our original attempt just built a map[int]int and passed it to the JSON package
// to be marshalled into a byte array. Unfortunately that puts reflection over the
// map value in the hot path for commit graph processing. We also can't avoid the
// reflection by passing a struct without changing the shape of the data persisted
// in the database.
//
// By serializing this value ourselves we minimize allocations. This change resulted
// in a 50% reduction of the memory required by BenchmarkCalculateVisibleUploads.
//
// This method is not safe for concurrent use.
func (s *uploadMetaListSerializer) Serialize(uploadMetas []commitgraph.UploadMeta) []byte {
	s.write(uploadMetas)
	return s.take()
}

func (s *uploadMetaListSerializer) write(uploadMetas []commitgraph.UploadMeta) {
	s.buf.WriteByte('{')
	for i, uploadMeta := range uploadMetas {
		if i > 0 {
			s.buf.WriteByte(',')
		}

		s.writeUploadMeta(uploadMeta)
	}
	s.buf.WriteByte('}')
}

func (s *uploadMetaListSerializer) writeUploadMeta(uploadMeta commitgraph.UploadMeta) {
	s.buf.WriteByte('"')
	s.writeInteger(uploadMeta.UploadID)
	s.buf.Write([]byte{'"', ':'})
	s.writeInteger(int(uploadMeta.Distance))
}

func (s *uploadMetaListSerializer) writeInteger(value int) {
	s.scratch = s.scratch[:0]
	s.scratch = strconv.AppendInt(s.scratch, int64(value), 10)
	s.buf.Write(s.scratch)
}

func (s *uploadMetaListSerializer) take() []byte {
	dest := make([]byte, s.buf.Len())
	copy(dest, s.buf.Bytes())
	s.buf.Reset()

	return dest
}

type sanitizedCommitInput struct {
	nearestUploadsRowValues       <-chan []interface{}
	nearestUploadsLinksRowValues  <-chan []interface{}
	uploadsVisibleAtTipRowValues  <-chan []interface{}
	numNearestUploadsRecords      uint32 // populated once nearestUploadsRowValues is exhausted
	numNearestUploadsLinksRecords uint32 // populated once nearestUploadsLinksRowValues is exhausted
	numUploadsVisibleAtTipRecords uint32 // populated once uploadsVisibleAtTipRowValues is exhausted
}

// sanitizeCommitInput reads the data that needs to be persisted from the given graph and writes the
// sanitized values (ensures values match the column types) into channels for insertion into a particular
// table.
func sanitizeCommitInput(
	ctx context.Context,
	graph *commitgraph.Graph,
	refDescriptions map[string]gitserver.RefDescription,
	maxAgeForNonStaleBranches time.Duration,
	maxAgeForNonStaleTags time.Duration,
) *sanitizedCommitInput {
	maxAges := map[gitserver.RefType]time.Duration{
		gitserver.RefTypeBranch: maxAgeForNonStaleBranches,
		gitserver.RefTypeTag:    maxAgeForNonStaleTags,
	}

	nearestUploadsRowValues := make(chan []interface{})
	nearestUploadsLinksRowValues := make(chan []interface{})
	uploadsVisibleAtTipRowValues := make(chan []interface{})

	sanitized := &sanitizedCommitInput{
		nearestUploadsRowValues:      nearestUploadsRowValues,
		nearestUploadsLinksRowValues: nearestUploadsLinksRowValues,
		uploadsVisibleAtTipRowValues: uploadsVisibleAtTipRowValues,
	}

	go func() {
		defer close(nearestUploadsRowValues)
		defer close(nearestUploadsLinksRowValues)
		defer close(uploadsVisibleAtTipRowValues)

		listSerializer := NewUploadMetaListSerializer()

		for envelope := range graph.Stream() {
			if envelope.Uploads != nil {
				if !countingWrite(
					ctx,
					nearestUploadsRowValues,
					&sanitized.numNearestUploadsRecords,
					// row values
					dbutil.CommitBytea(envelope.Uploads.Commit),
					listSerializer.Serialize(envelope.Uploads.Uploads),
				) {
					return
				}
			}

			if envelope.Links != nil {
				if !countingWrite(
					ctx,
					nearestUploadsLinksRowValues,
					&sanitized.numNearestUploadsLinksRecords,
					// row values
					dbutil.CommitBytea(envelope.Links.Commit),
					dbutil.CommitBytea(envelope.Links.AncestorCommit),
					envelope.Links.Distance,
				) {
					return
				}
			}
		}

		for commit, refDescription := range refDescriptions {
			if !refDescription.IsDefaultBranch {
				maxAge, ok := maxAges[refDescription.Type]
				if !ok || time.Since(refDescription.CreatedDate) > maxAge {
					continue
				}
			}

			for _, uploadMeta := range graph.UploadsVisibleAtCommit(commit) {
				if !countingWrite(
					ctx,
					uploadsVisibleAtTipRowValues,
					&sanitized.numUploadsVisibleAtTipRecords,
					// row values
					uploadMeta.UploadID,
					refDescription.Name,
					refDescription.IsDefaultBranch,
				) {
					return
				}
			}
		}
	}()

	return sanitized
}

// countingWrite writes the given slice of interfaces to the given channel. This function returns true
// if the write succeeded and false if the context was canceled. On success, the counter's underlying
// value will be incremented (non-atomically).
func countingWrite(ctx context.Context, ch chan<- []interface{}, counter *uint32, values ...interface{}) bool {
	select {
	case ch <- values:
		*counter++
		return true

	case <-ctx.Done():
		return false
	}
}
