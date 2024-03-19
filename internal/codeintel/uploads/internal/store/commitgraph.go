package store

import (
	"bytes"
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/keegancsmith/sqlf"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/commitgraph"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

// SetRepositoryAsDirty marks the given repository's commit graph as out of date.
func (s *store) SetRepositoryAsDirty(ctx context.Context, repositoryID int) (err error) {
	ctx, _, endObservation := s.operations.setRepositoryAsDirty.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", repositoryID),
	}})
	defer endObservation(1, observation.Args{})

	return s.db.Exec(ctx, sqlf.Sprintf(setRepositoryAsDirtyQuery, repositoryID))
}

// GetDirtyRepositories returns list of repositories whose commit graph is out of date. The dirty token should be
// passed to CalculateVisibleUploads in order to unmark the repository.
func (s *store) GetDirtyRepositories(ctx context.Context) (_ []shared.DirtyRepository, err error) {
	ctx, trace, endObservation := s.operations.getDirtyRepositories.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	repositories, err := scanDirtyRepositories(s.db.Query(ctx, sqlf.Sprintf(dirtyRepositoriesQuery)))
	if err != nil {
		return nil, err
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("numRepositories", len(repositories)))

	return repositories, nil
}

const dirtyRepositoriesQuery = `
SELECT ldr.repository_id, repo.name, ldr.dirty_token
  FROM lsif_dirty_repositories ldr
    INNER JOIN repo ON repo.id = ldr.repository_id
  WHERE ldr.dirty_token > ldr.update_token
    AND repo.deleted_at IS NULL
	AND repo.blocked IS NULL
`

var scanDirtyRepositories = basestore.NewSliceScanner(func(s dbutil.Scanner) (dr shared.DirtyRepository, _ error) {
	err := s.Scan(&dr.RepositoryID, &dr.RepositoryName, &dr.DirtyToken)
	return dr, err
})

// UpdateUploadsVisibleToCommits uses the given commit graph and the tip of non-stale branches and tags to determine the
// set of LSIF uploads that are visible for each commit, and the set of uploads which are visible at the tip of a
// non-stale branch or tag. The decorated commit graph is serialized to Postgres for use by find closest dumps
// queries.
//
// If dirtyToken is supplied, the repository will be unmarked when the supplied token does matches the most recent
// token stored in the database, the flag will not be cleared as another request for update has come in since this
// token has been read.
func (s *store) UpdateUploadsVisibleToCommits(
	ctx context.Context,
	repositoryID int,
	commitGraph *gitdomain.CommitGraph,
	refDescriptions map[string][]gitdomain.RefDescription,
	maxAgeForNonStaleBranches time.Duration,
	maxAgeForNonStaleTags time.Duration,
	dirtyToken int,
	now time.Time,
) (err error) {
	ctx, trace, endObservation := s.operations.updateUploadsVisibleToCommits.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", repositoryID),
		attribute.Int("numCommitGraphKeys", len(commitGraph.Order())),
		attribute.Int("numRefDescriptions", len(refDescriptions)),
		attribute.Int("dirtyToken", dirtyToken),
	}})
	defer endObservation(1, observation.Args{})

	return s.withTransaction(ctx, func(tx *store) error {
		// Determine the retention policy for this repository
		maxAgeForNonStaleBranches, maxAgeForNonStaleTags, err = refineRetentionConfiguration(ctx, tx.db, repositoryID, maxAgeForNonStaleBranches, maxAgeForNonStaleTags)
		if err != nil {
			return err
		}
		trace.AddEvent("TODO Domain Owner",
			attribute.String("maxAgeForNonStaleBranches", maxAgeForNonStaleBranches.String()),
			attribute.String("maxAgeForNonStaleTags", maxAgeForNonStaleTags.String()))

		// Pull all queryable upload metadata known to this repository so we can correlate
		// it with the current  commit graph.
		commitGraphView, err := scanCommitGraphView(tx.db.Query(ctx, sqlf.Sprintf(calculateVisibleUploadsCommitGraphQuery, repositoryID)))
		if err != nil {
			return err
		}
		trace.AddEvent("TODO Domain Owner",
			attribute.Int("numCommitGraphViewMetaKeys", len(commitGraphView.Meta)),
			attribute.Int("numCommitGraphViewTokenKeys", len(commitGraphView.Tokens)))

		// Determine which uploads are visible to which commits for this repository
		graph := commitgraph.NewGraph(commitGraph, commitGraphView)

		pctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Return a structure holding several channels that are populated by a background goroutine.
		// When we write this data to temporary tables, we have three consumers pulling values from
		// these channels in parallel. We need to make sure that once we return from this function that
		// the producer routine shuts down. This prevents the producer from leaking if there is an
		// error in one of the consumers before all values have been emitted.
		sanitizedInput := sanitizeCommitInput(pctx, graph, refDescriptions, maxAgeForNonStaleBranches, maxAgeForNonStaleTags)

		// Write the graph into temporary tables in Postgres
		if err := s.writeVisibleUploads(ctx, sanitizedInput, tx.db); err != nil {
			return err
		}

		// Persist data to permanent table: t_lsif_nearest_uploads -> lsif_nearest_uploads
		if err := s.persistNearestUploads(ctx, repositoryID, tx.db); err != nil {
			return err
		}

		// Persist data to permanent table: t_lsif_nearest_uploads_links -> lsif_nearest_uploads_links
		if err := s.persistNearestUploadsLinks(ctx, repositoryID, tx.db); err != nil {
			return err
		}

		// Persist data to permanent table: t_lsif_uploads_visible_at_tip -> lsif_uploads_visible_at_tip
		if err := s.persistUploadsVisibleAtTip(ctx, repositoryID, tx.db); err != nil {
			return err
		}

		if dirtyToken != 0 {
			// If the user requests us to clear a dirty token, set the updated_token value to
			// the dirty token if it wouldn't decrease the value. Dirty repositories are determined
			// by having a non-equal dirty and update token, and we want the most recent upload
			// token to win this write.
			nowTimestamp := sqlf.Sprintf("transaction_timestamp()")
			if !now.IsZero() {
				nowTimestamp = sqlf.Sprintf("%s", now)
			}
			if err := tx.db.Exec(ctx, sqlf.Sprintf(calculateVisibleUploadsDirtyRepositoryQuery, dirtyToken, nowTimestamp, repositoryID)); err != nil {
				return err
			}
		}

		// All completed uploads are now visible. Mark any uploads queued for deletion as deleted as
		// they are no longer reachable from the commit graph and cannot be used to fulfill any API
		// requests.
		unset, _ := tx.db.SetLocal(ctx, "codeintel.lsif_uploads_audit.reason", "upload not reachable within the commit graph")
		defer unset(ctx)
		if err := tx.db.Exec(ctx, sqlf.Sprintf(calculateVisibleUploadsDeleteUploadsQueuedForDeletionQuery, repositoryID)); err != nil {
			return err
		}

		return nil
	})
}

const calculateVisibleUploadsCommitGraphQuery = `
SELECT id, commit, md5(root || ':' || indexer) as token, 0 as distance FROM lsif_uploads WHERE state = 'completed' AND repository_id = %s
`

const calculateVisibleUploadsDirtyRepositoryQuery = `
UPDATE lsif_dirty_repositories SET update_token = GREATEST(update_token, %s), updated_at = %s WHERE repository_id = %s
`

const calculateVisibleUploadsDeleteUploadsQueuedForDeletionQuery = `
WITH
candidates AS (
	SELECT u.id
	FROM lsif_uploads u
	WHERE u.state = 'deleting' AND u.repository_id = %s

	-- Lock these rows in a deterministic order so that we don't
	-- deadlock with other processes updating the lsif_uploads table.
	ORDER BY u.id FOR UPDATE
)
UPDATE lsif_uploads
SET state = 'deleted'
WHERE id IN (SELECT id FROM candidates)
`

// refineRetentionConfiguration returns the maximum age for no-stale branches and tags, effectively, as configured
// for the given repository. If there is no retention configuration for the given repository, the given default
// values are returned unchanged.
func refineRetentionConfiguration(ctx context.Context, store *basestore.Store, repositoryID int, maxAgeForNonStaleBranches, maxAgeForNonStaleTags time.Duration) (_, _ time.Duration, err error) {
	rows, err := store.Query(ctx, sqlf.Sprintf(retentionConfigurationQuery, repositoryID))
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

// GetCommitsVisibleToUpload returns the set of commits for which the given upload can answer code intelligence queries.
// To paginate, supply the token returned from this method to the invocation for the next page.
func (s *store) GetCommitsVisibleToUpload(ctx context.Context, uploadID, limit int, token *string) (_ []string, nextToken *string, err error) {
	ctx, _, endObservation := s.operations.getCommitsVisibleToUpload.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("uploadID", uploadID),
		attribute.Int("limit", limit),
	}})
	defer endObservation(1, observation.Args{})

	after := ""
	if token != nil {
		after = *token
	}

	commits, err := basestore.ScanStrings(s.db.Query(ctx, sqlf.Sprintf(commitsVisibleToUploadQuery, strconv.Itoa(uploadID), after, limit)))
	if err != nil {
		return nil, nil, err
	}

	if len(commits) > 0 {
		last := commits[len(commits)-1]
		nextToken = &last
	}

	return commits, nextToken, nil
}

const commitsVisibleToUploadQuery = `
WITH
direct_commits AS (
	SELECT nu.repository_id, nu.commit_bytea
	FROM lsif_nearest_uploads nu
	WHERE nu.uploads ? %s
),
linked_commits AS (
	SELECT ul.commit_bytea
	FROM direct_commits dc
	JOIN lsif_nearest_uploads_links ul
	ON
		ul.repository_id = dc.repository_id AND
		ul.ancestor_commit_bytea = dc.commit_bytea
),
combined_commits AS (
	SELECT dc.commit_bytea FROM direct_commits dc
	UNION ALL
	SELECT lc.commit_bytea FROM linked_commits lc
)
SELECT encode(c.commit_bytea, 'hex') as commit
FROM combined_commits c
WHERE decode(%s, 'hex') < c.commit_bytea
ORDER BY c.commit_bytea
LIMIT %s
`

// FindClosestCompletedUploads returns the set of uploads that can most accurately answer queries for the given repository, commit, path, and
// optional indexer. If rootMustEnclosePath is true, then only dumps with a root which is a prefix of path are returned. Otherwise,
// any dump with a root intersecting the given path is returned.
//
// This method should be used when the commit is known to exist in the lsif_nearest_uploads table. If it doesn't, then this method
// will return no dumps (as the input commit is not reachable from anything with an upload). The nearest uploads table must be
// refreshed before calling this method when the commit is unknown.
//
// Because refreshing the commit graph can be very expensive, we also provide FindClosestCompletedUploadsFromGraphFragment. That method should
// be used instead in low-latency paths. It should be supplied a small fragment of the commit graph that contains the input commit
// as well as a commit that is likely to exist in the lsif_nearest_uploads table. This is enough to propagate the correct upload
// visibility data down the graph fragment.
//
// The graph supplied to FindClosestCompletedUploadsFromGraphFragment will also determine whether or not it is possible to produce a partial set
// of visible uploads (ideally, we'd like to return the complete set of visible uploads, or fail). If the graph fragment is complete
// by depth (e.g. if the graph contains an ancestor at depth d, then the graph also contains all other ancestors up to depth d), then
// we get the ideal behavior. Only if we contain a partial row of ancestors will we return partial results.
//
// It is possible for some dumps to overlap theoretically, e.g. if someone uploads one dump covering the repository root and then later
// splits the repository into multiple dumps. For this reason, the returned dumps are always sorted in most-recently-finished order to
// prevent returning data from stale dumps.
func (s *store) FindClosestCompletedUploads(ctx context.Context, repositoryID int, commit, path string, rootMustEnclosePath bool, indexer string) (_ []shared.CompletedUpload, err error) {
	ctx, trace, endObservation := s.operations.findClosestCompletedUploads.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", repositoryID),
		attribute.String("commit", commit),
		attribute.String("path", path),
		attribute.Bool("rootMustEnclosePath", rootMustEnclosePath),
		attribute.String("indexer", indexer),
	}})
	defer endObservation(1, observation.Args{})

	conds := makeFindClosestProcessUploadsConditions(path, rootMustEnclosePath, indexer)
	// TODO(id: completed-state-check) Make sure we only return uploads with state = 'completed' here
	query := sqlf.Sprintf(findClosestCompletedUploadsQuery, makeVisibleUploadsQuery(repositoryID, commit), sqlf.Join(conds, " AND "))

	uploads, err := scanCompletedUploads(s.db.Query(ctx, query))
	if err != nil {
		return nil, err
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("numUploads", len(uploads)))

	return uploads, nil
}

const findClosestCompletedUploadsQuery = `
WITH
visible_uploads AS (%s)
SELECT
	u.id,
	u.commit,
	u.root,
	EXISTS (` + visibleAtTipSubselectQuery + `) AS visible_at_tip,
	u.uploaded_at,
	u.state,
	u.failure_message,
	u.started_at,
	u.finished_at,
	u.process_after,
	u.num_resets,
	u.num_failures,
	u.repository_id,
	u.repository_name,
	u.indexer,
	u.indexer_version,
	u.associated_index_id
FROM visible_uploads vu
JOIN lsif_dumps_with_repository_name u ON u.id = vu.upload_id
WHERE %s
ORDER BY u.finished_at DESC
`

// FindClosestCompletedUploadsFromGraphFragment returns the set of uploads that can most accurately answer queries for the given repository, commit,
// path, and optional indexer by only considering the given fragment of the full git graph. See FindClosestCompletedUploads for additional details.
func (s *store) FindClosestCompletedUploadsFromGraphFragment(ctx context.Context, repositoryID int, commit, path string, rootMustEnclosePath bool, indexer string, commitGraph *gitdomain.CommitGraph) (_ []shared.CompletedUpload, err error) {
	ctx, trace, endObservation := s.operations.findClosestCompletedUploadsFromGraphFragment.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", repositoryID),
		attribute.String("commit", commit),
		attribute.String("path", path),
		attribute.Bool("rootMustEnclosePath", rootMustEnclosePath),
		attribute.String("indexer", indexer),
		attribute.Int("numCommitGraphKeys", len(commitGraph.Order())),
	}})
	defer endObservation(1, observation.Args{})

	if len(commitGraph.Order()) == 0 {
		return nil, nil
	}

	commitQueries := make([]*sqlf.Query, 0, len(commitGraph.Graph()))
	for commit := range commitGraph.Graph() {
		commitQueries = append(commitQueries, sqlf.Sprintf("%s", dbutil.CommitBytea(commit)))
	}

	commitGraphView, err := scanCommitGraphView(s.db.Query(ctx, sqlf.Sprintf(
		findClosestCompletedUploadsFromGraphFragmentCommitGraphQuery,
		repositoryID,
		sqlf.Join(commitQueries, ", "),
		repositoryID,
		sqlf.Join(commitQueries, ", "),
	)))
	if err != nil {
		return nil, err
	}
	trace.AddEvent("TODO Domain Owner",
		attribute.Int("numCommitGraphViewMetaKeys", len(commitGraphView.Meta)),
		attribute.Int("numCommitGraphViewTokenKeys", len(commitGraphView.Tokens)))

	var ids []*sqlf.Query
	for _, uploadMeta := range commitgraph.NewGraph(commitGraph, commitGraphView).UploadsVisibleAtCommit(commit) {
		ids = append(ids, sqlf.Sprintf("%d", uploadMeta.UploadID))
	}
	if len(ids) == 0 {
		return nil, nil
	}

	conds := makeFindClosestProcessUploadsConditions(path, rootMustEnclosePath, indexer)
	// TODO(id: completed-state-check) Make sure we only return uploads with state = 'completed' here
	query := sqlf.Sprintf(findClosestCompletedUploadsFromGraphFragmentQuery, sqlf.Join(ids, ","), sqlf.Join(conds, " AND "))

	uploads, err := scanCompletedUploads(s.db.Query(ctx, query))
	if err != nil {
		return nil, err
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("numUploads", len(uploads)))

	return uploads, nil
}

const findClosestCompletedUploadsFromGraphFragmentCommitGraphQuery = `
WITH
visible_uploads AS (
	-- Select the set of uploads visible from one of the given commits. This is done by
	-- looking at each commit's row in the lsif_nearest_uploads table, and the (adjusted)
	-- set of uploads from each commit's nearest ancestor according to the data compressed
	-- in the links table.
	--
	-- NB: A commit should be present in at most one of these tables.
	SELECT
		nu.repository_id,
		upload_id::integer,
		nu.commit_bytea,
		u_distance::text::integer as distance
	FROM lsif_nearest_uploads nu
	CROSS JOIN jsonb_each(nu.uploads) as u(upload_id, u_distance)
	WHERE nu.repository_id = %s AND nu.commit_bytea IN (%s)
	UNION (
		SELECT
			nu.repository_id,
			upload_id::integer,
			ul.commit_bytea,
			u_distance::text::integer + ul.distance as distance
		FROM lsif_nearest_uploads_links ul
		JOIN lsif_nearest_uploads nu ON nu.repository_id = ul.repository_id AND nu.commit_bytea = ul.ancestor_commit_bytea
		CROSS JOIN jsonb_each(nu.uploads) as u(upload_id, u_distance)
		WHERE nu.repository_id = %s AND ul.commit_bytea IN (%s)
	)
)
SELECT
	vu.upload_id,
	encode(vu.commit_bytea, 'hex'),
	md5(u.root || ':' || u.indexer) as token,
	vu.distance
FROM visible_uploads vu
JOIN lsif_uploads u ON u.id = vu.upload_id
`

const findClosestCompletedUploadsFromGraphFragmentQuery = `
SELECT
	u.id,
	u.commit,
	u.root,
	EXISTS (` + visibleAtTipSubselectQuery + `) AS visible_at_tip,
	u.uploaded_at,
	u.state,
	u.failure_message,
	u.started_at,
	u.finished_at,
	u.process_after,
	u.num_resets,
	u.num_failures,
	u.repository_id,
	u.repository_name,
	u.indexer,
	u.indexer_version,
	u.associated_index_id
FROM lsif_dumps_with_repository_name u
WHERE u.id IN (%s) AND %s
`

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

// GetRepositoriesMaxStaleAge returns the longest duration that a repository has been (currently) stale for. This method considers
// only repositories that would be returned by DirtyRepositories. This method returns a duration of zero if there
// are no stale repositories.
func (s *store) GetRepositoriesMaxStaleAge(ctx context.Context) (_ time.Duration, err error) {
	ctx, _, endObservation := s.operations.getRepositoriesMaxStaleAge.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	ageSeconds, ok, err := basestore.ScanFirstInt(s.db.Query(ctx, sqlf.Sprintf(maxStaleAgeQuery)))
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}

	return time.Duration(ageSeconds) * time.Second, nil
}

const maxStaleAgeQuery = `
SELECT EXTRACT(EPOCH FROM NOW() - ldr.set_dirty_at)::integer AS age
  FROM lsif_dirty_repositories ldr
    INNER JOIN repo ON repo.id = ldr.repository_id
  WHERE ldr.dirty_token > ldr.update_token
    AND repo.deleted_at IS NULL
    AND repo.blocked IS NULL
  ORDER BY age DESC
  LIMIT 1
`

// CommitGraphMetadata returns whether or not the commit graph for the given repository is stale, along with the date of
// the most recent commit graph refresh for the given repository.
func (s *store) GetCommitGraphMetadata(ctx context.Context, repositoryID int) (stale bool, updatedAt *time.Time, err error) {
	ctx, _, endObservation := s.operations.getCommitGraphMetadata.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", repositoryID),
	}})
	defer endObservation(1, observation.Args{})

	updateToken, dirtyToken, updatedAt, exists, err := scanCommitGraphMetadata(s.db.Query(ctx, sqlf.Sprintf(commitGraphQuery, repositoryID)))
	if err != nil {
		return false, nil, err
	}
	if !exists {
		return false, nil, nil
	}

	return updateToken != dirtyToken, updatedAt, err
}

const commitGraphQuery = `
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

//
//

type sanitizedCommitInput struct {
	nearestUploadsRowValues       <-chan []any
	nearestUploadsLinksRowValues  <-chan []any
	uploadsVisibleAtTipRowValues  <-chan []any
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
	refDescriptions map[string][]gitdomain.RefDescription,
	maxAgeForNonStaleBranches time.Duration,
	maxAgeForNonStaleTags time.Duration,
) *sanitizedCommitInput {
	maxAges := map[gitdomain.RefType]time.Duration{
		gitdomain.RefTypeBranch: maxAgeForNonStaleBranches,
		gitdomain.RefTypeTag:    maxAgeForNonStaleTags,
	}

	nearestUploadsRowValues := make(chan []any)
	nearestUploadsLinksRowValues := make(chan []any)
	uploadsVisibleAtTipRowValues := make(chan []any)

	sanitized := &sanitizedCommitInput{
		nearestUploadsRowValues:      nearestUploadsRowValues,
		nearestUploadsLinksRowValues: nearestUploadsLinksRowValues,
		uploadsVisibleAtTipRowValues: uploadsVisibleAtTipRowValues,
	}

	go func() {
		defer close(nearestUploadsRowValues)
		defer close(nearestUploadsLinksRowValues)
		defer close(uploadsVisibleAtTipRowValues)

		listSerializer := newUploadMetaListSerializer()

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

		for commit, refDescriptions := range refDescriptions {
			isDefaultBranch := false
			names := make([]string, 0, len(refDescriptions))

			for _, refDescription := range refDescriptions {
				if refDescription.IsDefaultBranch {
					isDefaultBranch = true
				} else {
					maxAge, ok := maxAges[refDescription.Type]
					if !ok || refDescription.CreatedDate == nil || time.Since(*refDescription.CreatedDate) > maxAge {
						continue
					}
				}

				names = append(names, refDescription.Name)
			}
			sort.Strings(names)

			if len(names) == 0 {
				continue
			}

			for _, uploadMeta := range graph.UploadsVisibleAtCommit(commit) {
				if !countingWrite(
					ctx,
					uploadsVisibleAtTipRowValues,
					&sanitized.numUploadsVisibleAtTipRecords,
					// row values
					uploadMeta.UploadID,
					strings.Join(names, ","),
					isDefaultBranch,
				) {
					return
				}
			}
		}
	}()

	return sanitized
}

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
func (s *store) writeVisibleUploads(ctx context.Context, sanitizedInput *sanitizedCommitInput, tx *basestore.Store) (err error) {
	ctx, trace, endObservation := s.operations.writeVisibleUploads.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	defer func() {
		trace.AddEvent(
			"TODO Domain Owner",
			// Only read these after the associated channels are exhausted
			attribute.Int("numNearestUploadsRecords", int(sanitizedInput.numNearestUploadsRecords)),
			attribute.Int("numNearestUploadsLinksRecords", int(sanitizedInput.numNearestUploadsLinksRecords)),
			attribute.Int("numUploadsVisibleAtTipRecords", int(sanitizedInput.numUploadsVisibleAtTipRecords)),
		)
	}()

	if err := s.createTemporaryNearestUploadsTables(ctx, tx); err != nil {
		return err
	}

	return withTriplyNestedBatchInserters(
		ctx,
		tx.Handle(),
		batch.MaxNumPostgresParameters,
		"t_lsif_nearest_uploads", []string{"commit_bytea", "uploads"},
		"t_lsif_nearest_uploads_links", []string{"commit_bytea", "ancestor_commit_bytea", "distance"},
		"t_lsif_uploads_visible_at_tip", []string{"upload_id", "branch_or_tag_name", "is_default_branch"},
		func(nearestUploadsInserter, nearestUploadsLinksInserter, uploadsVisibleAtTipInserter *batch.Inserter) error {
			return populateInsertersFromChannels(
				ctx,
				nearestUploadsInserter, sanitizedInput.nearestUploadsRowValues,
				nearestUploadsLinksInserter, sanitizedInput.nearestUploadsLinksRowValues,
				uploadsVisibleAtTipInserter, sanitizedInput.uploadsVisibleAtTipRowValues,
			)
		},
	)
}

func withTriplyNestedBatchInserters(
	ctx context.Context,
	db dbutil.DB,
	maxNumParameters int,
	tableName1 string, columnNames1 []string,
	tableName2 string, columnNames2 []string,
	tableName3 string, columnNames3 []string,
	f func(inserter1, inserter2, inserter3 *batch.Inserter) error,
) error {
	return batch.WithInserter(ctx, db, tableName1, maxNumParameters, columnNames1, func(inserter1 *batch.Inserter) error {
		return batch.WithInserter(ctx, db, tableName2, maxNumParameters, columnNames2, func(inserter2 *batch.Inserter) error {
			return batch.WithInserter(ctx, db, tableName3, maxNumParameters, columnNames3, func(inserter3 *batch.Inserter) error {
				return f(inserter1, inserter2, inserter3)
			})
		})
	})
}

func populateInsertersFromChannels(
	ctx context.Context,
	inserter1 *batch.Inserter, values1 <-chan []any,
	inserter2 *batch.Inserter, values2 <-chan []any,
	inserter3 *batch.Inserter, values3 <-chan []any,
) error {
	for values1 != nil || values2 != nil || values3 != nil {
		select {
		case rowValues, ok := <-values1:
			if ok {
				if err := inserter1.Insert(ctx, rowValues...); err != nil {
					return err
				}
			} else {
				// The loop continues until all three channels are nil. Setting this channel to
				// nil now marks it not ready for communication, effectively blocking on the next
				// loop iteration.
				values1 = nil
			}

		case rowValues, ok := <-values2:
			if ok {
				if err := inserter2.Insert(ctx, rowValues...); err != nil {
					return err
				}
			} else {
				values2 = nil
			}

		case rowValues, ok := <-values3:
			if ok {
				if err := inserter3.Insert(ctx, rowValues...); err != nil {
					return err
				}
			} else {
				values3 = nil
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// persistNearestUploads modifies the lsif_nearest_uploads table so that it has same data
// as t_lsif_nearest_uploads for the given repository.
func (s *store) persistNearestUploads(ctx context.Context, repositoryID int, tx *basestore.Store) (err error) {
	ctx, trace, endObservation := s.operations.persistNearestUploads.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	rowsInserted, rowsUpdated, rowsDeleted, err := s.bulkTransfer(
		ctx,
		sqlf.Sprintf(nearestUploadsInsertQuery, repositoryID, repositoryID),
		sqlf.Sprintf(nearestUploadsUpdateQuery, repositoryID),
		sqlf.Sprintf(nearestUploadsDeleteQuery, repositoryID),
		tx,
	)
	if err != nil {
		return err
	}
	trace.AddEvent("TODO Domain Owner",
		attribute.Int("lsif_nearest_uploads.ins", rowsInserted),
		attribute.Int("lsif_nearest_uploads.upd", rowsUpdated),
		attribute.Int("lsif_nearest_uploads.del", rowsDeleted))

	return nil
}

const nearestUploadsInsertQuery = `
INSERT INTO lsif_nearest_uploads
SELECT %s, source.commit_bytea, source.uploads
FROM t_lsif_nearest_uploads source
WHERE source.commit_bytea NOT IN (SELECT nu.commit_bytea FROM lsif_nearest_uploads nu WHERE nu.repository_id = %s)
`

const nearestUploadsUpdateQuery = `
UPDATE lsif_nearest_uploads nu
SET uploads = source.uploads
FROM t_lsif_nearest_uploads source
WHERE
	nu.repository_id = %s AND
	nu.commit_bytea = source.commit_bytea AND
	nu.uploads != source.uploads
`

const nearestUploadsDeleteQuery = `
DELETE FROM lsif_nearest_uploads nu
WHERE
	nu.repository_id = %s AND
	nu.commit_bytea NOT IN (SELECT source.commit_bytea FROM t_lsif_nearest_uploads source)
`

// persistNearestUploadsLinks modifies the lsif_nearest_uploads_links table so that it has same
// data as t_lsif_nearest_uploads_links for the given repository.
func (s *store) persistNearestUploadsLinks(ctx context.Context, repositoryID int, tx *basestore.Store) (err error) {
	ctx, trace, endObservation := s.operations.persistNearestUploadsLinks.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	rowsInserted, rowsUpdated, rowsDeleted, err := s.bulkTransfer(
		ctx,
		sqlf.Sprintf(nearestUploadsLinksInsertQuery, repositoryID, repositoryID),
		sqlf.Sprintf(nearestUploadsLinksUpdateQuery, repositoryID),
		sqlf.Sprintf(nearestUploadsLinksDeleteQuery, repositoryID),
		tx,
	)
	if err != nil {
		return err
	}
	trace.AddEvent("TODO Domain Owner",
		attribute.Int("lsif_nearest_uploads_links.ins", rowsInserted),
		attribute.Int("lsif_nearest_uploads_links.upd", rowsUpdated),
		attribute.Int("lsif_nearest_uploads_links.del", rowsDeleted))

	return nil
}

const nearestUploadsLinksInsertQuery = `
INSERT INTO lsif_nearest_uploads_links
SELECT %s, source.commit_bytea, source.ancestor_commit_bytea, source.distance
FROM t_lsif_nearest_uploads_links source
WHERE source.commit_bytea NOT IN (SELECT nul.commit_bytea FROM lsif_nearest_uploads_links nul WHERE nul.repository_id = %s)
`

const nearestUploadsLinksUpdateQuery = `
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
DELETE FROM lsif_nearest_uploads_links nul
WHERE
	nul.repository_id = %s AND
	nul.commit_bytea NOT IN (SELECT source.commit_bytea FROM t_lsif_nearest_uploads_links source)
`

// persistUploadsVisibleAtTip modifies the lsif_uploads_visible_at_tip table so that it has same
// data as t_lsif_uploads_visible_at_tip for the given repository.
func (s *store) persistUploadsVisibleAtTip(ctx context.Context, repositoryID int, tx *basestore.Store) (err error) {
	ctx, trace, endObservation := s.operations.persistUploadsVisibleAtTip.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	insertQuery := sqlf.Sprintf(uploadsVisibleAtTipInsertQuery, repositoryID, repositoryID)
	deleteQuery := sqlf.Sprintf(uploadsVisibleAtTipDeleteQuery, repositoryID)

	rowsInserted, rowsUpdated, rowsDeleted, err := s.bulkTransfer(ctx, insertQuery, nil, deleteQuery, tx)
	if err != nil {
		return err
	}
	trace.AddEvent("TODO Domain Owner",
		attribute.Int("lsif_uploads_visible_at_tip.ins", rowsInserted),
		attribute.Int("lsif_uploads_visible_at_tip.upd", rowsUpdated),
		attribute.Int("lsif_uploads_visible_at_tip.del", rowsDeleted))

	return nil
}

const uploadsVisibleAtTipInsertQuery = `
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
func (s *store) bulkTransfer(ctx context.Context, insertQuery, updateQuery, deleteQuery *sqlf.Query, tx *basestore.Store) (rowsInserted int, rowsUpdated int, rowsDeleted int, err error) {
	prepareQuery := func(query *sqlf.Query) *sqlf.Query {
		if query == nil {
			return sqlf.Sprintf("SELECT 0")
		}

		return sqlf.Sprintf("%s RETURNING 1", query)
	}

	rows, err := tx.Query(ctx, sqlf.Sprintf(bulkTransferQuery, prepareQuery(insertQuery), prepareQuery(updateQuery), prepareQuery(deleteQuery)))
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
WITH
	ins AS (%s),
	upd AS (%s),
	del AS (%s)
SELECT
	(SELECT COUNT(*) FROM ins) AS num_ins,
	(SELECT COUNT(*) FROM upd) AS num_upd,
	(SELECT COUNT(*) FROM del) AS num_del
`

func (s *store) createTemporaryNearestUploadsTables(ctx context.Context, tx *basestore.Store) error {
	temporaryTableQueries := []string{
		temporaryNearestUploadsTableQuery,
		temporaryNearestUploadsLinksTableQuery,
		temporaryUploadsVisibleAtTipTableQuery,
	}

	for _, temporaryTableQuery := range temporaryTableQueries {
		if err := tx.Exec(ctx, sqlf.Sprintf(temporaryTableQuery)); err != nil {
			return err
		}
	}

	return nil
}

const temporaryNearestUploadsTableQuery = `
CREATE TEMPORARY TABLE t_lsif_nearest_uploads (
	commit_bytea bytea NOT NULL,
	uploads      jsonb NOT NULL
) ON COMMIT DROP
`

const temporaryNearestUploadsLinksTableQuery = `
CREATE TEMPORARY TABLE t_lsif_nearest_uploads_links (
	commit_bytea          bytea NOT NULL,
	ancestor_commit_bytea bytea NOT NULL,
	distance              integer NOT NULL
) ON COMMIT DROP
`

const temporaryUploadsVisibleAtTipTableQuery = `
CREATE TEMPORARY TABLE t_lsif_uploads_visible_at_tip (
	upload_id integer NOT NULL,
	branch_or_tag_name text NOT NULL,
	is_default_branch boolean NOT NULL
) ON COMMIT DROP
`

// countingWrite writes the given slice of interfaces to the given channel. This function returns true
// if the write succeeded and false if the context was canceled. On success, the counter's underlying
// value will be incremented (non-atomically).
func countingWrite(ctx context.Context, ch chan<- []any, counter *uint32, values ...any) bool {
	select {
	case ch <- values:
		*counter++
		return true

	case <-ctx.Done():
		return false
	}
}

//
//

type uploadMetaListSerializer struct {
	buf     bytes.Buffer
	scratch []byte
}

func newUploadMetaListSerializer() *uploadMetaListSerializer {
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

//
//

func makeFindClosestProcessUploadsConditions(path string, rootMustEnclosePath bool, indexer string) (conds []*sqlf.Query) {
	if rootMustEnclosePath {
		// Ensure that the root is a prefix of the path
		conds = append(conds, sqlf.Sprintf(`%s LIKE (u.root || '%%%%')`, path))
	} else {
		// Ensure that the root is a prefix of the path or vice versa
		conds = append(conds, sqlf.Sprintf(`(%s LIKE (u.root || '%%%%') OR u.root LIKE (%s || '%%%%'))`, path, path))
	}
	if indexer != "" {
		conds = append(conds, sqlf.Sprintf("indexer = %s", indexer))
	}

	return conds
}
