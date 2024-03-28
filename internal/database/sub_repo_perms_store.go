package database

import (
	"context"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// SubRepoPermsVersion is defines the version we are using to encode our include
// and exclude patterns.
const SubRepoPermsVersion = 1

var (
	SubRepoSupportedCodeHostTypes = []string{extsvc.TypePerforce}
	supportedTypesQuery           = make([]*sqlf.Query, len(SubRepoSupportedCodeHostTypes))
)

func init() {
	// Build this up at startup, so we don't need to rebuild it every time
	// RepoSupported is called
	for i, hostType := range SubRepoSupportedCodeHostTypes {
		supportedTypesQuery[i] = sqlf.Sprintf("%s", hostType)
	}
}

type SubRepoPermsStore interface {
	basestore.ShareableStore
	With(other basestore.ShareableStore) SubRepoPermsStore
	Transact(ctx context.Context) (SubRepoPermsStore, error)
	Done(err error) error
	Upsert(ctx context.Context, userID int32, repoID api.RepoID, perms authz.SubRepoPermissions) error
	UpsertWithSpec(ctx context.Context, userID int32, spec api.ExternalRepoSpec, perms authz.SubRepoPermissions) error
	Get(ctx context.Context, userID int32, repoID api.RepoID) (*authz.SubRepoPermissions, error)
	GetByUser(ctx context.Context, userID int32) (map[api.RepoName]authz.SubRepoPermissions, error)
	// GetByUserAndService gets the sub repo permissions for a user, but filters down
	// to only repos that come from a specific external service.
	GetByUserAndService(ctx context.Context, userID int32, serviceType string, serviceID string) (map[api.ExternalRepoSpec]authz.SubRepoPermissions, error)
	RepoIDSupported(ctx context.Context, repoID api.RepoID) (bool, error)
	RepoSupported(ctx context.Context, repo api.RepoName) (bool, error)
	DeleteByUser(ctx context.Context, userID int32) error
}

// subRepoPermsStore is the unified interface for managing sub repository
// permissions explicitly in the database. It is concurrency-safe and maintains
// data consistency over sub_repo_permissions table.
type subRepoPermsStore struct {
	*basestore.Store
}

var _ SubRepoPermsStore = (*subRepoPermsStore)(nil)

func SubRepoPermsWith(other basestore.ShareableStore) SubRepoPermsStore {
	return &subRepoPermsStore{Store: basestore.NewWithHandle(other.Handle())}
}

func (s *subRepoPermsStore) With(other basestore.ShareableStore) SubRepoPermsStore {
	return &subRepoPermsStore{Store: s.Store.With(other)}
}

// Transact begins a new transaction and make a new SubRepoPermsStore over it.
func (s *subRepoPermsStore) Transact(ctx context.Context) (SubRepoPermsStore, error) {
	txBase, err := s.Store.Transact(ctx)
	return &subRepoPermsStore{Store: txBase}, err
}

func (s *subRepoPermsStore) Done(err error) error {
	return s.Store.Done(err)
}

// Upsert will upsert sub repo permissions data.
func (s *subRepoPermsStore) Upsert(ctx context.Context, userID int32, repoID api.RepoID, perms authz.SubRepoPermissions) error {
	q := sqlf.Sprintf(`
INSERT INTO sub_repo_permissions (user_id, repo_id, paths, version, updated_at)
VALUES (%s, %s, %s, %s, now())
ON CONFLICT (user_id, repo_id, version)
DO UPDATE
SET
  user_id = EXCLUDED.user_ID,
  repo_id = EXCLUDED.repo_id,
  paths = EXCLUDED.paths,
  version = EXCLUDED.version,
  updated_at = now()
`, userID, repoID, pq.Array(perms.Paths), SubRepoPermsVersion)
	return errors.Wrap(s.Exec(ctx, q), "upserting sub repo permissions")
}

// UpsertWithSpec will upsert sub repo permissions data using the provided
// external repo spec to map to our internal repo id. If there is no mapping,
// nothing is written.
func (s *subRepoPermsStore) UpsertWithSpec(ctx context.Context, userID int32, spec api.ExternalRepoSpec, perms authz.SubRepoPermissions) error {
	q := sqlf.Sprintf(`
INSERT INTO sub_repo_permissions (user_id, repo_id, paths, version, updated_at)
SELECT %s, id, %s, %s, now()
FROM repo
WHERE external_service_id = %s
  AND external_service_type = %s
  AND external_id = %s
ON CONFLICT (user_id, repo_id, version)
DO UPDATE
SET
  user_id = EXCLUDED.user_ID,
  repo_id = EXCLUDED.repo_id,
  paths = EXCLUDED.paths,
  version = EXCLUDED.version,
  updated_at = now()
`, userID, pq.Array(perms.Paths), SubRepoPermsVersion, spec.ServiceID, spec.ServiceType, spec.ID)

	return errors.Wrap(s.Exec(ctx, q), "upserting sub repo permissions with spec")
}

// Get will fetch sub repo rules for the given repo and user combination.
func (s *subRepoPermsStore) Get(ctx context.Context, userID int32, repoID api.RepoID) (*authz.SubRepoPermissions, error) {
	q := sqlf.Sprintf(`
SELECT paths
FROM sub_repo_permissions
WHERE repo_id = %s
  AND user_id = %s
  AND version = %s
`, userID, repoID, SubRepoPermsVersion)

	rows, err := s.Query(ctx, q)
	if err != nil {
		return nil, errors.Wrap(err, "getting sub repo permissions")
	}

	perms := new(authz.SubRepoPermissions)
	for rows.Next() {
		var paths []string
		if err := rows.Scan(pq.Array(&paths)); err != nil {
			return nil, errors.Wrap(err, "scanning row")
		}
		perms.Paths = append(perms.Paths, paths...)
	}

	if err := rows.Close(); err != nil {
		return nil, errors.Wrap(err, "closing rows")
	}

	return perms, nil
}

// GetByUser fetches all sub repo perms for a user keyed by repo.
func (s *subRepoPermsStore) GetByUser(ctx context.Context, userID int32) (map[api.RepoName]authz.SubRepoPermissions, error) {
	enforceForSiteAdmins := conf.Get().AuthzEnforceForSiteAdmins

	q := sqlf.Sprintf(`
	SELECT r.name, paths
	FROM sub_repo_permissions
	JOIN repo r on r.id = repo_id
	JOIN users u on u.id = user_id
	WHERE user_id = %s
	AND version = %s
	-- When user is a site admin and AuthzEnforceForSiteAdmins is FALSE
	-- we want to return zero results. This causes us to fall back to
	-- repo level checks and allows access to all paths in all repos.
	AND NOT (is_user_site_admin(u.id) AND NOT %t)
	`, userID, SubRepoPermsVersion, enforceForSiteAdmins)

	rows, err := s.Query(ctx, q)
	if err != nil {
		return nil, errors.Wrap(err, "getting sub repo permissions by user")
	}

	result := make(map[api.RepoName]authz.SubRepoPermissions)
	for rows.Next() {
		var perms authz.SubRepoPermissions
		var repoName api.RepoName
		if err := rows.Scan(&repoName, pq.Array(&perms.Paths)); err != nil {
			return nil, errors.Wrap(err, "scanning row")
		}
		result[repoName] = perms
	}

	if err := rows.Close(); err != nil {
		return nil, errors.Wrap(err, "closing rows")
	}

	return result, nil
}

func (s *subRepoPermsStore) GetByUserAndService(ctx context.Context, userID int32, serviceType string, serviceID string) (map[api.ExternalRepoSpec]authz.SubRepoPermissions, error) {
	q := sqlf.Sprintf(`
SELECT r.external_id, paths
FROM sub_repo_permissions
JOIN repo r on r.id = repo_id
WHERE user_id = %s
  AND version = %s
  AND r.external_service_type = %s
  AND r.external_service_id = %s
`, userID, SubRepoPermsVersion, serviceType, serviceID)

	rows, err := s.Query(ctx, q)
	if err != nil {
		return nil, errors.Wrap(err, "getting sub repo permissions by user and service")
	}

	result := make(map[api.ExternalRepoSpec]authz.SubRepoPermissions)
	for rows.Next() {
		var perms authz.SubRepoPermissions
		spec := api.ExternalRepoSpec{
			ServiceType: serviceType,
			ServiceID:   serviceID,
		}
		if err := rows.Scan(&spec.ID, pq.Array(&perms.Paths)); err != nil {
			return nil, errors.Wrap(err, "scanning row")
		}
		result[spec] = perms
	}

	if err := rows.Close(); err != nil {
		return nil, errors.Wrap(err, "closing rows")
	}

	return result, nil
}

// RepoIDSupported returns true if repo with the given ID has sub-repo permissions
// (i.e. it is private and its type is one of the SubRepoSupportedCodeHostTypes)
func (s *subRepoPermsStore) RepoIDSupported(ctx context.Context, repoID api.RepoID) (bool, error) {
	q := sqlf.Sprintf(`
SELECT EXISTS(
SELECT
FROM repo
WHERE id = %s
AND private = TRUE
AND external_service_type IN (%s)
)
`, repoID, sqlf.Join(supportedTypesQuery, ","))

	exists, _, err := basestore.ScanFirstBool(s.Query(ctx, q))
	if err != nil {
		return false, errors.Wrap(err, "querying database")
	}
	return exists, nil
}

// RepoSupported returns true if repo has sub-repo permissions
// (i.e. it is private and its type is one of the SubRepoSupportedCodeHostTypes)
func (s *subRepoPermsStore) RepoSupported(ctx context.Context, repo api.RepoName) (bool, error) {
	q := sqlf.Sprintf(`
SELECT EXISTS(
SELECT
FROM repo
WHERE name = %s
AND private = TRUE
AND external_service_type IN (%s)
)
`, repo, sqlf.Join(supportedTypesQuery, ","))

	exists, _, err := basestore.ScanFirstBool(s.Query(ctx, q))
	if err != nil {
		return false, errors.Wrap(err, "querying database")
	}
	return exists, nil
}

// DeleteByUser deletes all rows associated with the given user
func (s *subRepoPermsStore) DeleteByUser(ctx context.Context, userID int32) error {
	q := sqlf.Sprintf(`
DELETE FROM sub_repo_permissions WHERE user_id = %d
`, userID)
	return s.Exec(ctx, q)
}
