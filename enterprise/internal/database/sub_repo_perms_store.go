package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

// SubRepoPermsStore is the unified interface for managing sub repository
// permissions explicitly in the database. It is concurrency-safe and maintains
// data consistency over sub_repo_permissions table.
type SubRepoPermsStore struct {
	*basestore.Store

	clock func() time.Time
}

// SubRepoPerms returns a new SubRepoPermsStore with the given parameters.
func SubRepoPerms(db dbutil.DB, clock func() time.Time) *SubRepoPermsStore {
	return &SubRepoPermsStore{Store: basestore.NewWithDB(db, sql.TxOptions{}), clock: clock}
}

func (s *SubRepoPermsStore) With(other basestore.ShareableStore) *SubRepoPermsStore {
	return &SubRepoPermsStore{Store: s.Store.With(other), clock: s.clock}
}

// Transact begins a new transaction and make a new SubRepoPermsStore over it.
func (s *SubRepoPermsStore) Transact(ctx context.Context) (*SubRepoPermsStore, error) {
	if Mocks.Perms.Transact != nil {
		return Mocks.SubRepoPerms.Transact(ctx)
	}

	txBase, err := s.Store.Transact(ctx)
	return &SubRepoPermsStore{Store: txBase, clock: s.clock}, err
}

func (s *SubRepoPermsStore) Done(err error) error {
	if Mocks.Perms.Transact != nil {
		return err
	}

	return s.Store.Done(err)
}

// Upsert will upsert sub repo permissions data
func (s *SubRepoPermsStore) Upsert(ctx context.Context, userID, repoID int32, perms authz.SubRepoPermissions) error {
	// TODO: Replace params with authz type once merged
	q := sqlf.Sprintf(`
INSERT INTO sub_repo_permissions (user_id, repo_id, path_includes, path_excludes, updated_at)
VALUES (%s, %s, %s, %s, now())
ON CONFLICT (user_id, repo_id) DO UPDATE
SET (user_id, repo_id, path_includes, path_excludes, updated_at) =
(EXCLUDED.user_id, EXCLUDED.repo_id, EXCLUDED.path_includes, EXCLUDED.path_excludes, now())
`, userID, repoID, pq.Array(perms.PathIncludes), pq.Array(perms.PathExcludes))
	return errors.Wrap(s.Exec(ctx, q), "upserting sub repo permissions")
}

// GetRules will fetch sub repo rules for the given repo and user combination
func (s *SubRepoPermsStore) GetRules(ctx context.Context, userID, repoID int32) (*authz.SubRepoPermissions, error) {
	q := sqlf.Sprintf(`
SELECT path_includes, path_excludes
FROM sub_repo_permissions
WHERE user_id = %s
AND repo_id = %s
`, userID, repoID)

	rows, err := s.Query(ctx, q)
	if err != nil {
		return nil, errors.Wrap(err, "getting sub repo permissions")
	}

	perms := new(authz.SubRepoPermissions)
	for rows.Next() {
		var includes []string
		var excludes []string
		if err := rows.Scan(pq.Array(&includes), pq.Array(&excludes)); err != nil {
			return nil, errors.Wrap(err, "scanning row")
		}
		perms.PathIncludes = append(perms.PathIncludes, includes...)
		perms.PathExcludes = append(perms.PathExcludes, excludes...)
	}

	if err := rows.Close(); err != nil {
		return nil, errors.Wrap(err, "closing rows")
	}

	return perms, nil
}

type MockSubRepoPerms struct {
	Transact func(ctx context.Context) (*SubRepoPermsStore, error)
}
