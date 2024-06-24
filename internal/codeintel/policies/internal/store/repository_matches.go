package store

import (
	"context"

	"github.com/keegancsmith/sqlf"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/policies/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func (s *store) GetRepoIDsByGlobPatterns(ctx context.Context, patterns []string, limit, offset int) (_ []int, _ int, err error) {
	ctx, _, endObservation := s.operations.getRepoIDsByGlobPatterns.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numPatterns", len(patterns)),
		attribute.Int("limit", limit),
		attribute.Int("offset", offset),
	}})
	defer endObservation(1, observation.Args{})

	if len(patterns) == 0 {
		return nil, 0, nil
	}
	cond := makePatternCondition(patterns, false)

	var a []int
	var b int
	err = s.db.WithTransact(ctx, func(tx *basestore.Store) error {
		authzConds, err := database.AuthzQueryConds(ctx, database.NewDBWith(s.logger, tx))
		if err != nil {
			return err
		}

		// TODO - standardize counting techniques
		totalCount, _, err := basestore.ScanFirstInt(tx.Query(ctx, sqlf.Sprintf(repoIDsByGlobPatternsCountQuery, cond, authzConds)))
		if err != nil {
			return err
		}

		ids, err := basestore.ScanInts(tx.Query(ctx, sqlf.Sprintf(repoIDsByGlobPatternsQuery, cond, authzConds, limit, offset)))
		if err != nil {
			return err
		}

		a = ids
		b = totalCount
		return nil
	})
	return a, b, err
}

const repoIDsByGlobPatternsCountQuery = `
SELECT COUNT(*)
FROM repo
WHERE
	(%s) AND
	deleted_at IS NULL AND
	blocked IS NULL AND
	(%s)
`

const repoIDsByGlobPatternsQuery = `
SELECT id
FROM repo
WHERE
	(%s) AND
	deleted_at IS NULL AND
	blocked IS NULL AND
	(%s)
ORDER BY stars DESC NULLS LAST, id
LIMIT %s
OFFSET %s
`

func (s *store) UpdateReposMatchingPatterns(ctx context.Context, patterns []string, policyID int, repositoryMatchLimit *int) (err error) {
	ctx, _, endObservation := s.operations.updateReposMatchingPatterns.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numPatterns", len(patterns)),
		attribute.StringSlice("pattern", patterns),
		attribute.Int("policyID", policyID),
	}})
	defer endObservation(1, observation.Args{})

	// We'll get a SQL syntax error if we try to join an empty disjunction list, so we
	// put this sentinel value here. Note that we choose FALSE over TRUE because we want
	// the absence of patterns to match NO repositories, not ALL repositories.
	cond := makePatternCondition(patterns, false)
	limitExpression := optionalLimit(repositoryMatchLimit)

	return s.db.Exec(ctx, sqlf.Sprintf(
		updateReposMatchingPatternsQuery,
		cond,
		limitExpression,
		policyID,
		policyID,
		policyID,
	))
}

const updateReposMatchingPatternsQuery = `
WITH
matching_repositories AS (
	SELECT id AS repo_id
	FROM repo
	WHERE
		(%s) AND
		deleted_at IS NULL AND
		blocked IS NULL
	ORDER BY stars DESC NULLS LAST, id
	%s
),
inserted AS (
	-- Insert records that match the policy but don't yet exist
	INSERT INTO lsif_configuration_policies_repository_pattern_lookup(policy_id, repo_id)
	SELECT %s, r.repo_id
	FROM (
		SELECT r.repo_id
		FROM matching_repositories r
		WHERE r.repo_id NOT IN (
			SELECT repo_id
			FROM lsif_configuration_policies_repository_pattern_lookup
			WHERE policy_id = %s
		)
	) r
	ORDER BY r.repo_id
	RETURNING 1
),
locked_outdated_matching_repository_records AS (
	SELECT policy_id, repo_id
	FROM lsif_configuration_policies_repository_pattern_lookup
	WHERE
		policy_id = %s AND
		repo_id NOT IN (SELECT repo_id FROM matching_repositories)
	ORDER BY policy_id, repo_id FOR UPDATE
),
deleted AS (
	-- Delete records that no longer match the policy
	DELETE FROM lsif_configuration_policies_repository_pattern_lookup
	WHERE (policy_id, repo_id) IN (
		SELECT policy_id, repo_id
		FROM locked_outdated_matching_repository_records
	)
	RETURNING 1
)
SELECT
	(SELECT COUNT(*) FROM inserted) AS num_inserted,
	(SELECT COUNT(*) FROM deleted) AS num_deleted
`

func (s *store) SelectPoliciesForRepositoryMembershipUpdate(ctx context.Context, batchSize int) (_ []shared.ConfigurationPolicy, err error) {
	ctx, _, endObservation := s.operations.selectPoliciesForRepositoryMembershipUpdate.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("batchSize", batchSize),
	}})
	defer endObservation(1, observation.Args{})

	return scanConfigurationPolicies(s.db.Query(ctx, sqlf.Sprintf(
		selectPoliciesForRepositoryMembershipUpdate,
		batchSize,
		timeutil.Now(),
	)))
}

const selectPoliciesForRepositoryMembershipUpdate = `
WITH
candidate_policies AS (
	SELECT p.id
	FROM lsif_configuration_policies p
	ORDER BY p.last_resolved_at NULLS FIRST, p.id
	LIMIT %d
),
locked_policies AS (
	SELECT c.id
	FROM candidate_policies c
	ORDER BY c.id FOR UPDATE
)
UPDATE lsif_configuration_policies
SET last_resolved_at = %s
WHERE id IN (SELECT id FROM locked_policies)
RETURNING
	id,
	repository_id,
	repository_patterns,
	name,
	type,
	pattern,
	protected,
	retention_enabled,
	retention_duration_hours,
	retain_intermediate_commits,
	indexing_enabled,
	index_commit_max_age_hours,
	index_intermediate_commits
`
