package store

import (
	"context"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/policies/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// GetConfigurationPolicies retrieves the set of configuration policies matching the the given options.
// If a repository identifier is supplied (is non-zero), then only the configuration policies that apply
// to repository are returned. If repository is not supplied, then all policies may be returned.
func (s *store) GetConfigurationPolicies(ctx context.Context, opts shared.GetConfigurationPoliciesOptions) (_ []shared.ConfigurationPolicy, totalCount int, err error) {
	attrs := []attribute.KeyValue{
		attribute.Int("repositoryID", opts.RepositoryID),
		attribute.String("term", opts.Term),
		attribute.Int("limit", opts.Limit),
		attribute.Int("offset", opts.Offset),
	}
	if opts.ForDataRetention != nil {
		attrs = append(attrs, attribute.Bool("forDataRetention", *opts.ForDataRetention))
	}
	if opts.ForPreciseIndexing != nil {
		attrs = append(attrs, attribute.Bool("forPreciseIndexing", *opts.ForPreciseIndexing))
	}
	if opts.ForSyntacticIndexing != nil {
		attrs = append(attrs, attribute.Bool("forSyntacticIndexing", *opts.ForSyntacticIndexing))
	}

	ctx, trace, endObservation := s.operations.getConfigurationPolicies.With(ctx, &err, observation.Args{Attrs: attrs})
	defer endObservation(1, observation.Args{})

	makeConfigurationPolicySearchCondition := func(term string) *sqlf.Query {
		searchableColumns := []string{
			"p.name",
		}

		var termConds []*sqlf.Query
		for _, column := range searchableColumns {
			termConds = append(termConds, sqlf.Sprintf(column+" ILIKE %s", "%"+term+"%"))
		}

		return sqlf.Sprintf("(%s)", sqlf.Join(termConds, " OR "))
	}
	conds := make([]*sqlf.Query, 0, 5)
	if opts.RepositoryID != 0 {
		conds = append(conds, sqlf.Sprintf(`(
			(p.repository_id IS NULL AND p.repository_patterns IS NULL) OR
			p.repository_id = %s OR
			EXISTS (
				SELECT 1
				FROM lsif_configuration_policies_repository_pattern_lookup l
				WHERE l.policy_id = p.id AND l.repo_id = %s
			)
		)`, opts.RepositoryID, opts.RepositoryID))
	}
	if opts.Term != "" {
		conds = append(conds, makeConfigurationPolicySearchCondition(opts.Term))
	}
	if opts.Protected != nil {
		if *opts.Protected {
			conds = append(conds, sqlf.Sprintf("p.protected"))
		} else {
			conds = append(conds, sqlf.Sprintf("NOT p.protected"))
		}
	}
	if opts.ForDataRetention != nil {
		if *opts.ForDataRetention {
			conds = append(conds, sqlf.Sprintf("p.retention_enabled"))
		} else {
			conds = append(conds, sqlf.Sprintf("NOT p.retention_enabled"))
		}
	}
	if opts.ForPreciseIndexing != nil {
		if *opts.ForPreciseIndexing {
			conds = append(conds, sqlf.Sprintf("p.indexing_enabled"))
		} else {
			conds = append(conds, sqlf.Sprintf("NOT p.indexing_enabled"))
		}
	}
	if opts.ForSyntacticIndexing != nil {
		if *opts.ForSyntacticIndexing {
			conds = append(conds, sqlf.Sprintf("p.syntactic_indexing_enabled"))
		} else {
			conds = append(conds, sqlf.Sprintf("NOT p.syntactic_indexing_enabled"))
		}
	}
	if len(conds) == 0 {
		conds = append(conds, sqlf.Sprintf("TRUE"))
	}

	var policiesBatch []shared.ConfigurationPolicy
	var totalCountResult int

	err = s.db.WithTransact(ctx, func(tx *basestore.Store) error {
		// TODO - standardize counting techniques
		totalCountQuery := sqlf.Sprintf(
			getConfigurationPoliciesCountQuery,
			sqlf.Join(conds, "AND"),
		)

		totalCount, _, err = basestore.ScanFirstInt(tx.Query(ctx, totalCountQuery))
		if err != nil {
			return err
		}
		trace.AddEvent("TODO Domain Owner", attribute.Int("totalCount", totalCount))

		finalQuery := sqlf.Sprintf(
			getConfigurationPoliciesLimitedQuery,
			sqlf.Join(conds, "AND"),
			opts.Limit,
			opts.Offset,
		)

		configurationPolicies, err := scanConfigurationPolicies(tx.Query(ctx, finalQuery))
		if err != nil {
			return err
		}

		trace.AddEvent("TODO Domain Owner", attribute.Int("numConfigurationPolicies", len(configurationPolicies)))

		policiesBatch = configurationPolicies
		totalCountResult = totalCount
		return nil
	})

	return policiesBatch, totalCountResult, err
}

const getConfigurationPoliciesCountQuery = `
SELECT COUNT(*)
FROM lsif_configuration_policies p
LEFT JOIN repo ON repo.id = p.repository_id
WHERE %s
`

const getConfigurationPoliciesLimitedQuery = `
SELECT
	p.id,
	p.repository_id,
	p.repository_patterns,
	p.name,
	p.type,
	p.pattern,
	p.protected,
	p.retention_enabled,
	p.retention_duration_hours,
	p.retain_intermediate_commits,
	p.indexing_enabled,
	p.index_commit_max_age_hours,
	p.index_intermediate_commits
FROM lsif_configuration_policies p
LEFT JOIN repo ON repo.id = p.repository_id
WHERE %s
ORDER BY p.name
LIMIT %s
OFFSET %s
`

func (s *store) GetConfigurationPolicyByID(ctx context.Context, id int) (_ shared.ConfigurationPolicy, _ bool, err error) {
	ctx, _, endObservation := s.operations.getConfigurationPolicyByID.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("id", id),
	}})
	defer endObservation(1, observation.Args{})

	authzConds, err := database.AuthzQueryConds(ctx, database.NewDBWith(s.logger, s.db))
	if err != nil {
		return shared.ConfigurationPolicy{}, false, err
	}

	return scanFirstConfigurationPolicy(s.db.Query(ctx, sqlf.Sprintf(
		getConfigurationPolicyByIDQuery,
		id,
		authzConds,
	)))
}

const getConfigurationPolicyByIDQuery = `
SELECT
	p.id,
	p.repository_id,
	p.repository_patterns,
	p.name,
	p.type,
	p.pattern,
	p.protected,
	p.retention_enabled,
	p.retention_duration_hours,
	p.retain_intermediate_commits,
	p.indexing_enabled,
	p.index_commit_max_age_hours,
	p.index_intermediate_commits
FROM lsif_configuration_policies p
LEFT JOIN repo ON repo.id = p.repository_id
WHERE
	p.id = %s AND
	-- Global policies are visible to anyone
	-- Repository-specific policies must check repository permissions
	(p.repository_id IS NULL OR (%s))
`

func (s *store) CreateConfigurationPolicy(ctx context.Context, configurationPolicy shared.ConfigurationPolicy) (_ shared.ConfigurationPolicy, err error) {
	ctx, _, endObservation := s.operations.createConfigurationPolicy.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	retentionDurationHours := optionalNumHours(configurationPolicy.RetentionDuration)
	indexingCommitMaxAgeHours := optionalNumHours(configurationPolicy.IndexCommitMaxAge)
	repositoryPatterns := optionalArray(configurationPolicy.RepositoryPatterns)

	hydratedConfigurationPolicy, _, err := scanFirstConfigurationPolicy(s.db.Query(ctx, sqlf.Sprintf(
		createConfigurationPolicyQuery,
		configurationPolicy.RepositoryID,
		repositoryPatterns,
		configurationPolicy.Name,
		configurationPolicy.Type,
		configurationPolicy.Pattern,
		configurationPolicy.RetentionEnabled,
		retentionDurationHours,
		configurationPolicy.RetainIntermediateCommits,
		configurationPolicy.IndexingEnabled,
		indexingCommitMaxAgeHours,
		configurationPolicy.IndexIntermediateCommits,
	)))
	if err != nil {
		return shared.ConfigurationPolicy{}, err
	}

	return hydratedConfigurationPolicy, nil
}

const createConfigurationPolicyQuery = `
INSERT INTO lsif_configuration_policies (
	repository_id,
	repository_patterns,
	name,
	type,
	pattern,
	retention_enabled,
	retention_duration_hours,
	retain_intermediate_commits,
	indexing_enabled,
	index_commit_max_age_hours,
	index_intermediate_commits
) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
RETURNING
	id,
	repository_id,
	repository_patterns,
	name,
	type,
	pattern,
	false as protected,
	retention_enabled,
	retention_duration_hours,
	retain_intermediate_commits,
	indexing_enabled,
	index_commit_max_age_hours,
	index_intermediate_commits
`

var (
	errUnknownConfigurationPolicy       = errors.New("unknown configuration policy")
	errIllegalConfigurationPolicyUpdate = errors.New("protected configuration policies must keep the same names, types, patterns, and retention values (except duration)")
	errIllegalConfigurationPolicyDelete = errors.New("protected configuration policies cannot be deleted")
)

func (s *store) UpdateConfigurationPolicy(ctx context.Context, policy shared.ConfigurationPolicy) (err error) {
	ctx, _, endObservation := s.operations.updateConfigurationPolicy.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("id", policy.ID),
	}})
	defer endObservation(1, observation.Args{})

	retentionDuration := optionalNumHours(policy.RetentionDuration)
	indexCommitMaxAge := optionalNumHours(policy.IndexCommitMaxAge)
	repositoryPatterns := optionalArray(policy.RepositoryPatterns)

	return s.db.WithTransact(ctx, func(tx *basestore.Store) error {
		// First, pull current policy to see if it's protected, and if so whether or not the
		// fields that must remain stable (names, types, patterns, and retention enabled) have
		// the same current and target values.

		currentPolicy, ok, err := scanFirstConfigurationPolicy(tx.Query(ctx, sqlf.Sprintf(updateConfigurationPolicySelectQuery, policy.ID)))
		if err != nil {
			return err
		}
		if !ok {
			return errUnknownConfigurationPolicy
		}
		if currentPolicy.Protected {
			if policy.Name != currentPolicy.Name || policy.Type != currentPolicy.Type || policy.Pattern != currentPolicy.Pattern || policy.RetentionEnabled != currentPolicy.RetentionEnabled || policy.RetainIntermediateCommits != currentPolicy.RetainIntermediateCommits {
				return errIllegalConfigurationPolicyUpdate
			}
		}

		return tx.Exec(ctx, sqlf.Sprintf(updateConfigurationPolicyQuery,
			policy.Name,
			repositoryPatterns,
			policy.Type,
			policy.Pattern,
			policy.RetentionEnabled,
			retentionDuration,
			policy.RetainIntermediateCommits,
			policy.IndexingEnabled,
			indexCommitMaxAge,
			policy.IndexIntermediateCommits,
			policy.ID,
		))
	})
}

const updateConfigurationPolicySelectQuery = `
SELECT
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
FROM lsif_configuration_policies
WHERE id = %s
FOR UPDATE
`

const updateConfigurationPolicyQuery = `
UPDATE lsif_configuration_policies SET
	name = %s,
	repository_patterns = %s,
	type = %s,
	pattern = %s,
	retention_enabled = %s,
	retention_duration_hours = %s,
	retain_intermediate_commits = %s,
	indexing_enabled = %s,
	index_commit_max_age_hours = %s,
	index_intermediate_commits = %s
WHERE id = %s
`

func (s *store) DeleteConfigurationPolicyByID(ctx context.Context, id int) (err error) {
	ctx, _, endObservation := s.operations.deleteConfigurationPolicyByID.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("id", id),
	}})
	defer endObservation(1, observation.Args{})

	return s.db.WithTransact(ctx, func(tx *basestore.Store) error {
		protected, ok, err := basestore.ScanFirstBool(s.db.Query(ctx, sqlf.Sprintf(deleteConfigurationPolicyByIDQuery, id)))
		if err != nil {
			return err
		}
		if !ok {
			return errUnknownConfigurationPolicy
		}
		if protected {
			return errIllegalConfigurationPolicyDelete
		}
		_, err = s.db.Query(ctx, sqlf.Sprintf(deleteConfigurationPoliciesRepositoryPatternLookup, id))
		if err != nil {
			return err
		}
		return nil
	})
}

const deleteConfigurationPoliciesRepositoryPatternLookup = `
	DELETE FROM lsif_configuration_policies_repository_pattern_lookup WHERE policy_id = %s
`

const deleteConfigurationPolicyByIDQuery = `
WITH
candidate AS (
	SELECT id, protected
	FROM lsif_configuration_policies
	WHERE id = %s
	ORDER BY id FOR UPDATE
),
deleted AS (
	DELETE FROM lsif_configuration_policies WHERE id IN (SELECT id FROM candidate WHERE NOT protected)
)
SELECT protected FROM candidate
`

//
//

func scanConfigurationPolicy(s dbutil.Scanner) (configurationPolicy shared.ConfigurationPolicy, err error) {
	var retentionDurationHours, indexCommitMaxAgeHours *int
	var repositoryPatterns []string

	if err := s.Scan(
		&configurationPolicy.ID,
		&configurationPolicy.RepositoryID,
		pq.Array(&repositoryPatterns),
		&configurationPolicy.Name,
		&configurationPolicy.Type,
		&configurationPolicy.Pattern,
		&configurationPolicy.Protected,
		&configurationPolicy.RetentionEnabled,
		&retentionDurationHours,
		&configurationPolicy.RetainIntermediateCommits,
		&configurationPolicy.IndexingEnabled,
		&indexCommitMaxAgeHours,
		&configurationPolicy.IndexIntermediateCommits,
	); err != nil {
		return configurationPolicy, err
	}

	configurationPolicy.RetentionDuration = optionalDuration(retentionDurationHours)
	configurationPolicy.IndexCommitMaxAge = optionalDuration(indexCommitMaxAgeHours)
	configurationPolicy.RepositoryPatterns = optionalSlice(repositoryPatterns)

	return configurationPolicy, nil
}

var (
	scanConfigurationPolicies    = basestore.NewSliceScanner(scanConfigurationPolicy)
	scanFirstConfigurationPolicy = basestore.NewFirstScanner(scanConfigurationPolicy)
)
