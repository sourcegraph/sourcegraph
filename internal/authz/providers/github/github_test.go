package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gregjones/httpcache"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func mustURL(t *testing.T, u string) *url.URL {
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func memGroupsCache() *cachedGroups {
	return &cachedGroups{cache: httpcache.NewMemoryCache()}
}

func mockClientFunc(mockClient client) func() (client, error) {
	return func() (client, error) {
		return mockClient, nil
	}
}

func stableSortRepoID(v []extsvc.RepoID) {
	slices.Sort(v)
}

// newMockClientWithTokenMock is used to keep the behaviour of WithToken function mocking
// which is lost during moving the client interface to mockgen usage
func newMockClientWithTokenMock() *MockClient {
	mockClient := NewMockClient()
	mockClient.WithAuthenticatorFunc.SetDefaultReturn(mockClient)
	return mockClient
}

func TestProvider_FetchUserPerms(t *testing.T) {
	db := dbmocks.NewMockDB()
	t.Run("nil account", func(t *testing.T) {
		p, err := NewProvider("", ProviderOptions{GitHubURL: mustURL(t, "https://github.com"), DB: db})
		require.NoError(t, err)
		_, err = p.FetchUserPerms(context.Background(), nil, authz.FetchPermsOptions{})
		want := "no account provided"
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("not the code host of the account", func(t *testing.T) {
		p, err := NewProvider("", ProviderOptions{GitHubURL: mustURL(t, "https://github.com"), DB: db})
		require.NoError(t, err)
		_, err = p.FetchUserPerms(context.Background(),
			&extsvc.Account{
				AccountSpec: extsvc.AccountSpec{
					ServiceType: "gitlab",
					ServiceID:   "https://gitlab.com/",
				},
			},
			authz.FetchPermsOptions{},
		)
		want := `not a code host of the account: want "https://gitlab.com/" but have "https://github.com/"`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("no token found in account data", func(t *testing.T) {
		p, err := NewProvider("", ProviderOptions{GitHubURL: mustURL(t, "https://github.com"), DB: db})
		require.NoError(t, err)
		_, err = p.FetchUserPerms(context.Background(),
			&extsvc.Account{
				AccountSpec: extsvc.AccountSpec{
					ServiceType: "github",
					ServiceID:   "https://github.com/",
				},
				AccountData: extsvc.AccountData{},
			},
			authz.FetchPermsOptions{},
		)
		want := `no token found in the external account data`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	var (
		authData    = json.RawMessage(`{"access_token": "my_access_token"}`)
		mockAccount = &extsvc.Account{
			AccountSpec: extsvc.AccountSpec{
				AccountID:   "4567",
				ServiceType: "github",
				ServiceID:   "https://github.com/",
			},
			AccountData: extsvc.AccountData{
				AuthData: extsvc.NewUnencryptedData(authData),
			},
		}

		mockListAffiliatedRepositories = func(_ context.Context, _ github.Visibility, page int, perPage int, _ ...github.RepositoryAffiliation) ([]*github.Repository, bool, int, error) {
			switch page {
			case 1:
				return []*github.Repository{
					{ID: "MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE="},
					{ID: "MDEwOlJlcG9zaXRvcnkyNDQ1MTc1MzY="},
				}, true, 1, nil
			case 2:
				return []*github.Repository{
					{ID: "MDEwOlJlcG9zaXRvcnkyNDI2NTEwMDA="},
				}, false, 1, nil
			}

			return []*github.Repository{}, false, 1, nil
		}

		mockOrgNoRead      = &github.OrgDetails{Org: github.Org{Login: "not-sourcegraph"}, DefaultRepositoryPermission: "none"}
		mockOrgNoRead2     = &github.OrgDetails{Org: github.Org{Login: "not-sourcegraph-2"}, DefaultRepositoryPermission: "none"}
		mockOrgRead        = &github.OrgDetails{Org: github.Org{Login: "sourcegraph"}, DefaultRepositoryPermission: "read"}
		mockListOrgDetails = func(_ context.Context, page int) (orgs []github.OrgDetailsAndMembership, hasNextPage bool, rateLimitCost int, err error) {
			switch page {
			case 1:
				return []github.OrgDetailsAndMembership{{
					// does not have access to this org
					OrgDetails: mockOrgNoRead,
				}, {
					// does not have access to this org
					OrgDetails: mockOrgNoRead2,
					// but is an admin, so has access to all org repos
					OrgMembership: &github.OrgMembership{State: "active", Role: "admin"},
				}}, true, 1, nil
			case 2:
				return []github.OrgDetailsAndMembership{{
					// has access to this org
					OrgDetails: mockOrgRead,
				}}, false, 1, nil
			}
			return nil, false, 1, nil
		}

		mockListOrgRepositories = func(_ context.Context, org string, page int, _ string) (repos []*github.Repository, hasNextPage bool, rateLimitCost int, err error) {
			switch org {
			case mockOrgRead.Login:
				switch page {
				case 1:
					return []*github.Repository{
						{ID: "MDEwOlJlcG9zaXRvcnkyNDQ1MTc1234="},
						{ID: "MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE="}, // existing repo
					}, true, 1, nil
				case 2:
					return []*github.Repository{
						{ID: "MDEwOlJlcG9zaXRvcnkyNDI2NTE5678="},
					}, false, 1, nil
				}
			case mockOrgNoRead2.Login:
				return []*github.Repository{{ID: "MDEwOlJlcG9zaXRvcnkyNDI2NTadmin="}}, false, 1, nil
			}
			t.Fatalf("unexpected call to ListOrgRepositories with org %q page %d", org, page)
			return nil, false, 1, nil
		}
	)

	t.Run("cache disabled", func(t *testing.T) {
		mockClient := newMockClientWithTokenMock()
		mockClient.ListAffiliatedRepositoriesFunc.SetDefaultHook(
			func(ctx context.Context, visibility github.Visibility, page int, perPage int, affiliations ...github.RepositoryAffiliation) (repos []*github.Repository, hasNextPage bool, rateLimitCost int, err error) {
				if len(affiliations) != 0 {
					t.Fatalf("Expected 0 affiliations, got %+v", affiliations)
				}
				return mockListAffiliatedRepositories(ctx, visibility, page, perPage, affiliations...)
			})

		p, err := NewProvider("", ProviderOptions{
			GitHubURL:      mustURL(t, "https://github.com"),
			GroupsCacheTTL: time.Duration(-1),
			DB:             db,
		})
		require.NoError(t, err)
		p.client = mockClientFunc(mockClient)
		if p.groupsCache != nil {
			t.Fatal("expected nil groupsCache")
		}

		repoIDs, err := p.FetchUserPerms(context.Background(),
			mockAccount,
			authz.FetchPermsOptions{},
		)
		if err != nil {
			t.Fatal(err)
		}

		wantRepoIDs := []extsvc.RepoID{
			"MDEwOlJlcG9zaXRvcnkyNDI2NTEwMDA=",
			"MDEwOlJlcG9zaXRvcnkyNDQ1MTc1MzY=",
			"MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE=",
		}

		stableSortRepoID(wantRepoIDs)
		stableSortRepoID(repoIDs.Exacts)
		if diff := cmp.Diff(wantRepoIDs, repoIDs.Exacts); diff != "" {
			t.Fatalf("RepoIDs mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("cache enabled", func(t *testing.T) {
		t.Run("user has no orgs and teams", func(t *testing.T) {
			mockClient := newMockClientWithTokenMock()
			mockClient.ListAffiliatedRepositoriesFunc.SetDefaultHook(mockListAffiliatedRepositories)
			mockClient.GetAuthenticatedUserOrgsDetailsAndMembershipFunc.SetDefaultHook(
				func(ctx context.Context, page int) (orgs []github.OrgDetailsAndMembership, hasNextPage bool, rateLimitCost int, err error) {
					// No orgs
					return nil, false, 1, nil
				})
			mockClient.GetAuthenticatedUserTeamsFunc.SetDefaultHook(
				func(ctx context.Context, page int) (teams []*github.Team, hasNextPage bool, rateLimitCost int, err error) {
					// No teams
					return nil, false, 1, nil
				})
			// should call with token
			calledWithToken := false
			mockClient.WithAuthenticatorFunc.SetDefaultHook(
				func(_ auth.Authenticator) client {
					calledWithToken = true
					return mockClient
				})

			p, err := NewProvider("", ProviderOptions{GitHubURL: mustURL(t, "https://github.com"), DB: db})
			require.NoError(t, err)
			p.client = mockClientFunc(mockClient)
			if p.groupsCache == nil {
				t.Fatal("expected groupsCache")
			}
			p.groupsCache = memGroupsCache()

			repoIDs, err := p.FetchUserPerms(context.Background(), mockAccount, authz.FetchPermsOptions{})
			if err != nil {
				t.Fatal(err)
			}

			if !calledWithToken {
				t.Fatal("!calledWithToken")
			}

			wantRepoIDs := []extsvc.RepoID{
				"MDEwOlJlcG9zaXRvcnkyNDI2NTEwMDA=",
				"MDEwOlJlcG9zaXRvcnkyNDQ1MTc1MzY=",
				"MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE=",
			}
			stableSortRepoID(wantRepoIDs)
			stableSortRepoID(repoIDs.Exacts)
			if diff := cmp.Diff(wantRepoIDs, repoIDs.Exacts); diff != "" {
				t.Fatalf("RepoIDs mismatch (-want +got):\n%s", diff)
			}
		})

		t.Run("user in orgs", func(t *testing.T) {
			mockClient := newMockClientWithTokenMock()
			mockClient.ListAffiliatedRepositoriesFunc.SetDefaultHook(mockListAffiliatedRepositories)
			mockClient.GetAuthenticatedUserOrgsDetailsAndMembershipFunc.SetDefaultHook(mockListOrgDetails)
			mockClient.GetAuthenticatedUserTeamsFunc.SetDefaultHook(
				func(ctx context.Context, page int) (teams []*github.Team, hasNextPage bool, rateLimitCost int, err error) {
					// No teams
					return nil, false, 1, nil
				})
			mockClient.ListOrgRepositoriesFunc.SetDefaultHook(mockListOrgRepositories)

			p := setupProvider(t, mockClient)

			repoIDs, err := p.FetchUserPerms(context.Background(),
				mockAccount,
				authz.FetchPermsOptions{},
			)
			if err != nil {
				t.Fatal(err)
			}

			wantRepoIDs := []extsvc.RepoID{
				"MDEwOlJlcG9zaXRvcnkyNDI2NTE5678=",
				"MDEwOlJlcG9zaXRvcnkyNDI2NTEwMDA=",
				"MDEwOlJlcG9zaXRvcnkyNDI2NTadmin=",
				"MDEwOlJlcG9zaXRvcnkyNDQ1MTc1234=",
				"MDEwOlJlcG9zaXRvcnkyNDQ1MTc1MzY=",
				"MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE=",
			}
			stableSortRepoID(wantRepoIDs)
			stableSortRepoID(repoIDs.Exacts)
			if diff := cmp.Diff(wantRepoIDs, repoIDs.Exacts); diff != "" {
				t.Fatalf("RepoIDs mismatch (-want +got):\n%s", diff)
			}
		})

		t.Run("user in orgs and teams", func(t *testing.T) {
			mockClient := newMockClientWithTokenMock()
			mockClient.ListAffiliatedRepositoriesFunc.SetDefaultHook(mockListAffiliatedRepositories)
			mockClient.GetAuthenticatedUserOrgsDetailsAndMembershipFunc.SetDefaultHook(mockListOrgDetails)
			mockClient.GetAuthenticatedUserTeamsFunc.SetDefaultHook(
				func(_ context.Context, page int) (teams []*github.Team, hasNextPage bool, rateLimitCost int, err error) {
					switch page {
					case 1:
						return []*github.Team{
							// should not get repos from this team because parent org has default read permissions
							{Organization: &mockOrgRead.Org, Name: "ns team", Slug: "ns-team"},
							// should not get repos from this team since it has no repos
							{Organization: &mockOrgNoRead.Org, Name: "ns team", Slug: "ns-team", ReposCount: 0},
						}, true, 1, nil
					case 2:
						return []*github.Team{
							// should get repos from this team
							{Organization: &mockOrgNoRead.Org, Name: "ns team 2", Slug: "ns-team-2", ReposCount: 3},
						}, false, 1, nil
					}
					return nil, false, 1, nil
				})
			mockClient.ListOrgRepositoriesFunc.SetDefaultHook(mockListOrgRepositories)
			mockClient.ListTeamRepositoriesFunc.SetDefaultHook(
				func(_ context.Context, org, team string, page int) (repos []*github.Repository, hasNextPage bool, rateLimitCost int, err error) {
					switch org {
					case "not-sourcegraph":
						switch team {
						case "ns-team-2":
							switch page {
							case 1:
								return []*github.Repository{
									{ID: "MDEwOlJlcG9zaXRvcnkyNDI2NTEwMDA="}, // existing repo
									{ID: "MDEwOlJlcG9zaXRvcnkyNDQ1nsteam1="},
								}, true, 1, nil
							case 2:
								return []*github.Repository{
									{ID: "MDEwOlJlcG9zaXRvcnkyNDI2nsteam2="},
								}, false, 1, nil
							}
						}
					}
					t.Fatalf("unexpected call to ListTeamRepositories with org %q team %q page %d", org, team, page)
					return nil, false, 1, nil
				})

			p := setupProvider(t, mockClient)

			repoIDs, err := p.FetchUserPerms(context.Background(),
				mockAccount,
				authz.FetchPermsOptions{InvalidateCaches: true},
			)
			if err != nil {
				t.Fatal(err)
			}

			wantRepoIDs := []extsvc.RepoID{
				"MDEwOlJlcG9zaXRvcnkyNDI2NTE5678=",
				"MDEwOlJlcG9zaXRvcnkyNDI2NTEwMDA=",
				"MDEwOlJlcG9zaXRvcnkyNDI2NTadmin=",
				"MDEwOlJlcG9zaXRvcnkyNDI2nsteam2=",
				"MDEwOlJlcG9zaXRvcnkyNDQ1MTc1234=",
				"MDEwOlJlcG9zaXRvcnkyNDQ1MTc1MzY=",
				"MDEwOlJlcG9zaXRvcnkyNDQ1nsteam1=",
				"MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE=",
			}
			stableSortRepoID(wantRepoIDs)
			stableSortRepoID(repoIDs.Exacts)
			if diff := cmp.Diff(wantRepoIDs, repoIDs.Exacts); diff != "" {
				t.Fatalf("RepoIDs mismatch (-want +got):\n%s", diff)
			}
		})

		makeStatusCodeTest := func(code int) func(t *testing.T) {
			return func(t *testing.T) {
				mockClient := newMockClientWithTokenMock()
				mockClient.ListAffiliatedRepositoriesFunc.SetDefaultHook(mockListAffiliatedRepositories)
				mockClient.GetAuthenticatedUserOrgsDetailsAndMembershipFunc.SetDefaultHook(mockListOrgDetails)
				mockClient.GetAuthenticatedUserTeamsFunc.SetDefaultHook(
					func(_ context.Context, page int) (teams []*github.Team, hasNextPage bool, rateLimitCost int, err error) {
						switch page {
						case 1:
							return []*github.Team{
								// should not get repos from this team because parent org has default read permissions
								{Organization: &mockOrgRead.Org, Name: "ns team", Slug: "ns-team"},
								// should not get repos from this team since it has no repos
								{Organization: &mockOrgNoRead.Org, Name: "ns team", Slug: "ns-team", ReposCount: 0},
							}, true, 1, nil
						case 2:
							return []*github.Team{
								// should get repos from this team
								{Organization: &mockOrgNoRead.Org, Name: "ns team 2", Slug: "ns-team-2", ReposCount: 3},
							}, false, 1, nil
						}
						return nil, false, 1, nil
					})
				mockClient.ListOrgRepositoriesFunc.SetDefaultHook(mockListOrgRepositories)
				mockClient.ListTeamRepositoriesFunc.SetDefaultHook(
					func(_ context.Context, org, team string, page int) (repos []*github.Repository, hasNextPage bool, rateLimitCost int, err error) {
						return nil, false, 1, &github.APIError{Code: code}
					})

				p := setupProvider(t, mockClient)

				repoIDs, err := p.FetchUserPerms(context.Background(),
					mockAccount,
					authz.FetchPermsOptions{InvalidateCaches: true},
				)
				if err != nil {
					t.Fatal(err)
				}

				wantRepoIDs := []extsvc.RepoID{
					"MDEwOlJlcG9zaXRvcnkyNDI2NTE5678=", // from ListOrgRepositories
					"MDEwOlJlcG9zaXRvcnkyNDI2NTEwMDA=", // from ListAffiliatedRepos
					"MDEwOlJlcG9zaXRvcnkyNDI2NTadmin=", // from ListOrgRepositories
					"MDEwOlJlcG9zaXRvcnkyNDQ1MTc1234=", // from ListOrgRepositories
					"MDEwOlJlcG9zaXRvcnkyNDQ1MTc1MzY=", // from ListAffiliatedRepos
					"MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE=", // from ListAffiliatedRepos
				}
				stableSortRepoID(wantRepoIDs)
				stableSortRepoID(repoIDs.Exacts)
				if diff := cmp.Diff(wantRepoIDs, repoIDs.Exacts); diff != "" {
					t.Fatalf("RepoIDs mismatch (-want +got):\n%s", diff)
				}
				_, found := p.groupsCache.getGroup("not-sourcegraph", "ns-team-2")
				if !found {
					t.Error("expected to find group in cache")
				}
			}
		}

		t.Run("special case: ListTeamRepositories returns 404", makeStatusCodeTest(404))
		t.Run("special case: ListTeamRepositories returns 403", makeStatusCodeTest(403))

		t.Run("cache and invalidate: user in orgs and teams", func(t *testing.T) {
			callsToListOrgRepos := 0
			callsToListTeamRepos := 0
			mockClient := newMockClientWithTokenMock()
			mockClient.ListAffiliatedRepositoriesFunc.SetDefaultHook(mockListAffiliatedRepositories)
			mockClient.GetAuthenticatedUserOrgsDetailsAndMembershipFunc.SetDefaultHook(mockListOrgDetails)
			mockClient.GetAuthenticatedUserTeamsFunc.SetDefaultHook(
				func(ctx context.Context, page int) (teams []*github.Team, hasNextPage bool, rateLimitCost int, err error) {
					return []*github.Team{
						{Organization: &mockOrgNoRead.Org, Name: "ns team 2", Slug: "ns-team-2", ReposCount: 3},
					}, false, 1, nil
				})
			mockClient.ListOrgRepositoriesFunc.SetDefaultHook(
				func(ctx context.Context, org string, page int, repoType string) (repos []*github.Repository, hasNextPage bool, rateLimitCost int, err error) {
					callsToListOrgRepos++
					return mockListOrgRepositories(ctx, org, page, repoType)
				})
			mockClient.ListTeamRepositoriesFunc.SetDefaultHook(
				func(_ context.Context, _, _ string, _ int) (repos []*github.Repository, hasNextPage bool, rateLimitCost int, err error) {
					callsToListTeamRepos++
					return []*github.Repository{
						{ID: "MDEwOlJlcG9zaXRvcnkyNDI2nsteam1="},
					}, false, 1, nil
				})

			p, err := NewProvider("", ProviderOptions{GitHubURL: mustURL(t, "https://github.com"), DB: db})
			require.NoError(t, err)
			p.client = mockClientFunc(mockClient)
			memCache := memGroupsCache()
			p.groupsCache = memCache

			wantRepoIDs := []extsvc.RepoID{
				"MDEwOlJlcG9zaXRvcnkyNDI2NTE5678=",
				"MDEwOlJlcG9zaXRvcnkyNDI2NTEwMDA=",
				"MDEwOlJlcG9zaXRvcnkyNDI2NTadmin=",
				"MDEwOlJlcG9zaXRvcnkyNDI2nsteam1=",
				"MDEwOlJlcG9zaXRvcnkyNDQ1MTc1234=",
				"MDEwOlJlcG9zaXRvcnkyNDQ1MTc1MzY=",
				"MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE=",
			}

			stableSortRepoID(wantRepoIDs)
			// first call
			t.Run("first call", func(t *testing.T) {
				repoIDs, err := p.FetchUserPerms(context.Background(),
					mockAccount,
					authz.FetchPermsOptions{},
				)
				if err != nil {
					t.Fatal(err)
				}
				if callsToListOrgRepos == 0 || callsToListTeamRepos == 0 {
					t.Fatalf("expected repos to be listed: callsToListOrgRepos=%d, callsToListTeamRepos=%d",
						callsToListOrgRepos, callsToListTeamRepos)
				}
				stableSortRepoID(repoIDs.Exacts)
				if diff := cmp.Diff(wantRepoIDs, repoIDs.Exacts); diff != "" {
					t.Fatalf("RepoIDs mismatch (-want +got):\n%s", diff)
				}
			})

			// second call should use cache
			t.Run("second call", func(t *testing.T) {
				callsToListOrgRepos = 0
				callsToListTeamRepos = 0
				repoIDs, err := p.FetchUserPerms(context.Background(),
					mockAccount,
					authz.FetchPermsOptions{InvalidateCaches: false},
				)
				if err != nil {
					t.Fatal(err)
				}
				if callsToListOrgRepos > 0 || callsToListTeamRepos > 0 {
					t.Fatalf("expected repos not to be listed: callsToListOrgRepos=%d, callsToListTeamRepos=%d",
						callsToListOrgRepos, callsToListTeamRepos)
				}
				stableSortRepoID(repoIDs.Exacts)
				if diff := cmp.Diff(wantRepoIDs, repoIDs.Exacts); diff != "" {
					t.Fatalf("RepoIDs mismatch (-want +got):\n%s", diff)
				}
			})

			// third call should make a fresh query when invalidating cache
			t.Run("third call", func(t *testing.T) {
				callsToListOrgRepos = 0
				callsToListTeamRepos = 0
				repoIDs, err := p.FetchUserPerms(context.Background(),
					mockAccount,
					authz.FetchPermsOptions{InvalidateCaches: true},
				)
				if err != nil {
					t.Fatal(err)
				}
				if callsToListOrgRepos == 0 || callsToListTeamRepos == 0 {
					t.Fatalf("expected repos to be listed: callsToListOrgRepos=%d, callsToListTeamRepos=%d",
						callsToListOrgRepos, callsToListTeamRepos)
				}
				stableSortRepoID(repoIDs.Exacts)
				if diff := cmp.Diff(wantRepoIDs, repoIDs.Exacts); diff != "" {
					t.Fatalf("RepoIDs mismatch (-want +got):\n%s", diff)
				}
			})
		})

		t.Run("cache partial update", func(t *testing.T) {
			mockClient := newMockClientWithTokenMock()
			mockClient.ListAffiliatedRepositoriesFunc.SetDefaultHook(mockListAffiliatedRepositories)
			mockClient.GetAuthenticatedUserOrgsDetailsAndMembershipFunc.SetDefaultHook(mockListOrgDetails)
			mockClient.GetAuthenticatedUserTeamsFunc.SetDefaultHook(
				func(ctx context.Context, page int) (teams []*github.Team, hasNextPage bool, rateLimitCost int, err error) {
					return []*github.Team{
						{Organization: &mockOrgNoRead.Org, Name: "ns team 2", Slug: "ns-team-2", ReposCount: 3},
					}, false, 1, nil
				})
			mockClient.ListOrgRepositoriesFunc.SetDefaultHook(mockListOrgRepositories)
			mockClient.ListTeamRepositoriesFunc.SetDefaultHook(
				func(ctx context.Context, org, team string, page int) (repos []*github.Repository, hasNextPage bool, rateLimitCost int, err error) {
					return []*github.Repository{
						{ID: "MDEwOlJlcG9zaXRvcnkyNDI2nsteam1="},
					}, false, 1, nil
				})

			p, err := NewProvider("", ProviderOptions{
				GitHubURL: mustURL(t, "https://github.com"),
				DB:        db,
			})
			require.NoError(t, err)
			p.client = mockClientFunc(mockClient)
			memCache := memGroupsCache()
			p.groupsCache = memCache

			// cache populated from repo-centric sync (should add self)
			p.groupsCache.setGroup(cachedGroup{
				Org:          mockOrgRead.Login,
				Users:        []extsvc.AccountID{"1234"},
				Repositories: []extsvc.RepoID{},
			},
			)
			// cache populated from user-centric sync (should not add self)
			p.groupsCache.setGroup(cachedGroup{
				Org:          mockOrgNoRead.Login,
				Team:         "ns-team-2",
				Users:        []extsvc.AccountID{},
				Repositories: []extsvc.RepoID{"MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE="},
			},
			)

			// run a sync
			_, err = p.FetchUserPerms(context.Background(),
				mockAccount,
				authz.FetchPermsOptions{InvalidateCaches: false},
			)
			if err != nil {
				t.Fatal(err)
			}

			// mock user should have added self to complete cache
			group, found := p.groupsCache.getGroup(mockOrgRead.Login, "")
			if !found {
				t.Fatal("expected group")
			}
			if len(group.Users) != 2 {
				t.Fatal("expected an additional user in partial cache group")
			}

			// mock user should not have added self to incomplete cache
			group, found = p.groupsCache.getGroup(mockOrgNoRead.Login, "ns-team-2")
			if !found {
				t.Fatal("expected group")
			}
			if len(group.Users) != 0 {
				t.Fatal("expected users not to be updated")
			}
		})
	})
}

func TestProvider_FetchRepoPerms(t *testing.T) {
	t.Run("nil repository", func(t *testing.T) {
		p, err := NewProvider("", ProviderOptions{GitHubURL: mustURL(t, "https://github.com")})
		require.NoError(t, err)
		_, err = p.FetchRepoPerms(context.Background(), nil, authz.FetchPermsOptions{})
		want := "no repository provided"
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("not the code host of the repository", func(t *testing.T) {
		p, err := NewProvider("", ProviderOptions{GitHubURL: mustURL(t, "https://github.com")})
		require.NoError(t, err)
		_, err = p.FetchRepoPerms(context.Background(),
			&extsvc.Repository{
				URI: "gitlab.com/user/repo",
				ExternalRepoSpec: api.ExternalRepoSpec{
					ServiceType: "gitlab",
					ServiceID:   "https://gitlab.com/",
				},
			},
			authz.FetchPermsOptions{},
		)
		want := `not a code host of the repository: want "https://gitlab.com/" but have "https://github.com/"`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	var (
		mockUserRepo = extsvc.Repository{
			URI: "github.com/user/user-repo",
			ExternalRepoSpec: api.ExternalRepoSpec{
				ID:          "github_project_id",
				ServiceType: "github",
				ServiceID:   "https://github.com/",
			},
		}

		mockOrgRepo = extsvc.Repository{
			URI: "github.com/org/org-repo",
			ExternalRepoSpec: api.ExternalRepoSpec{
				ID:          "github_project_id",
				ServiceType: "github",
				ServiceID:   "https://github.com/",
			},
		}

		mockListCollaborators = func(_ context.Context, _, _ string, page int, _ github.CollaboratorAffiliation) ([]*github.Collaborator, bool, error) {
			switch page {
			case 1:
				return []*github.Collaborator{
					{DatabaseID: 57463526},
					{DatabaseID: 67471},
				}, true, nil
			case 2:
				return []*github.Collaborator{
					{DatabaseID: 187831},
				}, false, nil
			}

			return []*github.Collaborator{}, false, nil
		}
	)

	t.Run("cache disabled", func(t *testing.T) {
		p, err := NewProvider("", ProviderOptions{
			GitHubURL:      mustURL(t, "https://github.com"),
			GroupsCacheTTL: -1,
		})
		require.NoError(t, err)
		mockClient := newMockClientWithTokenMock()
		mockClient.ListRepositoryCollaboratorsFunc.SetDefaultHook(
			func(ctx context.Context, owner, repo string, page int, affiliation github.CollaboratorAffiliation) (users []*github.Collaborator, hasNextPage bool, _ error) {
				if affiliation != "" {
					t.Fatal("unexpected affiliation filter provided")
				}
				return mockListCollaborators(ctx, owner, repo, page, affiliation)
			})
		p.client = mockClientFunc(mockClient)

		accountIDs, err := p.FetchRepoPerms(context.Background(), &mockUserRepo,
			authz.FetchPermsOptions{})
		if err != nil {
			t.Fatal(err)
		}

		wantAccountIDs := []extsvc.AccountID{
			// mockListCollaborators members
			"57463526",
			"67471",
			"187831",
		}
		if diff := cmp.Diff(wantAccountIDs, accountIDs); diff != "" {
			t.Fatalf("AccountIDs mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("cache enabled", func(t *testing.T) {
		t.Run("repo not in org", func(t *testing.T) {
			p, err := NewProvider("", ProviderOptions{
				GitHubURL: mustURL(t, "https://github.com"),
			})
			require.NoError(t, err)
			mockClient := newMockClientWithTokenMock()
			mockClient.ListRepositoryCollaboratorsFunc.SetDefaultHook(
				func(ctx context.Context, owner, repo string, page int, affiliation github.CollaboratorAffiliation) (users []*github.Collaborator, hasNextPage bool, _ error) {
					if affiliation == "" {
						t.Fatal("expected affiliation filter")
					}
					return mockListCollaborators(ctx, owner, repo, page, affiliation)
				})
			mockClient.GetOrganizationFunc.SetDefaultHook(
				func(_ context.Context, login string) (org *github.OrgDetails, err error) {
					if login == "user" {
						return nil, &github.OrgNotFoundError{}
					}
					t.Fatalf("unexpected call to GetOrganization with %q", login)
					return nil, nil
				})
			p.client = mockClientFunc(mockClient)
			if p.groupsCache == nil {
				t.Fatal("expected groupsCache")
			}
			memCache := memGroupsCache()
			p.groupsCache = memCache

			accountIDs, err := p.FetchRepoPerms(context.Background(), &mockUserRepo,
				authz.FetchPermsOptions{})
			if err != nil {
				t.Fatal(err)
			}

			wantAccountIDs := []extsvc.AccountID{
				// mockListCollaborators members
				"57463526",
				"67471",
				"187831",
			}
			if diff := cmp.Diff(wantAccountIDs, accountIDs); diff != "" {
				t.Fatalf("AccountIDs mismatch (-want +got):\n%s", diff)
			}
		})

		t.Run("repo in read org", func(t *testing.T) {
			p, err := NewProvider("", ProviderOptions{
				GitHubURL: mustURL(t, "https://github.com"),
			})
			require.NoError(t, err)
			mockClient := newMockClientWithTokenMock()
			mockClient.ListRepositoryCollaboratorsFunc.SetDefaultHook(
				func(ctx context.Context, owner, repo string, page int, affiliation github.CollaboratorAffiliation) (users []*github.Collaborator, hasNextPage bool, _ error) {
					if affiliation == "" {
						t.Fatal("expected affiliation filter")
					}
					return mockListCollaborators(ctx, owner, repo, page, affiliation)
				})
			mockClient.GetOrganizationFunc.SetDefaultHook(
				func(_ context.Context, login string) (org *github.OrgDetails, err error) {
					if login == "org" {
						return &github.OrgDetails{
							DefaultRepositoryPermission: "read",
						}, nil
					}
					t.Fatalf("unexpected call to GetOrganization with %q", login)
					return nil, nil
				})
			mockClient.ListOrganizationMembersFunc.SetDefaultHook(
				func(_ context.Context, _ string, page int, adminOnly bool) (users []*github.Collaborator, hasNextPage bool, _ error) {
					if adminOnly {
						t.Fatal("unexpected adminOnly ListOrganizationMembers")
					}
					switch page {
					case 1:
						return []*github.Collaborator{
							{DatabaseID: 1234},
							{DatabaseID: 67471}, // duplicate from collaborators
						}, true, nil
					case 2:
						return []*github.Collaborator{
							{DatabaseID: 5678},
						}, false, nil
					}

					return []*github.Collaborator{}, false, nil
				})
			p.client = mockClientFunc(mockClient)
			memCache := memGroupsCache()
			p.groupsCache = memCache

			accountIDs, err := p.FetchRepoPerms(context.Background(), &mockOrgRepo,
				authz.FetchPermsOptions{})
			if err != nil {
				t.Fatal(err)
			}

			wantAccountIDs := []extsvc.AccountID{
				// mockListCollaborators members
				"57463526",
				"67471",
				"187831",
				// dedpulicated MockListOrganizationMembers users
				"1234",
				"5678",
			}
			if diff := cmp.Diff(wantAccountIDs, accountIDs); diff != "" {
				t.Fatalf("AccountIDs mismatch (-want +got):\n%s", diff)
			}
		})

		t.Run("internal repo in org", func(t *testing.T) {
			mockInternalOrgRepo := github.Repository{
				ID:         "github_repo_id",
				IsPrivate:  true,
				Visibility: github.VisibilityInternal,
			}

			p, err := NewProvider("", ProviderOptions{
				GitHubURL: mustURL(t, "https://github.com"),
			})
			require.NoError(t, err)

			mockClient := newMockClientWithTokenMock()
			mockClient.ListRepositoryCollaboratorsFunc.SetDefaultHook(mockListCollaborators)
			mockClient.ListOrganizationMembersFunc.SetDefaultHook(
				func(_ context.Context, _ string, page int, adminOnly bool) (users []*github.Collaborator, hasNextPage bool, _ error) {
					if adminOnly {
						return []*github.Collaborator{
							{DatabaseID: 9999},
						}, false, nil
					}

					switch page {
					case 1:
						return []*github.Collaborator{
							{DatabaseID: 1234},
							{DatabaseID: 67471}, // duplicate from collaborators
						}, true, nil
					case 2:
						return []*github.Collaborator{
							{DatabaseID: 5678},
						}, false, nil
					}

					return []*github.Collaborator{}, false, nil
				})
			mockClient.ListRepositoryTeamsFunc.SetDefaultHook(
				func(ctx context.Context, owner, repo string, page int) (teams []*github.Team, hasNextPage bool, _ error) {
					// No team has exlicit access to mockInternalOrgRepo. It's an internal repo so everyone in the org should have access to it.
					return []*github.Team{}, false, nil
				})
			mockClient.GetRepositoryFunc.SetDefaultHook(
				func(ctx context.Context, owner, repo string) (*github.Repository, error) {
					return &mockInternalOrgRepo, nil
				})
			mockClient.GetOrganizationFunc.SetDefaultHook(
				func(_ context.Context, login string) (org *github.OrgDetails, err error) {
					if login == "org" {
						return &github.OrgDetails{
							DefaultRepositoryPermission: "none",
						}, nil
					}

					t.Fatalf("unexpected call to GetOrganization with %q", login)
					return nil, nil
				})

			p.client = mockClientFunc(mockClient)
			// Ideally don't want a feature flag for this and want this internal repos to sync for
			// all users inside an org. Since we're introducing a new feature this is guarded behind
			// a feature flag, thus we also test against it. Once we're reasonably sure this works
			// as intended, we will remove the feature flag and enable the behaviour by default.
			t.Run("feature flag disabled", func(t *testing.T) {
				p.enableGithubInternalRepoVisibility = false

				memCache := memGroupsCache()
				p.groupsCache = memCache

				accountIDs, err := p.FetchRepoPerms(
					context.Background(), &mockOrgRepo, authz.FetchPermsOptions{},
				)
				if err != nil {
					t.Fatal(err)
				}

				// These account IDs will have access to the internal repo.
				wantAccountIDs := []extsvc.AccountID{
					// expect mockListCollaborators members only - we do not want to include org members
					// if internal repository support is not enabled.
					"57463526",
					"67471",
					"187831",
					// The admin is expected to be in this list.
					"9999",
				}
				if diff := cmp.Diff(wantAccountIDs, accountIDs); diff != "" {
					t.Fatalf("AccountIDs mismatch (-want +got):\n%s", diff)
				}
			})

			t.Run("feature flag enabled", func(t *testing.T) {
				p.enableGithubInternalRepoVisibility = true
				memCache := memGroupsCache()
				p.groupsCache = memCache

				accountIDs, err := p.FetchRepoPerms(
					context.Background(), &mockOrgRepo, authz.FetchPermsOptions{},
				)
				if err != nil {
					t.Fatal(err)
				}

				// These account IDs will have access to the internal repo.
				wantAccountIDs := []extsvc.AccountID{
					// mockListCollaborators members.
					"57463526",
					"67471",
					"187831",
					// expect dedpulicated MockListOrganizationMembers users as well since we want to grant access
					// to org members as well if the target repo has visibility "internal"
					"1234",
					"5678",
				}
				if diff := cmp.Diff(wantAccountIDs, accountIDs); diff != "" {
					t.Fatalf("AccountIDs mismatch (-want +got):\n%s", diff)
				}
			})
		})

		t.Run("repo in non-read org but in teams", func(t *testing.T) {
			p, err := NewProvider("", ProviderOptions{
				GitHubURL: mustURL(t, "https://github.com"),
			})
			require.NoError(t, err)
			mockClient := newMockClientWithTokenMock()
			mockClient.ListRepositoryCollaboratorsFunc.SetDefaultHook(
				func(ctx context.Context, owner, repo string, page int, affiliation github.CollaboratorAffiliation) (users []*github.Collaborator, hasNextPage bool, _ error) {
					if affiliation == "" {
						t.Fatal("expected affiliation filter")
					}
					return mockListCollaborators(ctx, owner, repo, page, affiliation)
				})
			mockClient.GetOrganizationFunc.SetDefaultHook(
				func(_ context.Context, login string) (org *github.OrgDetails, err error) {
					if login == "org" {
						return &github.OrgDetails{
							DefaultRepositoryPermission: "none",
						}, nil
					}
					t.Fatalf("unexpected call to GetOrganization with %q", login)
					return nil, nil
				})
			mockClient.ListOrganizationMembersFunc.SetDefaultHook(
				func(_ context.Context, org string, _ int, adminOnly bool) (users []*github.Collaborator, hasNextPage bool, _ error) {
					if org != "org" {
						t.Fatalf("unexpected call to list org members with %q", org)
					}
					if !adminOnly {
						t.Fatal("expected adminOnly ListOrganizationMembers")
					}
					return []*github.Collaborator{
						{DatabaseID: 3456},
					}, false, nil
				})
			mockClient.ListRepositoryTeamsFunc.SetDefaultHook(func(_ context.Context, _, _ string, page int) (teams []*github.Team, hasNextPage bool, _ error) {
				switch page {
				case 1:
					return []*github.Team{
						{Slug: "team1"},
					}, true, nil
				case 2:
					return []*github.Team{
						{Slug: "team2"},
					}, false, nil
				}

				return []*github.Team{}, false, nil
			})
			mockClient.ListTeamMembersFunc.SetDefaultHook(func(_ context.Context, _, team string, page int) (users []*github.Collaborator, hasNextPage bool, _ error) {
				switch page {
				case 1:
					return []*github.Collaborator{
						{DatabaseID: 1234}, // duplicate across both teams
					}, true, nil
				case 2:
					switch team {
					case "team1":
						return []*github.Collaborator{
							{DatabaseID: 5678},
						}, false, nil
					case "team2":
						return []*github.Collaborator{
							{DatabaseID: 6789},
						}, false, nil
					}
				}

				return []*github.Collaborator{}, false, nil
			})
			p.client = mockClientFunc(mockClient)
			memCache := memGroupsCache()
			p.groupsCache = memCache

			accountIDs, err := p.FetchRepoPerms(context.Background(), &mockOrgRepo,
				authz.FetchPermsOptions{})
			if err != nil {
				t.Fatal(err)
			}

			wantAccountIDs := []extsvc.AccountID{
				// mockListCollaborators members
				"57463526",
				"67471",
				"187831",
				// MockListOrganizationMembers users
				"3456",
				// deduplicated MockListTeamMembers users
				"1234",
				"5678",
				"6789",
			}
			if diff := cmp.Diff(wantAccountIDs, accountIDs); diff != "" {
				t.Fatalf("AccountIDs mismatch (-want +got):\n%s", diff)
			}
		})

		t.Run("cache and invalidate", func(t *testing.T) {
			p, err := NewProvider("", ProviderOptions{
				GitHubURL: mustURL(t, "https://github.com"),
			})
			require.NoError(t, err)
			callsToListOrgMembers := 0
			mockClient := newMockClientWithTokenMock()
			mockClient.ListRepositoryCollaboratorsFunc.SetDefaultHook(
				func(ctx context.Context, owner, repo string, page int, affiliation github.CollaboratorAffiliation) (users []*github.Collaborator, hasNextPage bool, _ error) {
					if affiliation == "" {
						t.Fatal("expected affiliation filter")
					}
					return mockListCollaborators(ctx, owner, repo, page, affiliation)
				})
			mockClient.GetOrganizationFunc.SetDefaultHook(
				func(_ context.Context, login string) (org *github.OrgDetails, err error) {
					if login == "org" {
						return &github.OrgDetails{
							DefaultRepositoryPermission: "read",
						}, nil
					}
					t.Fatalf("unexpected call to GetOrganization with %q", login)
					return nil, nil
				})
			mockClient.ListOrganizationMembersFunc.SetDefaultHook(
				func(_ context.Context, _ string, page int, _ bool) (users []*github.Collaborator, hasNextPage bool, _ error) {
					callsToListOrgMembers++

					switch page {
					case 1:
						return []*github.Collaborator{
							{DatabaseID: 1234},
						}, true, nil
					case 2:
						return []*github.Collaborator{
							{DatabaseID: 5678},
						}, false, nil
					}

					return []*github.Collaborator{}, false, nil
				})
			p.client = mockClientFunc(mockClient)
			memCache := memGroupsCache()
			p.groupsCache = memCache

			wantAccountIDs := []extsvc.AccountID{
				// mockListCollaborators members
				"57463526",
				"67471",
				"187831",
				// MockListOrganizationMembers users
				"1234",
				"5678",
			}

			// first call
			t.Run("first call", func(t *testing.T) {
				accountIDs, err := p.FetchRepoPerms(context.Background(), &mockOrgRepo,
					authz.FetchPermsOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if callsToListOrgMembers == 0 {
					t.Fatalf("expected members to be listed: callsToListOrgMembers=%d",
						callsToListOrgMembers)
				}
				if diff := cmp.Diff(wantAccountIDs, accountIDs); diff != "" {
					t.Fatalf("AccountIDs mismatch (-want +got):\n%s", diff)
				}
			})

			// second call should use cache
			t.Run("second call", func(t *testing.T) {
				callsToListOrgMembers = 0
				accountIDs, err := p.FetchRepoPerms(context.Background(), &mockOrgRepo,
					authz.FetchPermsOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if callsToListOrgMembers > 0 {
					t.Fatalf("expected members not to be listed: callsToListOrgMembers=%d",
						callsToListOrgMembers)
				}
				if diff := cmp.Diff(wantAccountIDs, accountIDs); diff != "" {
					t.Fatalf("AccountIDs mismatch (-want +got):\n%s", diff)
				}
			})

			// third call should make a fresh query when invalidating cache
			t.Run("third call", func(t *testing.T) {
				callsToListOrgMembers = 0
				accountIDs, err := p.FetchRepoPerms(context.Background(), &mockOrgRepo,
					authz.FetchPermsOptions{InvalidateCaches: true})
				if err != nil {
					t.Fatal(err)
				}
				if callsToListOrgMembers == 0 {
					t.Fatalf("expected members to be listed: callsToListOrgMembers=%d",
						callsToListOrgMembers)
				}
				if diff := cmp.Diff(wantAccountIDs, accountIDs); diff != "" {
					t.Fatalf("AccountIDs mismatch (-want +got):\n%s", diff)
				}
			})
		})

		t.Run("cache partial update", func(t *testing.T) {
			p, err := NewProvider("", ProviderOptions{
				GitHubURL: mustURL(t, "https://github.com"),
			})
			require.NoError(t, err)
			mockClient := newMockClientWithTokenMock()
			mockClient.ListRepositoryCollaboratorsFunc.SetDefaultHook(
				mockListCollaborators)
			mockClient.GetOrganizationFunc.SetDefaultHook(
				func(ctx context.Context, login string) (org *github.OrgDetails, err error) {
					// use teams
					return &github.OrgDetails{DefaultRepositoryPermission: "none"}, nil
				})
			mockClient.ListOrganizationMembersFunc.SetDefaultHook(
				func(ctx context.Context, owner string, page int, adminOnly bool) (users []*github.Collaborator, hasNextPage bool, _ error) {
					return []*github.Collaborator{}, false, nil
				})
			mockClient.ListRepositoryTeamsFunc.SetDefaultHook(
				func(ctx context.Context, owner, repo string, page int) (teams []*github.Team, hasNextPage bool, _ error) {
					return []*github.Team{
						{Slug: "team1"},
						{Slug: "team2"},
					}, false, nil
				})
			mockClient.ListTeamMembersFunc.SetDefaultHook(
				func(_ context.Context, _, team string, _ int) (users []*github.Collaborator, hasNextPage bool, _ error) {
					switch team {
					case "team1":
						return []*github.Collaborator{
							{DatabaseID: 5678},
						}, false, nil
					case "team2":
						return []*github.Collaborator{
							{DatabaseID: 6789},
						}, false, nil
					}
					return []*github.Collaborator{}, false, nil
				})
			p.client = mockClientFunc(mockClient)
			memCache := memGroupsCache()
			p.groupsCache = memCache

			// cache populated from user-centric sync (should add self)
			p.groupsCache.setGroup(cachedGroup{
				Org:          "org",
				Team:         "team1",
				Users:        []extsvc.AccountID{},
				Repositories: []extsvc.RepoID{"MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE="},
			},
			)
			// cache populated from repo-centric sync (should not add self)
			p.groupsCache.setGroup(cachedGroup{
				Org:          "org",
				Team:         "team2",
				Users:        []extsvc.AccountID{"1234"},
				Repositories: []extsvc.RepoID{},
			},
			)

			// run a sync
			_, err = p.FetchRepoPerms(context.Background(),
				&mockOrgRepo,
				authz.FetchPermsOptions{InvalidateCaches: false},
			)
			if err != nil {
				t.Fatal(err)
			}

			// mock user should have added self to complete cache
			group, found := p.groupsCache.getGroup("org", "team1")
			if !found {
				t.Fatal("expected group")
			}
			if len(group.Repositories) != 2 {
				t.Fatal("expected an additional repo in partial cache group")
			}

			// mock user should not have added self to incomplete cache
			group, found = p.groupsCache.getGroup("org", "team2")
			if !found {
				t.Fatal("expected group")
			}
			if len(group.Repositories) != 0 {
				t.Fatal("expected repos not to be updated")
			}
		})
	})
}

func TestProvider_ValidateConnection(t *testing.T) {
	t.Run("cache disabled: scopes ok", func(t *testing.T) {
		p, err := NewProvider("", ProviderOptions{
			GitHubURL:      mustURL(t, "https://github.com"),
			GroupsCacheTTL: -1,
		})
		require.NoError(t, err)
		err = p.ValidateConnection(context.Background())
		if err != nil {
			t.Fatal("expected validate to pass")
		}
	})

	t.Run("cache enabled", func(t *testing.T) {
		p, err := NewProvider("", ProviderOptions{
			GitHubURL:      mustURL(t, "https://github.com"),
			GroupsCacheTTL: 72,
		})
		require.NoError(t, err)

		t.Run("error getting scopes", func(t *testing.T) {
			mockClient := newMockClientWithTokenMock()
			mockClient.GetAuthenticatedOAuthScopesFunc.SetDefaultHook(
				func(ctx context.Context) ([]string, error) {
					return nil, errors.New("scopes error")
				})
			p.client = mockClientFunc(mockClient)
			err := p.ValidateConnection(context.Background())
			if err == nil {
				t.Fatal("expected 1 problem")
			}
			if !strings.Contains(err.Error(), "scopes error") {
				t.Fatalf("unexpected problem: %q", err.Error())
			}
		})

		t.Run("missing org scope", func(t *testing.T) {
			mockClient := newMockClientWithTokenMock()
			mockClient.GetAuthenticatedOAuthScopesFunc.SetDefaultHook(
				func(ctx context.Context) ([]string, error) {
					return []string{}, nil
				})
			p.client = mockClientFunc(mockClient)
			err := p.ValidateConnection(context.Background())
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "read:org") {
				t.Fatalf("unexpected problem: %q", err.Error())
			}
		})

		t.Run("scopes ok org scope", func(t *testing.T) {
			for _, testCase := range [][]string{
				{"read:org"},
				{"write:org"},
				{"admin:org"},
			} {
				mockClient := newMockClientWithTokenMock()
				mockClient.GetAuthenticatedOAuthScopesFunc.SetDefaultHook(
					func(ctx context.Context) ([]string, error) {
						return testCase, nil
					})
				p.client = mockClientFunc(mockClient)
				err := p.ValidateConnection(context.Background())
				if err != nil {
					t.Fatalf("expected validate to pass for scopes=%+v", testCase)
				}
			}
		})
	})
}

func setupProvider(t *testing.T, mc *MockClient) *Provider {
	db := dbmocks.NewMockDB()
	p, err := NewProvider("", ProviderOptions{GitHubURL: mustURL(t, "https://github.com"), DB: db})
	require.NoError(t, err)
	p.client = mockClientFunc(mc)
	p.groupsCache = memGroupsCache()
	return p
}
