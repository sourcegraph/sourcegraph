package reposcheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/keegancsmith/sqlf"
	logger "github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"go.opentelemetry.io/otel/attribute"
)

type RepoRev struct {
	ID           int
	RepositoryID int
	Rev          string
}

type RepositorySchedulingStore interface {
	WithTransaction(ctx context.Context, f func(tx RepositorySchedulingStore) error) error
	GetRepositoriesForIndexScan(ctx context.Context, batchOptions RepositoryBatchOptions, now time.Time) ([]int, error)
}

type StoreType int32

const (
	Precise   StoreType = 1
	Syntactic StoreType = 2
)

type store struct {
	db         *basestore.Store
	logger     logger.Logger
	operations *operations
	storeType  StoreType
}

var _ RepositorySchedulingStore = &store{}

func NewPreciseStore(observationCtx *observation.Context, db database.DB) RepositorySchedulingStore {
	return &store{
		db:         basestore.NewWithHandle(db.Handle()),
		logger:     logger.Scoped("reposcheduler.store"),
		operations: newOperations(observationCtx),
		storeType:  Precise,
	}
}

func NewSyntacticStore(observationCtx *observation.Context, db database.DB) RepositorySchedulingStore {
	return &store{
		db:         basestore.NewWithHandle(db.Handle()),
		logger:     logger.Scoped("reposcheduler.store"),
		operations: newOperations(observationCtx),
		storeType:  Syntactic,
	}
}

func (s *store) WithTransaction(ctx context.Context, f func(s RepositorySchedulingStore) error) error {
	return s.withTransaction(ctx, func(s *store) error { return f(s) })
}

func (s *store) withTransaction(ctx context.Context, f func(s *store) error) error {
	return basestore.InTransaction(ctx, s, f)
}

func (s *store) Transact(ctx context.Context) (*store, error) {
	tx, err := s.db.Transact(ctx)
	if err != nil {
		return nil, err
	}

	return &store{
		logger:     s.logger,
		db:         tx,
		operations: s.operations,
		storeType:  s.storeType,
	}, nil
}

func (s *store) Done(err error) error {
	return s.db.Done(err)
}

// GetRepositoriesForIndexScan returns a set of repository identifiers that should be considered
// for indexing jobs. Repositories that were returned previously from this call within the given
// process delay are not returned.
//
// If allowGlobalPolicies is false, then configuration policies that define neither a repository id
// nor a non-empty set of repository patterns wl be ignored. When true, such policies apply over all
// repositories known to the instance.
func (s *store) GetRepositoriesForIndexScan(
	ctx context.Context,
	batchOptions RepositoryBatchOptions,
	now time.Time,
) (_ []int, err error) {
	var repositoryMatchLimitValue int
	if batchOptions.RepositoryMatchLimit != nil {
		repositoryMatchLimitValue = *batchOptions.RepositoryMatchLimit
	}

	var enabledFieldName string
	var indexingType string
	if s.storeType == Precise {
		enabledFieldName = "indexing_enabled"
		indexingType = "precise"
	} else {
		enabledFieldName = "syntactic_indexing_enabled"
		indexingType = "syntactic"
	}

	ctx, _, endObservation := s.operations.getRepositoriesForIndexScan.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Bool("allowGlobalPolicies", batchOptions.AllowGlobalPolicies),
		attribute.Int("repositoryMatchLimit", repositoryMatchLimitValue),
		attribute.Int("limit", batchOptions.Limit),
	}})
	defer endObservation(1, observation.Args{})

	queries := make([]*sqlf.Query, 0, 3)
	if batchOptions.AllowGlobalPolicies {
		queries = append(queries, sqlf.Sprintf(
			getRepositoriesForIndexScanGlobalRepositoriesQuery,
			optionalLimit(batchOptions.RepositoryMatchLimit),
		))
	}
	queries = append(queries, sqlf.Sprintf(getRepositoriesForIndexScanRepositoriesWithPolicyQuery(enabledFieldName)))
	queries = append(queries, sqlf.Sprintf(getRepositoriesForIndexScanRepositoriesWithPolicyViaPatternQuery(enabledFieldName)))

	for i, query := range queries {
		queries[i] = sqlf.Sprintf("(%s)", query)
	}

	query := getRepositoriesForIndexScanQuery(enabledFieldName)

	finalQuery := sqlf.Sprintf(
		query,
		sqlf.Join(queries, " UNION ALL "),
		indexingType,
		now,
		int(batchOptions.ProcessDelay/time.Second),
		batchOptions.Limit,
		now,
		indexingType,
		now,
	)

	// fmt.Println("Final query", finalQuery)

	return basestore.ScanInts(s.db.Query(ctx, finalQuery))

}

func getRepositoriesForIndexScanQuery(enabledFieldName string) string {
	return fmt.Sprintf(`
WITH
-- This CTE will contain a single row if there is at least one global policy, and will return an empty
-- result set otherwise. If any global policy is for HEAD, the value for the column is_head_policy will
-- be true.
global_policy_descriptor AS MATERIALIZED (
	SELECT (p.type = 'GIT_COMMIT' AND p.pattern = 'HEAD') AS is_head_policy
	FROM lsif_configuration_policies p
	WHERE
		p.%s AND
		p.repository_id IS NULL AND
		p.repository_patterns IS NULL
	ORDER BY is_head_policy DESC
	LIMIT 1
),
repositories_matching_policy AS (
	%%s
),
repositories AS (
	SELECT rmp.id
	FROM repositories_matching_policy rmp
	LEFT JOIN lsif_last_index_scan lrs ON lrs.repository_id = rmp.id and lrs.indexing_type = %%s
	WHERE
		-- Records that have not been checked within the global reindex threshold are also eligible for
		-- indexing. Note that condition here is true for a record that has never been indexed.
		(%%s - lrs.last_index_scan_at > (%%s * '1 second'::interval)) IS DISTINCT FROM FALSE OR
		-- Records that have received an update since their last scan are also eligible for re-indexing.
		-- Note that last_changed is NULL unless the repository is attached to a policy for HEAD.
		(rmp.last_changed > lrs.last_index_scan_at)
	ORDER BY
		lrs.last_index_scan_at NULLS FIRST,
		rmp.id -- tie breaker
	LIMIT %%s
)
INSERT INTO lsif_last_index_scan (repository_id, last_index_scan_at, indexing_Type)
SELECT DISTINCT r.id, %%s::timestamp, %%s::indexing_type FROM repositories r
ON CONFLICT (repository_id) DO UPDATE
SET last_index_scan_at = %%s
RETURNING repository_id
`, enabledFieldName)
}

const getRepositoriesForIndexScanGlobalRepositoriesQuery = `
SELECT
	r.id,
	CASE
		-- Return non-NULL last_changed only for policies that are attached to a HEAD commit.
		-- We don't want to superfluously return the same repos because they had an update, but
		-- we only (for example) index a branch that doesn't have many active commits.
		WHEN gpd.is_head_policy THEN gr.last_changed
		ELSE NULL
	END AS last_changed
FROM repo r
JOIN gitserver_repos gr ON gr.repo_id = r.id
JOIN global_policy_descriptor gpd ON TRUE
WHERE
	r.deleted_at IS NULL AND
	r.blocked IS NULL AND
	gr.clone_status = 'cloned'
ORDER BY stars DESC NULLS LAST, id
%s
`

func getRepositoriesForIndexScanRepositoriesWithPolicyQuery(enabledFieldName string) string {

	return fmt.Sprintf(`
SELECT
	r.id,
	CASE
		-- Return non-NULL last_changed only for policies that are attached to a HEAD commit.
		-- We don't want to superfluously return the same repos because they had an update, but
		-- we only (for example) index a branch that doesn't have many active commits.
		WHEN p.type = 'GIT_COMMIT' AND p.pattern = 'HEAD' THEN gr.last_changed
		ELSE NULL
	END AS last_changed
FROM repo r
JOIN gitserver_repos gr ON gr.repo_id = r.id
JOIN lsif_configuration_policies p ON p.repository_id = r.id
WHERE
	r.deleted_at IS NULL AND
	r.blocked IS NULL AND
	p.%s AND
	gr.clone_status = 'cloned'
`, enabledFieldName)
}

func getRepositoriesForIndexScanRepositoriesWithPolicyViaPatternQuery(enabledFieldName string) string {
	return fmt.Sprintf(`
SELECT
	r.id,
	CASE
		-- Return non-NULL last_changed only for policies that are attached to a HEAD commit.
		-- We don't want to superfluously return the same repos because they had an update, but
		-- we only (for example) index a branch that doesn't have many active commits.
		WHEN p.type = 'GIT_COMMIT' AND p.pattern = 'HEAD' THEN gr.last_changed
		ELSE NULL
	END AS last_changed
FROM repo r
JOIN gitserver_repos gr ON gr.repo_id = r.id
JOIN lsif_configuration_policies_repository_pattern_lookup rpl ON rpl.repo_id = r.id
JOIN lsif_configuration_policies p ON p.id = rpl.policy_id
WHERE
	r.deleted_at IS NULL AND
	r.blocked IS NULL AND
	p.%s AND
	gr.clone_status = 'cloned'
`, enabledFieldName)
}

//
//

// func (s *store) GetQueuedRepoRev(ctx context.Context, batchSize int) (_ []RepoRev, err error) {
// 	ctx, _, endObservation := s.operations.getQueuedRepoRev.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
// 		attribute.Int("batchSize", batchSize),
// 	}})
// 	defer endObservation(1, observation.Args{})

// 	return scanRepoRevs(s.db.Query(ctx, sqlf.Sprintf(getQueuedRepoRevQuery, batchSize)))
// }

// const getQueuedRepoRevQuery = `
// SELECT
// 	id,
// 	repository_id,
// 	rev
// FROM codeintel_autoindex_queue
// WHERE processed_at IS NULL
// ORDER BY queued_at ASC
// FOR UPDATE SKIP LOCKED
// LIMIT %s
// `

// func (s *store) MarkRepoRevsAsProcessed(ctx context.Context, ids []int) (err error) {
// 	ctx, _, endObservation := s.operations.markRepoRevsAsProcessed.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
// 		attribute.Int("numIDs", len(ids)),
// 	}})
// 	defer endObservation(1, observation.Args{})

// 	return s.db.Exec(ctx, sqlf.Sprintf(markRepoRevsAsProcessedQuery, pq.Array(ids)))
// }

// const markRepoRevsAsProcessedQuery = `
// UPDATE codeintel_autoindex_queue
// SET processed_at = NOW()
// WHERE id = ANY(%s)
// `

// func (s *store) QueueRepoRev(ctx context.Context, repositoryID int, rev string) (err error) {
// 	ctx, _, endObservation := s.operations.queueRepoRev.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
// 		attribute.Int("repositoryID", repositoryID),
// 		attribute.String("rev", rev),
// 	}})
// 	defer endObservation(1, observation.Args{})

// 	return s.withTransaction(ctx, func(tx *store) error {
// 		isQueued, err := tx.IsQueued(ctx, repositoryID, rev)
// 		logger.Error(err)
// 		if err != nil {
// 			return err
// 		}
// 		if isQueued {
// 			return nil
// 		}

// 		return tx.db.Exec(ctx, sqlf.Sprintf(queueRepoRevQuery, repositoryID, rev))
// 	})
// }

// const queueRepoRevQuery = `
// INSERT INTO codeintel_autoindex_queue (repository_id, rev)
// VALUES (%s, %s)
// ON CONFLICT DO NOTHING
// `

// func (s *store) IsQueued(ctx context.Context, repositoryID int, commit string) (_ bool, err error) {
// 	ctx, _, endObservation := s.operations.isQueued.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
// 		attribute.Int("repositoryID", repositoryID),
// 		attribute.String("commit", commit),
// 	}})
// 	defer endObservation(1, observation.Args{})

// 	isQueued, _, err := basestore.ScanFirstBool(s.db.Query(ctx, sqlf.Sprintf(
// 		isQueuedQuery,
// 		repositoryID, commit,
// 		repositoryID, commit,
// 	)))
// 	logger.Error(err)
// 	return isQueued, err
// }

// const isQueuedQuery = `
// -- The query has two parts, 'A' UNION 'B', where 'A' is true if there's a manual and
// -- reachable upload for a repo/commit pair. This signifies that the user has configured
// -- manual indexing on a repo and we shouldn't clobber it with autoindexing. The other
// -- query 'B' is true if there's an auto-index record already enqueued for this repo. This
// -- signifies that we've already infered jobs for this repo/commit pair so we can skip it
// -- (we should infer the same jobs).
// -- We added a way to say "you might infer different jobs" for part 'B' by adding the
// -- check on u.should_reindex. We're now adding a way to say "the indexer might result
// -- in a different output_ for part A, allowing auto-indexing to clobber records that
// -- have undergone some possibly lossy transformation (like LSIF -> SCIP conversion in-db).
// SELECT
// 	EXISTS (
// 		SELECT 1
// 		FROM lsif_uploads u
// 		WHERE
// 			repository_id = %s AND
// 			commit = %s AND
// 			state NOT IN ('deleting', 'deleted') AND
// 			associated_index_id IS NULL AND
// 			NOT u.should_reindex
// 	)
// 	OR
// 	-- We want IsQueued to return true when there exists auto-indexing job records
// 	-- and none of them are marked for reindexing. If we have one or more rows and
// 	-- ALL of them are not marked for re-indexing, we'll block additional indexing
// 	-- attempts.
// 	(
// 		SELECT COALESCE(bool_and(NOT should_reindex), false)
// 		FROM (
// 			-- For each distinct (root, indexer) pair, use the most recently queued
// 			-- index as the authoritative attempt.
// 			SELECT DISTINCT ON (root, indexer) should_reindex
// 			FROM lsif_indexes
// 			WHERE repository_id = %s AND commit = %s
// 			ORDER BY root, indexer, queued_at DESC
// 		) _
// 	)
// `

//
//

func scanRepoRev(s dbutil.Scanner) (rr RepoRev, err error) {
	err = s.Scan(&rr.ID, &rr.RepositoryID, &rr.Rev)
	return rr, err
}

var scanRepoRevs = basestore.NewSliceScanner(scanRepoRev)

func optionalLimit(limit *int) *sqlf.Query {
	if limit != nil {
		return sqlf.Sprintf("LIMIT %d", *limit)
	}

	return sqlf.Sprintf("")
}
