package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

var (
	ErrPermsUpdatedAtNotSet = errors.New("permissions UpdatedAt timestamp must be set")
	ErrPermsSyncedAtNotSet  = errors.New("permissions SyncedAt timestamp must be set")
)

// PermsStore is the unified interface for managing permissions in the database.
type PermsStore interface {
	basestore.ShareableStore
	With(other basestore.ShareableStore) PermsStore
	// Transact begins a new transaction and make a new PermsStore over it.
	Transact(ctx context.Context) (PermsStore, error)
	Done(err error) error

	// LoadUserPermissions returns user permissions. An empty slice
	// is returned when there are no valid permissions available.
	LoadUserPermissions(ctx context.Context, userID int32) (p []authz.Permission, err error)
	// FetchReposByExternalAccount fetches repo ids that the originate from the given external account.
	FetchReposByExternalAccount(ctx context.Context, accountID int32) ([]api.RepoID, error)
	// LoadRepoPermissions returns stored repository permissions.
	// Empty slice is returned when there are no valid permissions available.
	// Slice with length 1 and userID == 0 is returned for unrestricted repo.
	LoadRepoPermissions(ctx context.Context, repoID int32) ([]authz.Permission, error)
	// SetUserExternalAccountPerms sets the users permissions for repos in the database. Uses setUserRepoPermissions internally.
	SetUserExternalAccountPerms(ctx context.Context, user authz.UserIDWithExternalAccountID, repoIDs []int32, source authz.PermsSource) (*SetPermissionsResult, error)
	// SetRepoPerms sets the users that can access a repo. Uses setUserRepoPermissions internally.
	SetRepoPerms(ctx context.Context, repoID int32, userIDs []authz.UserIDWithExternalAccountID, source authz.PermsSource) (*SetPermissionsResult, error)
	// SetRepoPermissionsUnrestricted sets the unrestricted on the
	// repo_permissions table for all the provided repos. Either all or non
	// are updated. If the repository ID is not in repo_permissions yet, a row
	// is inserted for read permission and an empty array of user ids. ids
	// must not contain duplicates.
	SetRepoPermissionsUnrestricted(ctx context.Context, ids []int32, unrestricted bool) error
	// LoadUserPendingPermissions returns pending permissions found by given
	// parameters. An ErrPermsNotFound is returned when there are no pending
	// permissions available.
	LoadUserPendingPermissions(ctx context.Context, p *authz.UserPendingPermissions) error
	// SetRepoPendingPermissions performs a full update for p with given accounts,
	// new account IDs found will be upserted and account IDs no longer in AccountIDs
	// will be removed.
	//
	// This method updates both `user_pending_permissions` and
	// `repo_pending_permissions` tables.
	//
	// This method starts its own transaction for update consistency if the caller
	// hasn't started one already.
	//
	// Example input:
	//  &extsvc.Accounts{
	//      ServiceType: "sourcegraph",
	//      ServiceID:   "https://sourcegraph.com/",
	//      AccountIDs:  []string{"alice", "bob"},
	//  }
	//  &authz.RepoPermissions{
	//      RepoID: 1,
	//      Perm: authz.Read,
	//  }
	//
	// Table states for input:
	// 	"user_pending_permissions":
	//   id | service_type |        service_id        | bind_id | permission | object_type | object_ids_ints | updated_at
	//  ----+--------------+--------------------------+---------+------------+-------------+-----------------+-----------
	//    1 | sourcegraph  | https://sourcegraph.com/ |   alice |       read |       repos |             {1} | <DateTime>
	//    2 | sourcegraph  | https://sourcegraph.com/ |     bob |       read |       repos |             {1} | <DateTime>
	//
	//  "repo_pending_permissions":
	//   repo_id | permission | user_ids_ints | updated_at
	//  ---------+------------+---------------+------------
	//         1 |       read |        {1, 2} | <DateTime>
	SetRepoPendingPermissions(ctx context.Context, accounts *extsvc.Accounts, p *authz.RepoPermissions) error
	// GrantPendingPermissions is used to grant pending permissions when the
	// associated "ServiceType", "ServiceID" and "AccountID" found in p becomes
	// effective for a given user, e.g. username as bind ID when a user is created,
	// email as bind ID when the email address is verified.
	//
	// Because there could be multiple external services and bind IDs that are
	// associated with a single user (e.g. same user on different code hosts,
	// multiple email addresses), it merges data from "repo_pending_permissions" and
	// "user_pending_permissions" tables to "user_repo_permissions" and legacy
	// "repo_permissions" and "user_permissions" tables for the user.
	//
	// Therefore, permissions are unioned not replaced, which is one of the main
	// differences from SetRepoPermissions and SetRepoPendingPermissions methods.
	// Another main difference is that multiple calls to this method are not
	// idempotent as it conceptually does nothing when there is no data in the
	// pending permissions tables for the user.
	//
	// This method starts its own transaction for update consistency if the caller
	// hasn't started one already.
	//
	// 🚨 SECURITY: This method takes arbitrary string as a valid account ID to bind
	// and does not interpret the meaning of the value it represents. Therefore, it is
	// caller's responsibility to ensure the legitimate relation between the given
	// user ID, user external account ID and the accountID found in p.
	GrantPendingPermissions(ctx context.Context, p *authz.UserGrantPermissions) error
	// ListPendingUsers returns a list of bind IDs who have pending permissions by
	// given service type and ID.
	ListPendingUsers(ctx context.Context, serviceType, serviceID string) (bindIDs []string, _ error)
	// DeleteAllUserPermissions deletes all rows with given user ID from the
	// "user_permissions" table, which effectively removes access to all repositories
	// for the user.
	DeleteAllUserPermissions(ctx context.Context, userID int32) error
	// DeleteAllUserPendingPermissions deletes all rows with given bind IDs from the
	// "user_pending_permissions" table. It accepts list of bind IDs because a user
	// has multiple bind IDs, e.g. username and email addresses.
	DeleteAllUserPendingPermissions(ctx context.Context, accounts *extsvc.Accounts) error
	// GetUserIDsByExternalAccounts returns all user IDs matched by given external
	// account specs. The returned set has mapping relation as "account ID -> user
	// ID". The number of results could be less than the candidate list due to some
	// users are not associated with any external account.
	GetUserIDsByExternalAccounts(ctx context.Context, accounts *extsvc.Accounts) (map[string]authz.UserIDWithExternalAccountID, error)
	// UserIDsWithNoPerms returns a list of user IDs with no permissions found in the
	// database.
	UserIDsWithNoPerms(ctx context.Context) ([]int32, error)
	// RepoIDsWithNoPerms returns a list of private repository IDs with no
	// permissions found in the database.
	RepoIDsWithNoPerms(ctx context.Context) ([]api.RepoID, error)
	// UserIDsWithOldestPerms returns a list of user ID and last updated pairs for
	// users who have the least recent synced permissions in the database and capped
	// results by the limit. If a user's permissions have been recently synced, based
	// on "age" they are ignored.
	UserIDsWithOldestPerms(ctx context.Context, limit int, age time.Duration) (map[int32]time.Time, error)
	// ReposIDsWithOldestPerms returns a list of repository ID and last updated pairs
	// for repositories that have the least recent synced permissions in the database
	// and caps results by the limit. If a repo's permissions have been recently
	// synced, based on "age" they are ignored.
	ReposIDsWithOldestPerms(ctx context.Context, limit int, age time.Duration) (map[api.RepoID]time.Time, error)
	// CountUsersWithNoPerms returns the count of users with no permissions found in the
	// database.
	CountUsersWithNoPerms(ctx context.Context) (int, error)
	// CountReposWithNoPerms returns the count of private repositories with no
	// permissions found in the database.
	CountReposWithNoPerms(ctx context.Context) (int, error)
	// CountUsersWithStalePerms returns the count of users who have the least
	// recent synced permissions in the database and capped. If a user's permissions
	// have been recently synced, based on "age" they are ignored.
	CountUsersWithStalePerms(ctx context.Context, age time.Duration) (int, error)
	// CountReposWithStalePerms returns the count of repositories that have the least
	// recent synced permissions in the database. If a repo's permissions have been recently
	// synced, based on "age" they are ignored.
	CountReposWithStalePerms(ctx context.Context, age time.Duration) (int, error)
	// Metrics returns calculated metrics values by querying the database. The
	// "staleDur" argument indicates how long ago was the last update to be
	// considered as stale.
	Metrics(ctx context.Context, staleDur time.Duration) (*PermsMetrics, error)
	// MapUsers takes a list of bind ids and a mapping configuration and maps them to the right user ids.
	// It filters out empty bindIDs and only returns users that exist in the database.
	// If a bind id doesn't map to any user, it is ignored.
	MapUsers(ctx context.Context, bindIDs []string, mapping *schema.PermissionsUserMapping) (map[string]int32, error)
	// ListUserPermissions returns list of repository permissions info the user has access to.
	ListUserPermissions(ctx context.Context, userID int32, args *ListUserPermissionsArgs) (perms []*UserPermission, err error)
	// ListRepoPermissions returns list of users the repo is accessible to.
	ListRepoPermissions(ctx context.Context, repoID api.RepoID, args *ListRepoPermissionsArgs) (perms []*RepoPermission, err error)
	// IsRepoUnrestructed returns if the repo is unrestricted.
	IsRepoUnrestricted(ctx context.Context, repoID api.RepoID) (unrestricted bool, err error)
}

// It is concurrency-safe and maintains data consistency over the 'user_permissions',
// 'repo_permissions', 'user_pending_permissions', and 'repo_pending_permissions' tables.
type permsStore struct {
	db     DB
	logger log.Logger
	*basestore.Store

	clock func() time.Time
}

var _ PermsStore = (*permsStore)(nil)

// Perms returns a new PermsStore with given parameters.
func Perms(logger log.Logger, db DB, clock func() time.Time) PermsStore {
	return perms(logger, db, clock)
}

func perms(logger log.Logger, db DB, clock func() time.Time) *permsStore {
	store := basestore.NewWithHandle(db.Handle())

	return &permsStore{logger: logger, Store: store, clock: clock, db: NewDBWith(logger, store)}
}

func PermsWith(logger log.Logger, other basestore.ShareableStore, clock func() time.Time) PermsStore {
	store := basestore.NewWithHandle(other.Handle())

	return &permsStore{logger: logger, Store: store, clock: clock, db: NewDBWith(logger, store)}
}

func (s *permsStore) With(other basestore.ShareableStore) PermsStore {
	store := s.Store.With(other)

	return &permsStore{logger: s.logger, Store: store, clock: s.clock, db: NewDBWith(s.logger, store)}
}

func (s *permsStore) Transact(ctx context.Context) (PermsStore, error) {
	return s.transact(ctx)
}

func (s *permsStore) transact(ctx context.Context) (*permsStore, error) {
	txBase, err := s.Store.Transact(ctx)
	return &permsStore{Store: txBase, clock: s.clock}, err
}

func (s *permsStore) Done(err error) error {
	return s.Store.Done(err)
}

func (s *permsStore) LoadUserPermissions(ctx context.Context, userID int32) (p []authz.Permission, err error) {
	ctx, save := s.observe(ctx, "LoadUserPermissions")
	defer func() {
		tracingFields := []attribute.KeyValue{}
		for _, perm := range p {
			tracingFields = append(tracingFields, perm.Attrs()...)
		}
		save(&err, tracingFields...)
	}()

	return s.loadUserRepoPermissions(ctx, userID, 0, 0)
}

var scanRepoIDs = basestore.NewSliceScanner(basestore.ScanAny[api.RepoID])

func (s *permsStore) FetchReposByExternalAccount(ctx context.Context, accountID int32) (ids []api.RepoID, err error) {
	const format = `
SELECT repo_id
FROM user_repo_permissions
WHERE user_external_account_id = %s;
`

	q := sqlf.Sprintf(format, accountID)

	ctx, save := s.observe(ctx, "FetchReposByExternalAccount")
	defer func() {
		save(&err)
	}()

	return scanRepoIDs(s.Query(ctx, q))
}

func (s *permsStore) LoadRepoPermissions(ctx context.Context, repoID int32) (p []authz.Permission, err error) {
	ctx, save := s.observe(ctx, "LoadRepoPermissions")
	defer func() {
		tracingFields := []attribute.KeyValue{}
		for _, perm := range p {
			tracingFields = append(tracingFields, perm.Attrs()...)
		}
		save(&err, tracingFields...)
	}()

	p, err = s.loadUserRepoPermissions(ctx, 0, 0, repoID)
	if err != nil {
		return nil, err
	}

	// handle unrestricted case
	for _, permission := range p {
		if permission.UserID == 0 {
			return []authz.Permission{permission}, nil
		}
	}
	return p, nil
}

// SetUserExternalAccountPerms sets the users permissions for repos in the database. Uses setUserRepoPermissions internally.
func (s *permsStore) SetUserExternalAccountPerms(ctx context.Context, user authz.UserIDWithExternalAccountID, repoIDs []int32, source authz.PermsSource) (*SetPermissionsResult, error) {
	return s.setUserExternalAccountPerms(ctx, user, repoIDs, source, true)
}

func (s *permsStore) setUserExternalAccountPerms(ctx context.Context, user authz.UserIDWithExternalAccountID, repoIDs []int32, source authz.PermsSource, replacePerms bool) (*SetPermissionsResult, error) {
	p := make([]authz.Permission, 0, len(repoIDs))

	for _, repoID := range repoIDs {
		p = append(p, authz.Permission{
			UserID:            user.UserID,
			ExternalAccountID: user.ExternalAccountID,
			RepoID:            repoID,
		})
	}

	entity := authz.PermissionEntity{
		UserID:            user.UserID,
		ExternalAccountID: user.ExternalAccountID,
	}

	return s.setUserRepoPermissions(ctx, p, entity, source, replacePerms)
}

// SetRepoPerms sets the users that can access a repo. Uses setUserRepoPermissions internally.
func (s *permsStore) SetRepoPerms(ctx context.Context, repoID int32, userIDs []authz.UserIDWithExternalAccountID, source authz.PermsSource) (*SetPermissionsResult, error) {
	p := make([]authz.Permission, 0, len(userIDs))

	for _, user := range userIDs {
		p = append(p, authz.Permission{
			UserID:            user.UserID,
			ExternalAccountID: user.ExternalAccountID,
			RepoID:            repoID,
		})
	}

	entity := authz.PermissionEntity{
		RepoID: repoID,
	}

	return s.setUserRepoPermissions(ctx, p, entity, source, true)
}

// setUserRepoPermissions performs a full update for p, new rows for pairs of user_id, repo_id
// found in p will be upserted and pairs of user_id, repo_id no longer in p will be removed.
// This method updates both `user_repo_permissions` table.
//
// Example input:
//
//	p := []UserPermissions{{
//		UserID: 1,
//		RepoID: 1,
//	 ExternalAccountID: 42,
//	}, {
//
//		UserID: 1,
//		RepoID: 233,
//	 ExternalAccountID: 42,
//	}}
//
// isUserSync := true
//
// Original table state:
//
//	 user_id | repo_id | user_external_account_id |           created_at |           updated_at | source
//	---------+------------+-------------+-----------------+------------+-----------------------
//	       1 |       1 |             42 | 2022-06-01T10:42:53Z | 2023-01-27T06:12:33Z | 'sync'
//	       1 |       2 |             42 | 2022-06-01T10:42:53Z | 2023-01-27T09:15:06Z | 'sync'
//
// New table state:
//
//	 user_id | repo_id | user_external_account_id |           created_at |           updated_at | source
//	---------+------------+-------------+-----------------+------------+-----------------------
//	       1 |       1 |             42 |          <Unchanged> | 2023-01-28T14:24:12Z | 'sync'
//	       1 |     233 |             42 | 2023-01-28T14:24:15Z | 2023-01-28T14:24:12Z | 'sync'
//
// So one repo {id:2} was removed and one was added {id:233} to the user
func (s *permsStore) setUserRepoPermissions(ctx context.Context, p []authz.Permission, entity authz.PermissionEntity, source authz.PermsSource, replacePerms bool) (_ *SetPermissionsResult, err error) {
	ctx, save := s.observe(ctx, "setUserRepoPermissions")
	defer func() {
		f := []attribute.KeyValue{}
		for _, permission := range p {
			f = append(f, permission.Attrs()...)
		}
		save(&err, f...)
	}()

	// Open a transaction for update consistency.
	txs, err := s.transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = txs.Done(err) }()

	currentTime := time.Now()
	var updates []bool
	if len(p) > 0 {
		// Update the rows with new data
		updates, err = txs.upsertUserRepoPermissions(ctx, p, currentTime, source)
		if err != nil {
			return nil, errors.Wrap(err, "upserting new user repo permissions")
		}
	}

	deleted := []int64{}
	if replacePerms {
		// Now delete rows that were updated before. This will delete all rows, that were not updated on the last update
		// which was tried above.
		deleted, err = txs.deleteOldUserRepoPermissions(ctx, entity, currentTime, source)
		if err != nil {
			return nil, errors.Wrap(err, "removing old user repo permissions")
		}
	}

	// count the number of added permissions
	added := 0
	for _, isNew := range updates {
		if isNew {
			added++
		}
	}

	return &SetPermissionsResult{
		Added:   added,
		Removed: len(deleted),
		Found:   len(p),
	}, nil
}

// upsertUserRepoPermissions upserts multiple rows of permissions. It also updates the updated_at and source
// columns for all the rows that match the permissions input parameter.
// We rely on the caller to call this method in a transaction.
func (s *permsStore) upsertUserRepoPermissions(ctx context.Context, permissions []authz.Permission, currentTime time.Time, source authz.PermsSource) ([]bool, error) {
	const format = `
INSERT INTO user_repo_permissions
	(user_id, user_external_account_id, repo_id, created_at, updated_at, source)
VALUES
	%s
ON CONFLICT (user_id, user_external_account_id, repo_id)
DO UPDATE SET
	updated_at = excluded.updated_at,
	source = excluded.source
RETURNING (created_at = updated_at) AS is_new_row;
`

	if !s.InTransaction() {
		return nil, errors.New("upsertUserRepoPermissions must be called in a transaction")
	}

	// we split into chunks, so that we don't exceed the maximum number of parameters
	// we supply 6 parameters per row, so we can only have 65535/6 = 10922 rows per chunk
	slicedPermissions, err := collections.SplitIntoChunks(permissions, 65535/6)
	if err != nil {
		return nil, err
	}

	output := make([]bool, 0, len(permissions))
	for _, permissionSlice := range slicedPermissions {
		values := make([]*sqlf.Query, 0, len(permissionSlice))
		for _, p := range permissionSlice {
			values = append(values, sqlf.Sprintf("(NULLIF(%s::integer, 0), NULLIF(%s::integer, 0), %s::integer, %s::timestamptz, %s::timestamptz, %s::text)",
				p.UserID,
				p.ExternalAccountID,
				p.RepoID,
				currentTime,
				currentTime,
				source,
			))
		}

		q := sqlf.Sprintf(format, sqlf.Join(values, ","))

		rows, err := basestore.ScanBools(s.Query(ctx, q))
		if err != nil {
			return nil, err
		}
		output = append(output, rows...)
	}
	return output, nil
}

// deleteOldUserRepoPermissions deletes multiple rows of permissions. It also updates the updated_at and source
// columns for all the rows that match the permissions input parameter
func (s *permsStore) deleteOldUserRepoPermissions(ctx context.Context, entity authz.PermissionEntity, currentTime time.Time, source authz.PermsSource) ([]int64, error) {
	const format = `
DELETE FROM user_repo_permissions
WHERE
	%s
	AND
	updated_at != %s
	AND %s
	RETURNING id
`
	whereSource := sqlf.Sprintf("source != %s", authz.SourceAPI)
	if source == authz.SourceAPI {
		whereSource = sqlf.Sprintf("source = %s", authz.SourceAPI)
	}

	var where *sqlf.Query
	if entity.UserID > 0 {
		where = sqlf.Sprintf("user_id = %d", entity.UserID)
		if entity.ExternalAccountID > 0 {
			where = sqlf.Sprintf("%s AND user_external_account_id = %d", where, entity.ExternalAccountID)
		}
	} else if entity.RepoID > 0 {
		where = sqlf.Sprintf("repo_id = %d", entity.RepoID)
	} else {
		return nil, errors.New("invalid entity for which to delete old permissions, need at least RepoID or UserID specified")
	}

	q := sqlf.Sprintf(format, where, currentTime, whereSource)
	return basestore.ScanInt64s(s.Query(ctx, q))
}

// upsertUserPermissionsBatchQuery composes a SQL query that does both addition (for `addedUserIDs`) and deletion (
// for `removedUserIDs`) of `objectIDs` using upsert.
func upsertUserPermissionsBatchQuery(addedUserIDs, removedUserIDs, objectIDs []int32, perm authz.Perms, permType authz.PermType, updatedAt time.Time) (*sqlf.Query, error) {
	const format = `
INSERT INTO user_permissions
	(user_id, permission, object_type, object_ids_ints, updated_at)
VALUES
	%s
ON CONFLICT ON CONSTRAINT
	user_permissions_perm_object_unique
DO UPDATE SET
	object_ids_ints = CASE
		-- When the user is part of "addedUserIDs"
		WHEN user_permissions.user_id = ANY (%s) THEN
			user_permissions.object_ids_ints | excluded.object_ids_ints
		ELSE
			user_permissions.object_ids_ints - %s::INT[]
		END,
	updated_at = excluded.updated_at
`
	if updatedAt.IsZero() {
		return nil, ErrPermsUpdatedAtNotSet
	}

	items := make([]*sqlf.Query, 0, len(addedUserIDs)+len(removedUserIDs))
	for _, userID := range addedUserIDs {
		items = append(items, sqlf.Sprintf("(%s, %s, %s, %s, %s)",
			userID,
			perm.String(),
			permType,
			pq.Array(objectIDs),
			updatedAt.UTC(),
		))
	}
	for _, userID := range removedUserIDs {
		items = append(items, sqlf.Sprintf("(%s, %s, %s, %s, %s)",
			userID,
			perm.String(),
			permType,

			// NOTE: Rows from `removedUserIDs` are expected to exist, but in case they do not,
			// we need to set it with empty object IDs to be safe (it's a removal anyway).
			pq.Array([]int32{}),

			updatedAt.UTC(),
		))
	}

	return sqlf.Sprintf(
		format,
		sqlf.Join(items, ","),
		pq.Array(addedUserIDs),

		// NOTE: Because we use empty object IDs for `removedUserIDs`, we can't reuse "excluded.object_ids_ints"
		// and have to explicitly set what object IDs to be removed.
		pq.Array(objectIDs),
	), nil
}

func (s *permsStore) legacySetRepoPermissionsUnrestricted(ctx context.Context, ids []int32, unrestricted bool) error {
	if len(ids) == 0 {
		return nil
	}

	const format = `
INSERT INTO repo_permissions
  (repo_id, permission, user_ids_ints, updated_at, synced_at, unrestricted)
SELECT unnest(%s::int[]), 'read', '{}'::int[], NOW(), NOW(), %s
ON CONFLICT ON CONSTRAINT
  repo_permissions_perm_unique
DO UPDATE SET
   updated_at = NOW(),
   unrestricted = %s;
`

	size := 65535 / 2 // 65535 is the max number of parameters in a query, 2 is the number of parameters in each row
	chunks, err := collections.SplitIntoChunks(ids, size)
	if err != nil {
		return err
	}

	for _, chunk := range chunks {
		q := sqlf.Sprintf(format, pq.Array(chunk), unrestricted, unrestricted)
		err := s.Exec(ctx, q)
		if err != nil {
			return errors.Wrap(err, "setting unrestricted flag")
		}
	}

	return nil
}

func (s *permsStore) SetRepoPermissionsUnrestricted(ctx context.Context, ids []int32, unrestricted bool) error {
	var txs *permsStore
	var err error
	if s.InTransaction() {
		txs = s
	} else {
		txs, err = s.transact(ctx)
		if err != nil {
			return err
		}
		defer func() { err = txs.Done(err) }()
	}

	if len(ids) == 0 {
		return nil
	}

	err = txs.legacySetRepoPermissionsUnrestricted(ctx, ids, unrestricted)
	if err != nil {
		return err
	}

	if !unrestricted {
		return txs.unsetRepoPermissionsUnrestricted(ctx, ids)
	}
	return txs.setRepoPermissionsUnrestricted(ctx, ids)
}

func (s *permsStore) unsetRepoPermissionsUnrestricted(ctx context.Context, ids []int32) error {
	format := `DELETE FROM user_repo_permissions WHERE repo_id = ANY(%s) AND user_id IS NULL;`
	size := 65535 - 1 // for unsetting unrestricted, we have only 1 parameter per row
	chunks, err := collections.SplitIntoChunks(ids, size)
	if err != nil {
		return err
	}
	for _, chunk := range chunks {
		err := s.Exec(ctx, sqlf.Sprintf(format, pq.Array(chunk)))
		if err != nil {
			return errors.Wrap(err, "removing unrestricted flag")
		}
	}

	return nil
}

func (s *permsStore) setRepoPermissionsUnrestricted(ctx context.Context, ids []int32) error {
	currentTime := time.Now()
	values := make([]*sqlf.Query, 0, len(ids))
	for _, repoID := range ids {
		values = append(values, sqlf.Sprintf("(NULL, %d, %s, %s, %s)", repoID, currentTime, currentTime, authz.SourceAPI))
	}

	format := `
INSERT INTO user_repo_permissions (user_id, repo_id, created_at, updated_at, source)
VALUES %s
ON CONFLICT DO NOTHING;
`

	size := 65535 / 4 // 65535 is the max number of parameters in a query, 4 is the number of parameters in each row
	chunks, err := collections.SplitIntoChunks(values, size)
	if err != nil {
		return err
	}

	for _, chunk := range chunks {
		err = s.Exec(ctx, sqlf.Sprintf(format, sqlf.Join(chunk, ",")))
		if err != nil {
			errors.Wrapf(err, "setting repositories as unrestricted %v", chunk)
		}
	}

	return nil
}

// upsertRepoPendingPermissionsQuery
func upsertRepoPendingPermissionsQuery(p *authz.RepoPermissions) (*sqlf.Query, error) {
	const format = `
INSERT INTO repo_pending_permissions
  (repo_id, permission, user_ids_ints, updated_at)
VALUES
  (%s, %s, %s, %s)
ON CONFLICT ON CONSTRAINT
  repo_pending_permissions_perm_unique
DO UPDATE SET
  user_ids_ints = excluded.user_ids_ints,
  updated_at = excluded.updated_at
`

	if p.UpdatedAt.IsZero() {
		return nil, ErrPermsUpdatedAtNotSet
	}

	userIDs := make([]int64, 0, len(p.PendingUserIDs))
	for key := range p.PendingUserIDs {
		userIDs = append(userIDs, key)
	}
	return sqlf.Sprintf(
		format,
		p.RepoID,
		p.Perm.String(),
		pq.Array(userIDs),
		p.UpdatedAt.UTC(),
	), nil
}

func (s *permsStore) LoadUserPendingPermissions(ctx context.Context, p *authz.UserPendingPermissions) (err error) {
	ctx, save := s.observe(ctx, "LoadUserPendingPermissions")
	defer func() { save(&err, p.Attrs()...) }()

	id, ids, updatedAt, err := s.loadUserPendingPermissions(ctx, p, "")
	if err != nil {
		return err
	}
	p.ID = id
	p.IDs = collections.NewSet(ids...)

	p.UpdatedAt = updatedAt
	return nil
}

func (s *permsStore) SetRepoPendingPermissions(ctx context.Context, accounts *extsvc.Accounts, p *authz.RepoPermissions) (err error) {
	ctx, save := s.observe(ctx, "SetRepoPendingPermissions")
	defer func() { save(&err, append(p.Attrs(), accounts.TracingFields()...)...) }()

	var txs *permsStore
	if s.InTransaction() {
		txs = s
	} else {
		txs, err = s.transact(ctx)
		if err != nil {
			return err
		}
		defer func() { err = txs.Done(err) }()
	}

	var q *sqlf.Query

	p.PendingUserIDs = collections.NewSet[int64]()
	p.UserIDs = collections.NewSet[int32]()

	// Insert rows for AccountIDs without one in the "user_pending_permissions"
	// table. The insert does not store any permission data but uses auto-increment
	// key to generate unique ID. This guarantees that rows of all AccountIDs exist
	// when getting user IDs in the next load query.
	updatedAt := txs.clock()
	p.UpdatedAt = updatedAt
	if len(accounts.AccountIDs) > 0 {
		// NOTE: The primary key of "user_pending_permissions" table is auto-incremented,
		// and it is monotonically growing even with upsert in Postgres (i.e. the primary
		// key is increased internally by one even if the row exists). This means with
		// large number of AccountIDs, the primary key will grow very quickly every time
		// we do an upsert, and not far from reaching the largest number an int8 (64-bit
		// integer) can hold (9,223,372,036,854,775,807). Therefore, load existing rows
		// would help us only upsert rows that are newly discovered. See NOTE in below
		// for why we do upsert not insert.
		q = loadExistingUserPendingPermissionsBatchQuery(accounts, p)
		bindIDsToIDs, err := txs.loadExistingUserPendingPermissionsBatch(ctx, q)
		if err != nil {
			return errors.Wrap(err, "loading existing user pending permissions")
		}

		missingAccounts := &extsvc.Accounts{
			ServiceType: accounts.ServiceType,
			ServiceID:   accounts.ServiceID,
			AccountIDs:  make([]string, 0, len(accounts.AccountIDs)-len(bindIDsToIDs)),
		}

		for _, bindID := range accounts.AccountIDs {
			id, ok := bindIDsToIDs[bindID]
			if ok {
				p.PendingUserIDs.Add(id)
			} else {
				missingAccounts.AccountIDs = append(missingAccounts.AccountIDs, bindID)
			}
		}

		// Only do upsert when there are missing accounts
		if len(missingAccounts.AccountIDs) > 0 {
			// NOTE: Row-level locking is not needed here because we're creating stub rows
			//  and not modifying permissions, which is also why it is best to use upsert not
			//  insert to avoid unique violation in case other transactions are trying to
			//  create overlapping stub rows.
			q, err = upsertUserPendingPermissionsBatchQuery(missingAccounts, p)
			if err != nil {
				return err
			}

			ids, err := txs.loadUserPendingPermissionsIDs(ctx, q)
			if err != nil {
				return errors.Wrap(err, "load user pending permissions IDs from upsert pending permissions")
			}

			// Make up p.PendingUserIDs from the result set.
			p.PendingUserIDs.Add(ids...)
		}

	}

	// Retrieve currently stored user IDs of this repository.
	_, ids, _, _, err := txs.loadRepoPendingPermissions(ctx, p, "FOR UPDATE")
	if err != nil && err != authz.ErrPermsNotFound {
		return errors.Wrap(err, "load repo pending permissions")
	}

	oldIDs := collections.NewSet(ids...)
	added, removed := computeDiff(oldIDs, p.PendingUserIDs)

	// In case there is nothing added or removed.
	if len(added) == 0 && len(removed) == 0 {
		return nil
	}

	if q, err = updateUserPendingPermissionsBatchQuery(added, removed, []int32{p.RepoID}, p.Perm, authz.PermRepos, updatedAt); err != nil {
		return err
	} else if err = txs.execute(ctx, q); err != nil {
		return errors.Wrap(err, "execute append user pending permissions batch query")
	}

	if q, err = upsertRepoPendingPermissionsQuery(p); err != nil {
		return err
	} else if err = txs.execute(ctx, q); err != nil {
		return errors.Wrap(err, "execute upsert repo pending permissions query")
	}

	return nil
}

func (s *permsStore) loadUserPendingPermissionsIDs(ctx context.Context, q *sqlf.Query) (ids []int64, err error) {
	ctx, save := s.observe(ctx, "loadUserPendingPermissionsIDs")
	defer func() {
		save(&err,
			attribute.String("Query.Query", q.Query(sqlf.PostgresBindVar)),
			attribute.String("Query.Args", fmt.Sprintf("%q", q.Args())),
		)
	}()

	rows, err := s.Query(ctx, q)
	return basestore.ScanInt64s(rows, err)
}

var scanBindIDs = basestore.NewMapScanner(func(s dbutil.Scanner) (bindID string, id int64, _ error) {
	err := s.Scan(&bindID, &id)
	return bindID, id, err
})

func (s *permsStore) loadExistingUserPendingPermissionsBatch(ctx context.Context, q *sqlf.Query) (bindIDsToIDs map[string]int64, err error) {
	ctx, save := s.observe(ctx, "loadExistingUserPendingPermissionsBatch")
	defer func() {
		save(&err,
			attribute.String("Query.Query", q.Query(sqlf.PostgresBindVar)),
			attribute.String("Query.Args", fmt.Sprintf("%q", q.Args())),
		)
	}()

	bindIDsToIDs, err = scanBindIDs(s.Query(ctx, q))
	if err != nil {
		return nil, err
	}

	return bindIDsToIDs, nil
}

// upsertUserPendingPermissionsBatchQuery generates a query for upserting the provided
// external service accounts into `user_pending_permissions`.
func upsertUserPendingPermissionsBatchQuery(
	accounts *extsvc.Accounts,
	p *authz.RepoPermissions,
) (*sqlf.Query, error) {
	// Above ~10,000 accounts (10,000 * 6 fields each = 60,000 parameters), we can run
	// into the Postgres parameter limit inserting with VALUES. Instead, we pass in fields
	// as arrays, where each array only counts for a single parameter.
	//
	// If changing the parameters used in this query, make sure to run relevant tests
	// named `postgresParameterLimitTest` using "go test -slow-tests".
	const format = `
INSERT INTO user_pending_permissions
	(service_type, service_id, bind_id, permission, object_type, updated_at)
	(
		SELECT %s::TEXT, %s::TEXT, UNNEST(%s::TEXT[]), %s::TEXT, %s::TEXT, %s::TIMESTAMPTZ
	)
ON CONFLICT ON CONSTRAINT
	user_pending_permissions_service_perm_object_unique
DO UPDATE SET
	updated_at = excluded.updated_at
RETURNING id
`
	if p.UpdatedAt.IsZero() {
		return nil, ErrPermsUpdatedAtNotSet
	}

	return sqlf.Sprintf(
		format,

		accounts.ServiceType,
		accounts.ServiceID,
		pq.Array(accounts.AccountIDs),

		p.Perm.String(),
		string(authz.PermRepos),
		p.UpdatedAt.UTC(),
	), nil
}

func loadExistingUserPendingPermissionsBatchQuery(accounts *extsvc.Accounts, p *authz.RepoPermissions) *sqlf.Query {
	const format = `
SELECT bind_id, id FROM user_pending_permissions
WHERE
	service_type = %s
AND service_id = %s
AND permission = %s
AND object_type = %s
AND bind_id IN (%s)
`

	bindIDs := make([]*sqlf.Query, len(accounts.AccountIDs))
	for i := range accounts.AccountIDs {
		bindIDs[i] = sqlf.Sprintf("%s", accounts.AccountIDs[i])
	}

	return sqlf.Sprintf(
		format,
		accounts.ServiceType,
		accounts.ServiceID,
		p.Perm.String(),
		authz.PermRepos,
		sqlf.Join(bindIDs, ","),
	)
}

// updateUserPendingPermissionsBatchQuery composes a SQL query that does both addition (for `addedUserIDs`) and deletion (
// for `removedUserIDs`) of `objectIDs` using update.
func updateUserPendingPermissionsBatchQuery(addedUserIDs, removedUserIDs []int64, objectIDs []int32, perm authz.Perms, permType authz.PermType, updatedAt time.Time) (*sqlf.Query, error) {
	const format = `
UPDATE user_pending_permissions
SET
	object_ids_ints = CASE
		-- When the user is part of "addedUserIDs"
		WHEN user_pending_permissions.id = ANY (%s) THEN
			user_pending_permissions.object_ids_ints | %s
		ELSE
			user_pending_permissions.object_ids_ints - %s
		END,
	updated_at = %s
WHERE
	id = ANY (%s)
AND permission = %s
AND object_type = %s
`
	if updatedAt.IsZero() {
		return nil, ErrPermsUpdatedAtNotSet
	}

	return sqlf.Sprintf(
		format,
		pq.Array(addedUserIDs),
		pq.Array(objectIDs),
		pq.Array(objectIDs),
		updatedAt.UTC(),
		pq.Array(append(addedUserIDs, removedUserIDs...)),
		perm.String(),
		permType,
	), nil
}

func (s *permsStore) GrantPendingPermissions(ctx context.Context, p *authz.UserGrantPermissions) (err error) {
	ctx, save := s.observe(ctx, "GrantPendingPermissions")
	defer func() { save(&err, p.Attrs()...) }()

	var txs *permsStore
	if s.InTransaction() {
		txs = s
	} else {
		txs, err = s.transact(ctx)
		if err != nil {
			return err
		}
		defer func() { err = txs.Done(err) }()
	}

	pendingPermissions := &authz.UserPendingPermissions{
		ServiceID:   p.ServiceID,
		ServiceType: p.ServiceType,
		BindID:      p.AccountID,
		Perm:        authz.Read,
		Type:        authz.PermRepos,
	}

	id, ids, _, err := txs.loadUserPendingPermissions(ctx, pendingPermissions, "FOR UPDATE")
	if err != nil {
		// Skip the whole grant process if the user has no pending permissions.
		if err == authz.ErrPermsNotFound {
			return nil
		}
		return errors.Wrap(err, "load user pending permissions")
	}

	uniqueRepoIDs := collections.NewSet(ids...)
	allRepoIDs := uniqueRepoIDs.Values()

	// Write to the unified user_repo_permissions table.
	_, err = txs.setUserExternalAccountPerms(ctx, authz.UserIDWithExternalAccountID{UserID: p.UserID, ExternalAccountID: p.UserExternalAccountID}, allRepoIDs, authz.SourceUserSync, false)
	if err != nil {
		return err
	}

	if len(allRepoIDs) == 0 {
		return nil
	}

	var (
		updatedAt  = txs.clock()
		allUserIDs = []int32{p.UserID}

		addQueue    = allRepoIDs
		hasNextPage = true
	)
	for hasNextPage {
		var page *upsertRepoPermissionsPage
		page, addQueue, _, hasNextPage = newUpsertRepoPermissionsPage(addQueue, nil)

		if q, err := upsertRepoPermissionsBatchQuery(page, allRepoIDs, allUserIDs, authz.Read, updatedAt); err != nil {
			return err
		} else if err = txs.execute(ctx, q); err != nil {
			return errors.Wrap(err, "execute upsert repo permissions batch query")
		}
	}

	if q, err := upsertUserPermissionsBatchQuery(allUserIDs, nil, allRepoIDs, authz.Read, authz.PermRepos, updatedAt); err != nil {
		return err
	} else if err = txs.execute(ctx, q); err != nil {
		return errors.Wrap(err, "execute upsert user permissions batch query")
	}

	pendingPermissions.ID = id
	pendingPermissions.IDs = uniqueRepoIDs

	// NOTE: Practically, we don't need to clean up "repo_pending_permissions" table because the value of "id" column
	// that is associated with this user will be invalidated automatically by deleting this row. Thus, we are able to
	// avoid database deadlocks with other methods (e.g. SetRepoPermissions, SetRepoPendingPermissions).
	if err = txs.execute(ctx, deleteUserPendingPermissionsQuery(pendingPermissions)); err != nil {
		return errors.Wrap(err, "execute delete user pending permissions query")
	}

	return nil
}

// upsertRepoPermissionsPage tracks entries to upsert in a upsertRepoPermissionsBatchQuery.
type upsertRepoPermissionsPage struct {
	addedRepoIDs   []int32
	removedRepoIDs []int32
}

// upsertRepoPermissionsPageSize restricts page size for newUpsertRepoPermissionsPage to
// stay within the Postgres parameter limit (see `defaultUpsertRepoPermissionsPageSize`).
//
// May be modified for testing.
var upsertRepoPermissionsPageSize = defaultUpsertRepoPermissionsPageSize

// defaultUpsertRepoPermissionsPageSize sets a default for upsertRepoPermissionsPageSize.
//
// Value set to avoid parameter limit of ~65k, because each page element counts for 4
// parameters (65k / 4 ~= 16k rows at a time)
const defaultUpsertRepoPermissionsPageSize = 15000

// newUpsertRepoPermissionsPage instantiates a page from the given add/remove queues.
// Callers should reassign their queues to the ones returned by this constructor.
func newUpsertRepoPermissionsPage(addQueue, removeQueue []int32) (
	page *upsertRepoPermissionsPage,
	newAddQueue, newRemoveQueue []int32,
	hasNextPage bool,
) {
	quota := upsertRepoPermissionsPageSize
	page = &upsertRepoPermissionsPage{}

	if len(addQueue) > 0 {
		if len(addQueue) < quota {
			page.addedRepoIDs = addQueue
			addQueue = nil
		} else {
			page.addedRepoIDs = addQueue[:quota]
			addQueue = addQueue[quota:]
		}
		quota -= len(page.addedRepoIDs)
	}

	if len(removeQueue) > 0 {
		if len(removeQueue) < quota {
			page.removedRepoIDs = removeQueue
			removeQueue = nil
		} else {
			page.removedRepoIDs = removeQueue[:quota]
			removeQueue = removeQueue[quota:]
		}
	}

	return page,
		addQueue,
		removeQueue,
		len(addQueue) > 0 || len(removeQueue) > 0
}

// upsertRepoPermissionsBatchQuery composes a SQL query that does both addition (for `addedRepoIDs`)
// and deletion (for `removedRepoIDs`) of `userIDs` using upsert.
//
// Pages should be set up using the helper function `newUpsertRepoPermissionsPage`
func upsertRepoPermissionsBatchQuery(page *upsertRepoPermissionsPage, allAddedRepoIDs, userIDs []int32, perm authz.Perms, updatedAt time.Time) (*sqlf.Query, error) {
	// If changing the parameters used in this query, make sure to run relevant tests
	// named `postgresParameterLimitTest` using "go test -slow-tests".
	const format = `
INSERT INTO repo_permissions
	(repo_id, permission, user_ids_ints, updated_at)
VALUES
	%s
ON CONFLICT ON CONSTRAINT
	repo_permissions_perm_unique
DO UPDATE SET
	user_ids_ints = CASE
		-- When the repository is part of "addedRepoIDs"
		WHEN repo_permissions.repo_id = ANY (%s) THEN
			repo_permissions.user_ids_ints | excluded.user_ids_ints
		ELSE
			repo_permissions.user_ids_ints - %s::INT[]
		END,
	updated_at = excluded.updated_at
`
	if updatedAt.IsZero() {
		return nil, ErrPermsUpdatedAtNotSet
	}

	items := make([]*sqlf.Query, 0, len(page.addedRepoIDs)+len(page.removedRepoIDs))
	for _, repoID := range page.addedRepoIDs {
		items = append(items, sqlf.Sprintf("(%s, %s, %s, %s)",
			repoID,
			perm.String(),
			pq.Array(userIDs),
			updatedAt.UTC(),
		))
	}
	for _, repoID := range page.removedRepoIDs {
		items = append(items, sqlf.Sprintf("(%s, %s, %s, %s)",
			repoID,
			perm.String(),

			// NOTE: Rows from `removedRepoIDs` are expected to exist, but in case they do not,
			// we need to set it with empty user IDs to be safe (it's a removal anyway).
			pq.Array([]int32{}),

			updatedAt.UTC(),
		))
	}

	return sqlf.Sprintf(
		format,
		sqlf.Join(items, ","),
		pq.Array(allAddedRepoIDs),

		// NOTE: Because we use empty user IDs for `removedRepoIDs`, we can't reuse "excluded.user_ids_ints"
		// and have to explicitly set what user IDs to be removed.
		pq.Array(userIDs),
	), nil
}

func deleteUserPendingPermissionsQuery(p *authz.UserPendingPermissions) *sqlf.Query {
	const format = `
DELETE FROM user_pending_permissions
WHERE service_type = %s
AND service_id = %s
AND permission = %s
AND object_type = %s
AND bind_id = %s
`

	return sqlf.Sprintf(
		format,
		p.ServiceType,
		p.ServiceID,
		p.Perm.String(),
		p.Type,
		p.BindID,
	)
}

func (s *permsStore) ListPendingUsers(ctx context.Context, serviceType, serviceID string) (bindIDs []string, err error) {
	ctx, save := s.observe(ctx, "ListPendingUsers")
	defer save(&err)

	q := sqlf.Sprintf(`
SELECT bind_id, object_ids_ints
FROM user_pending_permissions
WHERE service_type = %s
AND service_id = %s
`, serviceType, serviceID)

	var rows *sql.Rows
	rows, err = s.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var bindID string
		var ids []int64
		if err = rows.Scan(&bindID, pq.Array(&ids)); err != nil {
			return nil, err
		}

		// This user has no pending permissions, only has an empty record
		if len(ids) == 0 {
			continue
		}
		bindIDs = append(bindIDs, bindID)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return bindIDs, nil
}

func (s *permsStore) DeleteAllUserPermissions(ctx context.Context, userID int32) (err error) {
	ctx, save := s.observe(ctx, "DeleteAllUserPermissions")

	var txs *permsStore
	if s.InTransaction() {
		txs = s
	} else {
		txs, err = s.transact(ctx)
		if err != nil {
			return err
		}
		defer func() { err = txs.Done(err) }()
	}

	defer func() { save(&err, attribute.Int("userID", int(userID))) }()

	// first delete from the unified table
	if err = txs.execute(ctx, sqlf.Sprintf(`DELETE FROM user_repo_permissions WHERE user_id = %d`, userID)); err != nil {
		return errors.Wrap(err, "execute delete user repo permissions query")
	}
	// NOTE: Practically, we don't need to clean up "repo_permissions" table because the value of "id" column
	// that is associated with this user will be invalidated automatically by deleting this row.
	if err = txs.execute(ctx, sqlf.Sprintf(`DELETE FROM user_permissions WHERE user_id = %s`, userID)); err != nil {
		return errors.Wrap(err, "execute delete user permissions query")
	}

	return nil
}

func (s *permsStore) DeleteAllUserPendingPermissions(ctx context.Context, accounts *extsvc.Accounts) (err error) {
	ctx, save := s.observe(ctx, "DeleteAllUserPendingPermissions")
	defer func() { save(&err, accounts.TracingFields()...) }()

	// NOTE: Practically, we don't need to clean up "repo_pending_permissions" table because the value of "id" column
	// that is associated with this user will be invalidated automatically by deleting this row.
	items := make([]*sqlf.Query, len(accounts.AccountIDs))
	for i := range accounts.AccountIDs {
		items[i] = sqlf.Sprintf("%s", accounts.AccountIDs[i])
	}
	q := sqlf.Sprintf(`
DELETE FROM user_pending_permissions
WHERE service_type = %s
AND service_id = %s
AND bind_id IN (%s)`,
		accounts.ServiceType, accounts.ServiceID, sqlf.Join(items, ","))
	if err = s.execute(ctx, q); err != nil {
		return errors.Wrap(err, "execute delete user pending permissions query")
	}

	return nil
}

func (s *permsStore) execute(ctx context.Context, q *sqlf.Query, vs ...any) (err error) {
	ctx, save := s.observe(ctx, "execute")
	defer func() { save(&err, attribute.String("q", q.Query(sqlf.PostgresBindVar))) }()

	var rows *sql.Rows
	rows, err = s.Query(ctx, q)
	if err != nil {
		return err
	}

	if len(vs) > 0 {
		if !rows.Next() {
			// One row is expected, return ErrPermsNotFound if no other errors occurred.
			err = rows.Err()
			if err == nil {
				err = authz.ErrPermsNotFound
			}
			return err
		}

		if err = rows.Scan(vs...); err != nil {
			return err
		}
	}

	return rows.Close()
}

var ScanPermissions = basestore.NewSliceScanner(func(s dbutil.Scanner) (authz.Permission, error) {
	p := authz.Permission{}
	err := s.Scan(&dbutil.NullInt32{N: &p.UserID}, &dbutil.NullInt32{N: &p.ExternalAccountID}, &p.RepoID, &p.CreatedAt, &p.UpdatedAt, &p.Source)
	return p, err
})

func (s *permsStore) loadUserRepoPermissions(ctx context.Context, userID, userExternalAccountID, repoID int32) ([]authz.Permission, error) {
	clauses := []*sqlf.Query{sqlf.Sprintf("TRUE")}

	if userID != 0 {
		clauses = append(clauses, sqlf.Sprintf("user_id = %d", userID))
	}
	if userExternalAccountID != 0 {
		clauses = append(clauses, sqlf.Sprintf("user_external_account_id = %d", userExternalAccountID))
	}
	if repoID != 0 {
		clauses = append(clauses, sqlf.Sprintf("repo_id = %d", repoID))
	}

	query := sqlf.Sprintf(`
SELECT user_id, user_external_account_id, repo_id, created_at, updated_at, source
FROM user_repo_permissions
WHERE %s
`, sqlf.Join(clauses, " AND "))
	return ScanPermissions(s.Query(ctx, query))
}

// loadUserPendingPermissions is a method that scans three values from one user_pending_permissions table row:
// int64 (id), []int32 (ids), time.Time (updatedAt).
func (s *permsStore) loadUserPendingPermissions(ctx context.Context, p *authz.UserPendingPermissions, lock string) (id int64, ids []int32, updatedAt time.Time, err error) {
	const format = `
SELECT id, object_ids_ints, updated_at
FROM user_pending_permissions
WHERE service_type = %s
AND service_id = %s
AND permission = %s
AND object_type = %s
AND bind_id = %s
`
	q := sqlf.Sprintf(
		format+lock,
		p.ServiceType,
		p.ServiceID,
		p.Perm.String(),
		p.Type,
		p.BindID,
	)
	ctx, save := s.observe(ctx, "load")
	defer func() {
		save(&err,
			attribute.String("Query.Query", q.Query(sqlf.PostgresBindVar)),
			attribute.String("Query.Args", fmt.Sprintf("%q", q.Args())),
		)
	}()
	var rows *sql.Rows
	rows, err = s.Query(ctx, q)
	if err != nil {
		return -1, nil, time.Time{}, err
	}

	if !rows.Next() {
		// One row is expected, return ErrPermsNotFound if no other errors occurred.
		err = rows.Err()
		if err == nil {
			err = authz.ErrPermsNotFound
		}
		return -1, nil, time.Time{}, err
	}

	if err = rows.Scan(&id, pq.Array(&ids), &updatedAt); err != nil {
		return -1, nil, time.Time{}, err
	}
	if err = rows.Close(); err != nil {
		return -1, nil, time.Time{}, err
	}

	return id, ids, updatedAt, nil
}

// loadRepoPendingPermissions is a method that scans three values from one repo_pending_permissions table row:
// int32 (id), []int64 (ids), time.Time (updatedAt) and nullable time.Time (syncedAt).
func (s *permsStore) loadRepoPendingPermissions(ctx context.Context, p *authz.RepoPermissions, lock string) (id int32, ids []int64, updatedAt, syncedAt time.Time, err error) {
	const format = `
SELECT repo_id, user_ids_ints, updated_at, NULL
FROM repo_pending_permissions
WHERE repo_id = %s
AND permission = %s
`
	q := sqlf.Sprintf(
		format+lock,
		p.RepoID,
		p.Perm.String(),
	)
	ctx, save := s.observe(ctx, "load")
	defer func() {
		save(&err,
			attribute.String("Query.Query", q.Query(sqlf.PostgresBindVar)),
			attribute.String("Query.Args", fmt.Sprintf("%q", q.Args())),
		)
	}()
	var rows *sql.Rows
	rows, err = s.Query(ctx, q)
	if err != nil {
		return -1, nil, time.Time{}, time.Time{}, err
	}

	if !rows.Next() {
		// One row is expected, return ErrPermsNotFound if no other errors occurred.
		err = rows.Err()
		if err == nil {
			err = authz.ErrPermsNotFound
		}
		return -1, nil, time.Time{}, time.Time{}, err
	}

	if err = rows.Scan(&id, pq.Array(&ids), &updatedAt, &dbutil.NullTime{Time: &syncedAt}); err != nil {
		return -1, nil, time.Time{}, time.Time{}, err
	}
	if err = rows.Close(); err != nil {
		return -1, nil, time.Time{}, time.Time{}, err
	}

	return id, ids, updatedAt, syncedAt, nil
}

var scanUserIDsByExternalAccounts = basestore.NewMapScanner(func(s dbutil.Scanner) (accountID string, user authz.UserIDWithExternalAccountID, _ error) {
	err := s.Scan(&user.ExternalAccountID, &user.UserID, &accountID)
	return accountID, user, err
})

func (s *permsStore) GetUserIDsByExternalAccounts(ctx context.Context, accounts *extsvc.Accounts) (_ map[string]authz.UserIDWithExternalAccountID, err error) {
	ctx, save := s.observe(ctx, "ListUsersByExternalAccounts")
	defer func() { save(&err, accounts.TracingFields()...) }()

	items := make([]*sqlf.Query, len(accounts.AccountIDs))
	for i := range accounts.AccountIDs {
		items[i] = sqlf.Sprintf("%s", accounts.AccountIDs[i])
	}

	q := sqlf.Sprintf(`
SELECT id, user_id, account_id
FROM user_external_accounts
WHERE service_type = %s
AND service_id = %s
AND account_id IN (%s)
AND deleted_at IS NULL
AND expired_at IS NULL
`, accounts.ServiceType, accounts.ServiceID, sqlf.Join(items, ","))
	userIDs, err := scanUserIDsByExternalAccounts(s.Query(ctx, q))
	if err != nil {
		return nil, err
	}

	return userIDs, nil
}

// NOTE(naman): `countUsersWithNoPermsQuery` is different from `userIDsWithNoPermsQuery`
// as it only considers user_repo_permissions table to filter out users with permissions.
// Whereas the `userIDsWithNoPermsQuery` also filter out users who has any record of previous
// permissions sync job.
const countUsersWithNoPermsQuery = `
-- Filter out users with permissions
WITH users_having_permissions AS (SELECT DISTINCT user_id FROM user_repo_permissions)

SELECT COUNT(users.id)
FROM users
	LEFT OUTER JOIN users_having_permissions ON users_having_permissions.user_id = users.id
WHERE
	users.deleted_at IS NULL
	AND %s
	AND users_having_permissions.user_id IS NULL
`

func (s *permsStore) CountUsersWithNoPerms(ctx context.Context) (int, error) {
	// By default, site admins can access any repo
	filterSiteAdmins := sqlf.Sprintf("users.site_admin = FALSE")
	// Unless we enforce it in config
	if conf.Get().AuthzEnforceForSiteAdmins {
		filterSiteAdmins = sqlf.Sprintf("TRUE")
	}

	query := countUsersWithNoPermsQuery
	q := sqlf.Sprintf(query, filterSiteAdmins)
	return basestore.ScanInt(s.QueryRow(ctx, q))
}

// NOTE(naman): `countReposWithNoPermsQuery` is different from `repoIDsWithNoPermsQuery`
// as it only considers user_repo_permissions table to filter out users with permissions.
// Whereas the `repoIDsWithNoPermsQuery` also filter out users who has any record of previous
// permissions sync job.
const countReposWithNoPermsQuery = `
-- Filter out repos with permissions
WITH repos_with_permissions AS (SELECT DISTINCT repo_id FROM user_repo_permissions)

SELECT COUNT(repo.id)
FROM repo
	LEFT OUTER JOIN repos_with_permissions ON repos_with_permissions.repo_id = repo.id
WHERE
	repo.deleted_at IS NULL
	AND repo.private = TRUE
	AND repos_with_permissions.repo_id IS NULL
`

func (s *permsStore) CountReposWithNoPerms(ctx context.Context) (int, error) {
	query := countReposWithNoPermsQuery
	return basestore.ScanInt(s.QueryRow(ctx, sqlf.Sprintf(query)))
}

const countUsersWithStalePermsQuery = `
WITH us AS (
	SELECT DISTINCT ON(user_id) user_id, finished_at FROM permission_sync_jobs
	INNER JOIN users ON users.id = user_id AND users.deleted_at IS NULL
		WHERE user_id IS NOT NULL
	ORDER BY user_id ASC, finished_at DESC
)
SELECT COUNT(user_id) FROM us
WHERE %s
`

// CountUsersWithStalePerms lists the users with the oldest synced perms, limited
// to limit. If age is non-zero, users that have synced within "age" since now
// will be filtered out.
func (s *permsStore) CountUsersWithStalePerms(ctx context.Context, age time.Duration) (int, error) {
	q := sqlf.Sprintf(countUsersWithStalePermsQuery, s.getCutoffClause(age))
	return basestore.ScanInt(s.QueryRow(ctx, q))
}

const countReposWithStalePermsQuery = `
WITH us AS (
	SELECT DISTINCT ON(repository_id) repository_id, finished_at FROM permission_sync_jobs
	INNER JOIN repo ON repo.id = repository_id AND repo.deleted_at IS NULL
		WHERE repository_id IS NOT NULL
	ORDER BY repository_id ASC, finished_at DESC
)
SELECT COUNT(repository_id) FROM us
WHERE %s
`

func (s *permsStore) CountReposWithStalePerms(ctx context.Context, age time.Duration) (int, error) {
	q := sqlf.Sprintf(countReposWithStalePermsQuery, s.getCutoffClause(age))
	return basestore.ScanInt(s.QueryRow(ctx, q))
}

// NOTE(naman): we filter out users with any kind of sync job present
// and not only a completed job because even if the present job failed,
// the user will be re-scheduled as part of `userIDsWithOldestPerms`.
const userIDsWithNoPermsQuery = `
WITH rp AS (
	-- Filter out users with permissions
	SELECT DISTINCT user_id FROM user_repo_permissions
	UNION
	-- Filter out users with sync jobs
	SELECT DISTINCT user_id FROM permission_sync_jobs WHERE user_id IS NOT NULL
)
SELECT users.id
FROM users
LEFT OUTER JOIN rp ON rp.user_id = users.id
WHERE
	users.deleted_at IS NULL
AND %s
AND rp.user_id IS NULL
`

func (s *permsStore) UserIDsWithNoPerms(ctx context.Context) ([]int32, error) {
	// By default, site admins can access any repo
	filterSiteAdmins := sqlf.Sprintf("users.site_admin = FALSE")
	// Unless we enforce it in config
	if conf.Get().AuthzEnforceForSiteAdmins {
		filterSiteAdmins = sqlf.Sprintf("TRUE")
	}

	query := userIDsWithNoPermsQuery

	q := sqlf.Sprintf(query, filterSiteAdmins)
	return basestore.ScanInt32s(s.Query(ctx, q))
}

// NOTE(naman): we filter out repos with any kind of sync job present
// and not only a completed job because even if the present job failed,
// the repo will be re-scheduled as part of `repoIDsWithOldestPerms`.
const repoIDsWithNoPermsQuery = `
WITH rp AS (
	-- Filter out repos with permissions
	SELECT DISTINCT perms.repo_id FROM user_repo_permissions AS perms
	UNION
	-- Filter out repos with sync jobs
	SELECT DISTINCT syncs.repository_id AS repo_id FROM permission_sync_jobs AS syncs
		WHERE syncs.repository_id IS NOT NULL
)
SELECT r.id
FROM repo AS r
LEFT OUTER JOIN rp ON rp.repo_id = r.id
WHERE r.deleted_at IS NULL
AND r.private = TRUE
AND rp.repo_id IS NULL
`

func (s *permsStore) RepoIDsWithNoPerms(ctx context.Context) ([]api.RepoID, error) {
	return scanRepoIDs(s.Query(ctx, sqlf.Sprintf(repoIDsWithNoPermsQuery)))
}

func (s *permsStore) getCutoffClause(age time.Duration) *sqlf.Query {
	if age == 0 {
		return sqlf.Sprintf("TRUE")
	}
	cutoff := s.clock().Add(-1 * age)
	return sqlf.Sprintf("finished_at < %s OR finished_at IS NULL", cutoff)
}

const usersWithOldestPermsQuery = `
SELECT u.id as user_id, MAX(p.finished_at) as finished_at
FROM users u
LEFT JOIN permission_sync_jobs p ON u.id = p.user_id AND p.user_id IS NOT NULL
WHERE u.deleted_at IS NULL AND (%s)
AND NOT EXISTS (
	SELECT 1 FROM permission_sync_jobs p2
	WHERE p2.user_id = u.id AND (p2.state = 'queued' OR p2.state = 'processing')
)
GROUP BY u.id
ORDER BY finished_at ASC NULLS FIRST, user_id ASC
LIMIT %d;
`

// UserIDsWithOldestPerms lists the users with the oldest synced perms, limited
// to limit, for which there is no sync job scheduled at the moment. If age is non-zero, users that have synced within "age" since now
// will be filtered out.
func (s *permsStore) UserIDsWithOldestPerms(ctx context.Context, limit int, age time.Duration) (map[int32]time.Time, error) {
	q := sqlf.Sprintf(usersWithOldestPermsQuery, s.getCutoffClause(age), limit)
	return s.loadIDsWithTime(ctx, q)
}

const reposWithOldestPermsQuery = `
SELECT r.id as repo_id, MAX(p.finished_at) as finished_at
FROM repo r
LEFT JOIN permission_sync_jobs p ON r.id = p.repository_id AND p.repository_id IS NOT NULL
WHERE r.private AND r.deleted_at IS NULL AND (%s)
AND NOT EXISTS (
	SELECT 1 FROM permission_sync_jobs p2
	WHERE p2.repository_id = r.id AND (p2.state = 'queued' OR p2.state = 'processing')
)
GROUP BY r.id
ORDER BY finished_at ASC NULLS FIRST, repo_id ASC
LIMIT %d;
`

// ReposIDsWithOldestPerms lists the repositories with the oldest synced perms, limited
// to limit, for which there is no sync job scheduled at the moment. If age is non-zero, repos that have synced within "age" since now
// will be filtered out.
func (s *permsStore) ReposIDsWithOldestPerms(ctx context.Context, limit int, age time.Duration) (map[api.RepoID]time.Time, error) {
	q := sqlf.Sprintf(reposWithOldestPermsQuery, s.getCutoffClause(age), limit)

	pairs, err := s.loadIDsWithTime(ctx, q)
	if err != nil {
		return nil, err
	}

	// convert the map[int32]time.Time to map[api.RepoID]time.Time
	results := make(map[api.RepoID]time.Time, len(pairs))
	for id, t := range pairs {
		results[api.RepoID(id)] = t
	}
	return results, nil
}

var scanIDsWithTime = basestore.NewMapScanner(func(s dbutil.Scanner) (int32, time.Time, error) {
	var id int32
	var t time.Time
	err := s.Scan(&id, &dbutil.NullTime{Time: &t})
	return id, t, err
})

// loadIDsWithTime runs the query and returns a list of ID and nullable time pairs.
func (s *permsStore) loadIDsWithTime(ctx context.Context, q *sqlf.Query) (map[int32]time.Time, error) {
	return scanIDsWithTime(s.Query(ctx, q))
}

// PermsMetrics contains metrics values calculated by querying the database.
type PermsMetrics struct {
	// The number of users with stale permissions.
	UsersWithStalePerms int64
	// The seconds between users with oldest and the most up-to-date permissions.
	UsersPermsGapSeconds float64
	// The number of repositories with stale permissions.
	ReposWithStalePerms int64
	// The seconds between repositories with oldest and the most up-to-date permissions.
	ReposPermsGapSeconds float64
	// The number of repositories with stale sub-repo permissions.
	SubReposWithStalePerms int64
	// The seconds between repositories with oldest and the most up-to-date sub-repo
	// permissions.
	SubReposPermsGapSeconds float64
}

func (s *permsStore) Metrics(ctx context.Context, staleDur time.Duration) (*PermsMetrics, error) {
	m := &PermsMetrics{}

	// Calculate users with outdated permissions
	stale := s.clock().Add(-1 * staleDur)
	q := sqlf.Sprintf(`
SELECT COUNT(*)
FROM (
	SELECT user_id, MAX(finished_at) AS finished_at FROM permission_sync_jobs
	INNER JOIN users ON users.id = user_id
	WHERE user_id IS NOT NULL
		AND users.deleted_at IS NULL
	GROUP BY user_id
) as up
WHERE finished_at <= %s
`, stale)
	if err := s.execute(ctx, q, &m.UsersWithStalePerms); err != nil {
		return nil, errors.Wrap(err, "users with stale perms")
	}

	// Calculate the largest time gap between user permission syncs
	q = sqlf.Sprintf(`
SELECT EXTRACT(EPOCH FROM (MAX(finished_at) - MIN(finished_at)))
FROM (
	SELECT user_id, MAX(finished_at) AS finished_at
	FROM permission_sync_jobs
	INNER JOIN users ON users.id = user_id
	WHERE users.deleted_at IS NULL AND user_id IS NOT NULL
	GROUP BY user_id
) AS up
`)
	var seconds sql.NullFloat64
	if err := s.execute(ctx, q, &seconds); err != nil {
		return nil, errors.Wrap(err, "users perms gap seconds")
	}
	m.UsersPermsGapSeconds = seconds.Float64

	// Calculate repos with outdated perms
	q = sqlf.Sprintf(`
SELECT COUNT(*)
FROM (
	SELECT repository_id, MAX(finished_at) AS finished_at FROM permission_sync_jobs
	INNER JOIN repo ON repo.id = repository_id
	WHERE repository_id IS NOT NULL
		AND repo.deleted_at IS NULL
		AND repo.private = TRUE
	GROUP BY repository_id
) AS rp
WHERE finished_at <= %s
`, stale)
	if err := s.execute(ctx, q, &m.ReposWithStalePerms); err != nil {
		return nil, errors.Wrap(err, "repos with stale perms")
	}

	// Calculate maximum time gap between repo permission syncs
	q = sqlf.Sprintf(`
SELECT EXTRACT(EPOCH FROM (MAX(finished_at) - MIN(finished_at)))
FROM (
	SELECT repository_id, MAX(finished_at) AS finished_at
	FROM permission_sync_jobs
	INNER JOIN repo ON repo.id = repository_id
	WHERE repo.deleted_at IS NULL
		AND repository_id IS NOT NULL
		AND repo.private = TRUE
	GROUP BY repository_id
) AS rp
`)
	if err := s.execute(ctx, q, &seconds); err != nil {
		return nil, errors.Wrap(err, "repos perms gap seconds")
	}
	m.ReposPermsGapSeconds = seconds.Float64

	q = sqlf.Sprintf(`
SELECT COUNT(*) FROM sub_repo_permissions AS perms
WHERE perms.repo_id IN
	(
		SELECT repo.id FROM repo
		WHERE
			repo.deleted_at IS NULL
		AND repo.private = TRUE
	)
AND perms.updated_at <= %s
`, stale)
	if err := s.execute(ctx, q, &m.SubReposWithStalePerms); err != nil {
		return nil, errors.Wrap(err, "repos with stale sub-repo perms")
	}

	q = sqlf.Sprintf(`
SELECT EXTRACT(EPOCH FROM (MAX(perms.updated_at) - MIN(perms.updated_at)))
FROM sub_repo_permissions AS perms
WHERE perms.repo_id IN
	(
		SELECT repo.id FROM repo
		WHERE
			repo.deleted_at IS NULL
		AND repo.private = TRUE
	)
`)
	if err := s.execute(ctx, q, &seconds); err != nil {
		return nil, errors.Wrap(err, "sub-repo perms gap seconds")
	}
	m.SubReposPermsGapSeconds = seconds.Float64

	return m, nil
}

func (s *permsStore) observe(ctx context.Context, family string) (context.Context, func(*error, ...attribute.KeyValue)) { //nolint:unparam // unparam complains that `title` always has same value across call-sites, but that's OK
	began := s.clock()
	tr, ctx := trace.New(ctx, "database.PermsStore."+family)

	return ctx, func(err *error, attrs ...attribute.KeyValue) {
		now := s.clock()
		took := now.Sub(began)

		attrs = append(attrs, attribute.Stringer("Duration", took))

		tr.AddEvent("finish", attrs...)

		success := err == nil || *err == nil
		if !success {
			tr.SetError(*err)
		}

		tr.End()
	}
}

// MapUsers takes a list of bind ids and a mapping configuration and maps them to the right user ids.
// It filters out empty bindIDs and only returns users that exist in the database.
// If a bind id doesn't map to any user, it is ignored.
func (s *permsStore) MapUsers(ctx context.Context, bindIDs []string, mapping *schema.PermissionsUserMapping) (map[string]int32, error) {
	// Filter out bind IDs that only contains whitespaces.
	filtered := make([]string, 0, len(bindIDs))
	for _, bindID := range bindIDs {
		bindID := strings.TrimSpace(bindID)
		if bindID == "" {
			continue
		}
		filtered = append(bindIDs, bindID)
	}

	var userIDs map[string]int32

	switch mapping.BindID {
	case "email":
		emails, err := UserEmailsWith(s).GetVerifiedEmails(ctx, filtered...)
		if err != nil {
			return nil, err
		}

		userIDs = make(map[string]int32, len(emails))
		for i := range emails {
			for _, bindID := range filtered {
				if emails[i].Email == bindID {
					userIDs[bindID] = emails[i].UserID
					break
				}
			}
		}
	case "username":
		users, err := UsersWith(s.logger, s).GetByUsernames(ctx, filtered...)
		if err != nil {
			return nil, err
		}

		userIDs = make(map[string]int32, len(users))
		for i := range users {
			for _, bindID := range filtered {
				if users[i].Username == bindID {
					userIDs[bindID] = users[i].ID
					break
				}
			}
		}
	default:
		return nil, errors.Errorf("unrecognized user mapping bind ID type %q", mapping.BindID)
	}

	return userIDs, nil
}

// computeDiff determines which ids were added or removed when comparing the old
// list of ids, oldIDs, with the new set.
func computeDiff[T comparable](oldIDs collections.Set[T], newIDs collections.Set[T]) ([]T, []T) {
	return newIDs.Difference(oldIDs).Values(), oldIDs.Difference(newIDs).Values()
}

type ListUserPermissionsArgs struct {
	Query          string
	PaginationArgs *PaginationArgs
}

type UserPermission struct {
	Repo      *types.Repo
	Reason    UserRepoPermissionReason
	UpdatedAt time.Time
}

// ListUserPermissions gets the list of accessible repos for the provided user, along with the reason
// and timestamp for each permission.
func (s *permsStore) ListUserPermissions(ctx context.Context, userID int32, args *ListUserPermissionsArgs) ([]*UserPermission, error) {
	// Set actor with provided userID to context.
	ctx = actor.WithActor(ctx, actor.FromUser(userID))
	authzParams, err := GetAuthzQueryParameters(ctx, s.db)
	if err != nil {
		return nil, err
	}

	conds := []*sqlf.Query{authzParams.ToAuthzQuery()}
	order := sqlf.Sprintf("repo.name ASC")
	limit := sqlf.Sprintf("")

	if args != nil {
		if args.PaginationArgs != nil {
			pa := args.PaginationArgs.SQL()

			if pa.Where != nil {
				conds = append(conds, pa.Where)
			}

			if pa.Order != nil {
				order = pa.Order
			}

			if pa.Limit != nil {
				limit = pa.Limit
			}
		}

		if args.Query != "" {
			conds = append(conds, sqlf.Sprintf("repo.name ILIKE %s", "%"+args.Query+"%"))
		}
	}

	reposQuery := sqlf.Sprintf(
		reposPermissionsInfoQueryFmt,
		sqlf.Join(conds, " AND "),
		order,
		limit,
		userID,
	)

	return scanRepoPermissionsInfo(authzParams)(s.Query(ctx, reposQuery))
}

const reposPermissionsInfoQueryFmt = `
WITH accessible_repos AS (
	SELECT
		repo.id,
		repo.name,
		repo.private,
		BOOL_OR(es.unrestricted) as unrestricted,
		-- We need row_id to preserve the order, because ORDER BY is done in this subquery
		row_number() OVER() as row_id
	FROM repo
	LEFT JOIN external_service_repos AS esr ON esr.repo_id = repo.id
	LEFT JOIN external_services AS es ON esr.external_service_id = es.id
	WHERE
		repo.deleted_at IS NULL
		AND %s -- Authz Conds, Pagination Conds, Search
	GROUP BY repo.id
	ORDER BY %s
	%s -- Limit
)
SELECT
	ar.id,
	ar.name,
	ar.private,
	ar.unrestricted,
	urp.updated_at AS permission_updated_at,
	urp.source
FROM
	accessible_repos AS ar
	LEFT JOIN user_repo_permissions AS urp ON urp.user_id = %d
		AND urp.repo_id = ar.id
	ORDER BY row_id
`

var scanRepoPermissionsInfo = func(authzParams *AuthzQueryParameters) func(basestore.Rows, error) ([]*UserPermission, error) {
	return basestore.NewSliceScanner(func(s dbutil.Scanner) (*UserPermission, error) {
		var repo types.Repo
		var reason UserRepoPermissionReason
		var updatedAt time.Time
		var source *authz.PermsSource
		var unrestricted bool

		if err := s.Scan(
			&repo.ID,
			&repo.Name,
			&repo.Private,
			&unrestricted,
			&dbutil.NullTime{Time: &updatedAt},
			&source,
		); err != nil {
			return nil, err
		}

		// Access reason priorities are as follows:
		// 1. Public repo
		// 2. Unrestricted code host connection
		// 3. Site admin
		// 4. Explicit permissions
		// 5. Permissions sync
		if !repo.Private {
			reason = UserRepoPermissionReasonPublic
		} else if unrestricted {
			reason = UserRepoPermissionReasonUnrestricted
		} else if authzParams.BypassAuthzReasons.SiteAdmin {
			reason = UserRepoPermissionReasonSiteAdmin
		} else if source != nil {
			if *source == authz.SourceAPI {
				reason = UserRepoPermissionReasonExplicitPerms
			} else if *source == authz.SourceRepoSync || *source == authz.SourceUserSync {
				reason = UserRepoPermissionReasonPermissionsSync
			}
		}

		return &UserPermission{Repo: &repo, Reason: reason, UpdatedAt: updatedAt}, nil
	})
}

var defaultPageSize = 100

var defaultPaginationArgs = PaginationArgs{
	First:   &defaultPageSize,
	OrderBy: OrderBy{{Field: "users.id"}},
}

type ListRepoPermissionsArgs struct {
	Query          string
	PaginationArgs *PaginationArgs
}

type RepoPermission struct {
	User      *types.User
	Reason    UserRepoPermissionReason
	UpdatedAt time.Time
}

// ListRepoPermissions gets the list of users who has access to the repository, along with the reason
// and timestamp for each permission.
func (s *permsStore) ListRepoPermissions(ctx context.Context, repoID api.RepoID, args *ListRepoPermissionsArgs) ([]*RepoPermission, error) {
	authzParams, err := GetAuthzQueryParameters(context.Background(), s.db)
	if err != nil {
		return nil, err
	}

	repo, err := s.db.Repos().Get(ctx, repoID)
	if err != nil {
		return nil, err
	}

	permsQueryConditions := []*sqlf.Query{}
	unrestricted := false

	if authzParams.BypassAuthzReasons.NoAuthzProvider {
		// return all users as auth is bypassed for everyone
		permsQueryConditions = append(permsQueryConditions, sqlf.Sprintf("TRUE"))
		unrestricted = true
	} else {
		// find if the repo is unrestricted
		unrestricted, err = s.isRepoUnrestricted(ctx, repoID, authzParams)
		if err != nil {
			return nil, err
		}

		if unrestricted {
			// return all users as repo is unrestricted
			permsQueryConditions = append(permsQueryConditions, sqlf.Sprintf("TRUE"))
		} else {
			if !authzParams.AuthzEnforceForSiteAdmins {
				// include all site admins
				permsQueryConditions = append(permsQueryConditions, sqlf.Sprintf("users.site_admin"))
			}

			permsQueryConditions = append(permsQueryConditions, sqlf.Sprintf(`urp.repo_id = %d`, repoID))
		}
	}

	where := []*sqlf.Query{sqlf.Sprintf("(%s)", sqlf.Join(permsQueryConditions, " OR "))}

	paginationArgs := &defaultPaginationArgs

	if args != nil {
		if args.PaginationArgs != nil {
			paginationArgs = args.PaginationArgs
		}

		if args.Query != "" {
			pattern := "%" + args.Query + "%"
			where = append(where, sqlf.Sprintf("(users.username ILIKE %s OR users.display_name ILIKE %s)", pattern, pattern))
		}
	}

	pa := paginationArgs.SQL()
	if pa.Where != nil {
		where = append(where, pa.Where)
	}

	query := sqlf.Sprintf(usersPermissionsInfoQueryFmt, repoID, sqlf.Join(where, " AND "))
	query = pa.AppendOrderToQuery(query)
	query = pa.AppendLimitToQuery(query)

	rows, err := s.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	perms := make([]*RepoPermission, 0)
	for rows.Next() {
		user, updatedAt, source, err := scanUsersPermissionsInfo(rows)
		if err != nil {
			return nil, err
		}

		var reason UserRepoPermissionReason
		// Access reason priorities are as follows:
		// 1. Public repo
		// 2. Unrestricted code host connection
		// 3. Site admin
		// 4. Explicit permissions
		// 5. Permissions sync
		if !repo.Private {
			reason = UserRepoPermissionReasonPublic
		} else if unrestricted {
			reason = UserRepoPermissionReasonUnrestricted
		} else if !authzParams.AuthzEnforceForSiteAdmins && user.SiteAdmin {
			reason = UserRepoPermissionReasonSiteAdmin
		} else if source == authz.SourceAPI {
			reason = UserRepoPermissionReasonExplicitPerms
		} else if source == authz.SourceRepoSync || source == authz.SourceUserSync {
			reason = UserRepoPermissionReasonPermissionsSync
		}

		perms = append(perms, &RepoPermission{User: user, Reason: reason, UpdatedAt: updatedAt})
	}

	return perms, nil
}

func scanUsersPermissionsInfo(rows dbutil.Scanner) (*types.User, time.Time, authz.PermsSource, error) {
	var u types.User
	var updatedAt time.Time
	var source *string
	var displayName, avatarURL sql.NullString

	err := rows.Scan(
		&u.ID,
		&u.Username,
		&displayName,
		&avatarURL,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.SiteAdmin,
		&u.BuiltinAuth,
		&u.InvalidatedSessionsAt,
		&u.TosAccepted,
		&dbutil.NullTime{Time: &updatedAt},
		&source,
	)
	if err != nil {
		return nil, time.Time{}, "", err
	}

	u.DisplayName = displayName.String
	u.AvatarURL = avatarURL.String

	var permsSource authz.PermsSource
	if source != nil {
		permsSource = authz.PermsSource(*source)
	}

	return &u, updatedAt, permsSource, nil
}

const usersPermissionsInfoQueryFmt = `
SELECT
	users.id,
	users.username,
	users.display_name,
	users.avatar_url,
	users.created_at,
	users.updated_at,
	users.site_admin,
	users.passwd IS NOT NULL,
	users.invalidated_sessions_at,
	users.tos_accepted,
	urp.updated_at AS permissions_updated_at,
	urp.source
FROM
	users
	LEFT JOIN user_repo_permissions urp ON urp.user_id = users.id AND urp.repo_id = %d
WHERE
	users.deleted_at IS NULL
	AND %s
`

func (s *permsStore) IsRepoUnrestricted(ctx context.Context, repoID api.RepoID) (bool, error) {
	authzParams, err := GetAuthzQueryParameters(context.Background(), s.db)
	if err != nil {
		return false, err
	}

	return s.isRepoUnrestricted(ctx, repoID, authzParams)
}

func (s *permsStore) isRepoUnrestricted(ctx context.Context, repoID api.RepoID, authzParams *AuthzQueryParameters) (bool, error) {
	conditions := []*sqlf.Query{GetUnrestrictedReposCond()}

	if !authzParams.UsePermissionsUserMapping {
		conditions = append(conditions, ExternalServiceUnrestrictedCondition)
	}

	query := sqlf.Sprintf(isRepoUnrestrictedQueryFmt, sqlf.Join(conditions, "\nOR\n"), repoID)
	unrestricted, _, err := basestore.ScanFirstBool(s.Query(ctx, query))
	if err != nil {
		return false, err
	}

	return unrestricted, nil
}

const isRepoUnrestrictedQueryFmt = `
SELECT
	(%s) AS unrestricted
FROM repo
WHERE
	repo.id = %d
	AND repo.deleted_at IS NULL
`

type UserRepoPermissionReason string

// UserRepoPermissionReason constants.
const (
	UserRepoPermissionReasonSiteAdmin       UserRepoPermissionReason = "Site Admin"
	UserRepoPermissionReasonUnrestricted    UserRepoPermissionReason = "Unrestricted"
	UserRepoPermissionReasonPermissionsSync UserRepoPermissionReason = "Permissions Sync"
	UserRepoPermissionReasonExplicitPerms   UserRepoPermissionReason = "Explicit API"
	UserRepoPermissionReasonPublic          UserRepoPermissionReason = "Public"
)
