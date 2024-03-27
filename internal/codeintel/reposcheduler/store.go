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
	GetRepositoriesForIndexScan(ctx context.Context, batchOptions RepositoryBatchOptions, now time.Time) ([]RepositoryToIndex, error)
}

type store struct {
	db         *basestore.Store
	logger     logger.Logger
	operations *operations
	dbLayout   dbLayout
}

var _ RepositorySchedulingStore = &store{}

type storeType int8

const (
	preciseStore   storeType = 1
	syntacticStore storeType = 2
)

func NewPreciseStore(observationCtx *observation.Context, db database.DB) RepositorySchedulingStore {
	return &store{
		db:         basestore.NewWithHandle(db.Handle()),
		logger:     observationCtx.Logger.Scoped("reposcheduler.syntactic_store"),
		operations: newOperations(observationCtx, preciseStore),
		dbLayout: dbLayout{
			policyEnablementFieldName: "indexing_enabled",
			lastScanTableName:         "lsif_last_index_scan",
		},
	}
}

func NewSyntacticStore(observationCtx *observation.Context, db database.DB) RepositorySchedulingStore {
	return &store{
		db:         basestore.NewWithHandle(db.Handle()),
		logger:     observationCtx.Logger.Scoped("reposcheduler.precise_store"),
		operations: newOperations(observationCtx, syntacticStore),
		dbLayout: dbLayout{
			policyEnablementFieldName: "syntactic_indexing_enabled",
			lastScanTableName:         "syntactic_scip_last_index_scan",
		},
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
	}, nil
}

func (s *store) Done(err error) error {
	return s.db.Done(err)
}

type dbLayout struct {
	policyEnablementFieldName string
	lastScanTableName         string
}

// GetRepositoriesForIndexScan returns a set of repository identifiers that should be considered
// for indexing jobs. Repositories that were returned previously from this call within the given
// process delay are not returned.
func (store *store) GetRepositoriesForIndexScan(
	ctx context.Context,
	batchOptions RepositoryBatchOptions,
	now time.Time,
) (_ []RepositoryToIndex, err error) {
	var globalPolicyRepositoryMatchLimitValue int
	if batchOptions.GlobalPolicyRepositoriesMatchLimit != nil {
		globalPolicyRepositoryMatchLimitValue = *batchOptions.GlobalPolicyRepositoriesMatchLimit
	}

	ctx, _, endObservation := store.operations.getRepositoriesForIndexScan.With(ctx, &err,
		observation.Args{Attrs: []attribute.KeyValue{
			attribute.Bool("allowGlobalPolicies", batchOptions.AllowGlobalPolicies),
			attribute.Int("globalPolicyRepositoryMatchLimit", globalPolicyRepositoryMatchLimitValue),
			attribute.Int("limit", batchOptions.Limit),
		}})
	defer endObservation(1, observation.Args{})

	queries := make([]*sqlf.Query, 0, 3)
	if batchOptions.AllowGlobalPolicies {
		queries = append(queries, sqlf.Sprintf(
			getRepositoriesForIndexScanGlobalRepositoriesQuery,
			optionalLimit(batchOptions.GlobalPolicyRepositoriesMatchLimit),
		))
	}
	queries = append(queries, sqlf.Sprintf(getRepositoriesForIndexScanRepositoriesWithPolicyQuery(store.dbLayout)))
	queries = append(queries, sqlf.Sprintf(getRepositoriesForIndexScanRepositoriesWithPolicyViaPatternQuery(store.dbLayout)))

	for i, query := range queries {
		queries[i] = sqlf.Sprintf("(%s)", query)
	}

	query := getRepositoriesForIndexScanQuery(store.dbLayout)

	finalQuery := sqlf.Sprintf(
		query,
		sqlf.Join(queries, " UNION ALL "),
		now,
		int(batchOptions.ProcessDelay/time.Second),
		batchOptions.Limit,
		now,
		now,
	)

	repositoryIds, err := basestore.ScanInts(store.db.Query(ctx, finalQuery))

	if err != nil {
		return nil, err
	}

	repos := make([]RepositoryToIndex, len(repositoryIds))
	for i, repoId := range repositoryIds {
		repos[i] = RepositoryToIndex{ID: repoId}
	}

	return repos, nil

}

func getRepositoriesForIndexScanQuery(layout dbLayout) string {
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
	LEFT JOIN %s lrs ON lrs.repository_id = rmp.id
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
INSERT INTO %s (repository_id, last_index_scan_at)
SELECT DISTINCT r.id, %%s::timestamp FROM repositories r
ON CONFLICT (repository_id) DO UPDATE
SET last_index_scan_at = %%s
RETURNING repository_id
`, layout.policyEnablementFieldName,
		layout.lastScanTableName,
		layout.lastScanTableName,
	)
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

func getRepositoriesForIndexScanRepositoriesWithPolicyQuery(layout dbLayout) string {

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
`, layout.policyEnablementFieldName)
}

func getRepositoriesForIndexScanRepositoriesWithPolicyViaPatternQuery(layout dbLayout) string {
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
`, layout.policyEnablementFieldName)
}

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
