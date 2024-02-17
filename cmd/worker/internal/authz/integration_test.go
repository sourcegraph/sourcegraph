package authz

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/regexp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	authzGitHub "github.com/sourcegraph/sourcegraph/internal/authz/providers/github"
	authzGitLab "github.com/sourcegraph/sourcegraph/internal/authz/providers/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	extsvcGitHub "github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

var updateRegex = flag.String("update-integration", "", "Update testdata of tests matching the given regex")

func update(name string) bool {
	if updateRegex == nil || *updateRegex == "" {
		return false
	}
	return regexp.MustCompile(*updateRegex).MatchString(name)
}

func assertGitHubUserPermissions(t *testing.T, ctx context.Context, userID int32, ghURL string, syncer *permsSyncerImpl, permsStore database.PermsStore, wantIDs []int32) {
	t.Helper()

	_, providerStates, err := syncer.syncUserPerms(ctx, userID, false, authz.FetchPermsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, database.CodeHostStatusesSet{{
		ProviderID:   ghURL,
		ProviderType: "github",
		Status:       database.CodeHostStatusSuccess,
		Message:      "FetchUserPerms",
	}}, providerStates)

	p, err := permsStore.LoadUserPermissions(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	gotIDs := make([]int32, len(p))
	for i, perm := range p {
		gotIDs[i] = perm.RepoID
	}

	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("IDs mismatch (-want +got):\n%s", diff)
	}
}

func assertGitHubRepoPermissions(t *testing.T, ctx context.Context, repoID api.RepoID, userID int32, ghURL string, syncer *permsSyncerImpl, permsStore database.PermsStore, wantIDs []int32) {
	t.Helper()

	_, providerStates, err := syncer.syncRepoPerms(ctx, repoID, false, authz.FetchPermsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, database.CodeHostStatusesSet{{
		ProviderID:   ghURL,
		ProviderType: "github",
		Status:       database.CodeHostStatusSuccess,
		Message:      "FetchRepoPerms",
	}}, providerStates)

	p, err := permsStore.LoadUserPermissions(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	gotIDs := make([]int32, len(p))
	for i, perm := range p {
		gotIDs[i] = perm.RepoID
	}

	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Fatalf("IDs mismatch (-want +got):\n%s", diff)
	}
}

// NOTE: To update VCR for these tests, please use the token of "sourcegraph-vcr"
// for GITHUB_TOKEN, which can be found in 1Password.
//
// We also recommend setting up a new token for "sourcegraph-vcr" using the auth scope
// guidelines https://sourcegraph.com/docs/admin/external_service/github#github-api-access
// to ensure everything works, in case of new scopes being required.
func TestIntegration_GitHubPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ratelimit.SetupForTest(t)

	logger := logtest.Scoped(t)
	token := os.Getenv("GITHUB_TOKEN")

	spec := extsvc.AccountSpec{
		ServiceType: extsvc.TypeGitHub,
		ServiceID:   "https://github.com/",
		AccountID:   "66464926",
	}
	svc := types.ExternalService{
		Kind:      extsvc.KindGitHub,
		CreatedAt: timeutil.Now(),
		Config:    extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "authorization": {}, "token": "abc", "repos": ["owner/name"]}`),
	}
	uri, err := url.Parse("https://github.com/")
	if err != nil {
		t.Fatal(err)
	}

	newUser := database.NewUser{
		Email:           "sourcegraph-vcr-bob@sourcegraph.com",
		Username:        "sourcegraph-vcr-bob",
		EmailIsVerified: true,
	}
	testDB := database.NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	reposStore := repos.NewStore(logtest.Scoped(t), testDB)

	err = reposStore.ExternalServiceStore().Upsert(ctx, &svc)
	if err != nil {
		t.Fatal(err)
	}

	repo := types.Repo{
		Name:    "github.com/sourcegraph-vcr-repos/private-org-repo-1",
		Private: true,
		URI:     "github.com/sourcegraph-vcr-repos/private-org-repo-1",
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "MDEwOlJlcG9zaXRvcnkzOTk4OTQyODY=",
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com/",
		},
		Sources: map[string]*types.SourceInfo{
			svc.URN(): {
				ID: svc.URN(),
			},
		},
	}

	err = reposStore.RepoStore().Create(ctx, &repo)
	if err != nil {
		t.Fatal(err)
	}

	authData := json.RawMessage(fmt.Sprintf(`{"access_token": "%s"}`, token))
	user, err := testDB.Users().CreateWithExternalAccount(ctx, newUser,
		&extsvc.Account{
			AccountSpec: spec,
			AccountData: extsvc.AccountData{
				AuthData: extsvc.NewUnencryptedData(authData),
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	permsStore := database.Perms(logger, testDB, timeutil.Now)
	syncer := newPermsSyncer(logger, testDB, reposStore, permsStore, timeutil.Now)

	// This integration tests performs a repository-centric permissions syncing against
	// https://github.com, then check if permissions are correctly granted for the test
	// user "sourcegraph-vcr-bob", who is a outside collaborator of the repository
	// "sourcegraph-vcr-repos/private-org-repo-1".
	t.Run("repo-centric", func(t *testing.T) {
		t.Run("no-groups", func(t *testing.T) {
			cli := newTestRecorderClient(t, svc.URN(), uri, token)

			provider, err := authzGitHub.NewProvider(svc.URN(), authzGitHub.ProviderOptions{
				GitHubClient:   cli,
				GitHubURL:      uri,
				BaseAuther:     &auth.OAuthBearerToken{Token: token},
				GroupsCacheTTL: -1,
				DB:             testDB,
			})
			require.NoError(t, err)

			authz.SetProviders(false, []authz.Provider{provider})
			defer authz.SetProviders(true, nil)

			assertGitHubRepoPermissions(t, ctx, repo.ID, user.ID, uri.String(), syncer, permsStore, []int32{1})
		})

		t.Run("groups-enabled", func(t *testing.T) {
			cli := newTestRecorderClient(t, svc.URN(), uri, token)

			provider, err := authzGitHub.NewProvider(svc.URN(), authzGitHub.ProviderOptions{
				GitHubClient:   cli,
				GitHubURL:      uri,
				BaseAuther:     &auth.OAuthBearerToken{Token: token},
				GroupsCacheTTL: 72,
				DB:             testDB,
			})
			require.NoError(t, err)

			authz.SetProviders(false, []authz.Provider{provider})
			defer authz.SetProviders(true, nil)

			assertGitHubRepoPermissions(t, ctx, repo.ID, user.ID, uri.String(), syncer, permsStore, []int32{1})
		})
	})

	// This integration tests performs a user-centric permissions syncing against
	// https://github.com, then check if permissions are correctly granted for the test
	// user "sourcegraph-vcr-bob", who is a collaborator of "sourcegraph-vcr-repos/private-org-repo-1".
	t.Run("user-centric", func(t *testing.T) {
		t.Run("no-groups", func(t *testing.T) {
			cli := newTestRecorderClient(t, svc.URN(), uri, token)

			provider, err := authzGitHub.NewProvider(svc.URN(), authzGitHub.ProviderOptions{
				GitHubClient:   cli,
				GitHubURL:      uri,
				BaseAuther:     &auth.OAuthBearerToken{Token: token},
				GroupsCacheTTL: -1,
				DB:             testDB,
			})
			require.NoError(t, err)

			authz.SetProviders(false, []authz.Provider{provider})
			defer authz.SetProviders(true, nil)

			assertGitHubUserPermissions(t, ctx, user.ID, uri.String(), syncer, permsStore, []int32{1})
		})

		t.Run("groups-enabled", func(t *testing.T) {
			cli := newTestRecorderClient(t, svc.URN(), uri, token)

			provider, err := authzGitHub.NewProvider(svc.URN(), authzGitHub.ProviderOptions{
				GitHubClient:   cli,
				GitHubURL:      uri,
				BaseAuther:     &auth.OAuthBearerToken{Token: token},
				GroupsCacheTTL: 72,
				DB:             testDB,
			})
			require.NoError(t, err)

			authz.SetProviders(false, []authz.Provider{provider})
			defer authz.SetProviders(true, nil)

			assertGitHubUserPermissions(t, ctx, user.ID, uri.String(), syncer, permsStore, []int32{1})
		})
	})
}

func newTestRecorderClient(t *testing.T, urn string, apiURL *url.URL, token string) *extsvcGitHub.V3Client {
	name := t.Name()

	cf, save := httptestutil.NewGitHubRecorderFactory(t, update(name), name)
	t.Cleanup(save)
	cli, err := extsvcGitHub.NewV3Client(logtest.Scoped(t), urn, apiURL, &auth.OAuthBearerToken{Token: token}, cf)
	require.NoError(t, err)
	return cli
}

// TestIntegration_GitHubInternalRepositories asserts that internal repositories of
// organizations that a user belongs to are fetched alongside user permission
// syncs.
//
// The test setup requires a user that belongs to an organization with internal repos.
// It is kept separate from the other integration test since it connects to
// ghe.sgdev.org instead of github.com
func TestIntegration_GitHubInternalRepositories(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ratelimit.SetupForTest(t)
	rcache.SetupForTest(t)

	logger := logtest.Scoped(t)
	token := os.Getenv("GITHUB_TOKEN")

	spec := extsvc.AccountSpec{
		ServiceType: extsvc.TypeGitHub,
		ServiceID:   "https://ghe.sgdev.org/",
		AccountID:   "3",
	}
	svc := types.ExternalService{
		Kind:      extsvc.KindGitHub,
		CreatedAt: timeutil.Now(),
		Config:    extsvc.NewUnencryptedConfig(`{"url": "https://ghe.sgdev.org/", "authorization": {}, "token": "abc", "repos": ["owner/name"]}`),
	}
	uri, err := url.Parse("https://ghe.sgdev.org/")
	if err != nil {
		t.Fatal(err)
	}
	apiURI, _ := github.APIRoot(uri)

	cli := newTestRecorderClient(t, uri.String(), apiURI, token)

	testDB := database.NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	reposStore := repos.NewStore(logtest.Scoped(t), testDB)

	err = reposStore.ExternalServiceStore().Upsert(ctx, &svc)
	if err != nil {
		t.Fatal(err)
	}

	provider, err := authzGitHub.NewProvider(svc.URN(), authzGitHub.ProviderOptions{
		GitHubClient:   cli,
		GitHubURL:      uri,
		BaseAuther:     &auth.OAuthBearerToken{Token: token},
		GroupsCacheTTL: -1, // disable groups caching
		DB:             testDB,
	})
	require.NoError(t, err)

	authz.SetProviders(false, []authz.Provider{provider})
	defer authz.SetProviders(true, nil)

	repo := types.Repo{
		Name:    "ghe.sgdev.org/sourcegraph/sourcegraph_internal_repo",
		Private: true,
		URI:     "ghe.sgdev.org/sourcegraph/sourcegraph_internal_repo",
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "MDEwOlJlcG9zaXRvcnkxMDU3MDMy",
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://ghe.sgdev.org/",
		},
		Sources: map[string]*types.SourceInfo{
			svc.URN(): {
				ID: svc.URN(),
			},
		},
	}
	err = reposStore.RepoStore().Create(ctx, &repo)
	if err != nil {
		t.Fatal(err)
	}

	newUser := database.NewUser{
		Email:           "sourcegraph-vcr@sourcegraph.com",
		Username:        "sourcegraph-vcr",
		EmailIsVerified: true,
	}

	authData := json.RawMessage(fmt.Sprintf(`{"access_token": "%s"}`, token))
	user, err := testDB.Users().CreateWithExternalAccount(ctx, newUser, &extsvc.Account{
		AccountSpec: spec,
		AccountData: extsvc.AccountData{
			AuthData: extsvc.NewUnencryptedData(authData),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	permsStore := database.Perms(logger, testDB, timeutil.Now)
	syncer := newPermsSyncer(logger, testDB, reposStore, permsStore, timeutil.Now)

	assertGitHubUserPermissions(t, ctx, user.ID, uri.String(), syncer, permsStore, []int32{1})
}

func TestIntegration_GitLabPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	token := os.Getenv("GITLAB_TOKEN")

	spec := extsvc.AccountSpec{
		ServiceType: extsvc.TypeGitLab,
		ServiceID:   "https://gitlab.sgdev.org/",
		AccountID:   "107564",
	}
	svc := types.ExternalService{
		Kind:   extsvc.KindGitLab,
		Config: extsvc.NewUnencryptedConfig(`{"url": "https://gitlab.sgdev.org", "authorization": {"identityProvider": {"type": "oauth"}}, "token": "abc", "projectQuery": [ "projects?membership=true&archived=no" ]}`),
	}
	uri, err := url.Parse("https://gitlab.sgdev.org")
	if err != nil {
		t.Fatal(err)
	}

	newUser := database.NewUser{
		Email:           "sourcegraph-vcr@sourcegraph.com",
		Username:        "sourcegraph-vcr",
		EmailIsVerified: true,
	}

	// These tests require two repos to be set up:
	// Both schwifty2 and getschwifty are internal projects.
	// The user is an explicit collaborator on getschwifty, so
	// should have access to getschwifty regardless of the feature flag.
	// The user does not have explicit access to schwifty2, however
	// schwifty2 is configured so that anyone on the instance has read
	// access, so when the feature flag is enabled, the user should
	// see this repo as well.
	testRepos := []types.Repo{
		{
			Name:    "gitlab.sgdev.org/petrissupercoolgroup/schwifty2",
			Private: true,
			URI:     "gitlab.sgdev.org/petrissupercoolgroup/schwifty2",
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "371335",
				ServiceType: extsvc.TypeGitLab,
				ServiceID:   "https://gitlab.sgdev.org/",
			},
			Sources: map[string]*types.SourceInfo{
				svc.URN(): {
					ID: svc.URN(),
				},
			},
		},
		{
			Name:    "gitlab.sgdev.org/petri.last/getschwifty",
			Private: true,
			URI:     "gitlab.sgdev.org/petri.last/getschwifty",
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "371334",
				ServiceType: extsvc.TypeGitLab,
				ServiceID:   "https://gitlab.sgdev.org/",
			},
			Sources: map[string]*types.SourceInfo{
				svc.URN(): {
					ID: svc.URN(),
				},
			},
		},
	}

	authData := json.RawMessage(fmt.Sprintf(`{"access_token": "%s"}`, token))

	// This integration tests performs a user-centric permissions syncing against
	// https://gitlab.sgdev.org, then check if permissions are correctly granted for the test
	// user "sourcegraph-vcr".
	t.Run("test gitLabProjectVisibilityExperimental feature flag", func(t *testing.T) {
		name := t.Name()

		cf, save := httptestutil.NewRecorderFactory(t, update(name), name)
		defer save()

		testDB := database.NewDB(logger, dbtest.NewDB(t))

		ctx := actor.WithInternalActor(context.Background())

		reposStore := repos.NewStore(logtest.Scoped(t), testDB)

		err = reposStore.ExternalServiceStore().Upsert(ctx, &svc)
		require.NoError(t, err)

		provider, err := authzGitLab.NewOAuthProvider(authzGitLab.OAuthProviderOp{
			BaseURL:                     uri,
			DB:                          testDB,
			HTTPFactory:                 cf,
			SyncInternalRepoPermissions: true,
		})
		require.NoError(t, err)

		authz.SetProviders(false, []authz.Provider{provider})
		defer authz.SetProviders(true, nil)
		for _, repo := range testRepos {
			err = reposStore.RepoStore().Create(ctx, &repo)
			require.NoError(t, err)
		}

		user, err := testDB.Users().CreateWithExternalAccount(ctx, newUser, &extsvc.Account{
			AccountSpec: spec,
			AccountData: extsvc.AccountData{
				AuthData: extsvc.NewUnencryptedData(authData),
			},
		})
		require.NoError(t, err)

		permsStore := database.Perms(logger, testDB, timeutil.Now)
		syncer := newPermsSyncer(logger, testDB, reposStore, permsStore, timeutil.Now)

		assertUserPermissions := func(t *testing.T, wantIDs []int32) {
			t.Helper()
			_, providerStates, err := syncer.syncUserPerms(ctx, user.ID, false, authz.FetchPermsOptions{})
			require.NoError(t, err)

			assert.Equal(t, database.CodeHostStatusesSet{{
				ProviderID:   "https://gitlab.sgdev.org/",
				ProviderType: "gitlab",
				Status:       database.CodeHostStatusSuccess,
				Message:      "FetchUserPerms",
			}}, providerStates)

			p, err := permsStore.LoadUserPermissions(ctx, user.ID)
			require.NoError(t, err)

			gotIDs := make([]int32, len(p))
			for i, perm := range p {
				gotIDs[i] = perm.RepoID
			}

			if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
				t.Fatalf("IDs mismatch (-want +got):\n%s", diff)
			}
		}

		// With the feature flag disabled (default state) the user should only have access to one repo
		assertUserPermissions(t, []int32{2})

		// With the feature flag enabled the user should have access to both repositories
		_, err = testDB.FeatureFlags().CreateBool(ctx, "gitLabProjectVisibilityExperimental", true)
		require.NoError(t, err, "feature flag creation failed")

		assertUserPermissions(t, []int32{1, 2})
	})
}
