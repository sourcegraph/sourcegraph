package store

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/exp/slices"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies/shared"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Store provides the interface for package dependencies storage.
type Store interface {
	WithTransact(context.Context, func(Store) error) error

	ListPackageRepoRefs(ctx context.Context, opts ListDependencyReposOpts) (dependencyRepos []shared.PackageRepoReference, total int, hasMore bool, err error)
	InsertPackageRepoRefs(ctx context.Context, deps []shared.MinimalPackageRepoRef) (newDeps []shared.PackageRepoReference, newVersions []shared.PackageRepoRefVersion, err error)
	DeletePackageRepoRefsByID(ctx context.Context, ids ...int) (err error)
	DeletePackageRepoRefVersionsByID(ctx context.Context, ids ...int) (err error)

	ListPackageRepoRefFilters(ctx context.Context, opts ListPackageRepoRefFiltersOpts) ([]shared.PackageRepoFilter, bool, error)
	CreatePackageRepoFilter(ctx context.Context, input shared.MinimalPackageFilter) (filter *shared.PackageRepoFilter, err error)
	UpdatePackageRepoFilter(ctx context.Context, input shared.PackageRepoFilter) (err error)
	DeletePacakgeRepoFilter(ctx context.Context, id int) (err error)

	ShouldRefilterPackageRepoRefs(ctx context.Context) (exists bool, err error)
	UpdateAllBlockedStatuses(ctx context.Context, pkgs []shared.PackageRepoReference, startTime time.Time) (pkgsUpdated, versionsUpdated int, err error)
}

// store manages the database tables for package dependencies.
type store struct {
	db         *basestore.Store
	operations *operations
}

// New returns a new store.
func New(op *observation.Context, db database.DB) *store {
	return &store{
		db:         basestore.NewWithHandle(db.Handle()),
		operations: newOperations(op),
	}
}

func (s *store) WithTransact(ctx context.Context, f func(tx Store) error) error {
	return s.db.WithTransact(ctx, func(tx *basestore.Store) error {
		return f(&store{
			db:         tx,
			operations: s.operations,
		})
	})
}

type fuzziness int

const (
	FuzzinessExactMatch fuzziness = iota
	FuzzinessWildcard
	FuzzinessRegex
)

// ListDependencyReposOpts are options for listing dependency repositories.
type ListDependencyReposOpts struct {
	Scheme         string
	Name           reposource.PackageName
	Fuzziness      fuzziness
	After          int
	Limit          int
	IncludeBlocked bool
}

// ListDependencyRepos returns dependency repositories to be synced by gitserver.
func (s *store) ListPackageRepoRefs(ctx context.Context, opts ListDependencyReposOpts) (dependencyRepos []shared.PackageRepoReference, total int, hasMore bool, err error) {
	ctx, _, endObservation := s.operations.listPackageRepoRefs.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("scheme", opts.Scheme),
	}})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("numDependencyRepos", len(dependencyRepos)),
		}})
	}()

	query := sqlf.Sprintf(
		listDependencyReposQuery,
		sqlf.Sprintf(groupedVersionedPackageReposColumns),
		sqlf.Join([]*sqlf.Query{makeListDependencyReposConds(opts), makeOffset(opts.After)}, "AND"),
		sqlf.Sprintf("GROUP BY lr.id"),
		sqlf.Sprintf("ORDER BY lr.id ASC"),
		makeLimit(opts.Limit),
	)
	dependencyRepos, err = basestore.NewSliceScanner(scanDependencyRepoWithVersions)(s.db.Query(ctx, query))
	if err != nil {
		return nil, 0, false, errors.Wrap(err, "error listing dependency repos")
	}

	if opts.Limit != 0 && len(dependencyRepos) > opts.Limit {
		dependencyRepos = dependencyRepos[:opts.Limit]
		hasMore = true
	}

	query = sqlf.Sprintf(
		listDependencyReposQuery,
		sqlf.Sprintf("COUNT(DISTINCT(lr.id))"),
		makeListDependencyReposConds(opts),
		sqlf.Sprintf(""),
		sqlf.Sprintf(""),
		sqlf.Sprintf("LIMIT ALL"),
	)
	totalCount, _, err := basestore.ScanFirstInt(s.db.Query(ctx, query))
	if err != nil {
		return nil, 0, false, errors.Wrap(err, "error counting dependency repos")
	}

	return dependencyRepos, totalCount, hasMore, err
}

const groupedVersionedPackageReposColumns = `
	lr.id,
	lr.scheme,
	lr.name,
	lr.blocked,
	lr.last_checked_at,
	array_agg(prv.id ORDER BY prv.id) as vid,
	array_agg(prv.version ORDER BY prv.id) as version,
	array_agg(prv.blocked ORDER BY prv.id) as vers_blocked,
	array_agg(prv.last_checked_at ORDER BY prv.id) as vers_last_checked_at
`

const listDependencyReposQuery = `
SELECT %s
FROM lsif_dependency_repos lr
JOIN LATERAL (
    SELECT id, package_id, version, blocked, last_checked_at
    FROM package_repo_versions
    WHERE package_id = lr.id
    ORDER BY id
) prv
ON lr.id = prv.package_id
WHERE %s
%s -- group by
%s -- order by
%s -- limit
`

func makeListDependencyReposConds(opts ListDependencyReposOpts) *sqlf.Query {
	conds := make([]*sqlf.Query, 0, 4)

	if opts.Scheme != "" {
		conds = append(conds, sqlf.Sprintf("scheme = %s", opts.Scheme))
	}

	if opts.Name != "" {
		switch opts.Fuzziness {
		case FuzzinessExactMatch:
			conds = append(conds, sqlf.Sprintf("name = %s", opts.Name))
		case FuzzinessWildcard:
			conds = append(conds, sqlf.Sprintf("name LIKE ('%%%%' || %s || '%%%%')", opts.Name))
		case FuzzinessRegex:
			conds = append(conds, sqlf.Sprintf("name ~ %s", opts.Name))
		}
	}

	if !opts.IncludeBlocked {
		conds = append(conds, sqlf.Sprintf("lr.blocked <> true AND prv.blocked <> true"))
	}

	if len(conds) > 0 {
		return sqlf.Sprintf("%s", sqlf.Join(conds, "AND"))
	}

	return sqlf.Sprintf("TRUE")
}

func makeLimit(limit int) *sqlf.Query {
	if limit == 0 {
		return sqlf.Sprintf("LIMIT ALL")
	}
	// + 1 to check if more pages
	return sqlf.Sprintf("LIMIT %s", limit+1)
}

func makeOffset(id int) *sqlf.Query {
	if id > 0 {
		return sqlf.Sprintf("lr.id > %s", id)
	}

	return sqlf.Sprintf("TRUE")
}

// InsertDependencyRepos creates the given dependency repos if they don't yet exist. The values that did not exist previously are returned.
// [{npm, @types/nodejs, [v0.0.1]}, {npm, @types/nodejs, [v0.0.2]}] will be collapsed into [{npm, @types/nodejs, [v0.0.1, v0.0.2]}]
func (s *store) InsertPackageRepoRefs(ctx context.Context, deps []shared.MinimalPackageRepoRef) (newDeps []shared.PackageRepoReference, newVersions []shared.PackageRepoRefVersion, err error) {
	ctx, _, endObservation := s.operations.insertPackageRepoRefs.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numInputDeps", len(deps)),
	}})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("newDependencies", len(newDeps)),
			attribute.Int("newVersion", len(newVersions)),
			attribute.Int("numDedupedDeps", len(deps)),
		}})
	}()

	if len(deps) == 0 {
		return
	}

	slices.SortStableFunc(deps, func(a, b shared.MinimalPackageRepoRef) bool {
		if a.Scheme != b.Scheme {
			return a.Scheme < b.Scheme
		}

		return a.Name < b.Name
	})

	// first reduce
	var lastCommon int
	for i, dep := range deps[1:] {
		if dep.Name == deps[lastCommon].Name && dep.Scheme == deps[lastCommon].Scheme {
			deps[lastCommon].Versions = append(deps[lastCommon].Versions, dep.Versions...)
			deps[i+1] = shared.MinimalPackageRepoRef{}
		} else {
			lastCommon = i + 1
		}
	}

	// then collapse
	nonDupes := deps[:0]
	for _, dep := range deps {
		if dep.Name != "" && dep.Scheme != "" {
			nonDupes = append(nonDupes, dep)
		}
	}
	// replace the originals :wave
	deps = nonDupes

	tx, err := s.db.Transact(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		err = tx.Done(err)
	}()

	for _, tempTableQuery := range []string{temporaryPackageRepoRefsTableQuery, temporaryPackageRepoRefVersionsTableQuery} {
		if err := tx.Exec(ctx, sqlf.Sprintf(tempTableQuery)); err != nil {
			return nil, nil, errors.Wrap(err, "failed to create temporary tables")
		}
	}

	err = batch.WithInserter(
		ctx,
		tx.Handle(),
		"t_package_repo_refs",
		batch.MaxNumPostgresParameters,
		[]string{"scheme", "name", "blocked", "last_checked_at"},
		func(inserter *batch.Inserter) error {
			for _, pkg := range deps {
				if err := inserter.Insert(ctx, pkg.Scheme, pkg.Name, pkg.Blocked, pkg.LastCheckedAt); err != nil {
					return err
				}
			}
			return nil
		},
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to insert package repos in temporary table")
	}

	newDeps, err = basestore.NewSliceScanner(func(rows dbutil.Scanner) (dep shared.PackageRepoReference, err error) {
		err = rows.Scan(&dep.ID, &dep.Scheme, &dep.Name, &dep.Blocked, &dep.LastCheckedAt)
		return
	})(tx.Query(ctx, sqlf.Sprintf(transferPackageRepoRefsQuery)))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to transfer package repos from temporary table")
	}

	// we need the IDs of all newly inserted and already existing package repo references
	// for all of the references in `deps`, so that we have the package repo reference ID that
	// we need for the package repo reference versions table.
	// We already have the IDs of newly inserted ones (in `newDeps`), but for simplicity we'll
	// just search based on (scheme, name) tuple in `deps`.

	// we slice into `deps`, which will continuously shrink as we batch based on the amount of
	// postgres parameters we can fit. Divide by 2 because for each entry in the batch, we need 2 free params
	const maxBatchSize = batch.MaxNumPostgresParameters / 2
	remainingDeps := deps

	allIDs := make([]int, 0, len(deps))

	for len(remainingDeps) > 0 {
		// avoid slice out of bounds nonsense
		var batch []shared.MinimalPackageRepoRef
		if len(remainingDeps) <= maxBatchSize {
			batch, remainingDeps = remainingDeps, nil
		} else {
			batch, remainingDeps = remainingDeps[:maxBatchSize], remainingDeps[maxBatchSize:]
		}

		// dont over-allocate
		max := maxBatchSize
		if len(remainingDeps) < maxBatchSize {
			max = len(remainingDeps)
		}
		params := make([]*sqlf.Query, 0, max)
		for _, dep := range batch {
			params = append(params, sqlf.Sprintf("(%s, %s)", dep.Scheme, dep.Name))
		}

		query := sqlf.Sprintf(
			getAttemptedInsertDependencyReposQuery,
			sqlf.Join(params, ", "),
		)

		allIDsWindow, err := basestore.ScanInts(tx.Query(ctx, query))
		if err != nil {
			return nil, nil, err
		}
		allIDs = append(allIDs, allIDsWindow...)
	}

	err = batch.WithInserter(
		ctx,
		tx.Handle(),
		"t_package_repo_versions",
		batch.MaxNumPostgresParameters,
		[]string{"package_id", "version", "blocked", "last_checked_at"},
		func(inserter *batch.Inserter) error {
			for i, dep := range deps {
				for _, version := range dep.Versions {
					if err := inserter.Insert(ctx, allIDs[i], version.Version, version.Blocked, version.LastCheckedAt); err != nil {
						return err
					}
				}
			}
			return nil
		})
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to insert package repo versions in temporary table")
	}

	newVersions, err = basestore.NewSliceScanner(func(rows dbutil.Scanner) (version shared.PackageRepoRefVersion, err error) {
		err = rows.Scan(&version.ID, &version.PackageRefID, &version.Version, &version.Blocked, &version.LastCheckedAt)
		return
	})(tx.Query(ctx, sqlf.Sprintf(transferPackageRepoRefVersionsQuery)))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to transfer package repos from temporary table")
	}

	return newDeps, newVersions, err
}

const temporaryPackageRepoRefsTableQuery = `
CREATE TEMPORARY TABLE t_package_repo_refs (
	scheme TEXT NOT NULL,
	name TEXT NOT NULL,
	blocked BOOLEAN NOT NULL,
	last_checked_at TIMESTAMPTZ
) ON COMMIT DROP
`

const temporaryPackageRepoRefVersionsTableQuery = `
CREATE TEMPORARY TABLE t_package_repo_versions (
	package_id BIGINT NOT NULL,
	version TEXT NOT NULL,
	blocked BOOLEAN NOT NULL,
	last_checked_at TIMESTAMPTZ
) ON COMMIT DROP
`

const transferPackageRepoRefsQuery = `
INSERT INTO lsif_dependency_repos (scheme, name, blocked, last_checked_at)
SELECT scheme, name, blocked, last_checked_at
FROM t_package_repo_refs t
WHERE NOT EXISTS (
	SELECT scheme, name
	FROM lsif_dependency_repos
	WHERE scheme = t.scheme AND
	name = t.name
)
ORDER BY name
RETURNING id, scheme, name, blocked, last_checked_at
`

const transferPackageRepoRefVersionsQuery = `
INSERT INTO package_repo_versions (package_id, version, blocked, last_checked_at)
-- we dont reduce package repo versions,
-- so DISTINCT here to avoid conflict
SELECT DISTINCT ON (package_id, version) package_id, version, blocked, last_checked_at
FROM t_package_repo_versions t
WHERE NOT EXISTS (
	SELECT package_id, version
	FROM package_repo_versions
	WHERE package_id = t.package_id AND
	version = t.version
)
-- unit tests rely on a certain order
ORDER BY package_id, version
RETURNING id, package_id, version, blocked, last_checked_at
`

const getAttemptedInsertDependencyReposQuery = `
SELECT id FROM lsif_dependency_repos
WHERE (scheme, name) IN (VALUES %s)
ORDER BY (scheme, name)
`

// DeleteDependencyReposByID removes the dependency repos with the given ids, if they exist.
func (s *store) DeletePackageRepoRefsByID(ctx context.Context, ids ...int) (err error) {
	ctx, _, endObservation := s.operations.deletePackageRepoRefsByID.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numIDs", len(ids)),
	}})
	defer endObservation(1, observation.Args{})

	if len(ids) == 0 {
		return nil
	}

	return s.db.Exec(ctx, sqlf.Sprintf(deleteDependencyReposByIDQuery, pq.Array(ids)))
}

const deleteDependencyReposByIDQuery = `
DELETE FROM lsif_dependency_repos
WHERE id = ANY(%s)
`

func (s *store) DeletePackageRepoRefVersionsByID(ctx context.Context, ids ...int) (err error) {
	ctx, _, endObservation := s.operations.deletePackageRepoRefVersionsByID.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numIDs", len(ids)),
	}})
	defer endObservation(1, observation.Args{})

	if len(ids) == 0 {
		return nil
	}

	return s.db.Exec(ctx, sqlf.Sprintf(deleteDependencyRepoVersionsByID, pq.Array(ids)))
}

const deleteDependencyRepoVersionsByID = `
DELETE FROM package_repo_versions
WHERE id = ANY(%s)
`

type ListPackageRepoRefFiltersOpts struct {
	IDs            []int
	PackageScheme  string
	Behaviour      string
	IncludeDeleted bool
	After          int
	Limit          int
}

func (s *store) ListPackageRepoRefFilters(ctx context.Context, opts ListPackageRepoRefFiltersOpts) (_ []shared.PackageRepoFilter, hasMore bool, err error) {
	ctx, _, endObservation := s.operations.listPackageRepoFilters.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numPackageRepoFilterIDs", len(opts.IDs)),
		attribute.String("packageScheme", opts.PackageScheme),
		attribute.Int("after", opts.After),
		attribute.Int("limit", opts.Limit),
		attribute.String("behaviour", opts.Behaviour),
	}})
	defer endObservation(1, observation.Args{})

	conds := make([]*sqlf.Query, 0, 6)

	if !opts.IncludeDeleted {
		conds = append(conds, sqlf.Sprintf("deleted_at IS NULL"))
	}

	if len(opts.IDs) != 0 {
		conds = append(conds, sqlf.Sprintf("id = ANY(%s)", pq.Array(opts.IDs)))
	}

	if opts.PackageScheme != "" {
		conds = append(conds, sqlf.Sprintf("scheme = %s", opts.PackageScheme))
	}

	if opts.After != 0 {
		conds = append(conds, sqlf.Sprintf("id > %s", opts.After))
	}

	if opts.Behaviour != "" {
		conds = append(conds, sqlf.Sprintf("behaviour = %s", opts.Behaviour))
	}

	if len(conds) == 0 {
		conds = append(conds, sqlf.Sprintf("TRUE"))
	}

	limit := sqlf.Sprintf("")
	if opts.Limit != 0 {
		// + 1 to check if more pages
		limit = sqlf.Sprintf("LIMIT %s", opts.Limit+1)
	}

	filters, err := basestore.NewSliceScanner(scanPackageFilter)(
		s.db.Query(ctx, sqlf.Sprintf(
			listPackageRepoRefFiltersQuery,
			sqlf.Join(conds, "AND"),
			limit,
		)),
	)

	if opts.Limit != 0 && len(filters) > opts.Limit {
		filters = filters[:opts.Limit]
		hasMore = true
	}

	return filters, hasMore, err
}

const listPackageRepoRefFiltersQuery = `
SELECT id, behaviour, scheme, matcher, deleted_at, updated_at
FROM package_repo_filters
-- filter
WHERE %s
-- limit
%s
ORDER BY id
`

func (s *store) CreatePackageRepoFilter(ctx context.Context, input shared.MinimalPackageFilter) (filter *shared.PackageRepoFilter, err error) {
	ctx, _, endObservation := s.operations.createPackageRepoFilter.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("packageScheme", input.PackageScheme),
		attribute.String("behaviour", *input.Behaviour),
		attribute.String("versionFilter", fmt.Sprintf("%+v", input.VersionFilter)),
		attribute.String("nameFilter", fmt.Sprintf("%+v", input.NameFilter)),
	}})
	defer endObservation(1, observation.Args{})

	var matcherJSON driver.Value
	if input.NameFilter != nil {
		matcherJSON, err = json.Marshal(input.NameFilter)
		err = errors.Wrapf(err, "error marshalling %+v", input.NameFilter)
	} else if input.VersionFilter != nil {
		matcherJSON, err = json.Marshal(input.VersionFilter)
		err = errors.Wrapf(err, "error marshalling %+v", input.VersionFilter)
	}
	if err != nil {
		return nil, err
	}

	hydrated := &shared.PackageRepoFilter{
		Behaviour:     *input.Behaviour,
		PackageScheme: input.PackageScheme,
		NameFilter:    input.NameFilter,
		VersionFilter: input.VersionFilter,
		DeletedAt:     nil,
	}

	err = basestore.NewCallbackScanner(func(s dbutil.Scanner) (bool, error) {
		return false, s.Scan(&hydrated.ID, &hydrated.UpdatedAt)
	})(s.db.Query(ctx, sqlf.Sprintf(createPackageRepoFilter, input.Behaviour, input.PackageScheme, matcherJSON)))
	if err != nil {
		return nil, errors.Wrap(err, "error inserting package repo filter")
	}

	return hydrated, nil
}

const createPackageRepoFilter = `
INSERT INTO package_repo_filters (behaviour, scheme, matcher)
VALUES (%s, %s, %s)
ON CONFLICT (scheme, matcher)
DO UPDATE
	SET deleted_at = NULL,
	updated_at = now(),
	behaviour = EXCLUDED.behaviour
RETURNING id, updated_at
`

func (s *store) UpdatePackageRepoFilter(ctx context.Context, filter shared.PackageRepoFilter) (err error) {
	ctx, _, endObservation := s.operations.updatePackageRepoFilter.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("id", filter.ID),
		attribute.String("packageScheme", filter.PackageScheme),
		attribute.String("behaviour", filter.Behaviour),
		attribute.String("versionFilter", fmt.Sprintf("%+v", filter.VersionFilter)),
		attribute.String("nameFilter", fmt.Sprintf("%+v", filter.NameFilter)),
	}})
	defer endObservation(1, observation.Args{})

	var matcherJSON driver.Value
	if filter.NameFilter != nil {
		matcherJSON, err = json.Marshal(filter.NameFilter)
		err = errors.Wrapf(err, "error marshalling %+v", filter.NameFilter)
	} else if filter.VersionFilter != nil {
		matcherJSON, err = json.Marshal(filter.VersionFilter)
		err = errors.Wrapf(err, "error marshalling %+v", filter.VersionFilter)
	}
	if err != nil {
		return err
	}

	result, err := s.db.ExecResult(ctx, sqlf.Sprintf(
		updatePackageRepoFilterQuery,
		filter.PackageScheme,
		matcherJSON,
		filter.ID,
		filter.ID,
		filter.Behaviour,
		filter.PackageScheme,
		matcherJSON,
		filter.ID,
	))
	if err != nil {
		var pgerr *pgconn.PgError
		// check if conflict error code
		if errors.As(err, &pgerr) && pgerr.Code == "23505" {
			return errors.Newf("conflicting package repo filter found for (scheme=%s,matcher=%s)", filter.PackageScheme, string(matcherJSON.([]byte)))
		}
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return errors.Newf("no package repo filters for ID %d", filter.ID)
	}
	return nil
}

const updatePackageRepoFilterQuery = `
-- hard-delete a conflicting one if its soft-deleted
WITH delete_conflicting_deleted AS (
	DELETE FROM package_repo_filters
	WHERE
		scheme = %s AND
		matcher = %s AND
		deleted_at IS NOT NULL
	RETURNING %s::integer AS id
),
-- if the above matches nothing, we still need to return something
-- else we join on nothing below and attempt update nothing, hence union
always_id AS (
	SELECT id
	FROM delete_conflicting_deleted
	UNION
	SELECT %s::integer AS id
)
UPDATE package_repo_filters prv
SET
	behaviour = %s,
	scheme = %s,
	matcher = %s
FROM always_id
WHERE prv.id = %s AND prv.id = always_id.id
`

func (s *store) DeletePacakgeRepoFilter(ctx context.Context, id int) (err error) {
	ctx, _, endObservation := s.operations.deletePackageRepoFilter.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("id", id),
	}})
	defer endObservation(1, observation.Args{})

	result, err := s.db.ExecResult(ctx, sqlf.Sprintf(deletePackagRepoFilterQuery, id))
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return errors.Newf("no package repo filters for ID %d", id)
	}
	return nil
}

const deletePackagRepoFilterQuery = `
UPDATE package_repo_filters
SET deleted_at = now()
WHERE id = %s
`

func (s *store) ShouldRefilterPackageRepoRefs(ctx context.Context) (exists bool, err error) {
	ctx, _, endObservation := s.operations.shouldRefilterPackageRepoRefs.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	_, exists, err = basestore.ScanFirstInt(s.db.Query(ctx, sqlf.Sprintf(doPackageRepoRefsRequireRefilteringQuery)))
	return
}

const doPackageRepoRefsRequireRefilteringQuery = `
WITH least_recently_checked AS (
	-- select oldest last_checked_at from either package_repo_versions
	-- or lsif_dependency_repos, prioritising NULL
    SELECT * FROM (
        (
			SELECT last_checked_at FROM lsif_dependency_repos
			ORDER BY last_checked_at ASC NULLS FIRST
			LIMIT 1
		)
        UNION ALL
        (
			SELECT last_checked_at FROM package_repo_versions
			ORDER BY last_checked_at ASC NULLS FIRST
			LIMIT 1
		)
    ) p
    ORDER BY last_checked_at ASC NULLS FIRST
    LIMIT 1
),
most_recently_updated_filter AS (
    SELECT COALESCE(deleted_at, updated_at)
	FROM package_repo_filters
	ORDER BY COALESCE(deleted_at, updated_at) DESC
	LIMIT 1
)
SELECT 1
WHERE
	-- comparisons on empty table from either least_recently_checked or most_recently_updated_filter
	-- will yield NULL, making the query return 1 if either CTE returns nothing
    (SELECT COUNT(*) FROM most_recently_updated_filter) <> 0 AND
    (SELECT COUNT(*) FROM least_recently_checked) <> 0 AND
    (
        (SELECT * FROM least_recently_checked) IS NULL OR
        (SELECT * FROM least_recently_checked) < (SELECT * FROM most_recently_updated_filter)
    );
`

func (s *store) UpdateAllBlockedStatuses(ctx context.Context, pkgs []shared.PackageRepoReference, startTime time.Time) (pkgsUpdated, versionsUpdated int, err error) {
	ctx, _, endObservation := s.operations.updateAllBlockedStatuses.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numPackages", len(pkgs)),
		attribute.String("startTime", startTime.Format(time.RFC3339)),
	}})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("packagesUpdated", pkgsUpdated),
			attribute.Int("versionsUpdated", versionsUpdated),
		}})
	}()

	err = s.db.WithTransact(ctx, func(tx *basestore.Store) error {
		for _, tempTableQuery := range []string{temporaryPackageRepoRefsBlockStatusTableQuery, temporaryPackageRepoRefVersionsBlockStatusTableQuery} {
			if err := tx.Exec(ctx, sqlf.Sprintf(tempTableQuery)); err != nil {
				return errors.Wrap(err, "failed to create temporary tables")
			}
		}

		err := batch.WithInserter(
			ctx,
			tx.Handle(),
			"t_lsif_dependency_repos",
			batch.MaxNumPostgresParameters,
			[]string{"id", "blocked"},
			func(inserter *batch.Inserter) error {
				for _, pkg := range pkgs {
					if err := inserter.Insert(ctx, pkg.ID, pkg.Blocked); err != nil {
						return errors.Wrapf(err, "error inserting (id=%d,blocked=%t)", pkg.ID, pkg.Blocked)
					}
				}
				return nil
			},
		)
		if err != nil {
			return errors.Wrap(err, "error inserting into temporary package repos table")
		}

		err = batch.WithInserter(ctx,
			tx.Handle(),
			"t_package_repo_versions",
			batch.MaxNumPostgresParameters,
			[]string{"id", "blocked"},
			func(inserter *batch.Inserter) error {
				for _, pkg := range pkgs {
					for _, version := range pkg.Versions {
						if err := inserter.Insert(ctx, version.ID, version.Blocked); err != nil {
							return errors.Wrapf(err, "error inserting (id=%d,blocked=%t)", version.ID, version.Blocked)
						}
					}
				}
				return nil
			},
		)
		if err != nil {
			return errors.Wrap(err, "error inserting into temporary package repo versions table")
		}

		err = basestore.NewCallbackScanner(func(s dbutil.Scanner) (bool, error) {
			return false, s.Scan(&pkgsUpdated, &versionsUpdated)
		})(tx.Query(ctx, sqlf.Sprintf(updateAllBlockedStatusesQuery, startTime, startTime)))
		return errors.Wrap(err, "error scanning update results")
	})

	return
}

const temporaryPackageRepoRefsBlockStatusTableQuery = `
CREATE TEMPORARY TABLE t_lsif_dependency_repos (
	id BIGINT NOT NULL,
	blocked BOOLEAN NOT NULL
) ON COMMIT DROP
`

const temporaryPackageRepoRefVersionsBlockStatusTableQuery = `
CREATE TEMPORARY TABLE t_package_repo_versions (
	id BIGINT NOT NULL,
	blocked BOOLEAN NOT NULL
) ON COMMIT DROP
`

const updateAllBlockedStatusesQuery = `
WITH updated_package_repos AS (
	UPDATE lsif_dependency_repos new
	SET
		blocked = temp.blocked,
		last_checked_at = %s
	FROM t_lsif_dependency_repos temp
	JOIN lsif_dependency_repos old
	ON temp.id = old.id
	WHERE old.id = new.id
	RETURNING old.blocked <> new.blocked AS changed
),
updated_package_repo_versions AS (
	UPDATE package_repo_versions new
	SET
		blocked = temp.blocked,
		last_checked_at = %s
	FROM t_package_repo_versions temp
	JOIN package_repo_versions old
	ON temp.id = old.id
	WHERE old.id = new.id
	RETURNING old.blocked <> new.blocked AS changed
)
SELECT (
	SELECT COUNT(*) FILTER (WHERE changed)
	FROM updated_package_repos
) AS packages_changed, (
	SELECT COUNT(*) FILTER (WHERE changed)
	FROM updated_package_repo_versions
) AS versions_changed
`
