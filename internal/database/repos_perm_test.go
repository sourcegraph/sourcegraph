package database

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

type fakeProvider struct {
	codeHost *extsvc.CodeHost
	extAcct  *extsvc.Account
}

func (p *fakeProvider) FetchAccount(context.Context, *types.User, []*extsvc.Account, []string) (mine *extsvc.Account, err error) {
	return p.extAcct, nil
}

func (p *fakeProvider) ServiceType() string { return p.codeHost.ServiceType }
func (p *fakeProvider) ServiceID() string   { return p.codeHost.ServiceID }
func (p *fakeProvider) URN() string         { return extsvc.URN(p.codeHost.ServiceType, 0) }

func (p *fakeProvider) ValidateConnection(context.Context) error { return nil }

func (p *fakeProvider) FetchUserPerms(context.Context, *extsvc.Account, authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	return nil, nil
}

func (p *fakeProvider) FetchUserPermsByToken(context.Context, string, authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	return nil, nil
}

func (p *fakeProvider) FetchRepoPerms(context.Context, *extsvc.Repository, authz.FetchPermsOptions) ([]extsvc.AccountID, error) {
	return nil, nil
}

func mockExplicitPermsConfig(enabled bool) func() {
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			PermissionsUserMapping: &schema.PermissionsUserMapping{
				Enabled: enabled,
				BindID:  "email",
			},
		},
	})
	return func() { conf.Mock(nil) }
}

// 🚨 SECURITY: Tests are necessary to ensure security.
func TestAuthzQueryConds(t *testing.T) {
	cmpOpts := cmp.AllowUnexported(sqlf.Query{})

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))

	t.Run("When permissions user mapping is enabled", func(t *testing.T) {
		authz.SetProviders(false, []authz.Provider{&fakeProvider{}})
		cleanup := mockExplicitPermsConfig(true)
		t.Cleanup(func() {
			authz.SetProviders(true, nil)
			cleanup()
		})

		got, err := AuthzQueryConds(context.Background(), db)
		require.Nil(t, err, "unexpected error, should have passed without conflict")

		want := authzQuery(false, int32(0))
		if diff := cmp.Diff(want, got, cmpOpts); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("When permissions user mapping is enabled, unrestricted repos work correctly", func(t *testing.T) {
		authz.SetProviders(false, []authz.Provider{&fakeProvider{}})
		cleanup := mockExplicitPermsConfig(true)
		t.Cleanup(func() {
			authz.SetProviders(true, nil)
			cleanup()
		})

		got, err := AuthzQueryConds(context.Background(), db)
		require.NoError(t, err)
		want := authzQuery(false, int32(0))
		if diff := cmp.Diff(want, got, cmpOpts); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
		require.Contains(t, got.Query(sqlf.PostgresBindVar), ExternalServiceUnrestrictedCondition.Query(sqlf.PostgresBindVar))
	})

	u, err := db.Users().Create(context.Background(), NewUser{Username: "testuser"})
	require.NoError(t, err)
	tests := []struct {
		name                string
		setup               func(t *testing.T) (context.Context, DB)
		authzAllowByDefault bool
		wantQuery           *sqlf.Query
	}{
		{
			name: "internal actor bypass checks",
			setup: func(t *testing.T) (context.Context, DB) {
				return actor.WithInternalActor(context.Background()), db
			},
			wantQuery: authzQuery(true, int32(0)),
		},
		{
			name: "no authz provider and not allow by default",
			setup: func(t *testing.T) (context.Context, DB) {
				return context.Background(), db
			},
			wantQuery: authzQuery(false, int32(0)),
		},
		{
			name: "no authz provider but allow by default",
			setup: func(t *testing.T) (context.Context, DB) {
				return context.Background(), db
			},
			authzAllowByDefault: true,
			wantQuery:           authzQuery(true, int32(0)),
		},
		{
			name: "authenticated user is a site admin",
			setup: func(_ *testing.T) (context.Context, DB) {
				require.NoError(t, db.Users().SetIsSiteAdmin(context.Background(), u.ID, true))
				return actor.WithActor(context.Background(), &actor.Actor{UID: u.ID}), db
			},
			wantQuery: authzQuery(true, int32(1)),
		},
		{
			name: "authenticated user is a site admin and AuthzEnforceForSiteAdmins is set",
			setup: func(t *testing.T) (context.Context, DB) {
				require.NoError(t, db.Users().SetIsSiteAdmin(context.Background(), u.ID, true))
				conf.Get().AuthzEnforceForSiteAdmins = true
				t.Cleanup(func() {
					conf.Get().AuthzEnforceForSiteAdmins = false
				})
				return actor.WithActor(context.Background(), &actor.Actor{UID: u.ID}), db
			},
			wantQuery: authzQuery(false, int32(1)),
		},
		{
			name: "authenticated user is not a site admin",
			setup: func(_ *testing.T) (context.Context, DB) {
				require.NoError(t, db.Users().SetIsSiteAdmin(context.Background(), u.ID, false))
				return actor.WithActor(context.Background(), &actor.Actor{UID: 1}), db
			},
			wantQuery: authzQuery(false, int32(1)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authz.SetProviders(test.authzAllowByDefault, nil)
			defer authz.SetProviders(true, nil)

			ctx, mockDB := test.setup(t)
			q, err := AuthzQueryConds(ctx, mockDB)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(test.wantQuery, q, cmpOpts); diff != "" {
				t.Fatalf("Mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func execQuery(t *testing.T, ctx context.Context, db DB, q *sqlf.Query) {
	t.Helper()

	_, err := db.ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
	if err != nil {
		t.Fatalf("Error executing query %v, err: %v", q, err)
	}
}

func setupUnrestrictedDB(t *testing.T, ctx context.Context, db DB) (*types.User, *types.Repo) {
	t.Helper()

	// Add a single user who is NOT a site admin
	alice, err := db.Users().Create(ctx,
		NewUser{
			Email:                 "alice@example.com",
			Username:              "alice",
			Password:              "alice",
			EmailVerificationCode: "alice",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Users().SetIsSiteAdmin(ctx, alice.ID, false)
	if err != nil {
		t.Fatal(err)
	}

	// Set up a private repo that the user does not have access to
	internalCtx := actor.WithInternalActor(ctx)
	privateRepo := mustCreate(internalCtx, t, db,
		&types.Repo{
			Name:    "private_repo",
			Private: true,
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "private_repo",
				ServiceType: extsvc.TypeGitHub,
				ServiceID:   "https://github.com/",
			},
		},
	)
	unrestrictedRepo := mustCreate(internalCtx, t, db,
		&types.Repo{
			Name:    "unrestricted_repo",
			Private: true,
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "unrestricted_repo",
				ServiceType: extsvc.TypeGitHub,
				ServiceID:   "https://github.com/",
			},
		},
	)

	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}
	externalService := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "GITHUB #1",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "repositoryQuery": ["none"], "token": "abc", "authorization": {}}`),
	}
	err = db.ExternalServices().Create(ctx, confGet, externalService)
	if err != nil {
		t.Fatal(err)
	}

	execQuery(t, ctx, db, sqlf.Sprintf(`
INSERT INTO external_service_repos (external_service_id, repo_id, clone_url)
VALUES
	(%s, %s, ''),
	(%s, %s, '');
`,
		externalService.ID, privateRepo.ID,
		externalService.ID, unrestrictedRepo.ID,
	))

	// Insert the repo permissions, mark unrestrictedRepo as unrestricted
	execQuery(t, ctx, db, sqlf.Sprintf(`
INSERT INTO repo_permissions (repo_id, permission, updated_at, unrestricted)
VALUES
	(%s, 'read', %s, FALSE),
	(%s, 'read', %s, TRUE);
`,
		privateRepo.ID, time.Now(),
		unrestrictedRepo.ID, time.Now(),
	))

	// Insert the unified permissions, mark unrestrictedRepo as unrestricted
	execQuery(t, ctx, db, sqlf.Sprintf(`
INSERT INTO user_repo_permissions (user_id, repo_id)
VALUES (NULL, %s);
`, unrestrictedRepo.ID))

	return alice, unrestrictedRepo
}

func TestRepoStore_userCanSeeUnrestricedRepo(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	alice, unrestrictedRepo := setupUnrestrictedDB(t, ctx, db)

	authz.SetProviders(false, []authz.Provider{&fakeProvider{}})
	t.Cleanup(func() {
		authz.SetProviders(true, nil)
	})

	t.Run("Alice cannot see private repo, but can see unrestricted repo", func(t *testing.T) {
		aliceCtx := actor.WithActor(ctx, &actor.Actor{UID: alice.ID})
		repos, err := db.Repos().List(aliceCtx, ReposListOptions{})
		if err != nil {
			t.Fatal(err)
		}
		wantRepos := []*types.Repo{unrestrictedRepo}
		if diff := cmp.Diff(wantRepos, repos, cmpopts.IgnoreFields(types.Repo{}, "Sources")); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	})
}

func setupPublicRepo(t *testing.T, db DB) (*types.User, *types.Repo) {
	ctx := context.Background()

	// Add a single user who is NOT a site admin
	alice, err := db.Users().Create(ctx,
		NewUser{
			Email:                 "alice@example.com",
			Username:              "alice",
			Password:              "alice",
			EmailVerificationCode: "alice",
		},
	)
	require.NoError(t, err)
	require.NoError(t, db.Users().SetIsSiteAdmin(ctx, alice.ID, false))

	publicRepo := mustCreate(ctx, t, db,
		&types.Repo{
			Name:    "public_repo",
			Private: false,
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "public_repo",
				ServiceType: extsvc.TypeGitHub,
				ServiceID:   "https://github.com/",
			},
		},
	)

	externalService := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "GITHUB #1",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "repositoryQuery": ["none"], "token": "abc", "authorization": {}}`),
	}
	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}
	err = db.ExternalServices().Create(ctx, confGet, externalService)
	require.NoError(t, err)

	return alice, publicRepo
}

func TestRepoStore_userCanSeePublicRepo(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	alice, publicRepo := setupPublicRepo(t, db)

	t.Run("Alice can see public repo with explicit permissions ON", func(t *testing.T) {
		authz.SetProviders(false, []authz.Provider{&fakeProvider{}})
		cleanup := mockExplicitPermsConfig(true)

		t.Cleanup(func() {
			cleanup()
			authz.SetProviders(true, nil)
		})

		aliceCtx := actor.WithActor(ctx, &actor.Actor{UID: alice.ID})
		repos, err := db.Repos().List(aliceCtx, ReposListOptions{})
		require.NoError(t, err)
		wantRepos := []*types.Repo{publicRepo}
		if diff := cmp.Diff(wantRepos, repos, cmpopts.IgnoreFields(types.Repo{}, "Sources")); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	})
}

func createGitHubExternalService(t *testing.T, db DB) *types.ExternalService {
	now := time.Now()
	svc := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "authorization": {}, "token": "deadbeef", "repos": ["test/test"]}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := db.ExternalServices().Upsert(context.Background(), svc); err != nil {
		t.Fatal(err)
	}

	return svc
}

func setupDB(t *testing.T, ctx context.Context, db DB) (users map[string]*types.User, repos map[string]*types.Repo) {
	t.Helper()

	users = make(map[string]*types.User)
	repos = make(map[string]*types.Repo)

	createUser := func(username string) *types.User {
		user, err := db.Users().Create(ctx, NewUser{Username: username, Password: username})
		if err != nil {
			t.Fatal(err)
		}
		return user
	}
	// Set up 4 users: admin, alice, bob, cindy. Admin is site-admin because it's created first.
	for _, username := range []string{"admin", "alice", "bob", "cindy"} {
		users[username] = createUser(username)
	}

	// Set up default external service
	siteLevelGitHubService := createGitHubExternalService(t, db)

	// Set up unrestricted external service for cindy
	confGet := func() *conf.Unified { return &conf.Unified{} }
	cindyExternalService := &types.ExternalService{
		Kind:         extsvc.KindGitHub,
		DisplayName:  "GITHUB #1",
		Config:       extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "repositoryQuery": ["none"], "token": "abc"}`),
		Unrestricted: true,
	}
	err := db.ExternalServices().Create(ctx, confGet, cindyExternalService)
	if err != nil {
		t.Fatal(err)
	}

	// Set up repositories
	createRepo := func(name string, es *types.ExternalService) *types.Repo {
		internalCtx := actor.WithInternalActor(ctx)
		repo := mustCreate(internalCtx, t, db,
			&types.Repo{
				Name: api.RepoName(name),
				ExternalRepo: api.ExternalRepoSpec{
					ID:          name,
					ServiceType: extsvc.TypeGitHub,
					ServiceID:   "https://github.com/",
				},
				Private: strings.Contains(name, "_private_"),
			},
		)
		repo.Sources = map[string]*types.SourceInfo{
			es.URN(): {
				ID: es.URN(),
			},
		}
		// Make sure there is a record in external_service_repos table as well
		execQuery(t, ctx, db, sqlf.Sprintf(`
INSERT INTO external_service_repos (external_service_id, repo_id, clone_url)
VALUES (%s, %s, '')
`, es.ID, repo.ID))
		return repo
	}

	// Set public and private repos for both alice and bob
	for _, username := range []string{"alice", "bob"} {
		publicRepoName := username + "_public_repo"
		privateRepoName := username + "_private_repo"
		repos[publicRepoName] = createRepo(publicRepoName, siteLevelGitHubService)
		repos[privateRepoName] = createRepo(privateRepoName, siteLevelGitHubService)
	}
	// Setup repository for cindy
	repos["cindy_private_repo"] = createRepo("cindy_private_repo", cindyExternalService)

	// Convenience variables for alice and bob
	alice, bob := users["alice"], users["bob"]
	// Convenience variables for alice and bob private repositories
	alicePrivateRepo, bobPrivateRepo := repos["alice_private_repo"], repos["bob_private_repo"]

	// Set up external accounts for alice and bob
	for _, user := range []*types.User{alice, bob} {
		_, err = db.UserExternalAccounts().Upsert(ctx,
			&extsvc.Account{
				UserID:      user.ID,
				AccountSpec: extsvc.AccountSpec{ServiceType: extsvc.TypeGitHub, ServiceID: "https://github.com/", AccountID: user.Username},
			})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set up permissions: alice and bob have access to their own private repositories
	execQuery(t, ctx, db, sqlf.Sprintf(`
INSERT INTO user_permissions (user_id, permission, object_type, object_ids_ints, updated_at)
VALUES
	(%s, 'read', 'repos', %s, NOW()),
	(%s, 'read', 'repos', %s, NOW())
`,
		alice.ID, pq.Array([]int32{int32(alicePrivateRepo.ID)}),
		bob.ID, pq.Array([]int32{int32(bobPrivateRepo.ID)}),
	))

	execQuery(t, ctx, db, sqlf.Sprintf(`
INSERT INTO user_repo_permissions (user_id, user_external_account_id, repo_id)
VALUES
	(%d, %d, %d),
	(%d, %d, %d)
`,
		alice.ID, 1, alicePrivateRepo.ID,
		bob.ID, 2, bobPrivateRepo.ID,
	))

	return users, repos
}

// 🚨 SECURITY: Tests are necessary to ensure security.
func TestRepoStore_List_checkPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	users, repos := setupDB(t, ctx, db)
	admin, alice, bob, cindy := users["admin"], users["alice"], users["bob"], users["cindy"]
	alicePublicRepo, alicePrivateRepo, bobPublicRepo, bobPrivateRepo, cindyPrivateRepo := repos["alice_public_repo"], repos["alice_private_repo"], repos["bob_public_repo"], repos["bob_private_repo"], repos["cindy_private_repo"]

	authz.SetProviders(false, []authz.Provider{&fakeProvider{}})
	defer authz.SetProviders(true, nil)

	assertRepos := func(t *testing.T, ctx context.Context, want []*types.Repo) {
		t.Helper()
		repos, err := db.Repos().List(ctx, ReposListOptions{OrderBy: []RepoListSort{{Field: RepoListID}}})
		if err != nil {
			t.Fatal(err)
		}

		// sort the want slice as well, so that ordering does not matter
		sort.Slice(want, func(i, j int) bool {
			return want[i].ID < want[j].ID
		})

		if diff := cmp.Diff(want, repos); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	}

	t.Run("Internal actor should see all repositories", func(t *testing.T) {
		internalCtx := actor.WithInternalActor(ctx)
		wantRepos := maps.Values(repos)
		assertRepos(t, internalCtx, wantRepos)
	})

	t.Run("Alice should see authorized repositories", func(t *testing.T) {
		// Alice should see "alice_public_repo", "alice_private_repo",
		// "bob_public_repo", "cindy_private_repo".
		// The "cindy_private_repo" comes from an unrestricted external service
		aliceCtx := actor.WithActor(ctx, &actor.Actor{UID: alice.ID})
		wantRepos := []*types.Repo{alicePublicRepo, alicePrivateRepo, bobPublicRepo, cindyPrivateRepo}
		assertRepos(t, aliceCtx, wantRepos)
	})

	t.Run("Bob should see authorized repositories", func(t *testing.T) {
		// Bob should see "alice_public_repo", "bob_private_repo", "bob_public_repo",
		// "cindy_private_repo".
		// The "cindy_private_repo" comes from an unrestricted external service
		bobCtx := actor.WithActor(ctx, &actor.Actor{UID: bob.ID})
		wantRepos := []*types.Repo{alicePublicRepo, bobPublicRepo, bobPrivateRepo, cindyPrivateRepo}
		assertRepos(t, bobCtx, wantRepos)
	})

	t.Run("Site admins see all repos by default", func(t *testing.T) {
		adminCtx := actor.WithActor(ctx, &actor.Actor{UID: admin.ID})
		wantRepos := maps.Values(repos)
		assertRepos(t, adminCtx, wantRepos)
	})

	t.Run("Site admins only see their repos when AuthzEnforceForSiteAdmins is enabled", func(t *testing.T) {
		conf.Get().AuthzEnforceForSiteAdmins = true
		t.Cleanup(func() {
			conf.Get().AuthzEnforceForSiteAdmins = false
		})

		// since there are no permissions, only public and unrestricted repos are visible
		adminCtx := actor.WithActor(ctx, &actor.Actor{UID: admin.ID})
		wantRepos := []*types.Repo{alicePublicRepo, bobPublicRepo, cindyPrivateRepo}
		assertRepos(t, adminCtx, wantRepos)
	})

	t.Run("Cindy does not have permissions, only public and unrestricted repos are authorized", func(t *testing.T) {
		cindyCtx := actor.WithActor(ctx, &actor.Actor{UID: cindy.ID})
		wantRepos := []*types.Repo{alicePublicRepo, bobPublicRepo, cindyPrivateRepo}
		assertRepos(t, cindyCtx, wantRepos)
	})
}

// 🚨 SECURITY: Tests are necessary to ensure security.
func TestRepoStore_List_permissionsUserMapping(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	// Set up three users: alice, bob and admin
	alice, err := db.Users().Create(ctx, NewUser{
		Email:                 "alice@example.com",
		Username:              "alice",
		Password:              "alice",
		EmailVerificationCode: "alice",
	})
	if err != nil {
		t.Fatal(err)
	}
	bob, err := db.Users().Create(ctx, NewUser{
		Email:                 "bob@example.com",
		Username:              "bob",
		Password:              "bob",
		EmailVerificationCode: "bob",
	})
	if err != nil {
		t.Fatal(err)
	}
	admin, err := db.Users().Create(ctx, NewUser{
		Email:                 "admin@example.com",
		Username:              "admin",
		Password:              "admin",
		EmailVerificationCode: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Ensure only "admin" is the site admin, alice was prompted as site admin
	// because it was the first user.
	err = db.Users().SetIsSiteAdmin(ctx, admin.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Users().SetIsSiteAdmin(ctx, alice.ID, false)
	if err != nil {
		t.Fatal(err)
	}

	siteLevelGitHubService := createGitHubExternalService(t, db)

	// Set up some repositories: public and private for both alice and bob
	internalCtx := actor.WithInternalActor(ctx)
	alicePublicRepo := mustCreate(internalCtx, t, db,
		&types.Repo{
			Name: "alice_public_repo",
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "alice_public_repo",
				ServiceType: extsvc.TypeGitHub,
				ServiceID:   "https://github.com/",
			},
		},
	)
	alicePublicRepo.Sources = map[string]*types.SourceInfo{
		siteLevelGitHubService.URN(): {
			ID: siteLevelGitHubService.URN(),
		},
	}

	alicePrivateRepo := mustCreate(internalCtx, t, db,
		&types.Repo{
			Name:    "alice_private_repo",
			Private: true,
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "alice_private_repo",
				ServiceType: extsvc.TypeGitHub,
				ServiceID:   "https://github.com/",
			},
		},
	)
	alicePrivateRepo.Sources = map[string]*types.SourceInfo{
		siteLevelGitHubService.URN(): {
			ID: siteLevelGitHubService.URN(),
		},
	}

	bobPublicRepo := mustCreate(internalCtx, t, db,
		&types.Repo{
			Name: "bob_public_repo",
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "bob_public_repo",
				ServiceType: extsvc.TypeGitHub,
				ServiceID:   "https://github.com/",
			},
		},
	)
	bobPublicRepo.Sources = map[string]*types.SourceInfo{
		siteLevelGitHubService.URN(): {
			ID: siteLevelGitHubService.URN(),
		},
	}

	bobPrivateRepo := mustCreate(internalCtx, t, db,
		&types.Repo{
			Name:    "bob_private_repo",
			Private: true,
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "bob_private_repo",
				ServiceType: extsvc.TypeGitHub,
				ServiceID:   "https://github.com/",
			},
		},
	)
	bobPrivateRepo.Sources = map[string]*types.SourceInfo{
		siteLevelGitHubService.URN(): {
			ID: siteLevelGitHubService.URN(),
		},
	}

	// Make sure that alicePublicRepo, alicePrivateRepo, bobPublicRepo and bobPrivateRepo have an
	// entry in external_service_repos table.
	repoIDs := []api.RepoID{
		alicePublicRepo.ID,
		alicePrivateRepo.ID,
		bobPublicRepo.ID,
		bobPrivateRepo.ID,
	}

	for _, id := range repoIDs {
		q := sqlf.Sprintf(`
INSERT INTO external_service_repos (external_service_id, repo_id, clone_url)
VALUES (%s, %s, '')
`, siteLevelGitHubService.ID, id)
		_, err = db.ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set up permissions: alice and bob have access to their own private repositories
	q := sqlf.Sprintf(`
INSERT INTO user_repo_permissions (user_id, repo_id, created_at, updated_at)
VALUES
	(%s, %s, NOW(), NOW()),
	(%s, %s, NOW(), NOW())
`,
		alice.ID, alicePrivateRepo.ID,
		bob.ID, bobPrivateRepo.ID,
	)
	_, err = db.ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
	if err != nil {
		t.Fatal(err)
	}

	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			PermissionsUserMapping: &schema.PermissionsUserMapping{
				Enabled: true,
				BindID:  "email",
			},
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })

	// Alice should see "alice_private_repo" and public repos, but not "bob_private_repo"
	aliceCtx := actor.WithActor(ctx, &actor.Actor{UID: alice.ID})
	repos, err := db.Repos().List(aliceCtx, ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantRepos := []*types.Repo{alicePublicRepo, alicePrivateRepo, bobPublicRepo}
	if diff := cmp.Diff(wantRepos, repos); diff != "" {
		t.Fatalf("Mismatch (-want +got):\n%s", diff)
	}

	// Bob should see "bob_private_repo" and public repos, but not "alice_public_repo"
	bobCtx := actor.WithActor(ctx, &actor.Actor{UID: bob.ID})
	repos, err = db.Repos().List(bobCtx, ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantRepos = []*types.Repo{alicePublicRepo, bobPublicRepo, bobPrivateRepo}
	if diff := cmp.Diff(wantRepos, repos); diff != "" {
		t.Fatalf("Mismatch (-want +got):\n%s", diff)
	}

	// By default, admins can see all repos
	adminCtx := actor.WithActor(ctx, &actor.Actor{UID: admin.ID})
	repos, err = db.Repos().List(adminCtx, ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantRepos = []*types.Repo{alicePublicRepo, alicePrivateRepo, bobPublicRepo, bobPrivateRepo}
	if diff := cmp.Diff(wantRepos, repos); diff != "" {
		t.Fatalf("Mismatch (-want +got):\n%s", diff)
	}

	// Admin can only see public repos as they have not been granted permissions and
	// AuthzEnforceForSiteAdmins is set
	conf.Get().AuthzEnforceForSiteAdmins = true
	t.Cleanup(func() {
		conf.Get().AuthzEnforceForSiteAdmins = false
	})
	adminCtx = actor.WithActor(ctx, &actor.Actor{UID: admin.ID})
	repos, err = db.Repos().List(adminCtx, ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantRepos = []*types.Repo{alicePublicRepo, bobPublicRepo}
	if diff := cmp.Diff(wantRepos, repos); diff != "" {
		t.Fatalf("Mismatch (-want +got):\n%s", diff)
	}

	// A random user sees only public repos
	repos, err = db.Repos().List(ctx, ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantRepos = []*types.Repo{alicePublicRepo, bobPublicRepo}
	if diff := cmp.Diff(wantRepos, repos); diff != "" {
		t.Fatalf("Mismatch (-want +got):\n%s", diff)
	}
}

func benchmarkAuthzQuery(b *testing.B, numRepos, numUsers, reposPerUser int) {
	// disable security access logs, which pollute the output of benchmark
	prevConf := conf.Get()
	conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{
		Log: &schema.Log{
			SecurityEventLog: &schema.SecurityEventLog{Location: "none"},
		},
	}})
	b.Cleanup(func() {
		conf.Mock(prevConf)
	})

	logger := logtest.Scoped(b)
	db := NewDB(logger, dbtest.NewDB(b))
	ctx := context.Background()

	b.Logf("Creating %d repositories...", numRepos)

	repoInserter := batch.NewInserter(ctx, db.Handle(), "repo", batch.MaxNumPostgresParameters, "name", "private")
	repoPermissionsInserter := batch.NewInserter(ctx, db.Handle(), "repo_permissions", batch.MaxNumPostgresParameters, "repo_id", "permission", "updated_at", "synced_at", "unrestricted")
	for i := 1; i <= numRepos; i++ {
		if err := repoInserter.Insert(ctx, fmt.Sprintf("repo-%d", i), true); err != nil {
			b.Fatal(err)
		}
		if err := repoPermissionsInserter.Insert(ctx, i, "read", "now()", "now()", false); err != nil {
			b.Fatal(err)
		}
	}
	if err := repoInserter.Flush(ctx); err != nil {
		b.Fatal(err)
	}
	if err := repoPermissionsInserter.Flush(ctx); err != nil {
		b.Fatal(err)
	}

	b.Logf("Done creating %d repositories.", numRepos)

	b.Logf("Creating %d users...", numRepos)

	userInserter := batch.NewInserter(ctx, db.Handle(), "users", batch.MaxNumPostgresParameters, "username")
	for i := 1; i <= numUsers; i++ {
		if err := userInserter.Insert(ctx, fmt.Sprintf("user-%d", i)); err != nil {
			b.Fatal(err)
		}
	}
	if err := userInserter.Flush(ctx); err != nil {
		b.Fatal(err)
	}

	b.Logf("Done creating %d users.", numUsers)

	b.Logf("Creating %d external accounts...", numUsers)

	externalAccountInserter := batch.NewInserter(ctx, db.Handle(), "user_external_accounts", batch.MaxNumPostgresParameters, "user_id", "account_id", "service_type", "service_id", "client_id")
	for i := 1; i <= numUsers; i++ {
		if err := externalAccountInserter.Insert(ctx, i, fmt.Sprintf("test-account-%d", i), "test", "test", "test"); err != nil {
			b.Fatal(err)
		}
	}
	if err := externalAccountInserter.Flush(ctx); err != nil {
		b.Fatal(err)
	}

	b.Logf("Done creating %d external accounts.", numUsers)

	b.Logf("Creating %d permissions...", numUsers*reposPerUser)

	userPermissionsInserter := batch.NewInserter(ctx, db.Handle(), "user_permissions", batch.MaxNumPostgresParameters, "user_id", "object_ids_ints", "permission", "object_type", "updated_at", "synced_at")
	userRepoPermissionsInserter := batch.NewInserter(ctx, db.Handle(), "user_repo_permissions", batch.MaxNumPostgresParameters, "user_id", "user_external_account_id", "repo_id", "source")
	for i := 1; i <= numUsers; i++ {
		objectIDs := make(map[int]struct{})
		// Assign a random set of repos to the user
		for range reposPerUser {
			repoID := rand.Intn(numRepos) + 1
			objectIDs[repoID] = struct{}{}
		}

		if err := userPermissionsInserter.Insert(ctx, i, maps.Keys(objectIDs), "read", "repos", "now()", "now()"); err != nil {
			b.Fatal(err)
		}

		for repoID := range objectIDs {
			if err := userRepoPermissionsInserter.Insert(ctx, i, i, repoID, "test"); err != nil {
				b.Fatal(err)
			}
		}
	}
	if err := userPermissionsInserter.Flush(ctx); err != nil {
		b.Fatal(err)
	}
	if err := userRepoPermissionsInserter.Flush(ctx); err != nil {
		b.Fatal(err)
	}

	b.Logf("Done creating %d permissions.", numUsers*reposPerUser)

	fetchMinRepos := func() {
		randomUserID := int32(rand.Intn(numUsers)) + 1
		ctx := actor.WithActor(ctx, &actor.Actor{UID: randomUserID})
		if _, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{}); err != nil {
			b.Fatalf("unexpected error: %s", err)
		}
	}

	b.ResetTimer()

	b.Run("list repos", func(b *testing.B) {
		for range b.N {
			fetchMinRepos()
		}
	})
}

// 2022-03-02 - MacBook Pro M1 Max
//
// φ go test -v -timeout=900s -run=XXX -benchtime=10s -bench BenchmarkAuthzQuery ./internal/database
// goos: darwin
// goarch: arm64
// pkg: github.com/sourcegraph/sourcegraph/internal/database
// BenchmarkAuthzQuery_ListMinimalRepos_1000repos_1000users_50reposPerUser/list_repos,_using_unified_user_repo_permissions_table-10                   23928            495697 ns/op
// BenchmarkAuthzQuery_ListMinimalRepos_1000repos_1000users_50reposPerUser/list_repos,_using_legacy_user_permissions_table-10                         24399            486467 ns/op

// BenchmarkAuthzQuery_ListMinimalRepos_10krepos_10kusers_150reposPerUser/list_repos,_using_unified_user_repo_permissions_table-10                     3180           4023709 ns/op
// BenchmarkAuthzQuery_ListMinimalRepos_10krepos_10kusers_150reposPerUser/list_repos,_using_legacy_user_permissions_table-10                           2911           4020591 ns/op

// BenchmarkAuthzQuery_ListMinimalRepos_10krepos_10kusers_500reposPerUser/list_repos,_using_unified_user_repo_permissions_table-10                     3201           4101237 ns/op
// BenchmarkAuthzQuery_ListMinimalRepos_10krepos_10kusers_500reposPerUser/list_repos,_using_legacy_user_permissions_table-10                           2944           4144971 ns/op

// BenchmarkAuthzQuery_ListMinimalRepos_500krepos_40kusers_500reposPerUser/list_repos,_using_unified_user_repo_permissions_table-10                      63         186395579 ns/op
// BenchmarkAuthzQuery_ListMinimalRepos_500krepos_40kusers_500reposPerUser/list_repos,_using_legacy_user_permissions_table-10                            62         190570966 ns/op

func BenchmarkAuthzQuery_ListMinimalRepos_1000repos_1000users_50reposPerUser(b *testing.B) {
	benchmarkAuthzQuery(b, 1_000, 1_000, 50)
}

func BenchmarkAuthzQuery_ListMinimalRepos_10krepos_10kusers_150reposPerUser(b *testing.B) {
	benchmarkAuthzQuery(b, 10_000, 10_000, 150)
}

func BenchmarkAuthzQuery_ListMinimalRepos_10krepos_10kusers_500reposPerUser(b *testing.B) {
	benchmarkAuthzQuery(b, 10_000, 10_000, 500)
}

func BenchmarkAuthzQuery_ListMinimalRepos_500krepos_40kusers_500reposPerUser(b *testing.B) {
	benchmarkAuthzQuery(b, 500_000, 40_000, 500)
}
