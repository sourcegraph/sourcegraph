package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func Test_GitLab_FetchAccount(t *testing.T) {
	// Test structures
	type call struct {
		description string

		user    *types.User
		current []*extsvc.Account

		expMine *extsvc.Account
	}
	type test struct {
		description string

		// authnProviders is the list of auth providers that are mocked
		authnProviders []providers.Provider

		// op configures the SudoProvider instance
		op SudoProviderOp

		calls []call
	}

	// Mocks
	gitlabMock := newMockGitLab(mockGitLabOp{
		t: t,
		users: []*gitlab.AuthUser{
			{
				ID:       101,
				Username: "b.l",
				Identities: []gitlab.Identity{
					{Provider: "okta.mine", ExternUID: "bl"},
					{Provider: "onelogin.mine", ExternUID: "bl"},
				},
			},
			{
				ID:         102,
				Username:   "k.l",
				Identities: []gitlab.Identity{{Provider: "okta.mine", ExternUID: "kl"}},
			},
			{
				ID:         199,
				Username:   "user-without-extern-id",
				Identities: nil,
			},
		},
	})
	gitlab.MockListUsers = gitlabMock.ListUsers

	// Test cases
	tests := []test{
		{
			description: "1 authn provider, basic authz provider",
			authnProviders: []providers.Provider{
				mockAuthnProvider{
					configID:  providers.ConfigID{ID: "okta.mine", Type: "saml"},
					serviceID: "https://okta.mine/",
				},
			},
			op: SudoProviderOp{
				BaseURL:           mustURL(t, "https://gitlab.mine"),
				AuthnConfigID:     providers.ConfigID{ID: "okta.mine", Type: "saml"},
				GitLabProvider:    "okta.mine",
				UseNativeUsername: false,
			},
			calls: []call{
				{
					description: "1 account, matches",
					user:        &types.User{ID: 123},
					current:     []*extsvc.Account{acct(t, 1, "saml", "https://okta.mine/", "bl")},
					expMine:     acct(t, 123, extsvc.TypeGitLab, "https://gitlab.mine/", "101"),
				},
				{
					description: "many accounts, none match",
					user:        &types.User{ID: 123},
					current: []*extsvc.Account{
						acct(t, 1, "saml", "https://okta.mine/", "nomatch"),
						acct(t, 1, "saml", "nomatch", "bl"),
						acct(t, 1, "nomatch", "https://okta.mine/", "bl"),
					},
					expMine: nil,
				},
				{
					description: "many accounts, 1 match",
					user:        &types.User{ID: 123},
					current: []*extsvc.Account{
						acct(t, 1, "saml", "nomatch", "bl"),
						acct(t, 1, "nomatch", "https://okta.mine/", "bl"),
						acct(t, 1, "saml", "https://okta.mine/", "bl"),
					},
					expMine: acct(t, 123, extsvc.TypeGitLab, "https://gitlab.mine/", "101"),
				},
				{
					description: "no user",
					user:        nil,
					current:     nil,
					expMine:     nil,
				},
			},
		},
		{
			description:    "0 authn providers, native username",
			authnProviders: nil,
			op: SudoProviderOp{
				BaseURL:           mustURL(t, "https://gitlab.mine"),
				UseNativeUsername: true,
			},
			calls: []call{
				{
					description: "username match",
					user:        &types.User{ID: 123, Username: "b.l"},
					expMine:     acct(t, 123, extsvc.TypeGitLab, "https://gitlab.mine/", "101"),
				},
				{
					description: "no username match",
					user:        &types.User{ID: 123, Username: "nomatch"},
					expMine:     nil,
				},
			},
		},
		{
			description:    "0 authn providers, basic authz provider",
			authnProviders: nil,
			op: SudoProviderOp{
				BaseURL:           mustURL(t, "https://gitlab.mine"),
				AuthnConfigID:     providers.ConfigID{ID: "okta.mine", Type: "saml"},
				GitLabProvider:    "okta.mine",
				UseNativeUsername: false,
			},
			calls: []call{
				{
					description: "no matches",
					user:        &types.User{ID: 123, Username: "b.l"},
					expMine:     nil,
				},
			},
		},
		{
			description: "2 authn providers, basic authz provider",
			authnProviders: []providers.Provider{
				mockAuthnProvider{
					configID:  providers.ConfigID{ID: "okta.mine", Type: "saml"},
					serviceID: "https://okta.mine/",
				},
				mockAuthnProvider{
					configID:  providers.ConfigID{ID: "onelogin.mine", Type: "openidconnect"},
					serviceID: "https://onelogin.mine/",
				},
			},
			op: SudoProviderOp{
				BaseURL:           mustURL(t, "https://gitlab.mine"),
				AuthnConfigID:     providers.ConfigID{ID: "onelogin.mine", Type: "openidconnect"},
				GitLabProvider:    "onelogin.mine",
				UseNativeUsername: false,
			},
			calls: []call{
				{
					description: "1 authn provider matches",
					user:        &types.User{ID: 123},
					current:     []*extsvc.Account{acct(t, 1, "openidconnect", "https://onelogin.mine/", "bl")},
					expMine:     acct(t, 123, extsvc.TypeGitLab, "https://gitlab.mine/", "101"),
				},
				{
					description: "0 authn providers match",
					user:        &types.User{ID: 123},
					current:     []*extsvc.Account{acct(t, 1, "openidconnect", "https://onelogin.mine/", "nomatch")},
					expMine:     nil,
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.description, func(t *testing.T) {
			providers.MockProviders = test.authnProviders
			defer func() { providers.MockProviders = nil }()

			ctx := context.Background()
			authzProvider, err := newSudoProvider(test.op, nil)
			require.NoError(t, err)
			for _, c := range test.calls {
				t.Run(c.description, func(t *testing.T) {
					acct, err := authzProvider.FetchAccount(ctx, c.user, c.current, nil)
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					// ignore Data field in comparison
					if acct != nil {
						acct.Data, c.expMine.Data = nil, nil
					}

					if !reflect.DeepEqual(acct, c.expMine) {
						dmp := diffmatchpatch.New()
						t.Errorf("wantUser != user\n%s",
							dmp.DiffPrettyText(dmp.DiffMain(spew.Sdump(c.expMine), spew.Sdump(acct), false)))
					}
				})
			}
		})
	}
}

func TestSudoProvider_FetchUserPerms(t *testing.T) {
	t.Run("nil account", func(t *testing.T) {
		p, err := newSudoProvider(SudoProviderOp{
			BaseURL: mustURL(t, "https://gitlab.com"),
		}, nil)
		require.NoError(t, err)
		_, err = p.FetchUserPerms(context.Background(), nil, authz.FetchPermsOptions{})
		want := "no account provided"
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("not the code host of the account", func(t *testing.T) {
		p, err := newSudoProvider(SudoProviderOp{
			BaseURL: mustURL(t, "https://gitlab.com"),
		}, nil)
		require.NoError(t, err)

		_, err = p.FetchUserPerms(context.Background(),
			&extsvc.Account{
				AccountSpec: extsvc.AccountSpec{
					ServiceType: extsvc.TypeGitHub,
					ServiceID:   "https://github.com/",
				},
			},
			authz.FetchPermsOptions{},
		)
		want := `not a code host of the account: want "https://github.com/" but have "https://gitlab.com/"`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("feature flag disabled", func(t *testing.T) {
		// The OAuthProvider uses the gitlab.Client under the hood,
		// which uses rcache, a caching layer that uses Redis.
		// We need to clear the cache before we run the tests
		rcache.SetupForTest(t)

		p, err := newSudoProvider(
			SudoProviderOp{
				BaseURL:                     mustURL(t, "https://gitlab.com"),
				SudoToken:                   "admin_token",
				SyncInternalRepoPermissions: true,
			},
			httpcli.NewFactory(nil, func(c *http.Client) error {
				c.Transport = httpcli.WrapTransport(&mockTransport{
					do: func(r *http.Request) (*http.Response, error) {
						visibility := r.URL.Query().Get("visibility")
						if visibility != "private" && visibility != "internal" {
							return nil, errors.Errorf("URL visibility: want private or internal, got %s", visibility)
						}
						want := fmt.Sprintf("https://gitlab.com/api/v4/projects?min_access_level=20&per_page=100&visibility=%s", visibility)
						if r.URL.String() != want {
							return nil, errors.Errorf("URL: want %q but got %q", want, r.URL)
						}

						want = "admin_token"
						got := r.Header.Get("Private-Token")
						if got != want {
							return nil, errors.Errorf("HTTP Private-Token: want %q but got %q", want, got)
						}

						want = "999"
						got = r.Header.Get("Sudo")
						if got != want {
							return nil, errors.Errorf("HTTP Sudo: want %q but got %q", want, got)
						}

						body := `[{"id": 1, "default_branch": "main"}, {"id": 2, "default_branch": "main"}]`
						if visibility == "internal" {
							body = `[{"id": 3, "default_branch": "main"}, {"id": 4}]`
						}
						return &http.Response{
							Status:     http.StatusText(http.StatusOK),
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(bytes.NewReader([]byte(body))),
						}, nil
					},
				}, http.DefaultTransport)
				return nil
			}),
		)
		require.NoError(t, err)

		accountData := json.RawMessage(`{"id": 999}`)
		repoIDs, err := p.FetchUserPerms(context.Background(),
			&extsvc.Account{
				AccountSpec: extsvc.AccountSpec{
					ServiceType: "gitlab",
					ServiceID:   "https://gitlab.com/",
				},
				AccountData: extsvc.AccountData{
					Data: extsvc.NewUnencryptedData(accountData),
				},
			},
			authz.FetchPermsOptions{},
		)
		if err != nil {
			t.Fatal(err)
		}

		expRepoIDs := []extsvc.RepoID{"1", "2", "3", "4"}
		if diff := cmp.Diff(expRepoIDs, repoIDs.Exacts); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("feature flag enabled", func(t *testing.T) {
		// The OAuthProvider uses the gitlab.Client under the hood,
		// which uses rcache, a caching layer that uses Redis.
		// We need to clear the cache before we run the tests
		rcache.SetupForTest(t)
		ctx := context.Background()
		flags := map[string]bool{"gitLabProjectVisibilityExperimental": true}
		ctx = featureflag.WithFlags(ctx, featureflag.NewMemoryStore(flags, flags, flags))

		p, err := newSudoProvider(
			SudoProviderOp{
				BaseURL:   mustURL(t, "https://gitlab.com"),
				SudoToken: "admin_token",
			},
			httpcli.NewFactory(nil, func(c *http.Client) error {
				c.Transport = httpcli.WrapTransport(&mockTransport{
					do: func(r *http.Request) (*http.Response, error) {
						visibility := r.URL.Query().Get("visibility")
						if visibility != "private" && visibility != "internal" {
							return nil, errors.Errorf("URL visibility: want private or internal, got %s", visibility)
						}
						want := fmt.Sprintf("https://gitlab.com/api/v4/projects?per_page=100&visibility=%s", visibility)
						if r.URL.String() != want {
							return nil, errors.Errorf("URL: want %q but got %q", want, r.URL)
						}

						want = "admin_token"
						got := r.Header.Get("Private-Token")
						if got != want {
							return nil, errors.Errorf("HTTP Private-Token: want %q but got %q", want, got)
						}

						want = "999"
						got = r.Header.Get("Sudo")
						if got != want {
							return nil, errors.Errorf("HTTP Sudo: want %q but got %q", want, got)
						}

						body := `[{"id": 1, "default_branch": "main"}, {"id": 2, "default_branch": "main"}]`
						if visibility == "internal" {
							body = `[{"id": 3, "default_branch": "main"}, {"id": 4}]`
						}
						return &http.Response{
							Status:     http.StatusText(http.StatusOK),
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(bytes.NewReader([]byte(body))),
						}, nil
					},
				}, http.DefaultTransport)
				return nil
			}),
		)
		require.NoError(t, err)

		accountData := json.RawMessage(`{"id": 999}`)
		acct := &extsvc.Account{
			AccountSpec: extsvc.AccountSpec{
				ServiceType: "gitlab",
				ServiceID:   "https://gitlab.com/",
			},
			AccountData: extsvc.AccountData{
				Data: extsvc.NewUnencryptedData(accountData),
			},
		}
		repoIDs, err := p.FetchUserPerms(ctx,
			acct,
			authz.FetchPermsOptions{},
		)
		if err != nil {
			t.Fatal(err)
		}

		expRepoIDs := []extsvc.RepoID{"1", "2"}
		if diff := cmp.Diff(expRepoIDs, repoIDs.Exacts); diff != "" {
			t.Fatal(diff)
		}

		// Now sync internal repositories as well
		p.syncInternalRepoPermissions = true

		repoIDs, err = p.FetchUserPerms(ctx,
			acct,
			authz.FetchPermsOptions{},
		)
		if err != nil {
			t.Fatal(err)
		}

		expRepoIDs = []extsvc.RepoID{"1", "2", "3"}
		if diff := cmp.Diff(expRepoIDs, repoIDs.Exacts); diff != "" {
			t.Fatal(diff)
		}
	})
}

func TestSudoProvider_FetchRepoPerms(t *testing.T) {
	t.Run("nil repository", func(t *testing.T) {
		p, err := newSudoProvider(SudoProviderOp{
			BaseURL: mustURL(t, "https://gitlab.com"),
		}, nil)
		require.NoError(t, err)

		_, err = p.FetchRepoPerms(context.Background(), nil, authz.FetchPermsOptions{})
		want := "no repository provided"
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("not the code host of the repository", func(t *testing.T) {
		p, err := newSudoProvider(SudoProviderOp{
			BaseURL: mustURL(t, "https://gitlab.com"),
		}, nil)
		require.NoError(t, err)

		_, err = p.FetchRepoPerms(context.Background(),
			&extsvc.Repository{
				URI: "https://github.com/user/repo",
				ExternalRepoSpec: api.ExternalRepoSpec{
					ServiceType: extsvc.TypeGitHub,
					ServiceID:   "https://github.com/",
				},
			},
			authz.FetchPermsOptions{},
		)
		want := `not a code host of the repository: want "https://github.com/" but have "https://gitlab.com/"`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	// The OAuthProvider uses the gitlab.Client under the hood,
	// which uses rcache, a caching layer that uses Redis.
	// We need to clear the cache before we run the tests
	rcache.SetupForTest(t)

	p, err := newSudoProvider(
		SudoProviderOp{
			BaseURL:   mustURL(t, "https://gitlab.com"),
			SudoToken: "admin_token",
		},
		httpcli.NewFactory(nil, func(c *http.Client) error {
			c.Transport = httpcli.WrapTransport(&mockTransport{
				do: func(r *http.Request) (*http.Response, error) {
					want := "https://gitlab.com/api/v4/projects/gitlab_project_id/members/all?per_page=100"
					if r.URL.String() != want {
						return nil, errors.Errorf("URL: want %q but got %q", want, r.URL)
					}

					want = "admin_token"
					got := r.Header.Get("Private-Token")
					if got != want {
						return nil, errors.Errorf("HTTP Private-Token: want %q but got %q", want, got)
					}

					body := `
			[
			{"id": 1, "access_level": 10},
			{"id": 2, "access_level": 20},
			{"id": 3, "access_level": 30}
			]`
					return &http.Response{
						Status:     http.StatusText(http.StatusOK),
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(body))),
					}, nil
				},
			}, http.DefaultTransport)
			return nil
		}),
	)
	require.NoError(t, err)

	accountIDs, err := p.FetchRepoPerms(context.Background(),
		&extsvc.Repository{
			URI: "https://gitlab.com/user/repo",
			ExternalRepoSpec: api.ExternalRepoSpec{
				ServiceType: "gitlab",
				ServiceID:   "https://gitlab.com/",
				ID:          "gitlab_project_id",
			},
		},
		authz.FetchPermsOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}

	// 1 should not be included because of "access_level" < 20
	expAccountIDs := []extsvc.AccountID{"2", "3"}
	if diff := cmp.Diff(expAccountIDs, accountIDs); diff != "" {
		t.Fatal(diff)
	}
}
