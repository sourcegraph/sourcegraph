package authz

import (
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/authz/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

type gitlabAuthzProviderParams struct {
	OAuthOp gitlab.OAuthProviderOp
	SudoOp  gitlab.SudoProviderOp
}

func (m gitlabAuthzProviderParams) Repos(ctx context.Context, repos []*types.Repo) (mine []*types.Repo, others []*types.Repo) {
	panic("should never be called")
}

func (m gitlabAuthzProviderParams) FetchAccount(ctx context.Context, user *types.User, current []*extsvc.Account) (mine *extsvc.Account, err error) {
	panic("should never be called")
}

func (m gitlabAuthzProviderParams) ServiceID() string {
	panic("should never be called")
}

func (m gitlabAuthzProviderParams) ServiceType() string {
	return extsvc.TypeGitLab
}

func (m gitlabAuthzProviderParams) URN() string {
	panic("should never be called")
}

func (m gitlabAuthzProviderParams) Validate() []string { return nil }

func (m gitlabAuthzProviderParams) FetchUserPerms(context.Context, *extsvc.Account) (*authz.ExternalUserPermissions, error) {
	panic("should never be called")
}

func (m gitlabAuthzProviderParams) FetchRepoPerms(context.Context, *extsvc.Repository) ([]extsvc.AccountID, error) {
	panic("should never be called")
}

func TestAuthzProvidersFromConfig(t *testing.T) {
	gitlab.NewOAuthProvider = func(op gitlab.OAuthProviderOp) authz.Provider {
		return gitlabAuthzProviderParams{OAuthOp: op}
	}
	gitlab.NewSudoProvider = func(op gitlab.SudoProviderOp) authz.Provider {
		return gitlabAuthzProviderParams{SudoOp: op}
	}

	providersEqual := func(want ...authz.Provider) func(*testing.T, []authz.Provider) {
		return func(t *testing.T, have []authz.Provider) {
			if diff := cmp.Diff(want, have); diff != "" {
				t.Errorf("authzProviders mismatch (-want +got):\n%s", diff)
			}
		}
	}

	const bogusKey = `LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlCUEFJQkFBSkJBUEpIaWprdG1UMUlLYUd0YTVFZXAzQVo5Q2VPZUw4alBESUZUN3dRZ0tabXQzRUZxRGhCCk93bitRVUhKdUs5Zm92UkROSmVWTDJvWTVCT0l6NHJ3L0cwQ0F3RUFBUUpCQU1BK0o5Mks0d2NQVllsbWMrM28KcHU5NmlKTkNwMmp5Nm5hK1pEQlQzK0VvSUo1VFJGdnN3R2kvTHUzZThYUWwxTDNTM21ub0xPSlZNcTF0bUxOMgpIY0VDSVFEK3daeS83RlYxUEFtdmlXeWlYVklETzJnNWJOaUJlbmdKQ3hFa3Nia1VtUUloQVBOMlZaczN6UFFwCk1EVG9vTlJXcnl0RW1URERkamdiOFpzTldYL1JPRGIxQWlCZWNKblNVQ05TQllLMXJ5VTFmNURTbitoQU9ZaDkKWDFBMlVnTDE3bWhsS1FJaEFPK2JMNmRDWktpTGZORWxmVnRkTUtxQnFjNlBIK01heFU2VzlkVlFvR1dkQWlFQQptdGZ5cE9zYTFiS2hFTDg0blovaXZFYkJyaVJHalAya3lERHYzUlg0V0JrPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=`

	tests := []struct {
		description                  string
		cfg                          conf.Unified
		gitlabConnections            []*schema.GitLabConnection
		bitbucketServerConnections   []*schema.BitbucketServerConnection
		expAuthzAllowAccessByDefault bool
		expAuthzProviders            func(*testing.T, []authz.Provider)
		expSeriousProblems           []string
	}{
		{
			description: "1 GitLab connection with authz enabled, 1 GitLab matching auth provider",
			cfg: conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthProviders: []schema.AuthProviders{{
						Gitlab: &schema.GitLabAuthProvider{
							ClientID:     "clientID",
							ClientSecret: "clientSecret",
							DisplayName:  "GitLab",
							Type:         extsvc.TypeGitLab,
							Url:          "https://gitlab.mine",
						},
					}},
				},
			},
			gitlabConnections: []*schema.GitLabConnection{
				{
					Authorization: &schema.GitLabAuthorization{
						IdentityProvider: schema.IdentityProvider{Oauth: &schema.OAuthIdentity{Type: "oauth"}},
					},
					Url:   "https://gitlab.mine",
					Token: "asdf",
				},
			},
			expAuthzAllowAccessByDefault: true,
			expAuthzProviders: providersEqual(
				gitlabAuthzProviderParams{
					OAuthOp: gitlab.OAuthProviderOp{
						URN:     "extsvc:gitlab:0",
						BaseURL: mustURLParse(t, "https://gitlab.mine"),
						Token:   "asdf",
					},
				},
			),
		},
		{
			description: "1 GitLab connection with authz enabled, 1 GitLab auth provider but doesn't match",
			cfg: conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthProviders: []schema.AuthProviders{{
						Gitlab: &schema.GitLabAuthProvider{
							ClientID:     "clientID",
							ClientSecret: "clientSecret",
							DisplayName:  "GitLab",
							Type:         extsvc.TypeGitLab,
							Url:          "https://gitlab.com",
						},
					}},
				},
			},
			gitlabConnections: []*schema.GitLabConnection{
				{
					Authorization: &schema.GitLabAuthorization{
						IdentityProvider: schema.IdentityProvider{Oauth: &schema.OAuthIdentity{Type: "oauth"}},
					},
					Url:   "https://gitlab.mine",
					Token: "asdf",
				},
			},
			expAuthzAllowAccessByDefault: false,
			expSeriousProblems:           []string{"Did not find authentication provider matching \"https://gitlab.mine\". Check the [**site configuration**](/site-admin/configuration) to verify an entry in [`auth.providers`](https://docs.sourcegraph.com/admin/auth) exists for https://gitlab.mine."},
		},
		{
			description: "1 GitLab connection with authz enabled, no GitLab auth provider",
			cfg: conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthProviders: []schema.AuthProviders{{
						Builtin: &schema.BuiltinAuthProvider{Type: "builtin"},
					}},
				},
			},
			gitlabConnections: []*schema.GitLabConnection{
				{
					Authorization: &schema.GitLabAuthorization{
						IdentityProvider: schema.IdentityProvider{Oauth: &schema.OAuthIdentity{Type: "oauth"}},
					},
					Url:   "https://gitlab.mine",
					Token: "asdf",
				},
			},
			expAuthzAllowAccessByDefault: false,
			expSeriousProblems:           []string{"Did not find authentication provider matching \"https://gitlab.mine\". Check the [**site configuration**](/site-admin/configuration) to verify an entry in [`auth.providers`](https://docs.sourcegraph.com/admin/auth) exists for https://gitlab.mine."},
		},
		{
			description: "Two GitLab connections with authz enabled, two matching GitLab auth providers",
			cfg: conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthProviders: []schema.AuthProviders{
						{
							Gitlab: &schema.GitLabAuthProvider{
								ClientID:     "clientID",
								ClientSecret: "clientSecret",
								DisplayName:  "GitLab.com",
								Type:         extsvc.TypeGitLab,
								Url:          "https://gitlab.com",
							},
						}, {
							Gitlab: &schema.GitLabAuthProvider{
								ClientID:     "clientID",
								ClientSecret: "clientSecret",
								DisplayName:  "GitLab.mine",
								Type:         extsvc.TypeGitLab,
								Url:          "https://gitlab.mine",
							},
						},
					},
				},
			},
			gitlabConnections: []*schema.GitLabConnection{
				{
					Authorization: &schema.GitLabAuthorization{
						IdentityProvider: schema.IdentityProvider{Oauth: &schema.OAuthIdentity{Type: "oauth"}},
					},
					Url:   "https://gitlab.mine",
					Token: "asdf",
				},
				{
					Authorization: &schema.GitLabAuthorization{
						IdentityProvider: schema.IdentityProvider{Oauth: &schema.OAuthIdentity{Type: "oauth"}},
					},
					Url:   "https://gitlab.com",
					Token: "asdf",
				},
			},
			expAuthzAllowAccessByDefault: true,
			expAuthzProviders: providersEqual(
				gitlabAuthzProviderParams{
					OAuthOp: gitlab.OAuthProviderOp{
						URN:     "extsvc:gitlab:0",
						BaseURL: mustURLParse(t, "https://gitlab.mine"),
						Token:   "asdf",
					},
				},
				gitlabAuthzProviderParams{
					OAuthOp: gitlab.OAuthProviderOp{
						URN:     "extsvc:gitlab:0",
						BaseURL: mustURLParse(t, "https://gitlab.com"),
						Token:   "asdf",
					},
				},
			),
		},
		{
			description: "1 GitLab connection with authz disabled",
			cfg: conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthProviders: []schema.AuthProviders{{
						Gitlab: &schema.GitLabAuthProvider{
							ClientID:     "clientID",
							ClientSecret: "clientSecret",
							DisplayName:  "GitLab",
							Type:         extsvc.TypeGitLab,
							Url:          "https://gitlab.mine",
						},
					}},
				},
			},
			gitlabConnections: []*schema.GitLabConnection{
				{
					Authorization: nil,
					Url:           "https://gitlab.mine",
					Token:         "asdf",
				},
			},
			expAuthzAllowAccessByDefault: true,
			expAuthzProviders:            nil,
		},
		{
			description: "external auth provider",
			cfg: conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthProviders: []schema.AuthProviders{{
						Saml: &schema.SAMLAuthProvider{
							ConfigID: "okta",
							Type:     "saml",
						},
					}},
				},
			},
			gitlabConnections: []*schema.GitLabConnection{
				{
					Authorization: &schema.GitLabAuthorization{
						IdentityProvider: schema.IdentityProvider{External: &schema.ExternalIdentity{
							Type:             "external",
							AuthProviderID:   "okta",
							AuthProviderType: "saml",
							GitlabProvider:   "my-external",
						}},
					},
					Url:   "https://gitlab.mine",
					Token: "asdf",
				},
			},
			expAuthzAllowAccessByDefault: true,
			expAuthzProviders: providersEqual(
				gitlabAuthzProviderParams{
					SudoOp: gitlab.SudoProviderOp{
						URN:     "extsvc:gitlab:0",
						BaseURL: mustURLParse(t, "https://gitlab.mine"),
						AuthnConfigID: providers.ConfigID{
							Type: "saml",
							ID:   "okta",
						},
						GitLabProvider:    "my-external",
						SudoToken:         "asdf",
						UseNativeUsername: false,
					},
				},
			),
		},
		{
			description: "exact username matching",
			cfg: conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthProviders: []schema.AuthProviders{},
				},
			},
			gitlabConnections: []*schema.GitLabConnection{
				{
					Authorization: &schema.GitLabAuthorization{
						IdentityProvider: schema.IdentityProvider{Username: &schema.UsernameIdentity{Type: "username"}},
					},
					Url:   "https://gitlab.mine",
					Token: "asdf",
				},
			},
			expAuthzAllowAccessByDefault: true,
			expAuthzProviders: providersEqual(
				gitlabAuthzProviderParams{
					SudoOp: gitlab.SudoProviderOp{
						URN:               "extsvc:gitlab:0",
						BaseURL:           mustURLParse(t, "https://gitlab.mine"),
						SudoToken:         "asdf",
						UseNativeUsername: true,
					},
				},
			),
		},
		{
			description: "1 BitbucketServer connection with authz disabled",
			bitbucketServerConnections: []*schema.BitbucketServerConnection{
				{
					Authorization: nil,
					Url:           "https://bitbucket.mycorp.org",
					Username:      "admin",
					Token:         "secret-token",
				},
			},
			expAuthzAllowAccessByDefault: true,
			expAuthzProviders:            providersEqual(),
		},
		{
			description: "Bitbucket Server Oauth config error",
			cfg:         conf.Unified{},
			bitbucketServerConnections: []*schema.BitbucketServerConnection{
				{
					Authorization: &schema.BitbucketServerAuthorization{
						IdentityProvider: schema.BitbucketServerIdentityProvider{
							Username: &schema.BitbucketServerUsernameIdentity{
								Type: "username",
							},
						},
						Oauth: schema.BitbucketServerOAuth{
							ConsumerKey: "sourcegraph",
							SigningKey:  "Invalid Key",
						},
					},
					Url:      "https://bitbucketserver.mycorp.org",
					Username: "admin",
					Token:    "secret-token",
				},
			},
			expAuthzAllowAccessByDefault: false,
			expSeriousProblems:           []string{"1 error occurred:\n\t* authorization.oauth.signingKey: illegal base64 data at input byte 7\n\n"},
		},
		{
			description: "Bitbucket Server exact username matching",
			cfg:         conf.Unified{},
			bitbucketServerConnections: []*schema.BitbucketServerConnection{
				{
					Authorization: &schema.BitbucketServerAuthorization{
						IdentityProvider: schema.BitbucketServerIdentityProvider{
							Username: &schema.BitbucketServerUsernameIdentity{
								Type: "username",
							},
						},
						Oauth: schema.BitbucketServerOAuth{
							ConsumerKey: "sourcegraph",
							SigningKey:  bogusKey,
						},
					},
					Url:      "https://bitbucketserver.mycorp.org",
					Username: "admin",
					Token:    "secret-token",
				},
			},
			expAuthzAllowAccessByDefault: true,
			expAuthzProviders: func(t *testing.T, have []authz.Provider) {
				if len(have) == 0 {
					t.Fatalf("no providers")
				}

				if have[0].ServiceType() != extsvc.TypeBitbucketServer {
					t.Fatalf("no Bitbucket Server authz provider returned")
				}
			},
		},

		// For Sourcegraph authz provider
		{
			description: "Conflicted configuration between Sourcegraph and GitLab authz provider",
			cfg: conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					PermissionsUserMapping: &schema.PermissionsUserMapping{
						Enabled: true,
						BindID:  "email",
					},
					AuthProviders: []schema.AuthProviders{{
						Gitlab: &schema.GitLabAuthProvider{
							ClientID:     "clientID",
							ClientSecret: "clientSecret",
							DisplayName:  "GitLab",
							Type:         extsvc.TypeGitLab,
							Url:          "https://gitlab.mine",
						},
					}},
				},
			},
			gitlabConnections: []*schema.GitLabConnection{
				{
					Authorization: &schema.GitLabAuthorization{
						IdentityProvider: schema.IdentityProvider{Oauth: &schema.OAuthIdentity{Type: "oauth"}},
					},
					Url:   "https://gitlab.mine",
					Token: "asdf",
				},
			},
			expAuthzAllowAccessByDefault: false,
			expSeriousProblems:           []string{"The permissions user mapping (site configuration `permissions.userMapping`) cannot be enabled when \"gitlab\" authorization providers are in use. Blocking access to all repositories until the conflict is resolved."},
		},
		{
			description: "Conflicted configuration between Sourcegraph and Bitbucket Server authz provider",
			cfg: conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					PermissionsUserMapping: &schema.PermissionsUserMapping{
						Enabled: true,
						BindID:  "email",
					},
				},
			},
			bitbucketServerConnections: []*schema.BitbucketServerConnection{
				{
					Authorization: &schema.BitbucketServerAuthorization{
						IdentityProvider: schema.BitbucketServerIdentityProvider{
							Username: &schema.BitbucketServerUsernameIdentity{
								Type: "username",
							},
						},
						Oauth: schema.BitbucketServerOAuth{
							ConsumerKey: "sourcegraph",
							SigningKey:  bogusKey,
						},
					},
					Url:      "https://bitbucketserver.mycorp.org",
					Username: "admin",
					Token:    "secret-token",
				},
			},
			expAuthzAllowAccessByDefault: false,
			expSeriousProblems:           []string{"The permissions user mapping (site configuration `permissions.userMapping`) cannot be enabled when \"bitbucketServer\" authorization providers are in use. Blocking access to all repositories until the conflict is resolved."},
		},
	}

	for _, test := range tests {
		t.Logf("Test %q", test.description)

		store := fakeStore{
			gitlabs:          test.gitlabConnections,
			bitbucketServers: test.bitbucketServerConnections,
		}

		allowAccessByDefault, authzProviders, seriousProblems, _ :=
			ProvidersFromConfig(context.Background(), &test.cfg, &store)
		if allowAccessByDefault != test.expAuthzAllowAccessByDefault {
			t.Errorf("allowAccessByDefault: (actual) %v != (expected) %v", asJSON(t, allowAccessByDefault), asJSON(t, test.expAuthzAllowAccessByDefault))
		}
		if test.expAuthzProviders != nil {
			test.expAuthzProviders(t, authzProviders)
		}
		if !reflect.DeepEqual(seriousProblems, test.expSeriousProblems) {
			t.Errorf("seriousProblems: (actual) %+v != (expected) %+v", asJSON(t, seriousProblems), asJSON(t, test.expSeriousProblems))
		}
	}
}

func mustURLParse(t *testing.T, u string) *url.URL {
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func asJSON(t *testing.T, v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type fakeStore struct {
	gitlabs          []*schema.GitLabConnection
	githubs          []*schema.GitHubConnection
	bitbucketServers []*schema.BitbucketServerConnection
}

func (s fakeStore) List(ctx context.Context, opt database.ExternalServicesListOptions) ([]*types.ExternalService, error) {
	mustMarshalJSONString := func(v interface{}) string {
		str, err := jsoniter.MarshalToString(v)
		if err != nil {
			panic(err)
		}
		return str
	}

	var svcs []*types.ExternalService
	for _, kind := range opt.Kinds {
		switch kind {
		case extsvc.KindGitHub:
			for _, github := range s.githubs {
				svcs = append(svcs, &types.ExternalService{
					Kind:   kind,
					Config: mustMarshalJSONString(github),
				})
			}
		case extsvc.KindGitLab:
			for _, gitlab := range s.gitlabs {
				svcs = append(svcs, &types.ExternalService{
					Kind:   kind,
					Config: mustMarshalJSONString(gitlab),
				})
			}
		case extsvc.KindBitbucketServer:
			for _, bbs := range s.bitbucketServers {
				svcs = append(svcs, &types.ExternalService{
					Kind:   kind,
					Config: mustMarshalJSONString(bbs),
				})
			}
		default:
			return nil, errors.Errorf("unexpected kind: %s", kind)
		}
	}

	return svcs, nil
}
