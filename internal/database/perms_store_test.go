package database

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/gitchander/permutation"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

// Toggles particularly slow tests. To enable, use `go test` with this flag, for example:
//
//	go test -timeout 360s -v -run ^TestIntegration_PermsStore$ github.com/sourcegraph/sourcegraph/internal/database -slow-tests
var slowTests = flag.Bool("slow-tests", false, "Enable very slow tests")

// postgresParameterLimitTest names tests that are focused on ensuring the default
// behaviour of various queries do not run into the Postgres parameter limit at scale
// (error `extended protocol limited to 65535 parameters`).
//
// They are typically flagged behind `-slow-tests` - when changing queries make sure to
// enable these tests and add more where relevant (see `slowTests`).
const postgresParameterLimitTest = "ensure we do not exceed postgres parameter limit"

func cleanupPermsTables(t *testing.T, s *permsStore) {
	t.Helper()

	q := `TRUNCATE TABLE permission_sync_jobs, user_permissions, repo_permissions, user_pending_permissions, repo_pending_permissions, user_repo_permissions;`
	executeQuery(t, context.Background(), s, sqlf.Sprintf(q))
}

func mapsetToArray(ms map[int32]struct{}) []int {
	ints := []int{}
	for id := range ms {
		ints = append(ints, int(id))
	}
	sort.Slice(ints, func(i, j int) bool { return ints[i] < ints[j] })

	return ints
}

func toMapset(ids ...int32) map[int32]struct{} {
	ms := map[int32]struct{}{}
	for _, id := range ids {
		ms[id] = struct{}{}
	}
	return ms
}

func TestPermsStore_LoadUserPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	ctx := context.Background()

	t.Run("no matching", func(t *testing.T) {
		s := perms(logger, db, clock)
		t.Cleanup(func() {
			cleanupPermsTables(t, s)
			cleanupUsersTable(t, s)
			cleanupReposTable(t, s)
		})

		setupPermsRelatedEntities(t, s, []authz.Permission{{UserID: 2, RepoID: 1}})

		if _, err := s.SetRepoPerms(ctx, 1, []authz.UserIDWithExternalAccountID{{UserID: 2}}, authz.SourceRepoSync); err != nil {
			t.Fatal(err)
		}

		up, err := s.LoadUserPermissions(context.Background(), 1)
		require.NoError(t, err)

		equal(t, "IDs", 0, len(up))
	})

	t.Run("found matching", func(t *testing.T) {
		s := perms(logger, db, clock)
		t.Cleanup(func() {
			cleanupPermsTables(t, s)
			cleanupUsersTable(t, s)
			cleanupReposTable(t, s)
		})

		setupPermsRelatedEntities(t, s, []authz.Permission{{UserID: 2, RepoID: 1}})

		if _, err := s.SetRepoPerms(ctx, 1, []authz.UserIDWithExternalAccountID{{UserID: 2}}, authz.SourceRepoSync); err != nil {
			t.Fatal(err)
		}

		up, err := s.LoadUserPermissions(context.Background(), 2)
		require.NoError(t, err)

		gotIDs := make([]int32, len(up))
		for i, perm := range up {
			gotIDs[i] = perm.RepoID
		}

		equal(t, "IDs", []int32{1}, gotIDs)
	})

	t.Run("add and change", func(t *testing.T) {
		s := perms(logger, db, clock)
		t.Cleanup(func() {
			cleanupPermsTables(t, s)
			cleanupUsersTable(t, s)
			cleanupReposTable(t, s)
		})

		setupPermsRelatedEntities(t, s, []authz.Permission{{UserID: 1, RepoID: 1}, {UserID: 2, RepoID: 1}, {UserID: 3, RepoID: 1}})

		if _, err := s.SetRepoPerms(ctx, 1, []authz.UserIDWithExternalAccountID{{UserID: 1}, {UserID: 2}}, authz.SourceRepoSync); err != nil {
			t.Fatal(err)
		}

		if _, err := s.SetRepoPerms(ctx, 1, []authz.UserIDWithExternalAccountID{{UserID: 2}, {UserID: 3}}, authz.SourceRepoSync); err != nil {
			t.Fatal(err)
		}

		up1, err := s.LoadUserPermissions(context.Background(), 1)
		require.NoError(t, err)

		equal(t, "No IDs", 0, len(up1))

		up2, err := s.LoadUserPermissions(context.Background(), 2)
		require.NoError(t, err)
		gotIDs := make([]int32, len(up2))
		for i, perm := range up2 {
			gotIDs[i] = perm.RepoID
		}

		equal(t, "IDs", []int32{1}, gotIDs)

		up3, err := s.LoadUserPermissions(context.Background(), 3)
		require.NoError(t, err)
		gotIDs = make([]int32, len(up3))
		for i, perm := range up3 {
			gotIDs[i] = perm.RepoID
		}

		equal(t, "IDs", []int32{1}, gotIDs)
	})
}

func TestPermsStore_LoadRepoPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	ctx := context.Background()

	t.Run("no matching", func(t *testing.T) {
		s := perms(logger, db, time.Now)
		t.Cleanup(func() {
			cleanupPermsTables(t, s)
			cleanupUsersTable(t, s)
			cleanupReposTable(t, s)
		})

		setupPermsRelatedEntities(t, s, []authz.Permission{{UserID: 2, RepoID: 1}})

		if _, err := s.SetRepoPerms(ctx, 1, []authz.UserIDWithExternalAccountID{{UserID: 2}}, authz.SourceRepoSync); err != nil {
			t.Fatal(err)
		}

		rp, err := s.LoadRepoPermissions(context.Background(), 2)
		require.NoError(t, err)
		require.Equal(t, 0, len(rp))
	})

	t.Run("found matching", func(t *testing.T) {
		s := perms(logger, db, time.Now)
		t.Cleanup(func() {
			cleanupPermsTables(t, s)
			cleanupUsersTable(t, s)
			cleanupReposTable(t, s)
		})

		setupPermsRelatedEntities(t, s, []authz.Permission{{UserID: 2, RepoID: 1}})

		if _, err := s.SetRepoPerms(ctx, 1, []authz.UserIDWithExternalAccountID{{UserID: 2}}, authz.SourceRepoSync); err != nil {
			t.Fatal(err)
		}

		rp, err := s.LoadRepoPermissions(context.Background(), 1)
		require.NoError(t, err)
		gotIDs := make([]int32, len(rp))
		for i, perm := range rp {
			gotIDs[i] = perm.UserID
		}

		equal(t, "permissions UserIDs", []int32{2}, gotIDs)
	})
}

func TestPermsStore_SetUserExternalAccountPerms(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)

	const countToExceedParameterLimit = 17000 // ~ 65535 / 4 parameters per row

	type testUpdate struct {
		userID            int32
		externalAccountID int32
		repoIDs           []int32
	}

	tests := []struct {
		name          string
		slowTest      bool
		updates       []testUpdate
		expectedPerms []authz.Permission
		expectedStats []*SetPermissionsResult
	}{
		{
			name: "empty",
			updates: []testUpdate{{
				userID:            1,
				externalAccountID: 1,
				repoIDs:           []int32{},
			}},
			expectedPerms: []authz.Permission{},
			expectedStats: []*SetPermissionsResult{{
				Added:   0,
				Removed: 0,
				Found:   0,
			}},
		},
		{
			name: "add",
			updates: []testUpdate{{
				userID:            1,
				externalAccountID: 1,
				repoIDs:           []int32{1},
			}, {
				userID:            2,
				externalAccountID: 2,
				repoIDs:           []int32{1, 2},
			}, {
				userID:            3,
				externalAccountID: 3,
				repoIDs:           []int32{3, 4},
			}},
			expectedPerms: []authz.Permission{{
				UserID:            1,
				ExternalAccountID: 1,
				RepoID:            1,
				Source:            authz.SourceUserSync,
			}, {
				UserID:            2,
				ExternalAccountID: 2,
				RepoID:            1,
				Source:            authz.SourceUserSync,
			}, {
				UserID:            2,
				ExternalAccountID: 2,
				RepoID:            2,
				Source:            authz.SourceUserSync,
			}, {
				UserID:            3,
				ExternalAccountID: 3,
				RepoID:            3,
				Source:            authz.SourceUserSync,
			}, {
				UserID:            3,
				ExternalAccountID: 3,
				RepoID:            4,
				Source:            authz.SourceUserSync,
			}},
			expectedStats: []*SetPermissionsResult{{
				Added:   1,
				Removed: 0,
				Found:   1,
			}, {
				Added:   2,
				Removed: 0,
				Found:   2,
			}, {
				Added:   2,
				Removed: 0,
				Found:   2,
			}},
		},
		{
			name: "add and update",
			updates: []testUpdate{{
				userID:            1,
				externalAccountID: 1,
				repoIDs:           []int32{1},
			}, {
				userID:            1,
				externalAccountID: 1,
				repoIDs:           []int32{2, 3},
			}, {
				userID:            2,
				externalAccountID: 2,
				repoIDs:           []int32{1, 2},
			}, {
				userID:            2,
				externalAccountID: 2,
				repoIDs:           []int32{1, 3},
			}},
			expectedPerms: []authz.Permission{{
				UserID:            1,
				ExternalAccountID: 1,
				RepoID:            2,
				Source:            authz.SourceUserSync,
			}, {
				UserID:            1,
				ExternalAccountID: 1,
				RepoID:            3,
				Source:            authz.SourceUserSync,
			}, {
				UserID:            2,
				ExternalAccountID: 2,
				RepoID:            1,
				Source:            authz.SourceUserSync,
			}, {
				UserID:            2,
				ExternalAccountID: 2,
				RepoID:            3,
				Source:            authz.SourceUserSync,
			}},
			expectedStats: []*SetPermissionsResult{{
				Added:   1,
				Removed: 0,
				Found:   1,
			}, {
				Added:   2,
				Removed: 1,
				Found:   2,
			}, {
				Added:   2,
				Removed: 0,
				Found:   2,
			}, {
				Added:   1,
				Removed: 1,
				Found:   2,
			}},
		},
		{
			name: "add and clear",
			updates: []testUpdate{{
				userID:            1,
				externalAccountID: 1,
				repoIDs:           []int32{1, 2, 3},
			}, {
				userID:            1,
				externalAccountID: 1,
				repoIDs:           []int32{},
			}},
			expectedPerms: []authz.Permission{},
			expectedStats: []*SetPermissionsResult{{
				Added:   3,
				Removed: 0,
				Found:   3,
			}, {
				Added:   0,
				Removed: 3,
				Found:   0,
			}},
		},
		{
			name:     postgresParameterLimitTest,
			slowTest: true,
			updates: func() []testUpdate {
				u := testUpdate{
					userID:            1,
					externalAccountID: 1,
					repoIDs:           make([]int32, countToExceedParameterLimit),
				}
				for i := 1; i <= countToExceedParameterLimit; i += 1 {
					u.repoIDs[i-1] = int32(i)
				}
				return []testUpdate{u}
			}(),
			expectedPerms: func() []authz.Permission {
				p := make([]authz.Permission, countToExceedParameterLimit)
				for i := 1; i <= countToExceedParameterLimit; i += 1 {
					p[i-1] = authz.Permission{
						UserID:            1,
						ExternalAccountID: 1,
						RepoID:            int32(i),
						Source:            authz.SourceUserSync,
					}
				}
				return p
			}(),
			expectedStats: func() []*SetPermissionsResult {
				result := make([]*SetPermissionsResult, countToExceedParameterLimit)
				for i := range countToExceedParameterLimit {
					result[i] = &SetPermissionsResult{
						Added:   1,
						Removed: 0,
						Found:   1,
					}
				}
				return result
			}(),
		},
	}

	t.Run("user-centric update should set permissions", func(t *testing.T) {
		logger := logtest.Scoped(t)
		s := perms(logger, db, clock)
		t.Cleanup(func() {
			cleanupUsersTable(t, s)
			cleanupReposTable(t, s)
			cleanupPermsTables(t, s)
		})

		expectedStats := &SetPermissionsResult{
			Added:   1,
			Removed: 0,
			Found:   1,
		}
		expectedPerms := []authz.Permission{
			{UserID: 2, ExternalAccountID: 1, RepoID: 1, Source: authz.SourceUserSync},
		}
		setupPermsRelatedEntities(t, s, expectedPerms)

		u := authz.UserIDWithExternalAccountID{
			UserID:            2,
			ExternalAccountID: 1,
		}
		repoIDs := []int32{1}
		var stats *SetPermissionsResult
		var err error
		if stats, err = s.SetUserExternalAccountPerms(context.Background(), u, repoIDs, authz.SourceUserSync); err != nil {
			t.Fatal(err)
		}

		checkUserRepoPermissions(t, s, sqlf.Sprintf("user_id = %d", u.UserID), expectedPerms)
		equal(t, "stats", expectedStats, stats)
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.slowTest && !*slowTests {
				t.Skip("slow-tests not enabled")
			}

			s := perms(logger, db, clock)
			t.Cleanup(func() {
				cleanupUsersTable(t, s)
				cleanupReposTable(t, s)
				cleanupPermsTables(t, s)
			})

			updates := []authz.Permission{}
			for _, u := range test.updates {
				for _, r := range u.repoIDs {
					updates = append(updates, authz.Permission{
						UserID:            u.userID,
						ExternalAccountID: u.externalAccountID,
						RepoID:            r,
					})
				}
			}
			if len(updates) > 0 {
				setupPermsRelatedEntities(t, s, updates)
			}

			for i, p := range test.updates {
				u := authz.UserIDWithExternalAccountID{
					UserID:            p.userID,
					ExternalAccountID: p.externalAccountID,
				}
				result, err := s.SetUserExternalAccountPerms(context.Background(), u, p.repoIDs, authz.SourceUserSync)
				require.NoError(t, err)
				equal(t, "result", test.expectedStats[i], result)
			}

			checkUserRepoPermissions(t, s, nil, test.expectedPerms)
		})
	}
}

func checkUserRepoPermissions(t *testing.T, s *permsStore, where *sqlf.Query, expectedPermissions []authz.Permission) {
	t.Helper()

	if where == nil {
		where = sqlf.Sprintf("TRUE")
	}
	format := "SELECT user_id, user_external_account_id, repo_id, created_at, updated_at, source FROM user_repo_permissions WHERE %s;"
	permissions, err := ScanPermissions(s.Query(context.Background(), sqlf.Sprintf(format, where)))
	if err != nil {
		t.Fatal(err)
	}
	// ScanPermissions returns nil if there are no results, but for the purpose of test readability,
	// we defined expectedPermissions to be an empty slice, which matches the empty permissions input to write to the db.
	// hence if permissions is nil, we set it to an empty slice.
	if permissions == nil {
		permissions = []authz.Permission{}
	}
	sort.Slice(permissions, func(i, j int) bool {
		if permissions[i].UserID == permissions[j].UserID && permissions[i].ExternalAccountID == permissions[j].ExternalAccountID {
			return permissions[i].RepoID < permissions[j].RepoID
		}
		if permissions[i].UserID == permissions[j].UserID {
			return permissions[i].ExternalAccountID < permissions[j].ExternalAccountID
		}
		return permissions[i].UserID < permissions[j].UserID
	})

	if diff := cmp.Diff(expectedPermissions, permissions, cmpopts.IgnoreFields(authz.Permission{}, "CreatedAt", "UpdatedAt")); diff != "" {
		t.Fatalf("Expected permissions: %v do not match actual permissions: %v; diff %v", expectedPermissions, permissions, diff)
	}
}

func setupPermsRelatedEntities(t *testing.T, s *permsStore, permissions []authz.Permission) {
	t.Helper()
	if len(permissions) == 0 {
		t.Fatal("no permissions to setup related entities for")
	}

	users := make(map[int32]*sqlf.Query, len(permissions))
	externalAccounts := make(map[int32]*sqlf.Query, len(permissions))
	repos := make(map[int32]*sqlf.Query, len(permissions))
	for _, p := range permissions {
		users[p.UserID] = sqlf.Sprintf("(%s::integer, %s::text)", p.UserID, fmt.Sprintf("user-%d", p.UserID))
		externalAccounts[p.ExternalAccountID] = sqlf.Sprintf("(%s::integer, %s::integer, %s::text, %s::text, %s::text, %s::text)", p.ExternalAccountID, p.UserID, "service_type", "service_id", fmt.Sprintf("account_id_%d", p.ExternalAccountID), "client_id")
		repos[p.RepoID] = sqlf.Sprintf("(%s::integer, %s::text)", p.RepoID, fmt.Sprintf("repo-%d", p.RepoID))
	}

	defaultErrMessage := "setup test related entities before actual test"
	if len(users) > 0 {
		usersQuery := sqlf.Sprintf(`INSERT INTO users(id, username) VALUES %s ON CONFLICT (id) DO NOTHING`, sqlf.Join(maps.Values(users), ","))
		if err := s.execute(context.Background(), usersQuery); err != nil {
			t.Fatal(defaultErrMessage, err)
		}
	}
	if len(externalAccounts) > 0 {
		externalAccountsQuery := sqlf.Sprintf(`INSERT INTO user_external_accounts(id, user_id, service_type, service_id, account_id, client_id) VALUES %s ON CONFLICT(id) DO NOTHING`, sqlf.Join(maps.Values(externalAccounts), ","))
		if err := s.execute(context.Background(), externalAccountsQuery); err != nil {
			t.Fatal(defaultErrMessage, err)
		}
	}
	if len(repos) > 0 {
		reposQuery := sqlf.Sprintf(`INSERT INTO repo(id, name) VALUES %s ON CONFLICT(id) DO NOTHING`, sqlf.Join(maps.Values(repos), ","))
		if err := s.execute(context.Background(), reposQuery); err != nil {
			t.Fatal(defaultErrMessage, err)
		}
	}
}

func TestPermsStore_SetUserRepoPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)

	source := authz.SourceUserSync

	tests := []struct {
		name                string
		origPermissions     []authz.Permission
		permissions         []authz.Permission
		expectedPermissions []authz.Permission
		entity              authz.PermissionEntity
		expectedResult      *SetPermissionsResult
		keepPerms           bool
	}{
		{
			name:                "empty",
			permissions:         []authz.Permission{},
			expectedPermissions: []authz.Permission{},
			entity:              authz.PermissionEntity{UserID: 1, ExternalAccountID: 1},
			expectedResult:      &SetPermissionsResult{Added: 0, Removed: 0, Found: 0},
		},
		{
			name: "add",
			permissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2},
				{UserID: 1, ExternalAccountID: 1, RepoID: 3},
			},
			expectedPermissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1, Source: source},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2, Source: source},
				{UserID: 1, ExternalAccountID: 1, RepoID: 3, Source: source},
			},
			entity:         authz.PermissionEntity{UserID: 1, ExternalAccountID: 1},
			expectedResult: &SetPermissionsResult{Added: 3, Removed: 0, Found: 3},
		},
		{
			name: "add, update and remove",
			origPermissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2},
				{UserID: 1, ExternalAccountID: 1, RepoID: 3},
			},
			permissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1},
				{UserID: 1, ExternalAccountID: 1, RepoID: 4},
			},
			expectedPermissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1, Source: source},
				{UserID: 1, ExternalAccountID: 1, RepoID: 4, Source: source},
			},
			entity:         authz.PermissionEntity{UserID: 1, ExternalAccountID: 1},
			expectedResult: &SetPermissionsResult{Added: 1, Removed: 2, Found: 2},
		},
		{
			name: "remove only",
			origPermissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2},
				{UserID: 1, ExternalAccountID: 1, RepoID: 3},
			},
			permissions:         []authz.Permission{},
			expectedPermissions: []authz.Permission{},
			entity:              authz.PermissionEntity{UserID: 1, ExternalAccountID: 1},
			expectedResult:      &SetPermissionsResult{Added: 0, Removed: 3, Found: 0},
		},
		{
			name: "does not touch explicit permissions when source is sync",
			origPermissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2, Source: authz.SourceAPI},
				{UserID: 1, ExternalAccountID: 1, RepoID: 3},
			},
			permissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 4},
			},
			expectedPermissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 2, Source: authz.SourceAPI},
				{UserID: 1, ExternalAccountID: 1, RepoID: 4, Source: source},
			},
			entity:         authz.PermissionEntity{UserID: 1, ExternalAccountID: 1},
			expectedResult: &SetPermissionsResult{Added: 1, Removed: 2, Found: 1},
		},
		{
			name: "does not delete old permissions when bool is false",
			origPermissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1},
				{UserID: 1, ExternalAccountID: 1, RepoID: 3},
			},
			permissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 2},
				{UserID: 1, ExternalAccountID: 1, RepoID: 4},
			},
			expectedPermissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1, Source: source},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2, Source: source},
				{UserID: 1, ExternalAccountID: 1, RepoID: 3, Source: source},
				{UserID: 1, ExternalAccountID: 1, RepoID: 4, Source: source},
			},
			entity:         authz.PermissionEntity{UserID: 1, ExternalAccountID: 1},
			expectedResult: &SetPermissionsResult{Added: 2, Removed: 0, Found: 2},
			keepPerms:      true,
		},
	}

	ctx := actor.WithInternalActor(context.Background())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := perms(logger, db, clock)
			t.Cleanup(func() {
				cleanupPermsTables(t, s)
				cleanupUsersTable(t, s)
				cleanupReposTable(t, s)
			})

			replacePerms := !test.keepPerms

			if len(test.origPermissions) > 0 {
				setupPermsRelatedEntities(t, s, test.origPermissions)
				syncedPermissions := []authz.Permission{}
				explicitPermissions := []authz.Permission{}
				for _, p := range test.origPermissions {
					if p.Source == authz.SourceAPI {
						explicitPermissions = append(explicitPermissions, p)
					} else {
						syncedPermissions = append(syncedPermissions, p)
					}
				}

				_, err := s.setUserRepoPermissions(ctx, syncedPermissions, test.entity, source, replacePerms)
				require.NoError(t, err)
				_, err = s.setUserRepoPermissions(ctx, explicitPermissions, test.entity, authz.SourceAPI, replacePerms)
				require.NoError(t, err)
			}

			if len(test.permissions) > 0 {
				setupPermsRelatedEntities(t, s, test.permissions)
			}
			var stats *SetPermissionsResult
			var err error
			if stats, err = s.setUserRepoPermissions(ctx, test.permissions, test.entity, source, replacePerms); err != nil {
				t.Fatal("testing user repo permissions", err)
			}

			if test.entity.UserID > 0 {
				checkUserRepoPermissions(t, s, sqlf.Sprintf("user_id = %d", test.entity.UserID), test.expectedPermissions)
			} else if test.entity.RepoID > 0 {
				checkUserRepoPermissions(t, s, sqlf.Sprintf("repo_id = %d", test.entity.RepoID), test.expectedPermissions)
			}

			require.Equal(t, test.expectedResult, stats)
		})
	}
}

func TestPermsStore_UnionExplicitAndSyncedPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)

	tests := []struct {
		name                    string
		origExplicitPermissions []authz.Permission
		origSyncedPermissions   []authz.Permission
		permissions             []authz.Permission
		expectedPermissions     []authz.Permission
		expectedResult          *SetPermissionsResult
		entity                  authz.PermissionEntity
		source                  authz.PermsSource
	}{
		{
			name: "add explicit permissions when synced are already there",
			origSyncedPermissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2},
			},
			permissions: []authz.Permission{
				{UserID: 1, RepoID: 3},
			},
			expectedPermissions: []authz.Permission{
				{UserID: 1, RepoID: 3, Source: authz.SourceAPI},
				{UserID: 1, ExternalAccountID: 1, RepoID: 1, Source: authz.SourceUserSync},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2, Source: authz.SourceUserSync},
			},
			expectedResult: &SetPermissionsResult{Added: 1, Removed: 0, Found: 1},
			entity:         authz.PermissionEntity{UserID: 1, ExternalAccountID: 1},
			source:         authz.SourceAPI,
		},
		{
			name: "add synced permissions when explicit are already there",
			origExplicitPermissions: []authz.Permission{
				{UserID: 1, RepoID: 1},
				{UserID: 1, RepoID: 3},
			},
			permissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 2},
			},
			expectedPermissions: []authz.Permission{
				{UserID: 1, RepoID: 1, Source: authz.SourceAPI},
				{UserID: 1, RepoID: 3, Source: authz.SourceAPI},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2, Source: authz.SourceUserSync},
			},
			expectedResult: &SetPermissionsResult{Added: 1, Removed: 0, Found: 1},
			entity:         authz.PermissionEntity{UserID: 1, ExternalAccountID: 1},
			source:         authz.SourceUserSync,
		},
		{
			name: "add, update and remove synced permissions, when explicit are already there",
			origExplicitPermissions: []authz.Permission{
				{UserID: 1, RepoID: 2},
				{UserID: 1, RepoID: 4},
			},
			origSyncedPermissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1},
				{UserID: 1, ExternalAccountID: 1, RepoID: 3},
			},
			permissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 3},
				{UserID: 1, ExternalAccountID: 1, RepoID: 5},
			},
			expectedPermissions: []authz.Permission{
				{UserID: 1, RepoID: 2, Source: authz.SourceAPI},
				{UserID: 1, RepoID: 4, Source: authz.SourceAPI},
				{UserID: 1, ExternalAccountID: 1, RepoID: 3, Source: authz.SourceUserSync},
				{UserID: 1, ExternalAccountID: 1, RepoID: 5, Source: authz.SourceUserSync},
			},
			expectedResult: &SetPermissionsResult{Added: 1, Removed: 1, Found: 2},
			entity:         authz.PermissionEntity{UserID: 1, ExternalAccountID: 1},
			source:         authz.SourceUserSync,
		},
		{
			name: "add synced permission to same entity as explicit permission adds new row",
			origExplicitPermissions: []authz.Permission{
				{UserID: 1, RepoID: 1},
			},
			permissions: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1},
			},
			expectedPermissions: []authz.Permission{
				{UserID: 1, RepoID: 1, Source: authz.SourceAPI},
				{UserID: 1, ExternalAccountID: 1, RepoID: 1, Source: authz.SourceUserSync},
			},
			expectedResult: &SetPermissionsResult{Added: 1, Removed: 0, Found: 1},
			entity:         authz.PermissionEntity{UserID: 1, ExternalAccountID: 1},
			source:         authz.SourceUserSync,
		},
	}

	ctx := actor.WithInternalActor(context.Background())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := perms(logger, db, clock)
			t.Cleanup(func() {
				cleanupPermsTables(t, s)
				cleanupUsersTable(t, s)
				cleanupReposTable(t, s)
			})

			if len(test.origExplicitPermissions) > 0 {
				setupPermsRelatedEntities(t, s, test.origExplicitPermissions)
				_, err := s.setUserRepoPermissions(ctx, test.origExplicitPermissions, test.entity, authz.SourceAPI, true)
				require.NoError(t, err)
			}
			if len(test.origSyncedPermissions) > 0 {
				setupPermsRelatedEntities(t, s, test.origSyncedPermissions)
				_, err := s.setUserRepoPermissions(ctx, test.origSyncedPermissions, test.entity, authz.SourceUserSync, true)
				require.NoError(t, err)
			}

			if len(test.permissions) > 0 {
				setupPermsRelatedEntities(t, s, test.permissions)
			}

			var stats *SetPermissionsResult
			var err error
			if stats, err = s.setUserRepoPermissions(ctx, test.permissions, test.entity, test.source, true); err != nil {
				t.Fatal("testing user repo permissions", err)
			}

			if test.entity.UserID > 0 {
				checkUserRepoPermissions(t, s, sqlf.Sprintf("user_id = %d", test.entity.UserID), test.expectedPermissions)
			} else if test.entity.RepoID > 0 {
				checkUserRepoPermissions(t, s, sqlf.Sprintf("repo_id = %d", test.entity.RepoID), test.expectedPermissions)
			}

			require.Equal(t, test.expectedResult, stats)
		})
	}
}

func TestPermsStore_FetchReposByExternalAccount(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)

	source := authz.SourceRepoSync

	tests := []struct {
		name              string
		origPermissions   []authz.Permission
		expected          []api.RepoID
		externalAccountID int32
	}{
		{
			name:              "empty",
			externalAccountID: 1,
			expected:          nil,
		},
		{
			name:              "one match",
			externalAccountID: 1,
			expected:          []api.RepoID{1},
			origPermissions: []authz.Permission{
				{
					UserID:            1,
					ExternalAccountID: 1,
					RepoID:            1,
				},
				{
					UserID:            1,
					ExternalAccountID: 2,
					RepoID:            2,
				},
				{
					UserID: 1,
					RepoID: 3,
				},
			},
		},
		{
			name:              "multiple matches",
			externalAccountID: 1,
			expected:          []api.RepoID{1, 2},
			origPermissions: []authz.Permission{
				{
					UserID:            1,
					ExternalAccountID: 1,
					RepoID:            1,
				},
				{
					UserID:            1,
					ExternalAccountID: 1,
					RepoID:            2,
				},
				{
					UserID: 1,
					RepoID: 3,
				},
			},
		},
	}
	ctx := actor.WithInternalActor(context.Background())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := perms(logger, db, clock)
			t.Cleanup(func() {
				cleanupPermsTables(t, s)
				cleanupUsersTable(t, s)
				cleanupReposTable(t, s)
			})

			if test.origPermissions != nil && len(test.origPermissions) > 0 {
				setupPermsRelatedEntities(t, s, test.origPermissions)
				_, err := s.setUserRepoPermissions(ctx, test.origPermissions, authz.PermissionEntity{UserID: 42}, source, true)
				if err != nil {
					t.Fatal("setup test permissions before actual test", err)
				}
			}

			ids, err := s.FetchReposByExternalAccount(ctx, test.externalAccountID)
			if err != nil {
				t.Fatal("testing fetch repos by user and external account", err)
			}

			assert.Equal(t, test.expected, ids, "no match found for repo IDs")
		})
	}
}

func TestPermsStore_SetRepoPermissionsUnrestricted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)

	ctx := context.Background()
	s := setupTestPerms(t, db, clock)

	legacyUnrestricted := func(t *testing.T, id int32, want bool) {
		t.Helper()

		p, err := s.LoadRepoPermissions(ctx, id)
		require.NoErrorf(t, err, "loading permissions for %d", id)

		unrestricted := len(p) == 1 && p[0].UserID == 0
		if unrestricted != want {
			t.Fatalf("Want %v, got %v for %d", want, unrestricted, id)
		}
	}

	assertUnrestricted := func(t *testing.T, id int32, want bool) {
		t.Helper()

		legacyUnrestricted(t, id, want)

		type unrestrictedResult struct {
			id     int32
			source authz.PermsSource
		}

		scanResults := basestore.NewSliceScanner(func(s dbutil.Scanner) (unrestrictedResult, error) {
			r := unrestrictedResult{}
			err := s.Scan(&r.id, &r.source)
			return r, err
		})

		q := sqlf.Sprintf("SELECT repo_id, source FROM user_repo_permissions WHERE repo_id = %d AND user_id IS NULL", id)
		results, err := scanResults(s.Handle().QueryContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...))
		if err != nil {
			t.Fatalf("loading user repo permissions for %d: %v", id, err)
		}
		if want && len(results) == 0 {
			t.Fatalf("Want unrestricted, but found no results for %d", id)
		}
		if !want && len(results) > 0 {
			t.Fatalf("Want restricted, but found results for %d: %v", id, results)
		}

		if want {
			for _, r := range results {
				require.Equal(t, authz.SourceAPI, r.source)
			}
		}
	}

	createRepo := func(t *testing.T, id int) {
		t.Helper()
		executeQuery(t, ctx, s, sqlf.Sprintf(`
		INSERT INTO repo (id, name, private)
		VALUES (%d, %s, TRUE)`, id, fmt.Sprintf("repo-%d", id)))
	}

	setupData := func() {
		// Add a couple of repos and a user
		executeQuery(t, ctx, s, sqlf.Sprintf(`INSERT INTO users (username) VALUES ('alice')`))
		executeQuery(t, ctx, s, sqlf.Sprintf(`INSERT INTO users (username) VALUES ('bob')`))
		for i := range 2 {
			createRepo(t, i+1)
			if _, err := s.SetRepoPerms(context.Background(), int32(i+1), []authz.UserIDWithExternalAccountID{{UserID: 2}}, authz.SourceRepoSync); err != nil {
				t.Fatal(err)
			}
		}
	}

	cleanupTables := func() {
		t.Helper()

		cleanupPermsTables(t, s)
		cleanupReposTable(t, s)
		cleanupUsersTable(t, s)
	}

	t.Run("Both repos are restricted by default", func(t *testing.T) {
		t.Cleanup(cleanupTables)
		setupData()

		assertUnrestricted(t, 1, false)
		assertUnrestricted(t, 2, false)
	})

	t.Run("Set both repos to unrestricted", func(t *testing.T) {
		t.Cleanup(cleanupTables)
		setupData()

		if err := s.SetRepoPermissionsUnrestricted(ctx, []int32{1, 2}, true); err != nil {
			t.Fatal(err)
		}
		assertUnrestricted(t, 1, true)
		assertUnrestricted(t, 2, true)
	})

	t.Run("Set unrestricted on a repo not in permissions table", func(t *testing.T) {
		t.Cleanup(cleanupTables)
		setupData()

		createRepo(t, 3)
		err := s.SetRepoPermissionsUnrestricted(ctx, []int32{1, 2, 3}, true)
		require.NoError(t, err)

		assertUnrestricted(t, 1, true)
		assertUnrestricted(t, 2, true)
		assertUnrestricted(t, 3, true)
	})

	t.Run("Unset restricted on a repo in and not in permissions table", func(t *testing.T) {
		t.Cleanup(cleanupTables)
		setupData()

		createRepo(t, 3)
		createRepo(t, 4)

		// set permissions on repo 4
		_, err := s.SetRepoPerms(ctx, 4, []authz.UserIDWithExternalAccountID{{UserID: 2}}, authz.SourceRepoSync)
		require.NoError(t, err)
		err = s.SetRepoPermissionsUnrestricted(ctx, []int32{1, 2, 3, 4}, true)
		require.NoError(t, err)
		err = s.SetRepoPermissionsUnrestricted(ctx, []int32{2, 3, 4}, false)
		require.NoError(t, err)

		assertUnrestricted(t, 1, true)
		assertUnrestricted(t, 2, false)
		assertUnrestricted(t, 3, false)
		assertUnrestricted(t, 4, false)
		checkUserRepoPermissions(t, s, sqlf.Sprintf("repo_id = 4"), []authz.Permission{{UserID: 2, RepoID: 4, Source: authz.SourceRepoSync}})
	})
}

func TestPermsStore_SetRepoPerms(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)

	type testUpdate struct {
		repoID int32
		users  []authz.UserIDWithExternalAccountID
	}
	tests := []struct {
		name          string
		updates       []testUpdate
		expectedPerms []authz.Permission
		expectedStats []*SetPermissionsResult
	}{
		{
			name: "empty",
			updates: []testUpdate{{
				repoID: 1,
				users:  []authz.UserIDWithExternalAccountID{},
			}},
			expectedPerms: []authz.Permission{},
			expectedStats: []*SetPermissionsResult{
				{
					Added:   0,
					Removed: 0,
					Found:   0,
				},
			},
		},
		{
			name: "add",
			updates: []testUpdate{
				{
					repoID: 1,
					users: []authz.UserIDWithExternalAccountID{{
						UserID:            1,
						ExternalAccountID: 1,
					}},
				}, {
					repoID: 2,
					users: []authz.UserIDWithExternalAccountID{{
						UserID:            1,
						ExternalAccountID: 1,
					}, {
						UserID:            2,
						ExternalAccountID: 2,
					}},
				}, {
					repoID: 3,
					users: []authz.UserIDWithExternalAccountID{{
						UserID:            3,
						ExternalAccountID: 3,
					}, {
						UserID:            4,
						ExternalAccountID: 4,
					}},
				},
			},
			expectedPerms: []authz.Permission{
				{RepoID: 1, UserID: 1, ExternalAccountID: 1, Source: authz.SourceRepoSync},
				{RepoID: 2, UserID: 1, ExternalAccountID: 1, Source: authz.SourceRepoSync},
				{RepoID: 2, UserID: 2, ExternalAccountID: 2, Source: authz.SourceRepoSync},
				{RepoID: 3, UserID: 3, ExternalAccountID: 3, Source: authz.SourceRepoSync},
				{RepoID: 3, UserID: 4, ExternalAccountID: 4, Source: authz.SourceRepoSync},
			},
			expectedStats: []*SetPermissionsResult{
				{
					Added:   1,
					Removed: 0,
					Found:   1,
				},
				{
					Added:   2,
					Removed: 0,
					Found:   2,
				},
				{
					Added:   2,
					Removed: 0,
					Found:   2,
				},
			},
		},
		{
			name: "add and update",
			updates: []testUpdate{
				{
					repoID: 1,
					users: []authz.UserIDWithExternalAccountID{{
						UserID:            1,
						ExternalAccountID: 1,
					}},
				}, {
					repoID: 1,
					users: []authz.UserIDWithExternalAccountID{{
						UserID:            2,
						ExternalAccountID: 2,
					}, {
						UserID:            3,
						ExternalAccountID: 3,
					}},
				}, {
					repoID: 2,
					users: []authz.UserIDWithExternalAccountID{{
						UserID:            1,
						ExternalAccountID: 1,
					}, {
						UserID:            2,
						ExternalAccountID: 2,
					}},
				}, {
					repoID: 2,
					users: []authz.UserIDWithExternalAccountID{{
						UserID:            3,
						ExternalAccountID: 3,
					}, {
						UserID:            4,
						ExternalAccountID: 4,
					}},
				},
			},
			expectedPerms: []authz.Permission{
				{RepoID: 1, UserID: 2, ExternalAccountID: 2, Source: authz.SourceRepoSync},
				{RepoID: 1, UserID: 3, ExternalAccountID: 3, Source: authz.SourceRepoSync},
				{RepoID: 2, UserID: 3, ExternalAccountID: 3, Source: authz.SourceRepoSync},
				{RepoID: 2, UserID: 4, ExternalAccountID: 4, Source: authz.SourceRepoSync},
			},
			expectedStats: []*SetPermissionsResult{
				{
					Added:   1,
					Removed: 0,
					Found:   1,
				},
				{
					Added:   2,
					Removed: 1,
					Found:   2,
				},
				{
					Added:   2,
					Removed: 0,
					Found:   2,
				},
				{
					Added:   2,
					Removed: 2,
					Found:   2,
				},
			},
		},
		{
			name: "add and clear",
			updates: []testUpdate{{
				repoID: 1,
				users: []authz.UserIDWithExternalAccountID{{
					UserID:            1,
					ExternalAccountID: 1,
				}, {
					UserID:            2,
					ExternalAccountID: 2,
				}, {
					UserID:            3,
					ExternalAccountID: 3,
				}},
			}, {
				repoID: 1,
				users:  []authz.UserIDWithExternalAccountID{},
			}},
			expectedPerms: []authz.Permission{},
			expectedStats: []*SetPermissionsResult{
				{
					Added:   3,
					Removed: 0,
					Found:   3,
				},
				{
					Added:   0,
					Removed: 3,
					Found:   0,
				},
			},
		},
	}

	t.Run("repo-centric update should set permissions", func(t *testing.T) {
		s := perms(logger, db, clock)
		t.Cleanup(func() {
			cleanupUsersTable(t, s)
			cleanupReposTable(t, s)
			cleanupPermsTables(t, s)
		})

		expectedPerms := []authz.Permission{
			{RepoID: 1, UserID: 2, Source: authz.SourceRepoSync},
		}
		setupPermsRelatedEntities(t, s, expectedPerms)

		_, err := s.SetRepoPerms(context.Background(), 1, []authz.UserIDWithExternalAccountID{{UserID: 2}}, authz.SourceRepoSync)
		require.NoError(t, err)

		checkUserRepoPermissions(t, s, nil, expectedPerms)
	})

	t.Run("setting repository as unrestricted", func(t *testing.T) {
		s := setupTestPerms(t, db, clock)

		expectedPerms := []authz.Permission{
			{RepoID: 1, Source: authz.SourceRepoSync},
		}
		setupPermsRelatedEntities(t, s, expectedPerms)

		_, err := s.SetRepoPerms(context.Background(), 1, []authz.UserIDWithExternalAccountID{{UserID: 0}}, authz.SourceRepoSync)
		require.NoError(t, err)

		checkUserRepoPermissions(t, s, nil, expectedPerms)
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := perms(logger, db, clock)
			t.Cleanup(func() {
				cleanupUsersTable(t, s)
				cleanupReposTable(t, s)
				cleanupPermsTables(t, s)
			})

			updates := []authz.Permission{}
			for _, up := range test.updates {
				for _, u := range up.users {
					updates = append(updates, authz.Permission{
						UserID:            u.UserID,
						ExternalAccountID: u.ExternalAccountID,
						RepoID:            up.repoID,
					})
				}
			}
			if len(updates) > 0 {
				setupPermsRelatedEntities(t, s, updates)
			}

			for i, up := range test.updates {
				result, err := s.SetRepoPerms(context.Background(), up.repoID, up.users, authz.SourceRepoSync)
				require.NoError(t, err)

				if diff := cmp.Diff(test.expectedStats[i], result); diff != "" {
					t.Fatal(diff)
				}
			}

			checkUserRepoPermissions(t, s, nil, test.expectedPerms)
		})
	}
}

func TestPermsStore_LoadUserPendingPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)

	t.Run("no matching with different account ID", func(t *testing.T) {
		s := perms(logger, db, clock)
		t.Cleanup(func() {
			cleanupPermsTables(t, s)
		})

		accounts := &extsvc.Accounts{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			AccountIDs:  []string{"bob"},
		}
		rp := &authz.RepoPermissions{
			RepoID: 1,
			Perm:   authz.Read,
		}
		if err := s.SetRepoPendingPermissions(context.Background(), accounts, rp); err != nil {
			t.Fatal(err)
		}

		alice := &authz.UserPendingPermissions{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			BindID:      "alice",
			Perm:        authz.Read,
			Type:        authz.PermRepos,
		}
		err := s.LoadUserPendingPermissions(context.Background(), alice)
		if err != authz.ErrPermsNotFound {
			t.Fatalf("err: want %q but got %q", authz.ErrPermsNotFound, err)
		}
		equal(t, "IDs", 0, len(mapsetToArray(alice.IDs)))
	})

	t.Run("no matching with different service ID", func(t *testing.T) {
		s := perms(logger, db, clock)
		t.Cleanup(func() {
			cleanupPermsTables(t, s)
		})

		accounts := &extsvc.Accounts{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			AccountIDs:  []string{"alice"},
		}
		rp := &authz.RepoPermissions{
			RepoID: 1,
			Perm:   authz.Read,
		}
		if err := s.SetRepoPendingPermissions(context.Background(), accounts, rp); err != nil {
			t.Fatal(err)
		}

		alice := &authz.UserPendingPermissions{
			ServiceType: extsvc.TypeGitLab,
			ServiceID:   "https://gitlab.com/",
			BindID:      "alice",
			Perm:        authz.Read,
			Type:        authz.PermRepos,
		}
		err := s.LoadUserPendingPermissions(context.Background(), alice)
		if err != authz.ErrPermsNotFound {
			t.Fatalf("err: want %q but got %q", authz.ErrPermsNotFound, err)
		}
		equal(t, "IDs", 0, len(mapsetToArray(alice.IDs)))
	})

	t.Run("found matching", func(t *testing.T) {
		s := perms(logger, db, clock)
		t.Cleanup(func() {
			cleanupPermsTables(t, s)
		})

		accounts := &extsvc.Accounts{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			AccountIDs:  []string{"alice"},
		}
		rp := &authz.RepoPermissions{
			RepoID: 1,
			Perm:   authz.Read,
		}
		if err := s.SetRepoPendingPermissions(context.Background(), accounts, rp); err != nil {
			t.Fatal(err)
		}

		alice := &authz.UserPendingPermissions{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			BindID:      "alice",
			Perm:        authz.Read,
			Type:        authz.PermRepos,
		}
		if err := s.LoadUserPendingPermissions(context.Background(), alice); err != nil {
			t.Fatal(err)
		}
		equal(t, "IDs", []int{1}, mapsetToArray(alice.IDs))
		equal(t, "UpdatedAt", now, alice.UpdatedAt.UnixNano())
	})

	t.Run("add and change", func(t *testing.T) {
		s := perms(logger, db, clock)
		t.Cleanup(func() {
			cleanupPermsTables(t, s)
		})

		accounts := &extsvc.Accounts{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			AccountIDs:  []string{"alice", "bob"},
		}
		rp := &authz.RepoPermissions{
			RepoID: 1,
			Perm:   authz.Read,
		}
		if err := s.SetRepoPendingPermissions(context.Background(), accounts, rp); err != nil {
			t.Fatal(err)
		}

		accounts.AccountIDs = []string{"bob", "cindy"}
		rp = &authz.RepoPermissions{
			RepoID: 1,
			Perm:   authz.Read,
		}
		if err := s.SetRepoPendingPermissions(context.Background(), accounts, rp); err != nil {
			t.Fatal(err)
		}

		alice := &authz.UserPendingPermissions{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			BindID:      "alice",
			Perm:        authz.Read,
			Type:        authz.PermRepos,
		}
		if err := s.LoadUserPendingPermissions(context.Background(), alice); err != nil {
			t.Fatal(err)
		}
		equal(t, "IDs", 0, len(mapsetToArray(alice.IDs)))

		bob := &authz.UserPendingPermissions{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			BindID:      "bob",
			Perm:        authz.Read,
			Type:        authz.PermRepos,
		}
		if err := s.LoadUserPendingPermissions(context.Background(), bob); err != nil {
			t.Fatal(err)
		}
		equal(t, "IDs", []int{1}, mapsetToArray(bob.IDs))
		equal(t, "UpdatedAt", now, bob.UpdatedAt.UnixNano())

		cindy := &authz.UserPendingPermissions{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			BindID:      "cindy",
			Perm:        authz.Read,
			Type:        authz.PermRepos,
		}
		if err := s.LoadUserPendingPermissions(context.Background(), cindy); err != nil {
			t.Fatal(err)
		}
		equal(t, "IDs", []int{1}, mapsetToArray(cindy.IDs))
		equal(t, "UpdatedAt", now, cindy.UpdatedAt.UnixNano())
	})
}

func checkUserPendingPermsTable(
	ctx context.Context,
	s *permsStore,
	expects map[extsvc.AccountSpec][]uint32,
) (
	idToSpecs map[int32]extsvc.AccountSpec,
	err error,
) {
	q := `SELECT id, service_type, service_id, bind_id, object_ids_ints FROM user_pending_permissions`
	rows, err := s.Handle().QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}

	// Collect id -> account mappings for later used by checkRepoPendingPermsTable.
	idToSpecs = make(map[int32]extsvc.AccountSpec)
	for rows.Next() {
		var id int32
		var spec extsvc.AccountSpec
		var ids []int64
		if err := rows.Scan(&id, &spec.ServiceType, &spec.ServiceID, &spec.AccountID, pq.Array(&ids)); err != nil {
			return nil, err
		}
		idToSpecs[id] = spec

		intIDs := make([]uint32, 0, len(ids))
		for _, id := range ids {
			intIDs = append(intIDs, uint32(id))
		}

		if expects[spec] == nil {
			return nil, errors.Errorf("unexpected row in table: (spec: %v) -> (ids: %v)", spec, intIDs)
		}
		want := fmt.Sprintf("%v", expects[spec])

		have := fmt.Sprintf("%v", intIDs)
		if have != want {
			return nil, errors.Errorf("intIDs - spec %q: want %q but got %q", spec, want, have)
		}
		delete(expects, spec)
	}

	if err = rows.Close(); err != nil {
		return nil, err
	}

	if len(expects) > 0 {
		return nil, errors.Errorf("missing rows from table: %v", expects)
	}

	return idToSpecs, nil
}

func checkRepoPendingPermsTable(
	ctx context.Context,
	s *permsStore,
	idToSpecs map[int32]extsvc.AccountSpec,
	expects map[int32][]extsvc.AccountSpec,
) error {
	rows, err := s.Handle().QueryContext(ctx, `SELECT repo_id, user_ids_ints FROM repo_pending_permissions`)
	if err != nil {
		return err
	}

	for rows.Next() {
		var id int32
		var ids []int64
		if err := rows.Scan(&id, pq.Array(&ids)); err != nil {
			return err
		}

		intIDs := make([]int, 0, len(ids))
		for _, id := range ids {
			intIDs = append(intIDs, int(id))
		}

		if expects[id] == nil {
			return errors.Errorf("unexpected row in table: (id: %v) -> (ids: %v)", id, intIDs)
		}

		haveSpecs := make([]extsvc.AccountSpec, 0, len(intIDs))
		for _, userID := range intIDs {
			spec, ok := idToSpecs[int32(userID)]
			if !ok {
				continue
			}

			haveSpecs = append(haveSpecs, spec)
		}
		wantSpecs := expects[id]

		// Verify Specs are the same, the ordering might not be the same but the elements/length are.
		if len(wantSpecs) != len(haveSpecs) {
			return errors.Errorf("initIDs - id %d: want %q but got %q", id, wantSpecs, haveSpecs)
		}
		wantSpecsSet := map[extsvc.AccountSpec]struct{}{}
		for _, spec := range wantSpecs {
			wantSpecsSet[spec] = struct{}{}
		}

		for _, spec := range haveSpecs {
			if _, ok := wantSpecsSet[spec]; !ok {
				return errors.Errorf("initIDs - id %d: want %q but got %q", id, wantSpecs, haveSpecs)
			}
		}

		delete(expects, id)
	}

	if err = rows.Close(); err != nil {
		return err
	}

	if len(expects) > 0 {
		return errors.Errorf("missing rows from table: %v", expects)
	}

	return nil
}

func TestPermsStore_SetRepoPendingPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)

	alice := extsvc.AccountSpec{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountID:   "alice",
	}
	bob := extsvc.AccountSpec{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountID:   "bob",
	}
	cindy := extsvc.AccountSpec{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountID:   "cindy",
	}
	cindyGitHub := extsvc.AccountSpec{
		ServiceType: "github",
		ServiceID:   "https://github.com/",
		AccountID:   "cindy",
	}
	const countToExceedParameterLimit = 11000 // ~ 65535 / 6 parameters per row

	type update struct {
		accounts *extsvc.Accounts
		perm     *authz.RepoPermissions
	}
	tests := []struct {
		name                   string
		slowTest               bool
		updates                []update
		expectUserPendingPerms map[extsvc.AccountSpec][]uint32 // account -> object_ids
		expectRepoPendingPerms map[int32][]extsvc.AccountSpec  // repo_id -> accounts
	}{
		{
			name: "empty",
			updates: []update{
				{
					accounts: &extsvc.Accounts{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountIDs:  nil,
					},
					perm: &authz.RepoPermissions{
						RepoID: 1,
						Perm:   authz.Read,
					},
				},
			},
		},
		{
			name: "add",
			updates: []update{
				{
					accounts: &extsvc.Accounts{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountIDs:  []string{"alice"},
					},
					perm: &authz.RepoPermissions{
						RepoID: 1,
						Perm:   authz.Read,
					},
				}, {
					accounts: &extsvc.Accounts{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountIDs:  []string{"alice", "bob"},
					},
					perm: &authz.RepoPermissions{
						RepoID: 2,
						Perm:   authz.Read,
					},
				}, {
					accounts: &extsvc.Accounts{
						ServiceType: "github",
						ServiceID:   "https://github.com/",
						AccountIDs:  []string{"cindy"},
					},
					perm: &authz.RepoPermissions{
						RepoID: 3,
						Perm:   authz.Read,
					},
				},
			},
			expectUserPendingPerms: map[extsvc.AccountSpec][]uint32{
				alice:       {1, 2},
				bob:         {2},
				cindyGitHub: {3},
			},
			expectRepoPendingPerms: map[int32][]extsvc.AccountSpec{
				1: {alice},
				2: {alice, bob},
				3: {cindyGitHub},
			},
		},
		{
			name: "add and update",
			updates: []update{
				{
					accounts: &extsvc.Accounts{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountIDs:  []string{"alice", "bob"},
					},
					perm: &authz.RepoPermissions{
						RepoID: 1,
						Perm:   authz.Read,
					},
				}, {
					accounts: &extsvc.Accounts{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountIDs:  []string{"bob", "cindy"},
					},
					perm: &authz.RepoPermissions{
						RepoID: 1,
						Perm:   authz.Read,
					},
				}, {
					accounts: &extsvc.Accounts{
						ServiceType: "github",
						ServiceID:   "https://github.com/",
						AccountIDs:  []string{"cindy"},
					},
					perm: &authz.RepoPermissions{
						RepoID: 2,
						Perm:   authz.Read,
					},
				},
			},
			expectUserPendingPerms: map[extsvc.AccountSpec][]uint32{
				alice:       {},
				bob:         {1},
				cindy:       {1},
				cindyGitHub: {2},
			},
			expectRepoPendingPerms: map[int32][]extsvc.AccountSpec{
				1: {bob, cindy},
				2: {cindyGitHub},
			},
		},
		{
			name: "add and clear",
			updates: []update{
				{
					accounts: &extsvc.Accounts{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountIDs:  []string{"alice", "bob", "cindy"},
					},
					perm: &authz.RepoPermissions{
						RepoID: 1,
						Perm:   authz.Read,
					},
				}, {
					accounts: &extsvc.Accounts{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountIDs:  []string{},
					},
					perm: &authz.RepoPermissions{
						RepoID: 1,
						Perm:   authz.Read,
					},
				},
			},
			expectUserPendingPerms: map[extsvc.AccountSpec][]uint32{
				alice: {},
				bob:   {},
				cindy: {},
			},
			expectRepoPendingPerms: map[int32][]extsvc.AccountSpec{
				1: {},
			},
		},
		{
			name:     postgresParameterLimitTest,
			slowTest: true,
			updates: func() []update {
				u := update{
					accounts: &extsvc.Accounts{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountIDs:  make([]string, countToExceedParameterLimit),
					},
					perm: &authz.RepoPermissions{
						RepoID: 1,
						Perm:   authz.Read,
					},
				}
				for i := 1; i <= countToExceedParameterLimit; i++ {
					u.accounts.AccountIDs[i-1] = fmt.Sprintf("%d", i)
				}
				return []update{u}
			}(),
			expectUserPendingPerms: func() map[extsvc.AccountSpec][]uint32 {
				perms := make(map[extsvc.AccountSpec][]uint32, countToExceedParameterLimit)
				for i := 1; i <= countToExceedParameterLimit; i++ {
					perms[extsvc.AccountSpec{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountID:   fmt.Sprintf("%d", i),
					}] = []uint32{1}
				}
				return perms
			}(),
			expectRepoPendingPerms: map[int32][]extsvc.AccountSpec{
				1: func() []extsvc.AccountSpec {
					accounts := make([]extsvc.AccountSpec, countToExceedParameterLimit)
					for i := 1; i <= countToExceedParameterLimit; i++ {
						accounts[i-1] = extsvc.AccountSpec{
							ServiceType: authz.SourcegraphServiceType,
							ServiceID:   authz.SourcegraphServiceID,
							AccountID:   fmt.Sprintf("%d", i),
						}
					}
					return accounts
				}(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.slowTest && !*slowTests {
				t.Skip("slow-tests not enabled")
			}

			s := perms(logger, db, clock)
			t.Cleanup(func() {
				cleanupPermsTables(t, s)
			})

			ctx := context.Background()

			for _, update := range test.updates {
				const numOps = 30
				g, ctx := errgroup.WithContext(ctx)
				for range numOps {
					// Make local copy to prevent race conditions
					accounts := *update.accounts
					perm := &authz.RepoPermissions{
						RepoID:    update.perm.RepoID,
						Perm:      update.perm.Perm,
						UpdatedAt: update.perm.UpdatedAt,
					}
					if update.perm.UserIDs != nil {
						perm.UserIDs = update.perm.UserIDs
					}
					g.Go(func() error {
						return s.SetRepoPendingPermissions(ctx, &accounts, perm)
					})
				}
				if err := g.Wait(); err != nil {
					t.Fatal(err)
				}
			}

			// Query and check rows in "user_pending_permissions" table.
			idToSpecs, err := checkUserPendingPermsTable(ctx, s, test.expectUserPendingPerms)
			if err != nil {
				t.Fatal("user_pending_permissions:", err)
			}

			// Query and check rows in "repo_pending_permissions" table.
			err = checkRepoPendingPermsTable(ctx, s, idToSpecs, test.expectRepoPendingPerms)
			if err != nil {
				t.Fatal("repo_pending_permissions:", err)
			}
		})
	}
}

func TestPermsStore_ListPendingUsers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)

	type update struct {
		accounts *extsvc.Accounts
		perm     *authz.RepoPermissions
	}
	tests := []struct {
		name               string
		updates            []update
		expectPendingUsers []string
	}{
		{
			name:               "no user with pending permissions",
			expectPendingUsers: nil,
		},
		{
			name: "has user with pending permissions",
			updates: []update{
				{
					accounts: &extsvc.Accounts{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountIDs:  []string{"alice"},
					},
					perm: &authz.RepoPermissions{
						RepoID: 1,
						Perm:   authz.Read,
					},
				},
			},
			expectPendingUsers: []string{"alice"},
		},
		{
			name: "has user but with empty object_ids",
			updates: []update{
				{
					accounts: &extsvc.Accounts{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountIDs:  []string{"bob@example.com"},
					},
					perm: &authz.RepoPermissions{
						RepoID: 1,
						Perm:   authz.Read,
					},
				}, {
					accounts: &extsvc.Accounts{
						ServiceType: authz.SourcegraphServiceType,
						ServiceID:   authz.SourcegraphServiceID,
						AccountIDs:  nil,
					},
					perm: &authz.RepoPermissions{
						RepoID: 1,
						Perm:   authz.Read,
					},
				},
			},
			expectPendingUsers: nil,
		},
		{
			name: "has user but with different service ID",
			updates: []update{
				{
					accounts: &extsvc.Accounts{
						ServiceType: extsvc.TypeGitLab,
						ServiceID:   "https://gitlab.com/",
						AccountIDs:  []string{"bob@example.com"},
					},
					perm: &authz.RepoPermissions{
						RepoID: 1,
						Perm:   authz.Read,
					},
				},
			},
			expectPendingUsers: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := perms(logger, db, clock)
			t.Cleanup(func() {
				cleanupPermsTables(t, s)
			})

			ctx := context.Background()

			for _, update := range test.updates {
				tmp := &authz.RepoPermissions{
					RepoID:    update.perm.RepoID,
					Perm:      update.perm.Perm,
					UpdatedAt: update.perm.UpdatedAt,
				}
				if update.perm.UserIDs != nil {
					tmp.UserIDs = update.perm.UserIDs
				}
				if err := s.SetRepoPendingPermissions(ctx, update.accounts, tmp); err != nil {
					t.Fatal(err)
				}
			}

			bindIDs, err := s.ListPendingUsers(ctx, authz.SourcegraphServiceType, authz.SourcegraphServiceID)
			if err != nil {
				t.Fatal(err)
			}
			equal(t, "bindIDs", test.expectPendingUsers, bindIDs)
		})
	}
}

func TestPermsStore_GrantPendingPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, clock)
	ctx := context.Background()

	alice := extsvc.AccountSpec{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountID:   "alice",
	}
	bob := extsvc.AccountSpec{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountID:   "bob",
	}

	type ExternalAccount struct {
		ID     int32
		UserID int32
		extsvc.AccountSpec
	}

	setupExternalAccounts := func(accounts []ExternalAccount) {
		users := make(map[int32]*sqlf.Query)
		values := make([]*sqlf.Query, 0, len(accounts))
		for _, a := range accounts {
			if _, ok := users[a.UserID]; !ok {
				users[a.UserID] = sqlf.Sprintf("(%s::integer, %s::text)", a.UserID, fmt.Sprintf("user-%d", a.UserID))
			}
			values = append(values, sqlf.Sprintf("(%s::integer, %s::integer, %s::text, %s::text, %s::text, %s::text)",
				a.ID, a.UserID, a.ServiceType, a.ServiceID, a.AccountID, a.ClientID))
		}
		userQuery := sqlf.Sprintf("INSERT INTO users(id, username) VALUES %s", sqlf.Join(maps.Values(users), ","))
		executeQuery(t, ctx, s, userQuery)

		accountQuery := sqlf.Sprintf("INSERT INTO user_external_accounts(id, user_id, service_type, service_id, account_id, client_id) VALUES %s", sqlf.Join(values, ","))
		executeQuery(t, ctx, s, accountQuery)
	}

	// this limit will also exceed param limit for user_repo_permissions,
	// as we are sending 6 parameter per row
	const countToExceedParameterLimit = 17000 // ~ 65535 / 4 parameters per row

	type pending struct {
		accounts *extsvc.Accounts
		perm     *authz.RepoPermissions
	}
	type update struct {
		regulars []*authz.RepoPermissions
		pendings []pending
	}
	tests := []struct {
		name                   string
		slowTest               bool
		updates                []update
		grants                 []*authz.UserGrantPermissions
		expectUserRepoPerms    []authz.Permission
		expectUserPerms        map[int32][]uint32              // user_id -> object_ids
		expectRepoPerms        map[int32][]uint32              // repo_id -> user_ids
		expectUserPendingPerms map[extsvc.AccountSpec][]uint32 // account -> object_ids
		expectRepoPendingPerms map[int32][]extsvc.AccountSpec  // repo_id -> accounts

		upsertRepoPermissionsPageSize int
	}{
		{
			name: "empty",
			grants: []*authz.UserGrantPermissions{
				{
					UserID:                1,
					UserExternalAccountID: 1,
					ServiceType:           authz.SourcegraphServiceType,
					ServiceID:             authz.SourcegraphServiceID,
					AccountID:             "alice",
				},
			},
			expectUserRepoPerms: []authz.Permission{},
		},
		{
			name: "no matching pending permissions",
			updates: []update{
				{
					regulars: []*authz.RepoPermissions{
						{
							RepoID:  1,
							Perm:    authz.Read,
							UserIDs: toMapset(1),
						}, {
							RepoID:  2,
							Perm:    authz.Read,
							UserIDs: toMapset(1, 2),
						},
					},
					pendings: []pending{
						{
							accounts: &extsvc.Accounts{
								ServiceType: authz.SourcegraphServiceType,
								ServiceID:   authz.SourcegraphServiceID,
								AccountIDs:  []string{"alice"},
							},
							perm: &authz.RepoPermissions{
								RepoID: 1,
								Perm:   authz.Read,
							},
						}, {
							accounts: &extsvc.Accounts{
								ServiceType: authz.SourcegraphServiceType,
								ServiceID:   authz.SourcegraphServiceID,
								AccountIDs:  []string{"bob"},
							},
							perm: &authz.RepoPermissions{
								RepoID: 2,
								Perm:   authz.Read,
							},
						},
					},
				},
			},
			grants: []*authz.UserGrantPermissions{
				{
					UserID:                1,
					UserExternalAccountID: 3,
					ServiceType:           authz.SourcegraphServiceType,
					ServiceID:             authz.SourcegraphServiceID,
					AccountID:             "cindy",
				},
			},
			expectUserRepoPerms: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1, Source: authz.SourceRepoSync},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2, Source: authz.SourceRepoSync},
				{UserID: 2, ExternalAccountID: 2, RepoID: 2, Source: authz.SourceRepoSync},
			},
			expectUserPerms: map[int32][]uint32{
				1: {1, 2},
				2: {2},
			},
			expectRepoPerms: map[int32][]uint32{
				1: {1},
				2: {1, 2},
			},
			expectUserPendingPerms: map[extsvc.AccountSpec][]uint32{
				alice: {1},
				bob:   {2},
			},
			expectRepoPendingPerms: map[int32][]extsvc.AccountSpec{
				1: {alice},
				2: {bob},
			},
		},
		{
			name: "grant pending permission",
			updates: []update{
				{
					regulars: []*authz.RepoPermissions{},
					pendings: []pending{{
						accounts: &extsvc.Accounts{
							ServiceType: authz.SourcegraphServiceType,
							ServiceID:   authz.SourcegraphServiceID,
							AccountIDs:  []string{"alice"},
						},
						perm: &authz.RepoPermissions{
							RepoID: 1,
							Perm:   authz.Read,
						},
					}},
				},
			},
			grants: []*authz.UserGrantPermissions{{
				UserID:                1,
				UserExternalAccountID: 1,
				ServiceType:           authz.SourcegraphServiceType,
				ServiceID:             authz.SourcegraphServiceID,
				AccountID:             "alice",
			}},
			expectUserRepoPerms: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1, Source: authz.SourceUserSync},
			},
			expectUserPerms: map[int32][]uint32{
				1: {1},
			},
			expectRepoPerms: map[int32][]uint32{
				1: {1},
			},
			expectUserPendingPerms: map[extsvc.AccountSpec][]uint32{},
			expectRepoPendingPerms: map[int32][]extsvc.AccountSpec{
				1: {},
			},
		},
		{
			name: "grant pending permission with existing permissions",
			updates: []update{
				{
					regulars: []*authz.RepoPermissions{
						{
							RepoID:  1,
							Perm:    authz.Read,
							UserIDs: toMapset(1),
						},
					},
					pendings: []pending{{
						accounts: &extsvc.Accounts{
							ServiceType: authz.SourcegraphServiceType,
							ServiceID:   authz.SourcegraphServiceID,
							AccountIDs:  []string{"alice"},
						},
						perm: &authz.RepoPermissions{
							RepoID: 2,
							Perm:   authz.Read,
						},
					}},
				},
			},
			grants: []*authz.UserGrantPermissions{{
				UserID:                1,
				UserExternalAccountID: 1,
				ServiceType:           authz.SourcegraphServiceType,
				ServiceID:             authz.SourcegraphServiceID,
				AccountID:             "alice",
			}},
			expectUserRepoPerms: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1, Source: authz.SourceRepoSync},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2, Source: authz.SourceUserSync},
			},
			expectUserPerms: map[int32][]uint32{
				1: {1, 2},
			},
			expectRepoPerms: map[int32][]uint32{
				1: {1},
				2: {1},
			},
			expectUserPendingPerms: map[extsvc.AccountSpec][]uint32{},
			expectRepoPendingPerms: map[int32][]extsvc.AccountSpec{
				2: {},
			},
		},
		{
			name: "union matching pending permissions for same account ID but different service IDs",
			updates: []update{
				{
					regulars: []*authz.RepoPermissions{
						{
							RepoID:  1,
							Perm:    authz.Read,
							UserIDs: toMapset(1),
						}, {
							RepoID:  2,
							Perm:    authz.Read,
							UserIDs: toMapset(1, 2),
						},
					},
					pendings: []pending{
						{
							accounts: &extsvc.Accounts{
								ServiceType: authz.SourcegraphServiceType,
								ServiceID:   authz.SourcegraphServiceID,
								AccountIDs:  []string{"alice"},
							},
							perm: &authz.RepoPermissions{
								RepoID: 1,
								Perm:   authz.Read,
							},
						},
						{
							accounts: &extsvc.Accounts{
								ServiceType: extsvc.TypeGitLab,
								ServiceID:   "https://gitlab.com/",
								AccountIDs:  []string{"alice"},
							},
							perm: &authz.RepoPermissions{
								RepoID: 2,
								Perm:   authz.Read,
							},
						},
						{
							accounts: &extsvc.Accounts{
								ServiceType: authz.SourcegraphServiceType,
								ServiceID:   authz.SourcegraphServiceID,
								AccountIDs:  []string{"bob"},
							},
							perm: &authz.RepoPermissions{
								RepoID: 3,
								Perm:   authz.Read,
							},
						},
					},
				},
			},
			grants: []*authz.UserGrantPermissions{
				{
					UserID:                3,
					UserExternalAccountID: 3,
					ServiceType:           authz.SourcegraphServiceType,
					ServiceID:             authz.SourcegraphServiceID,
					AccountID:             "alice",
				}, {
					UserID:                3,
					UserExternalAccountID: 4,
					ServiceType:           extsvc.TypeGitLab,
					ServiceID:             "https://gitlab.com/",
					AccountID:             "alice",
				},
			},
			expectUserRepoPerms: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1, Source: authz.SourceRepoSync},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2, Source: authz.SourceRepoSync},
				{UserID: 2, ExternalAccountID: 2, RepoID: 2, Source: authz.SourceRepoSync},
				{UserID: 3, ExternalAccountID: 3, RepoID: 1, Source: authz.SourceUserSync},
				{UserID: 3, ExternalAccountID: 4, RepoID: 2, Source: authz.SourceUserSync},
			},
			expectUserPerms: map[int32][]uint32{
				1: {1, 2},
				2: {2},
				3: {1, 2},
			},
			expectRepoPerms: map[int32][]uint32{
				1: {1, 3},
				2: {1, 2, 3},
			},
			expectUserPendingPerms: map[extsvc.AccountSpec][]uint32{
				bob: {3},
			},
			expectRepoPendingPerms: map[int32][]extsvc.AccountSpec{
				1: {},
				2: {},
				3: {bob},
			},
		},
		{
			name: "union matching pending permissions for same service ID but different account IDs",
			updates: []update{
				{
					regulars: []*authz.RepoPermissions{
						{
							RepoID:  1,
							Perm:    authz.Read,
							UserIDs: toMapset(1),
						}, {
							RepoID:  2,
							Perm:    authz.Read,
							UserIDs: toMapset(1, 2),
						},
					},
					pendings: []pending{
						{
							accounts: &extsvc.Accounts{
								ServiceType: authz.SourcegraphServiceType,
								ServiceID:   authz.SourcegraphServiceID,
								AccountIDs:  []string{"alice@example.com"},
							},
							perm: &authz.RepoPermissions{
								RepoID: 1,
								Perm:   authz.Read,
							},
						}, {
							accounts: &extsvc.Accounts{
								ServiceType: authz.SourcegraphServiceType,
								ServiceID:   authz.SourcegraphServiceID,
								AccountIDs:  []string{"alice2@example.com"},
							},
							perm: &authz.RepoPermissions{
								RepoID: 2,
								Perm:   authz.Read,
							},
						},
					},
				},
			},
			grants: []*authz.UserGrantPermissions{
				{
					UserID:                3,
					UserExternalAccountID: 3,
					ServiceType:           authz.SourcegraphServiceType,
					ServiceID:             authz.SourcegraphServiceID,
					AccountID:             "alice@example.com",
				}, {
					UserID:                3,
					UserExternalAccountID: 4,
					ServiceType:           authz.SourcegraphServiceType,
					ServiceID:             authz.SourcegraphServiceID,
					AccountID:             "alice2@example.com",
				},
			},
			expectUserRepoPerms: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1, Source: authz.SourceRepoSync},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2, Source: authz.SourceRepoSync},
				{UserID: 2, ExternalAccountID: 2, RepoID: 2, Source: authz.SourceRepoSync},
				{UserID: 3, ExternalAccountID: 3, RepoID: 1, Source: authz.SourceUserSync},
				{UserID: 3, ExternalAccountID: 4, RepoID: 2, Source: authz.SourceUserSync},
			},
			expectUserPerms: map[int32][]uint32{
				1: {1, 2},
				2: {2},
				3: {1, 2},
			},
			expectRepoPerms: map[int32][]uint32{
				1: {1, 3},
				2: {1, 2, 3},
			},
			expectUserPendingPerms: map[extsvc.AccountSpec][]uint32{},
			expectRepoPendingPerms: map[int32][]extsvc.AccountSpec{
				1: {},
				2: {},
			},
		},
		{
			name:                          "grant pending permission with pagination",
			upsertRepoPermissionsPageSize: 2,
			updates: []update{
				{
					regulars: []*authz.RepoPermissions{},
					pendings: []pending{{
						accounts: &extsvc.Accounts{
							ServiceType: authz.SourcegraphServiceType,
							ServiceID:   authz.SourcegraphServiceID,
							AccountIDs:  []string{"alice"},
						},
						perm: &authz.RepoPermissions{
							RepoID: 1,
							Perm:   authz.Read,
						},
					}, {
						accounts: &extsvc.Accounts{
							ServiceType: authz.SourcegraphServiceType,
							ServiceID:   authz.SourcegraphServiceID,
							AccountIDs:  []string{"alice"},
						},
						perm: &authz.RepoPermissions{
							RepoID: 2,
							Perm:   authz.Read,
						},
					}, {
						accounts: &extsvc.Accounts{
							ServiceType: authz.SourcegraphServiceType,
							ServiceID:   authz.SourcegraphServiceID,
							AccountIDs:  []string{"alice"},
						},
						perm: &authz.RepoPermissions{
							RepoID: 3,
							Perm:   authz.Read,
						},
					}},
				},
			},
			grants: []*authz.UserGrantPermissions{{
				UserID:                1,
				UserExternalAccountID: 1,
				ServiceType:           authz.SourcegraphServiceType,
				ServiceID:             authz.SourcegraphServiceID,
				AccountID:             "alice",
			}},
			expectUserRepoPerms: []authz.Permission{
				{UserID: 1, ExternalAccountID: 1, RepoID: 1, Source: authz.SourceUserSync},
				{UserID: 1, ExternalAccountID: 1, RepoID: 2, Source: authz.SourceUserSync},
				{UserID: 1, ExternalAccountID: 1, RepoID: 3, Source: authz.SourceUserSync},
			},
			expectUserPerms: map[int32][]uint32{
				1: {1, 2, 3},
			},
			expectRepoPerms: map[int32][]uint32{
				1: {1},
				2: {1},
				3: {1},
			},
			expectUserPendingPerms: map[extsvc.AccountSpec][]uint32{},
			expectRepoPendingPerms: map[int32][]extsvc.AccountSpec{
				1: {},
				2: {},
				3: {},
			},
		},
		{
			name:     postgresParameterLimitTest,
			slowTest: true,
			updates: []update{
				{
					regulars: []*authz.RepoPermissions{},
					pendings: func() []pending {
						accounts := &extsvc.Accounts{
							ServiceType: authz.SourcegraphServiceType,
							ServiceID:   authz.SourcegraphServiceID,
							AccountIDs:  []string{"alice"},
						}
						pendings := make([]pending, countToExceedParameterLimit)
						for i := 1; i <= countToExceedParameterLimit; i += 1 {
							pendings[i-1] = pending{
								accounts: accounts,
								perm: &authz.RepoPermissions{
									RepoID: int32(i),
									Perm:   authz.Read,
								},
							}
						}
						return pendings
					}(),
				},
			},
			grants: []*authz.UserGrantPermissions{
				{
					UserID:                1,
					UserExternalAccountID: 1,
					ServiceType:           authz.SourcegraphServiceType,
					ServiceID:             authz.SourcegraphServiceID,
					AccountID:             "alice",
				},
			},
			expectUserRepoPerms: func() []authz.Permission {
				perms := make([]authz.Permission, 0, countToExceedParameterLimit)
				for i := 1; i <= countToExceedParameterLimit; i += 1 {
					perms = append(perms, authz.Permission{
						UserID:            1,
						ExternalAccountID: 1,
						RepoID:            int32(i),
						Source:            authz.SourceUserSync,
					})
				}
				return perms
			}(),
			expectUserPerms: func() map[int32][]uint32 {
				repos := make([]uint32, countToExceedParameterLimit)
				for i := 1; i <= countToExceedParameterLimit; i += 1 {
					repos[i-1] = uint32(i)
				}
				return map[int32][]uint32{1: repos}
			}(),
			expectRepoPerms: func() map[int32][]uint32 {
				repos := make(map[int32][]uint32, countToExceedParameterLimit)
				for i := 1; i <= countToExceedParameterLimit; i += 1 {
					repos[int32(i)] = []uint32{1}
				}
				return repos
			}(),
			expectUserPendingPerms: map[extsvc.AccountSpec][]uint32{},
			expectRepoPendingPerms: func() map[int32][]extsvc.AccountSpec {
				repos := make(map[int32][]extsvc.AccountSpec, countToExceedParameterLimit)
				for i := 1; i <= countToExceedParameterLimit; i += 1 {
					repos[int32(i)] = []extsvc.AccountSpec{}
				}
				return repos
			}(),
		},
	}
	for _, test := range tests {
		if t.Failed() {
			break
		}

		t.Run(test.name, func(t *testing.T) {
			if test.slowTest && !*slowTests {
				t.Skip("slow-tests not enabled")
			}

			if test.upsertRepoPermissionsPageSize > 0 {
				upsertRepoPermissionsPageSize = test.upsertRepoPermissionsPageSize
			}

			t.Cleanup(func() {
				cleanupPermsTables(t, s)
				cleanupUsersTable(t, s)
				cleanupReposTable(t, s)

				if test.upsertRepoPermissionsPageSize > 0 {
					upsertRepoPermissionsPageSize = defaultUpsertRepoPermissionsPageSize
				}
			})

			accounts := make([]ExternalAccount, 0)
			for _, grant := range test.grants {
				accounts = append(accounts, ExternalAccount{
					ID:     grant.UserExternalAccountID,
					UserID: grant.UserID,
					AccountSpec: extsvc.AccountSpec{
						ServiceType: grant.ServiceType,
						ServiceID:   grant.ServiceID,
						AccountID:   grant.AccountID,
						ClientID:    "client_id",
					},
				})
			}

			// create related entities
			if len(accounts) > 0 {
				setupExternalAccounts(accounts)
			}

			if len(test.expectUserRepoPerms) > 0 {
				setupPermsRelatedEntities(t, s, test.expectUserRepoPerms)
			}

			for _, update := range test.updates {
				for _, p := range update.regulars {
					repoID := p.RepoID
					users := make([]authz.UserIDWithExternalAccountID, 0, len(p.UserIDs))
					for userID := range p.UserIDs {
						users = append(users, authz.UserIDWithExternalAccountID{
							UserID:            userID,
							ExternalAccountID: userID,
						})
					}

					if _, err := s.SetRepoPerms(ctx, repoID, users, authz.SourceRepoSync); err != nil {
						t.Fatal(err)
					}
				}
				for _, p := range update.pendings {
					if err := s.SetRepoPendingPermissions(ctx, p.accounts, p.perm); err != nil {
						t.Fatal(err)
					}
				}
			}

			for _, grant := range test.grants {
				err := s.GrantPendingPermissions(ctx, grant)
				if err != nil {
					t.Fatal(err)
				}
			}

			checkUserRepoPermissions(t, s, nil, test.expectUserRepoPerms)

			// Query and check rows in "user_pending_permissions" table.
			idToSpecs, err := checkUserPendingPermsTable(ctx, s, test.expectUserPendingPerms)
			if err != nil {
				t.Fatal("user_pending_permissions:", err)
			}

			// Query and check rows in "repo_pending_permissions" table.
			err = checkRepoPendingPermsTable(ctx, s, idToSpecs, test.expectRepoPendingPerms)
			if err != nil {
				t.Fatal("repo_pending_permissions:", err)
			}
		})
	}
}

// This test is used to ensure we ignore invalid pending user IDs on updating repository pending permissions
// because permissions have been granted for those users.
func TestPermsStore_SetPendingPermissionsAfterGrant(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, clock)
	defer cleanupPermsTables(t, s)

	ctx := context.Background()

	setupPermsRelatedEntities(t, s, []authz.Permission{
		{
			UserID:            1,
			RepoID:            1,
			ExternalAccountID: 1,
		},
		{
			UserID:            2,
			RepoID:            1,
			ExternalAccountID: 2,
		},
	})

	// Set up pending permissions for at least two users
	if err := s.SetRepoPendingPermissions(ctx, &extsvc.Accounts{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountIDs:  []string{"alice", "bob"},
	}, &authz.RepoPermissions{
		RepoID: 1,
		Perm:   authz.Read,
	}); err != nil {
		t.Fatal(err)
	}

	// Now grant permissions for these two users, which effectively remove corresponding rows
	// from the `user_pending_permissions` table.
	if err := s.GrantPendingPermissions(ctx, &authz.UserGrantPermissions{
		UserID:      1,
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountID:   "alice",
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.GrantPendingPermissions(ctx, &authz.UserGrantPermissions{
		UserID:      2,
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountID:   "bob",
	}); err != nil {
		t.Fatal(err)
	}

	// Now the `repo_pending_permissions` table has references to these two deleted rows,
	// it should just ignore them.
	if err := s.SetRepoPendingPermissions(ctx, &extsvc.Accounts{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountIDs:  []string{}, // Intentionally empty to cover "no-update" case
	}, &authz.RepoPermissions{
		RepoID: 1,
		Perm:   authz.Read,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPermsStore_DeleteAllUserPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, clock)
	t.Cleanup(func() {
		cleanupPermsTables(t, s)
		cleanupUsersTable(t, s)
		cleanupReposTable(t, s)
	})

	ctx := context.Background()

	// Create 2 users and their external accounts and repos
	// Set up test users and external accounts
	extSQL := `
	INSERT INTO user_external_accounts(user_id, service_type, service_id, account_id, client_id, created_at, updated_at, deleted_at, expired_at)
		VALUES(%s, %s, %s, %s, %s, %s, %s, %s, %s)
	`
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('alice')`), // ID=1
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('bob')`),   // ID=2

		sqlf.Sprintf(extSQL, 1, extsvc.TypeGitLab, "https://gitlab.com/", "alice_gitlab", "alice_gitlab_client_id", clock(), clock(), nil, nil), // ID=1
		sqlf.Sprintf(extSQL, 1, "github", "https://github.com/", "alice_github", "alice_github_client_id", clock(), clock(), nil, nil),          // ID=2
		sqlf.Sprintf(extSQL, 2, extsvc.TypeGitLab, "https://gitlab.com/", "bob_gitlab", "bob_gitlab_client_id", clock(), clock(), nil, nil),     // ID=3

		sqlf.Sprintf(`INSERT INTO repo(name, private) VALUES('private_repo_1', TRUE)`), // ID=1
		sqlf.Sprintf(`INSERT INTO repo(name, private) VALUES('private_repo_2', TRUE)`), // ID=2
	}
	for _, q := range qs {
		executeQuery(t, ctx, s, q)
	}

	// Set permissions for user 1 and 2
	for _, userID := range []int32{1, 2} {
		for _, repoID := range []int32{1, 2} {
			if _, err := s.SetUserExternalAccountPerms(ctx, authz.UserIDWithExternalAccountID{UserID: userID, ExternalAccountID: repoID}, []int32{repoID}, authz.SourceUserSync); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Remove all permissions for the user=1
	if err := s.DeleteAllUserPermissions(ctx, 1); err != nil {
		t.Fatal(err)
	}

	// Check user=1 should not have any legacy permissions now
	p, err := s.LoadUserPermissions(ctx, 1)
	require.NoError(t, err)
	assert.Zero(t, len(p))

	getUserRepoPermissions := func(userID int) ([]int32, error) {
		unifiedQuery := `SELECT repo_id FROM user_repo_permissions WHERE user_id = %d`
		q := sqlf.Sprintf(unifiedQuery, userID)
		return basestore.ScanInt32s(db.Handle().QueryContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...))
	}

	// Check user=1 should not have any permissions now
	results, err := getUserRepoPermissions(1)
	assert.NoError(t, err)
	assert.Nil(t, results)

	// Check user=2 shoud still have legacy permissions
	p, err = s.LoadUserPermissions(ctx, 2)
	require.NoError(t, err)
	gotIDs := make([]int32, len(p))
	for i, perm := range p {
		gotIDs[i] = perm.RepoID
	}
	slices.Sort(gotIDs)
	equal(t, "legacy IDs", []int32{1, 2}, gotIDs)

	// Check user=2 should still have permissions
	results, err = getUserRepoPermissions(2)
	assert.NoError(t, err)
	equal(t, "unified IDs", []int32{1, 2}, results)
}

func TestPermsStore_DeleteAllUserPendingPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, clock)
	t.Cleanup(func() {
		cleanupPermsTables(t, s)
	})

	ctx := context.Background()

	accounts := &extsvc.Accounts{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountIDs:  []string{"alice", "bob"},
	}

	// Set pending permissions for "alice" and "bob"
	if err := s.SetRepoPendingPermissions(ctx, accounts, &authz.RepoPermissions{
		RepoID: 1,
		Perm:   authz.Read,
	}); err != nil {
		t.Fatal(err)
	}

	// Remove all pending permissions for "alice"
	accounts.AccountIDs = []string{"alice"}
	if err := s.DeleteAllUserPendingPermissions(ctx, accounts); err != nil {
		t.Fatal(err)
	}

	// Check alice should not have any pending permissions now
	err := s.LoadUserPendingPermissions(ctx, &authz.UserPendingPermissions{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		BindID:      "alice",
		Perm:        authz.Read,
		Type:        authz.PermRepos,
	})
	if err != authz.ErrPermsNotFound {
		t.Fatalf("err: want %q but got %v", authz.ErrPermsNotFound, err)
	}

	// Check bob shoud not be affected
	p := &authz.UserPendingPermissions{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		BindID:      "bob",
		Perm:        authz.Read,
		Type:        authz.PermRepos,
	}
	err = s.LoadUserPendingPermissions(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "p.IDs", []int{1}, mapsetToArray(p.IDs))
}

func TestPermsStore_DatabaseDeadlocks(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, time.Now)
	t.Cleanup(func() {
		cleanupPermsTables(t, s)
	})

	ctx := context.Background()

	setupPermsRelatedEntities(t, s, []authz.Permission{
		{
			UserID:            1,
			RepoID:            1,
			ExternalAccountID: 1,
		},
	})

	setUserPermissions := func(ctx context.Context, t *testing.T) {
		_, err := s.SetUserExternalAccountPerms(ctx, authz.UserIDWithExternalAccountID{
			UserID:            1,
			ExternalAccountID: 1,
		}, []int32{1}, authz.SourceUserSync)
		require.NoError(t, err)
	}
	setRepoPermissions := func(ctx context.Context, t *testing.T) {
		_, err := s.SetRepoPerms(ctx, 1, []authz.UserIDWithExternalAccountID{{
			UserID:            1,
			ExternalAccountID: 1,
		}}, authz.SourceRepoSync)
		require.NoError(t, err)
	}
	setRepoPendingPermissions := func(ctx context.Context, t *testing.T) {
		accounts := &extsvc.Accounts{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			AccountIDs:  []string{"alice"},
		}
		if err := s.SetRepoPendingPermissions(ctx, accounts, &authz.RepoPermissions{
			RepoID: 1,
			Perm:   authz.Read,
		}); err != nil {
			t.Fatal(err)
		}
	}
	grantPendingPermissions := func(ctx context.Context, t *testing.T) {
		if err := s.GrantPendingPermissions(ctx, &authz.UserGrantPermissions{
			UserID:      1,
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			AccountID:   "alice",
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Ensure we've run all permutations of ordering of the 4 calls to avoid nondeterminism in
	// test coverage stats.
	funcs := []func(context.Context, *testing.T){
		setRepoPendingPermissions, grantPendingPermissions, setRepoPermissions, setUserPermissions,
	}
	permutated := permutation.New(permutation.MustAnySlice(funcs))
	for permutated.Next() {
		for _, f := range funcs {
			f(ctx, t)
		}
	}

	const numOps = 50
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		for range numOps {
			setUserPermissions(ctx, t)
		}
	}()
	go func() {
		defer wg.Done()
		for range numOps {
			setRepoPermissions(ctx, t)
		}
	}()
	go func() {
		defer wg.Done()
		for range numOps {
			setRepoPendingPermissions(ctx, t)
		}
	}()
	go func() {
		defer wg.Done()
		for range numOps {
			grantPendingPermissions(ctx, t)
		}
	}()

	wg.Wait()
}

func cleanupUsersTable(t *testing.T, s *permsStore) {
	t.Helper()

	q := `DELETE FROM user_external_accounts;`
	executeQuery(t, context.Background(), s, sqlf.Sprintf(q))

	q = `TRUNCATE TABLE users RESTART IDENTITY CASCADE;`
	executeQuery(t, context.Background(), s, sqlf.Sprintf(q))
}

func TestPermsStore_GetUserIDsByExternalAccounts(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)

	s := perms(logger, db, time.Now)
	t.Cleanup(func() {
		cleanupUsersTable(t, s)
	})

	ctx := context.Background()

	// Set up test users and external accounts
	extSQL := `
INSERT INTO user_external_accounts(user_id, service_type, service_id, account_id, client_id, created_at, updated_at, deleted_at, expired_at)
	VALUES(%s, %s, %s, %s, %s, %s, %s, %s, %s)
`
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('alice')`),  // ID=1
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('bob')`),    // ID=2
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('cindy')`),  // ID=3
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('denise')`), // ID=4

		sqlf.Sprintf(extSQL, 1, extsvc.TypeGitLab, "https://gitlab.com/", "alice_gitlab", "alice_gitlab_client_id", clock(), clock(), nil, nil), // ID=1
		sqlf.Sprintf(extSQL, 1, "github", "https://github.com/", "alice_github", "alice_github_client_id", clock(), clock(), nil, nil),          // ID=2
		sqlf.Sprintf(extSQL, 2, extsvc.TypeGitLab, "https://gitlab.com/", "bob_gitlab", "bob_gitlab_client_id", clock(), clock(), nil, nil),     // ID=3
		sqlf.Sprintf(extSQL, 3, extsvc.TypeGitLab, "https://gitlab.com/", "cindy_gitlab", "cindy_gitlab_client_id", clock(), clock(), nil, nil), // ID=4
		sqlf.Sprintf(extSQL, 3, "github", "https://github.com/", "cindy_github", "cindy_github_client_id", clock(), clock(), clock(), nil),      // ID=5, deleted
		sqlf.Sprintf(extSQL, 4, "github", "https://github.com/", "denise_github", "denise_github_client_id", clock(), clock(), nil, clock()),    // ID=6, expired
	}
	for _, q := range qs {
		if err := s.execute(ctx, q); err != nil {
			t.Fatal(err)
		}
	}

	accounts := &extsvc.Accounts{
		ServiceType: "gitlab",
		ServiceID:   "https://gitlab.com/",
		AccountIDs:  []string{"alice_gitlab", "bob_gitlab", "david_gitlab"},
	}
	userIDs, err := s.GetUserIDsByExternalAccounts(ctx, accounts)
	if err != nil {
		t.Fatal(err)
	}

	if len(userIDs) != 2 {
		t.Fatalf("len(userIDs): want 2 but got %v", userIDs)
	}

	assert.Equal(t, int32(1), userIDs["alice_gitlab"].UserID)
	assert.Equal(t, int32(1), userIDs["alice_gitlab"].ExternalAccountID)
	assert.Equal(t, int32(2), userIDs["bob_gitlab"].UserID)
	assert.Equal(t, int32(3), userIDs["bob_gitlab"].ExternalAccountID)

	accounts = &extsvc.Accounts{
		ServiceType: "github",
		ServiceID:   "https://github.com/",
		AccountIDs:  []string{"cindy_github", "denise_github"},
	}
	userIDs, err = s.GetUserIDsByExternalAccounts(ctx, accounts)
	require.Nil(t, err)
	assert.Empty(t, userIDs)
}

func executeQuery(t *testing.T, ctx context.Context, s *permsStore, q *sqlf.Query) {
	t.Helper()
	if t.Failed() {
		return
	}

	err := s.execute(ctx, q)
	if err != nil {
		t.Fatalf("Error executing query %v, err: %v", q, err)
	}
}

func TestPermsStore_UserIDsWithNoPerms(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, time.Now)

	t.Cleanup(func() {
		cleanupPermsTables(t, s)
		cleanupUsersTable(t, s)
		cleanupReposTable(t, s)
	})

	ctx := context.Background()

	// Create test users "alice" and "bob", test repo and test external account
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('alice')`),                    // ID=1
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('bob')`),                      // ID=2
		sqlf.Sprintf(`INSERT INTO users(username, deleted_at) VALUES('cindy', NOW())`), // ID=3
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('david')`),                    // ID=4
		sqlf.Sprintf(`INSERT INTO repo(name, private) VALUES('private_repo', TRUE)`),   // ID=1
		sqlf.Sprintf(`INSERT INTO user_external_accounts(user_id, service_type, service_id, account_id, client_id, created_at, updated_at, deleted_at, expired_at)
				VALUES(%s, %s, %s, %s, %s, %s, %s, %s, %s)`, 1, extsvc.TypeGitLab, "https://gitlab.com/", "alice_gitlab", "alice_gitlab_client_id", clock(), clock(), nil, nil), // ID=1
	}
	for _, q := range qs {
		executeQuery(t, ctx, s, q)
	}

	// "alice", "bob" and "david" have no permissions
	ids, err := s.UserIDsWithNoPerms(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	expIDs := []int32{1, 2, 4}
	if diff := cmp.Diff(expIDs, ids); diff != "" {
		t.Fatal(diff)
	}

	// mark sync jobs as completed for "alice" and add permissions for "bob"
	q := sqlf.Sprintf(`INSERT INTO permission_sync_jobs(user_id, finished_at, reason) VALUES(%d, NOW(), %s)`, 1, ReasonUserNoPermissions)
	executeQuery(t, ctx, s, q)

	_, err = s.SetUserExternalAccountPerms(ctx, authz.UserIDWithExternalAccountID{UserID: 2, ExternalAccountID: 1}, []int32{1}, authz.SourceUserSync)
	require.NoError(t, err)

	// Only "david" has no permissions at this point
	ids, err = s.UserIDsWithNoPerms(ctx)
	require.NoError(t, err)

	expIDs = []int32{4}
	if diff := cmp.Diff(expIDs, ids); diff != "" {
		t.Fatal(diff)
	}
}

func TestPermsStore_CountUsersWithNoPerms(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, time.Now)

	t.Cleanup(func() {
		cleanupPermsTables(t, s)
		cleanupUsersTable(t, s)
		cleanupReposTable(t, s)
	})

	ctx := context.Background()

	// Create test users "alice", "bob", "cindy" and "david", test repo and test external account.
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('alice')`),                    // ID=1
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('bob')`),                      // ID=2
		sqlf.Sprintf(`INSERT INTO users(username, deleted_at) VALUES('cindy', NOW())`), // ID=3
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('david')`),                    // ID=4
		sqlf.Sprintf(`INSERT INTO repo(name, private) VALUES('private_repo', TRUE)`),   // ID=1
		sqlf.Sprintf(`INSERT INTO user_external_accounts(user_id, service_type, service_id, account_id, client_id, created_at, updated_at, deleted_at, expired_at)
				VALUES(%s, %s, %s, %s, %s, %s, %s, %s, %s)`, 1, extsvc.TypeGitLab, "https://gitlab.com/", "alice_gitlab", "alice_gitlab_client_id", clock(), clock(), nil, nil), // ID=1
	}
	for _, q := range qs {
		executeQuery(t, ctx, s, q)
	}

	// "alice", "bob" and "david" have no permissions.
	count, err := s.CountUsersWithNoPerms(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, count)

	// Add permissions for "bob".
	_, err = s.SetUserExternalAccountPerms(ctx, authz.UserIDWithExternalAccountID{UserID: 2, ExternalAccountID: 1}, []int32{1}, authz.SourceUserSync)
	require.NoError(t, err)

	// Only "alice" and "david" has no permissions at this point.
	count, err = s.CountUsersWithNoPerms(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func cleanupReposTable(t *testing.T, s *permsStore) {
	t.Helper()

	q := `TRUNCATE TABLE repo RESTART IDENTITY CASCADE;`
	executeQuery(t, context.Background(), s, sqlf.Sprintf(q))
}

func TestPermsStore_RepoIDsWithNoPerms(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, time.Now)

	t.Cleanup(func() {
		cleanupPermsTables(t, s)
		cleanupReposTable(t, s)
		cleanupUsersTable(t, s)
	})

	ctx := context.Background()

	// Create three test repositories
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO repo(name, private) VALUES('private_repo', TRUE)`),                      // ID=1
		sqlf.Sprintf(`INSERT INTO repo(name) VALUES('public_repo')`),                                      // ID=2
		sqlf.Sprintf(`INSERT INTO repo(name, private) VALUES('private_repo_2', TRUE)`),                    // ID=3
		sqlf.Sprintf(`INSERT INTO repo(name, private, deleted_at) VALUES('private_repo_3', TRUE, NOW())`), // ID=4
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('alice')`),                                       // ID=1
		sqlf.Sprintf(`INSERT INTO user_external_accounts(user_id, service_type, service_id, account_id, client_id, created_at, updated_at, deleted_at, expired_at)
				VALUES(%s, %s, %s, %s, %s, %s, %s, %s, %s)`, 1, extsvc.TypeGitLab, "https://gitlab.com/", "alice_gitlab", "alice_gitlab_client_id", clock(), clock(), nil, nil), // ID=1
	}
	for _, q := range qs {
		executeQuery(t, ctx, s, q)
	}

	// Should get back two private repos that are not deleted
	ids, err := s.RepoIDsWithNoPerms(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	expIDs := []api.RepoID{1, 3}
	if diff := cmp.Diff(expIDs, ids); diff != "" {
		t.Fatal(diff)
	}

	// mark sync jobs as completed for "private_repo" and add permissions for "private_repo_2"
	q := sqlf.Sprintf(`INSERT INTO permission_sync_jobs(repository_id, finished_at, reason) VALUES(%d, NOW(), %s)`, 1, ReasonRepoNoPermissions)
	executeQuery(t, ctx, s, q)

	_, err = s.SetRepoPerms(ctx, 3, []authz.UserIDWithExternalAccountID{{UserID: 1, ExternalAccountID: 1}}, authz.SourceRepoSync)
	require.NoError(t, err)

	// No private repositories have any permissions at this point
	ids, err = s.RepoIDsWithNoPerms(ctx)
	require.NoError(t, err)

	assert.Nil(t, ids)
}

func TestPermsStore_CountReposWithNoPerms(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, time.Now)

	t.Cleanup(func() {
		cleanupPermsTables(t, s)
		cleanupReposTable(t, s)
		cleanupUsersTable(t, s)
	})

	ctx := context.Background()

	// Create four test repositories, test user and test external account.
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO repo(name, private) VALUES('private_repo', TRUE)`),                      // ID=1
		sqlf.Sprintf(`INSERT INTO repo(name) VALUES('public_repo')`),                                      // ID=2
		sqlf.Sprintf(`INSERT INTO repo(name, private) VALUES('private_repo_2', TRUE)`),                    // ID=3
		sqlf.Sprintf(`INSERT INTO repo(name, private, deleted_at) VALUES('private_repo_3', TRUE, NOW())`), // ID=4
		sqlf.Sprintf(`INSERT INTO users(username) VALUES('alice')`),                                       // ID=1
		sqlf.Sprintf(`INSERT INTO user_external_accounts(user_id, service_type, service_id, account_id, client_id, created_at, updated_at, deleted_at, expired_at)
				VALUES(%s, %s, %s, %s, %s, %s, %s, %s, %s)`, 1, extsvc.TypeGitLab, "https://gitlab.com/", "alice_gitlab", "alice_gitlab_client_id", clock(), clock(), nil, nil), // ID=1
	}
	for _, q := range qs {
		executeQuery(t, ctx, s, q)
	}

	// Should get back two private repos that are not deleted.
	count, err := s.CountReposWithNoPerms(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	_, err = s.SetRepoPerms(ctx, 3, []authz.UserIDWithExternalAccountID{{UserID: 1, ExternalAccountID: 1}}, authz.SourceRepoSync)
	require.NoError(t, err)

	// Private repository ID=1 has no permissions at this point.
	count, err = s.CountReposWithNoPerms(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestPermsStore_UserIDsWithOldestPerms(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, clock)
	ctx := context.Background()

	t.Cleanup(func() {
		cleanupPermsTables(t, s)
		cleanupReposTable(t, s)
		cleanupUsersTable(t, s)
	})

	// Set up some users and permissions
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO users(id, username) VALUES(1, 'alice')`),
		sqlf.Sprintf(`INSERT INTO users(id, username) VALUES(2, 'bob')`),
		sqlf.Sprintf(`INSERT INTO users(id, username, deleted_at) VALUES(3, 'cindy', NOW())`),
		sqlf.Sprintf(`INSERT INTO users(id, username) VALUES(4, 'david')`),
		sqlf.Sprintf(`INSERT INTO users(id, username) VALUES(5, 'erica')`),
	}
	for _, q := range qs {
		executeQuery(t, ctx, s, q)
	}

	// mark sync jobs as completed for users 1, 2 and 3
	user1UpdatedAt := clock().Add(-15 * time.Minute)
	user2UpdatedAt := clock().Add(-5 * time.Minute)
	user3UpdatedAt := clock().Add(-11 * time.Minute)
	q := sqlf.Sprintf(`INSERT INTO permission_sync_jobs(user_id, state, finished_at, reason) VALUES(%d, 'completed', %s, %s)`, 1, user1UpdatedAt, ReasonUserOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(user_id, state, finished_at, reason) VALUES(%d, 'completed', %s, %s)`, 2, user2UpdatedAt, ReasonUserOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(user_id, state, finished_at, reason) VALUES(%d, 'completed', %s, %s)`, 3, user3UpdatedAt, ReasonUserOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(user_id, state, reason) VALUES(%d, 'queued', %s)`, 4, ReasonUserOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(user_id, state, reason) VALUES(%d, 'processing', %s)`, 5, ReasonUserOutdatedPermissions)
	executeQuery(t, ctx, s, q)

	t.Run("One result when limit is 1. Queued user ignored", func(t *testing.T) {
		// Should only get user 1 back, because limit is 1
		// Users with queued and processing permissions syncs are ignored
		results, err := s.UserIDsWithOldestPerms(ctx, 1, 0)
		if err != nil {
			t.Fatal(err)
		}

		wantResults := map[int32]time.Time{1: user1UpdatedAt}
		if diff := cmp.Diff(wantResults, results); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("One result when limit is 10 and age is 10 minutes", func(t *testing.T) {
		// Should only get user 1 back, because age is 10 minutes
		results, err := s.UserIDsWithOldestPerms(ctx, 10, 10*time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		wantResults := map[int32]time.Time{1: user1UpdatedAt}
		if diff := cmp.Diff(wantResults, results); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("Both users are returned when limit is 10 and age is 1 minute, and deleted user is ignored", func(t *testing.T) {
		// Should get both users, since the limit is 10 and age is 1 minute only
		results, err := s.UserIDsWithOldestPerms(ctx, 10, 1*time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		wantResults := map[int32]time.Time{1: user1UpdatedAt, 2: user2UpdatedAt}
		if diff := cmp.Diff(wantResults, results); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("Both users are returned when limit is 10 and age is 0. Deleted users are ignored", func(t *testing.T) {
		// Should get both users, since the limit is 10 and age is 0
		results, err := s.UserIDsWithOldestPerms(ctx, 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		wantResults := map[int32]time.Time{1: user1UpdatedAt, 2: user2UpdatedAt}
		if diff := cmp.Diff(wantResults, results); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("Ignore users that have synced recently", func(t *testing.T) {
		// Should get no results, since the and age is 1 hour
		results, err := s.UserIDsWithOldestPerms(ctx, 1, 1*time.Hour)
		if err != nil {
			t.Fatal(err)
		}

		wantResults := make(map[int32]time.Time)
		if diff := cmp.Diff(wantResults, results); diff != "" {
			t.Fatal(diff)
		}
	})
}

func TestPermsStore_CountUsersWithStalePerms(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, clock)
	ctx := context.Background()

	t.Cleanup(func() {
		cleanupPermsTables(t, s)
		cleanupReposTable(t, s)
		cleanupUsersTable(t, s)
	})

	// Set up some users and permissions.
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO users(id, username) VALUES(1, 'alice')`),
		sqlf.Sprintf(`INSERT INTO users(id, username) VALUES(2, 'bob')`),
		sqlf.Sprintf(`INSERT INTO users(id, username, deleted_at) VALUES(3, 'cindy', NOW())`),
	}
	for _, q := range qs {
		executeQuery(t, ctx, s, q)
	}

	// Mark sync jobs as completed for users 1, 2 and 3.
	user1FinishedAt := clock().Add(-15 * time.Minute)
	user2FinishedAt := clock().Add(-5 * time.Minute)
	user3FinishedAt := clock().Add(-11 * time.Minute)
	q := sqlf.Sprintf(`INSERT INTO permission_sync_jobs(user_id, finished_at, reason) VALUES(%d, %s, %s)`, 1, user1FinishedAt, ReasonUserOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(user_id, finished_at, reason) VALUES(%d, %s, %s)`, 2, user2FinishedAt, ReasonUserOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(user_id, finished_at, reason) VALUES(%d, %s, %s)`, 3, user3FinishedAt, ReasonUserOutdatedPermissions)
	executeQuery(t, ctx, s, q)

	t.Run("One result when age is 10 minutes", func(t *testing.T) {
		// Should only get user 1 back, because age is 10 minutes.
		count, err := s.CountUsersWithStalePerms(ctx, 10*time.Minute)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("Both users are returned when age is 1 minute, and deleted user is ignored", func(t *testing.T) {
		// Should get both users, since the age is 1 minute only.
		count, err := s.CountUsersWithStalePerms(ctx, 1*time.Minute)
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})

	t.Run("Both users are returned when age is 0. Deleted users are ignored", func(t *testing.T) {
		// Should get both users, since the and age is 0 and cutoff clause if skipped.
		count, err := s.CountUsersWithStalePerms(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})

	t.Run("Ignore users that have synced recently", func(t *testing.T) {
		// Should get no results, since the age is 1 hour.
		count, err := s.CountUsersWithStalePerms(ctx, 1*time.Hour)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})
}

func TestPermsStore_ReposIDsWithOldestPerms(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, clock)
	ctx := context.Background()
	t.Cleanup(func() {
		cleanupPermsTables(t, s)
		cleanupReposTable(t, s)
	})

	// Set up some repositories and permissions
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(1, 'private_repo_1', TRUE)`),                    // id=1
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(2, 'private_repo_2', TRUE)`),                    // id=2
		sqlf.Sprintf(`INSERT INTO repo(id, name, private, deleted_at) VALUES(3, 'private_repo_3', TRUE, NOW())`), // id=3
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(4, 'private_repo_4', TRUE)`),                    // id=2
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(5, 'private_repo_5', TRUE)`),                    // id=2
	}
	for _, q := range qs {
		executeQuery(t, ctx, s, q)
	}

	// mark sync jobs as completed for private_repo_1 and private_repo_2
	repo1UpdatedAt := clock().Add(-15 * time.Minute)
	repo2UpdatedAt := clock().Add(-5 * time.Minute)
	repo3UpdatedAt := clock().Add(-10 * time.Minute)
	q := sqlf.Sprintf(`INSERT INTO permission_sync_jobs(repository_id, state, finished_at, reason) VALUES(%d, 'completed', %s, %s)`, 1, repo1UpdatedAt, ReasonRepoOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(repository_id, state, finished_at, reason) VALUES(%d, 'completed', %s, %s)`, 2, repo2UpdatedAt, ReasonRepoOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(repository_id, state, finished_at, reason) VALUES(%d, 'completed', %s, %s)`, 3, repo3UpdatedAt, ReasonRepoOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(repository_id, state, reason) VALUES(%d, 'queued', %s)`, 4, ReasonRepoOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(repository_id, state, reason) VALUES(%d, 'processing', %s)`, 5, ReasonRepoOutdatedPermissions)
	executeQuery(t, ctx, s, q)

	t.Run("One result when limit is 1", func(t *testing.T) {
		// Should only get private_repo_1 back, because limit is 1
		// Repos with jobs queued or in progress are ignored
		results, err := s.ReposIDsWithOldestPerms(ctx, 1, 0)
		if err != nil {
			t.Fatal(err)
		}

		wantResults := map[api.RepoID]time.Time{1: repo1UpdatedAt}
		if diff := cmp.Diff(wantResults, results); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("One result when limit is 10 and age is 10 minutes", func(t *testing.T) {
		// Should only get private_repo_1 back, because age is 10 minutes
		results, err := s.ReposIDsWithOldestPerms(ctx, 10, 10*time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		wantResults := map[api.RepoID]time.Time{1: repo1UpdatedAt}
		if diff := cmp.Diff(wantResults, results); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("Both repos are returned when limit is 10 and age is 1 minute", func(t *testing.T) {
		// Should get both private_repo_1 and private_repo_2, since the limit is 10 and age is 1 minute only
		results, err := s.ReposIDsWithOldestPerms(ctx, 10, 1*time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		wantResults := map[api.RepoID]time.Time{1: repo1UpdatedAt, 2: repo2UpdatedAt}
		if diff := cmp.Diff(wantResults, results); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("Both repos are returned when limit is 10 and age is 0 and deleted repos are ignored", func(t *testing.T) {
		// Should get both private_repo_1 and private_repo_2, since the limit is 10 and age is 0
		results, err := s.ReposIDsWithOldestPerms(ctx, 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		wantResults := map[api.RepoID]time.Time{1: repo1UpdatedAt, 2: repo2UpdatedAt}
		if diff := cmp.Diff(wantResults, results); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("Ignore repos that have synced recently", func(t *testing.T) {
		// Should get no results, since the and age is 1 hour
		results, err := s.ReposIDsWithOldestPerms(ctx, 1, 1*time.Hour)
		if err != nil {
			t.Fatal(err)
		}

		wantResults := make(map[api.RepoID]time.Time)
		if diff := cmp.Diff(wantResults, results); diff != "" {
			t.Fatal(diff)
		}
	})
}

func TestPermsStore_CountReposWithStalePerms(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, clock)
	ctx := context.Background()
	t.Cleanup(func() {
		cleanupPermsTables(t, s)
		cleanupReposTable(t, s)
	})

	// Set up some repositories and permissions.
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(1, 'private_repo_1', TRUE)`),                    // id=1
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(2, 'private_repo_2', TRUE)`),                    // id=2
		sqlf.Sprintf(`INSERT INTO repo(id, name, private, deleted_at) VALUES(3, 'private_repo_3', TRUE, NOW())`), // id=3
	}
	for _, q := range qs {
		executeQuery(t, ctx, s, q)
	}

	// Mark sync jobs as completed for all repos.
	repo1FinishedAt := clock().Add(-15 * time.Minute)
	repo2FinishedAt := clock().Add(-5 * time.Minute)
	repo3FinishedAt := clock().Add(-10 * time.Minute)
	q := sqlf.Sprintf(`INSERT INTO permission_sync_jobs(repository_id, finished_at, reason) VALUES(%d, %s, %s)`, 1, repo1FinishedAt, ReasonRepoOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(repository_id, finished_at, reason) VALUES(%d, %s, %s)`, 2, repo2FinishedAt, ReasonRepoOutdatedPermissions)
	executeQuery(t, ctx, s, q)
	q = sqlf.Sprintf(`INSERT INTO permission_sync_jobs(repository_id, finished_at, reason) VALUES(%d, %s, %s)`, 3, repo3FinishedAt, ReasonRepoOutdatedPermissions)
	executeQuery(t, ctx, s, q)

	t.Run("One result when age is 10 minutes", func(t *testing.T) {
		// Should only get private_repo_1 back, because age is 10 minutes.
		count, err := s.CountReposWithStalePerms(ctx, 10*time.Minute)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("Both repos are returned when age is 1 minute", func(t *testing.T) {
		// Should get both private_repo_1 and private_repo_2, since the age is 1 minute only.
		count, err := s.CountReposWithStalePerms(ctx, 1*time.Minute)
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})

	t.Run("Both repos are returned when age is 0 and deleted repos are ignored", func(t *testing.T) {
		// Should get both private_repo_1 and private_repo_2, since the age is 0 and cutoff clause if skipped.
		count, err := s.CountReposWithStalePerms(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})

	t.Run("Ignore repos that have synced recently", func(t *testing.T) {
		// Should get no results, since the age is 1 hour
		count, err := s.CountReposWithStalePerms(ctx, 1*time.Hour)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})
}

func TestPermsStore_MapUsers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, clock)
	ctx := context.Background()

	t.Cleanup(func() {
		if t.Failed() {
			return
		}

		q := `TRUNCATE TABLE external_services, orgs, users CASCADE`
		if err := s.execute(ctx, sqlf.Sprintf(q)); err != nil {
			t.Fatal(err)
		}
	})

	// Set up 3 users
	users := db.Users()

	igor, err := users.Create(ctx,
		NewUser{
			Email:           "igor@example.com",
			Username:        "igor",
			EmailIsVerified: true,
		},
	)
	require.NoError(t, err)
	shreah, err := users.Create(ctx,
		NewUser{
			Email:           "shreah@example.com",
			Username:        "shreah",
			EmailIsVerified: true,
		},
	)
	require.NoError(t, err)
	omar, err := users.Create(ctx,
		NewUser{
			Email:           "omar@example.com",
			Username:        "omar",
			EmailIsVerified: true,
		},
	)
	require.NoError(t, err)

	// emails: map with a mixed load of existing, space only and non existing users
	has, err := s.MapUsers(ctx, []string{"igor@example.com", "", "omar@example.com", "  	", "sayako@example.com"}, &schema.PermissionsUserMapping{BindID: "email"})
	assert.NoError(t, err)
	assert.Equal(t, map[string]int32{
		"igor@example.com": igor.ID,
		"omar@example.com": omar.ID,
	}, has)

	// usernames: map with a mixed load of existing, space only and non existing users
	has, err = s.MapUsers(ctx, []string{"igor", "", "shreah", "  	", "carlos"}, &schema.PermissionsUserMapping{BindID: "username"})
	assert.NoError(t, err)
	assert.Equal(t, map[string]int32{
		"igor":   igor.ID,
		"shreah": shreah.ID,
	}, has)

	// use a non-existing mapping
	_, err = s.MapUsers(ctx, []string{"igor", "", "shreah", "  	", "carlos"}, &schema.PermissionsUserMapping{BindID: "shoeSize"})
	assert.Error(t, err)
}

func TestPermsStore_Metrics(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, clock)

	ctx := context.Background()
	t.Cleanup(func() {
		cleanupPermsTables(t, s)
		cleanupUsersTable(t, s)
		if t.Failed() {
			return
		}

		if err := s.execute(ctx, sqlf.Sprintf(`DELETE FROM repo`)); err != nil {
			t.Fatal(err)
		}
	})

	// Set up repositories in various states (public/private, deleted/not, etc.)
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(1, 'private_repo_1', TRUE)`),
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(2, 'private_repo_2', TRUE)`),
		sqlf.Sprintf(`INSERT INTO repo(id, name, private, deleted_at) VALUES(3, 'private_repo_3', TRUE, NOW())`),
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(4, 'public_repo_4', FALSE)`),
	}
	for _, q := range qs {
		if err := s.execute(ctx, q); err != nil {
			t.Fatal(err)
		}
	}

	// Set up users in various states (deleted/not, etc.)
	qs = []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO users(id, username) VALUES(1, 'user1')`),
		sqlf.Sprintf(`INSERT INTO users(id, username) VALUES(2, 'user2')`),
		sqlf.Sprintf(`INSERT INTO users(id, username, deleted_at) VALUES(3, 'user3', NOW())`),
	}
	for _, q := range qs {
		if err := s.execute(ctx, q); err != nil {
			t.Fatal(err)
		}
	}

	// Set up permissions for the various repos.
	ep := make([]authz.Permission, 0, 4)
	for i := 1; i <= 4; i++ {
		for j := 1; j <= 4; j++ {
			ep = append(ep, authz.Permission{
				UserID:            int32(j),
				ExternalAccountID: int32(j),
				RepoID:            int32(i),
			})
		}
	}
	setupPermsRelatedEntities(t, s, []authz.Permission{
		{UserID: 1, ExternalAccountID: 1},
		{UserID: 2, ExternalAccountID: 2},
		{UserID: 3, ExternalAccountID: 3},
		{UserID: 4, ExternalAccountID: 4},
	})
	_, err := s.setUserRepoPermissions(ctx, ep, authz.PermissionEntity{}, authz.SourceRepoSync, false)
	require.NoError(t, err)

	// Mock rows for testing
	qs = []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO permission_sync_jobs (finished_at, user_id, reason) VALUES(%s, 1, 'TEST')`, clock()),
		sqlf.Sprintf(`INSERT INTO permission_sync_jobs (finished_at, user_id, reason) VALUES(%s, 2, 'TEST')`, clock().Add(-1*time.Minute)),
		sqlf.Sprintf(`INSERT INTO permission_sync_jobs (finished_at, user_id, reason) VALUES(%s, 3, 'TEST')`, clock().Add(-2*time.Minute)), // Meant to be excluded because it has been deleted
		sqlf.Sprintf(`INSERT INTO permission_sync_jobs (finished_at, repository_id, reason) VALUES(%s, 1, 'TEST')`, clock()),
		sqlf.Sprintf(`INSERT INTO permission_sync_jobs (finished_at, repository_id, reason) VALUES(%s, 2, 'TEST')`, clock().Add(-2*time.Minute)),
		sqlf.Sprintf(`INSERT INTO permission_sync_jobs (finished_at, repository_id, reason) VALUES(%s, 3, 'TEST')`, clock().Add(-3*time.Minute)), // Meant to be excluded because it has been deleted
		sqlf.Sprintf(`INSERT INTO permission_sync_jobs (finished_at, repository_id, reason) VALUES(%s, 4, 'TEST')`, clock().Add(-3*time.Minute)), // Meant to be excluded because it is public
	}
	for _, q := range qs {
		if err := s.execute(ctx, q); err != nil {
			t.Fatal(err)
		}
	}

	m, err := s.Metrics(ctx, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	expMetrics := &PermsMetrics{
		UsersWithStalePerms:  1,
		UsersPermsGapSeconds: 60,
		ReposWithStalePerms:  1,
		ReposPermsGapSeconds: 120,
	}

	if diff := cmp.Diff(expMetrics, m); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func setupTestPerms(t *testing.T, db DB, clock func() time.Time) *permsStore {
	t.Helper()
	logger := logtest.Scoped(t)
	s := perms(logger, db, clock)
	t.Cleanup(func() {
		cleanupPermsTables(t, s)
	})
	return s
}

func TestPermsStore_ListUserPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)
	s := perms(logger, db, clock)
	ctx := context.Background()

	t.Cleanup(func() {
		cleanupPermsTables(t, s)

		if t.Failed() {
			return
		}
		q := `TRUNCATE TABLE external_services, repo, users CASCADE`
		if err := s.execute(ctx, sqlf.Sprintf(q)); err != nil {
			t.Fatal(err)
		}
	})
	// Set fake authz providers otherwise authz is bypassed
	authz.SetProviders(false, []authz.Provider{&fakePermsProvider{}})
	defer authz.SetProviders(true, nil)
	// Set up some repositories and permissions
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO users(id, username, site_admin) VALUES(555, 'user555', FALSE)`),
		sqlf.Sprintf(`INSERT INTO users(id, username, site_admin) VALUES(777, 'user777', TRUE)`),
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(1, 'private_repo_1', TRUE)`),
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(2, 'private_repo_2', TRUE)`),
		sqlf.Sprintf(`INSERT INTO repo(id, name, private, deleted_at) VALUES(3, 'private_repo_3_deleted', TRUE, NOW())`),
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(4, 'public_repo_4', FALSE)`),
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(5, 'public_repo_5', TRUE)`),
		sqlf.Sprintf(`INSERT INTO external_services(id, display_name, kind, config) VALUES(1, 'GitHub #1', 'GITHUB', '{}')`),
		sqlf.Sprintf(`INSERT INTO external_service_repos(repo_id, external_service_id, clone_url)
                                 VALUES(1, 1, ''), (2, 1, ''), (3, 1, ''), (4, 1, '')`),
		sqlf.Sprintf(`INSERT INTO external_services(id, display_name, kind, config, unrestricted) VALUES(2, 'GitHub #2 Unrestricted', 'GITHUB', '{}', TRUE)`),
		sqlf.Sprintf(`INSERT INTO external_service_repos(repo_id, external_service_id, clone_url)
                                 VALUES(5, 2, ''), (4, 2, '')`),
	}
	for _, q := range qs {
		if err := s.execute(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	q := sqlf.Sprintf(`INSERT INTO user_repo_permissions(user_id, repo_id, source) VALUES(555, 1, 'user_sync'), (777, 2, 'user_sync'), (555, 3, 'api'), (777, 3, 'api');`)
	if err := s.execute(ctx, q); err != nil {
		t.Fatal(err)
	}

	tests := []listUserPermissionsTest{
		{
			Name:   "TestNonSiteAdminUser",
			UserID: 555,
			WantResults: []*listUserPermissionsResult{
				{
					// private repo but have access via user_permissions
					RepoId: 1,
					Reason: UserRepoPermissionReasonPermissionsSync,
				},
				{
					// public repo
					RepoId: 4,
					Reason: UserRepoPermissionReasonPublic,
				},
				{
					// private repo but unrestricted
					RepoId: 5,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
			},
		},
		{
			Name:   "TestPagination",
			UserID: 555,
			Args: &ListUserPermissionsArgs{
				PaginationArgs: &PaginationArgs{First: pointers.Ptr(2), After: []any{"public_repo_5"}, OrderBy: OrderBy{{Field: "name"}}},
			},
			WantResults: []*listUserPermissionsResult{
				{
					RepoId: 1,
					Reason: UserRepoPermissionReasonPermissionsSync,
				},
				{
					RepoId: 4,
					Reason: UserRepoPermissionReasonPublic,
				},
			},
		},
		{
			Name:   "TestSearchQuery",
			UserID: 555,
			Args: &ListUserPermissionsArgs{
				Query: "repo_5",
			},
			WantResults: []*listUserPermissionsResult{
				{
					RepoId: 5,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
			},
		},
		{
			Name:   "TestSiteAdminUser",
			UserID: 777,
			WantResults: []*listUserPermissionsResult{
				{
					// do not have direct access but user is site admin
					RepoId: 1,
					Reason: UserRepoPermissionReasonSiteAdmin,
				},
				{
					// private repo but have access via user_permissions (still site admin)
					RepoId: 2,
					Reason: UserRepoPermissionReasonSiteAdmin,
				},
				{
					// public repo
					RepoId: 4,
					Reason: UserRepoPermissionReasonPublic,
				},
				{
					// private repo but unrestricted
					RepoId: 5,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			results, err := s.ListUserPermissions(ctx, int32(test.UserID), test.Args)
			if err != nil {
				t.Fatal(err)
			}
			if len(test.WantResults) != len(results) {
				t.Fatalf("Results mismatch. Want: %d Got: %d", len(test.WantResults), len(results))
			}
			for index, result := range results {
				if diff := cmp.Diff(test.WantResults[index], &listUserPermissionsResult{RepoId: int32(result.Repo.ID), Reason: result.Reason}); diff != "" {
					t.Fatalf("Results (%d) mismatch (-want +got):\n%s", index, diff)
				}
			}
		})
	}
}

type listUserPermissionsTest struct {
	Name        string
	UserID      int
	Args        *ListUserPermissionsArgs
	WantResults []*listUserPermissionsResult
}

type listUserPermissionsResult struct {
	RepoId int32
	Reason UserRepoPermissionReason
}

func TestPermsStore_ListRepoPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	testDb := dbtest.NewDB(t)
	db := NewDB(logger, testDb)

	s := perms(logtest.Scoped(t), db, clock)
	ctx := context.Background()
	t.Cleanup(func() {
		cleanupPermsTables(t, s)

		if t.Failed() {
			return
		}

		q := `TRUNCATE TABLE external_services, repo, users CASCADE`
		if err := s.execute(ctx, sqlf.Sprintf(q)); err != nil {
			t.Fatal(err)
		}
	})

	// Set up some repositories and permissions
	qs := []*sqlf.Query{
		sqlf.Sprintf(`INSERT INTO users(id, username, site_admin) VALUES(555, 'user555', FALSE)`),
		sqlf.Sprintf(`INSERT INTO users(id, username, site_admin) VALUES(666, 'user666', FALSE)`),
		sqlf.Sprintf(`INSERT INTO users(id, username, site_admin) VALUES(777, 'user777', TRUE)`),
		sqlf.Sprintf(`INSERT INTO users(id, username, site_admin, deleted_at) VALUES(888, 'user888', TRUE, NOW())`),
		sqlf.Sprintf(`INSERT INTO users(id, username, site_admin) VALUES(999, 'user999', TRUE)`), // Site admin user with explicit access to all repos
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(1, 'private_repo_1', TRUE)`),
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(2, 'public_repo_2', FALSE)`),
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(3, 'unrestricted_repo_3', TRUE)`),
		sqlf.Sprintf(`INSERT INTO repo(id, name, private) VALUES(4, 'unrestricted_repo_4', TRUE)`),
		sqlf.Sprintf(`INSERT INTO external_services(id, display_name, kind, config) VALUES(1, 'GitHub #1', 'GITHUB', '{}')`),
		sqlf.Sprintf(`INSERT INTO external_service_repos(repo_id, external_service_id, clone_url)
                                 VALUES(1, 1, ''), (2, 1, ''), (3, 1, '')`),
		sqlf.Sprintf(`INSERT INTO external_services(id, display_name, kind, config, unrestricted) VALUES(2, 'GitHub #2 Unrestricted', 'GITHUB', '{}', TRUE)`),
		sqlf.Sprintf(`INSERT INTO external_service_repos(repo_id, external_service_id, clone_url)
                                 VALUES(4, 2, '')`),
	}

	for _, q := range qs {
		if err := s.execute(ctx, q); err != nil {
			t.Fatal(err)
		}
	}

	q := sqlf.Sprintf(`INSERT INTO user_repo_permissions(user_id, repo_id, source) VALUES(555, 1, 'user_sync'), (666, 1, 'api'), (NULL, 3, 'api'), (666, 4, 'user_sync'), (999, 1, 'user_sync')`)
	if err := s.execute(ctx, q); err != nil {
		t.Fatal(err)
	}

	ctx = actor.WithActor(ctx, &actor.Actor{UID: 999})

	tests := []listRepoPermissionsTest{
		{
			Name:   "TestPrivateRepo",
			RepoID: 1,
			Args:   nil,
			WantResults: []*listRepoPermissionsResult{
				{
					// have access and site-admin
					UserID: 999,
					Reason: UserRepoPermissionReasonSiteAdmin,
				},
				{
					// do not have access but site-admin
					UserID: 777,
					Reason: UserRepoPermissionReasonSiteAdmin,
				},
				{
					// have access
					UserID: 666,
					Reason: UserRepoPermissionReasonExplicitPerms,
				},
				{
					// have access
					UserID: 555,
					Reason: UserRepoPermissionReasonPermissionsSync,
				},
			},
		},
		{
			Name:             "TestPrivateRepoWithNoAuthzProviders",
			RepoID:           1,
			Args:             nil,
			NoAuthzProviders: true,
			// all users have access
			WantResults: []*listRepoPermissionsResult{
				{
					UserID: 999,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 777,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 666,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 555,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
			},
		},
		{
			Name:   "TestPaginationWithPrivateRepo",
			RepoID: 1,
			Args: &ListRepoPermissionsArgs{
				PaginationArgs: &PaginationArgs{First: pointers.Ptr(1), After: []any{555}, OrderBy: OrderBy{{Field: "users.id"}}, Ascending: true},
			},
			WantResults: []*listRepoPermissionsResult{
				{
					UserID: 666,
					Reason: UserRepoPermissionReasonExplicitPerms,
				},
			},
		},
		{
			Name:   "TestSearchQueryWithPrivateRepo",
			RepoID: 1,
			Args: &ListRepoPermissionsArgs{
				Query: "6",
			},
			WantResults: []*listRepoPermissionsResult{
				{
					UserID: 666,
					Reason: UserRepoPermissionReasonExplicitPerms,
				},
			},
		},
		{
			Name:   "TestPublicRepo",
			RepoID: 2,
			Args:   nil,
			// all users have access
			WantResults: []*listRepoPermissionsResult{
				{
					UserID: 999,
					Reason: UserRepoPermissionReasonPublic,
				},
				{
					UserID: 777,
					Reason: UserRepoPermissionReasonPublic,
				},
				{
					UserID: 666,
					Reason: UserRepoPermissionReasonPublic,
				},
				{
					UserID: 555,
					Reason: UserRepoPermissionReasonPublic,
				},
			},
		},
		{
			Name:   "TestUnrestrictedViaPermsTableRepo",
			RepoID: 3,
			Args:   nil,
			// all users have access
			WantResults: []*listRepoPermissionsResult{
				{
					UserID: 999,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 777,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 666,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 555,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
			},
		},
		{
			Name:   "TestUnrestrictedViaExternalServiceRepo",
			RepoID: 4,
			Args:   nil,
			// all users have access
			WantResults: []*listRepoPermissionsResult{
				{
					UserID: 999,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 777,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 666,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 555,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
			},
		},
		{
			Name:                      "TestUnrestrictedViaExternalServiceRepoWithoutPermsMapping",
			RepoID:                    4,
			Args:                      nil,
			NoAuthzProviders:          true,
			UsePermissionsUserMapping: false,
			// restricted access
			WantResults: []*listRepoPermissionsResult{
				{
					UserID: 999,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 777,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 666,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
				{
					UserID: 555,
					Reason: UserRepoPermissionReasonUnrestricted,
				},
			},
		},
		{
			Name:                      "TestPrivateRepoWithAuthzEnforceForSiteAdminsEnabled",
			RepoID:                    1,
			Args:                      nil,
			AuthzEnforceForSiteAdmins: true,
			WantResults: []*listRepoPermissionsResult{
				{
					// have access
					UserID: 999,
					Reason: UserRepoPermissionReasonPermissionsSync,
				},
				{
					// have access
					UserID: 666,
					Reason: UserRepoPermissionReasonExplicitPerms,
				},
				{
					// have access
					UserID: 555,
					Reason: UserRepoPermissionReasonPermissionsSync,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if !test.NoAuthzProviders {
				// Set fake authz providers otherwise authz is bypassed
				authz.SetProviders(false, []authz.Provider{&fakePermsProvider{}})
				defer authz.SetProviders(true, nil)
			}

			conf.Mock(
				&conf.Unified{
					SiteConfiguration: schema.SiteConfiguration{
						PermissionsUserMapping: &schema.PermissionsUserMapping{
							Enabled: test.UsePermissionsUserMapping,
							BindID:  "email",
						},
						AuthzEnforceForSiteAdmins: test.AuthzEnforceForSiteAdmins,
					},
				},
			)
			t.Cleanup(func() {
				conf.Mock(nil)
			})

			results, err := s.ListRepoPermissions(ctx, api.RepoID(test.RepoID), test.Args)
			if err != nil {
				t.Fatal(err)
			}

			if len(test.WantResults) != len(results) {
				t.Fatalf("Results mismatch. Want: %d Got: %d", len(test.WantResults), len(results))
			}

			actualResults := make([]*listRepoPermissionsResult, 0, len(results))
			for _, result := range results {
				actualResults = append(actualResults, &listRepoPermissionsResult{UserID: result.User.ID, Reason: result.Reason})
			}

			if diff := cmp.Diff(test.WantResults, actualResults); diff != "" {
				t.Fatalf("Results mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type listRepoPermissionsTest struct {
	Name                      string
	RepoID                    int
	Args                      *ListRepoPermissionsArgs
	WantResults               []*listRepoPermissionsResult
	NoAuthzProviders          bool
	UsePermissionsUserMapping bool
	AuthzEnforceForSiteAdmins bool
}

type listRepoPermissionsResult struct {
	UserID int32
	Reason UserRepoPermissionReason
}

type fakePermsProvider struct {
	codeHost *extsvc.CodeHost
	extAcct  *extsvc.Account
}

func (p *fakePermsProvider) FetchAccount(context.Context, *types.User, []*extsvc.Account, []string) (mine *extsvc.Account, err error) {
	return p.extAcct, nil
}

func (p *fakePermsProvider) ServiceType() string { return p.codeHost.ServiceType }
func (p *fakePermsProvider) ServiceID() string   { return p.codeHost.ServiceID }
func (p *fakePermsProvider) URN() string         { return extsvc.URN(p.codeHost.ServiceType, 0) }

func (p *fakePermsProvider) ValidateConnection(context.Context) error { return nil }

func (p *fakePermsProvider) FetchUserPerms(context.Context, *extsvc.Account, authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	return nil, nil
}

func (p *fakePermsProvider) FetchRepoPerms(context.Context, *extsvc.Repository, authz.FetchPermsOptions) ([]extsvc.AccountID, error) {
	return nil, nil
}

func equal(t testing.TB, name string, want, have any) {
	t.Helper()
	if diff := cmp.Diff(want, have); diff != "" {
		t.Fatalf("%q: %s", name, diff)
	}
}
