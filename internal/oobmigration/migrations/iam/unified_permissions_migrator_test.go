package iam

import (
	"context"
	"strconv"
	"testing"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/schema"
)

func addUser(t *testing.T, ctx context.Context, store *basestore.Store, userName string, withExternalAccount bool) *extsvc.Account {
	t.Helper()

	userID, _, err := basestore.ScanFirstInt(store.Query(ctx, sqlf.Sprintf(`INSERT INTO users(username, display_name, created_at) VALUES (%s, %s, NOW()) RETURNING id`, userName, userName)))
	require.NoError(t, err)

	if !withExternalAccount {
		return &extsvc.Account{UserID: int32(userID)}
	}
	externalAccountID, _, err := basestore.ScanFirstInt(store.Query(ctx, sqlf.Sprintf(`
		INSERT INTO user_external_accounts(user_id, service_type, service_id, account_id, client_id, created_at, updated_at)
		VALUES(%s, %s, %s, %s, %s, NOW(), NOW()) RETURNING id`,
		userID,
		"test-service-type",
		"test-service-id",
		"test-"+userName,
		"test-client-id-"+userName,
	)))
	require.NoError(t, err)

	return &extsvc.Account{
		UserID: int32(userID),
		ID:     int32(externalAccountID),
		AccountSpec: extsvc.AccountSpec{
			ServiceType: "test-service-type",
			ServiceID:   "test-service-id",
			AccountID:   "test-" + userName,
			ClientID:    "test-client-id-" + userName,
		},
	}
}

func addRepos(t *testing.T, ctx context.Context, store *basestore.Store, accessibleBy []*extsvc.Account, count int) {
	t.Helper()

	currentCount, _, err := basestore.ScanFirstInt(store.Query(ctx, sqlf.Sprintf(`SELECT COUNT(*) FROM repo`)))
	require.NoError(t, err)

	values := make([]*sqlf.Query, 0, count)
	for i := currentCount; i < count+currentCount; i++ {
		values = append(values, sqlf.Sprintf("(%s, 'test-service-type', 'test-service-id')", "test-repo-"+strconv.Itoa(i)))
	}
	repoIDs, err := basestore.ScanInt32s(store.Query(ctx, sqlf.Sprintf(`
	INSERT INTO repo(name, external_service_type, external_service_id)
	VALUES %s
	RETURNING id`, sqlf.Join(values, ","))))
	require.NoError(t, err)

	userPerms := make([]*sqlf.Query, 0, len(accessibleBy))
	for _, account := range accessibleBy {
		userPerms = append(userPerms, sqlf.Sprintf("(%s::integer, 'read', 'repos', NOW(), NOW(), %s, FALSE)", account.UserID, pq.Array(repoIDs)))
	}

	if len(userPerms) == 0 {
		return
	}

	err = store.Exec(ctx, sqlf.Sprintf(`
	INSERT INTO user_permissions AS p (user_id, permission, object_type, updated_at, synced_at, object_ids_ints, migrated)
	VALUES %s
	ON CONFLICT ON CONSTRAINT
  		user_permissions_perm_object_unique
	DO UPDATE SET
		object_ids_ints = p.object_ids_ints || excluded.object_ids_ints`,
		sqlf.Join(userPerms, ",")))
	require.NoError(t, err)
}

func addPermissions(t *testing.T, ctx context.Context, store *basestore.Store, userID int32, repoIDs []int32) {
	t.Helper()

	err := store.Exec(ctx, sqlf.Sprintf(`
	INSERT INTO user_permissions AS p (user_id, permission, object_type, updated_at, synced_at, object_ids_ints, migrated)
	VALUES (%s::integer, 'read', 'repos', NOW(), NOW(), %s, FALSE)
	ON CONFLICT ON CONSTRAINT
  		user_permissions_perm_object_unique
	DO UPDATE SET
		object_ids_ints = p.object_ids_ints || excluded.object_ids_ints`,
		userID, pq.Array(repoIDs)))
	require.NoError(t, err)
}

var scanPermissions = basestore.NewSliceScanner(func(s dbutil.Scanner) (*authz.Permission, error) {
	var p authz.Permission
	if err := s.Scan(&p.UserID, &p.ExternalAccountID, &p.RepoID, &p.Source); err != nil {
		return nil, err
	}
	return &p, nil
})

func cleanUpTables(ctx context.Context, store *basestore.Store) error {
	return store.Exec(ctx, sqlf.Sprintf(`
		DELETE FROM user_permissions;
		DELETE FROM user_repo_permissions;
		DELETE FROM user_external_accounts;
		DELETE FROM users;
		DELETE FROM repo;
	`))
}

func TestUnifiedPermissionsMigrator(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := basestore.NewWithHandle(db.Handle())

	t.Run("Migrator uses default values for params", func(t *testing.T) {
		migrator := NewUnifiedPermissionsMigrator(store)
		assert.Equal(t, 100, migrator.batchSize)
		assert.Equal(t, 60, int(migrator.Interval().Seconds()))
	})

	t.Run("Params can be overriden", func(t *testing.T) {
		unifiedPermsMigratorBatchSize = 20
		unifiedPermsMigratorIntervalSeconds = 120

		t.Cleanup(func() {
			unifiedPermsMigratorBatchSize = 100
			unifiedPermsMigratorIntervalSeconds = 60
		})

		migrator := NewUnifiedPermissionsMigrator(store)
		assert.Equal(t, 20, migrator.batchSize)
		assert.Equal(t, 120, int(migrator.Interval().Seconds()))
	})

	t.Run("Works in batches and progress is updated", func(t *testing.T) {
		t.Cleanup(func() {
			require.NoError(t, cleanUpTables(ctx, store))
		})

		// setup 100 users with 3 repos each
		for i := range 100 {
			account := addUser(t, ctx, store, "user-"+strconv.Itoa(i), true)
			addRepos(t, ctx, store, []*extsvc.Account{account}, 3)
		}

		// Ensure there is no progress before migration
		migrator := newUnifiedPermissionsMigrator(store, 10, 60)
		require.Equal(t, 10, migrator.batchSize)

		progress, err := migrator.Progress(ctx, false)
		require.NoError(t, err)
		require.Equal(t, float64(0), progress)

		for i := range 10 {
			err = migrator.Up(ctx)
			require.NoError(t, err)

			progress, err = migrator.Progress(ctx, false)
			require.NoError(t, err)
			require.Equal(t, float64(i+1)/10, progress)
		}

		require.Equal(t, float64(1), progress)
	})

	t.Run("Progress works correctly even for rows that do not have matching user, repo, external_account", func(t *testing.T) {
		t.Cleanup(func() {
			if !t.Failed() {
				require.NoError(t, cleanUpTables(ctx, store))
			}
		})

		// setup 100 users with different combinations of repos and external accounts, deleted_at, etc
		for i := range 100 {
			userName := "user-" + strconv.Itoa(i)
			// Add 20 users with no external accounts
			account := addUser(t, ctx, store, userName, i < 40 || i >= 60)
			if i >= 20 && i < 40 {
				// Add 20 users with no repos
				addPermissions(t, ctx, store, account.UserID, []int32{})
				continue
			}
			if i >= 60 && i < 80 {
				// mark 20 users as deleted
				err := store.Exec(ctx, sqlf.Sprintf("UPDATE users SET deleted_at = NOW() WHERE id = %s", account.UserID))
				require.NoError(t, err)
			}
			addRepos(t, ctx, store, []*extsvc.Account{account}, 3)
			if i >= 80 {
				// Mark repos as deleted
				base := i*3 - 60 // there are 20 users without repos, so we need to offset by 60
				err := store.Exec(ctx, sqlf.Sprintf("UPDATE repo SET deleted_at = NOW() WHERE id IN(%d, %d, %d)", (base+1), (base+2), (base+3)))
				require.NoError(t, err)
			}
		}

		// Ensure there is no progress before migration
		migrator := newUnifiedPermissionsMigrator(store, 10, 60)
		require.Equal(t, 10, migrator.batchSize)

		progress, err := migrator.Progress(ctx, false)
		require.NoError(t, err)
		require.Equal(t, float64(0), progress)

		for i := range 10 {
			err = migrator.Up(ctx)
			require.NoError(t, err)

			progress, err = migrator.Progress(ctx, false)
			require.NoError(t, err)
			require.Equal(t, float64(i+1)/10, progress)
		}

		require.Equal(t, float64(1), progress)
	})

	runDataCheckTest := func(t *testing.T, source authz.PermsSource) {
		t.Helper()

		// Set up test data
		alice, bob := addUser(t, ctx, store, "alice", true), addUser(t, ctx, store, "bob", true)
		addRepos(t, ctx, store, []*extsvc.Account{alice, bob}, 2)
		addRepos(t, ctx, store, []*extsvc.Account{alice}, 3)
		addRepos(t, ctx, store, []*extsvc.Account{bob}, 1)
		addRepos(t, ctx, store, []*extsvc.Account{}, 1)

		// Ensure there is no progress before migration
		migrator := NewUnifiedPermissionsMigrator(store)

		progress, err := migrator.Progress(ctx, false)
		require.NoError(t, err)
		require.Equal(t, 0.0, progress)

		// Perform the migration and recheck the progress
		err = migrator.Up(ctx)
		require.NoError(t, err)

		progress, err = migrator.Progress(ctx, false)
		require.NoError(t, err)
		require.Equal(t, 1.0, progress)

		// Ensure rows were marked as migrated
		userIDs, err := basestore.ScanInt32s(store.Query(ctx, sqlf.Sprintf(`SELECT user_id FROM user_permissions WHERE NOT migrated`)))
		require.NoError(t, err)
		require.Empty(t, userIDs)

		// Ensure the permissions were migrated correctly
		permissions, err := scanPermissions(store.Query(ctx, sqlf.Sprintf(`SELECT user_id, user_external_account_id, repo_id, source FROM user_repo_permissions`)))
		require.NoError(t, err)
		require.NotEmpty(t, permissions)
		assert.Equal(t, 8, len(permissions), "unexpected number of permissions")

		alicePerms := make([]*authz.Permission, 0, 5)
		bobPerms := make([]*authz.Permission, 0, 3)
		aliceRepos := make(map[int32]struct{})
		for _, p := range permissions {
			assert.Equal(t, source, p.Source, "unexpected source for permission")
			if p.UserID == alice.UserID {
				alicePerms = append(alicePerms, p)
				assert.Equal(t, alice.ID, p.ExternalAccountID, "unexpected external account id for alice")
				aliceRepos[p.RepoID] = struct{}{}
			} else if p.UserID == bob.UserID {
				bobPerms = append(bobPerms, p)
				assert.Equal(t, bob.ID, p.ExternalAccountID, "unexpected external account id for bob")
			}
		}

		assert.Equal(t, 5, len(alicePerms), "unexpected number of permissions for alice")
		assert.Equal(t, 3, len(bobPerms), "unexpected number of permissions for bob")

		commonCount := 0
		for _, p := range bobPerms {
			if _, ok := aliceRepos[p.RepoID]; ok {
				commonCount++
			}
		}

		assert.Equal(t, 2, commonCount, "unexpected number of common permissions between alice and bob")
	}

	t.Run("Migrates data correctly for synced perms", func(t *testing.T) {
		t.Cleanup(func() {
			require.NoError(t, cleanUpTables(ctx, store))
		})

		runDataCheckTest(t, authz.SourceUserSync)
	})

	t.Run("Migrates data correctly for explicit API perms", func(t *testing.T) {
		t.Cleanup(func() {
			require.NoError(t, cleanUpTables(ctx, store))
		})

		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				PermissionsUserMapping: &schema.PermissionsUserMapping{
					Enabled: true,
					BindID:  "email",
				},
			},
		})
		t.Cleanup(func() { conf.Mock(nil) })

		runDataCheckTest(t, authz.SourceAPI)
	})
}
