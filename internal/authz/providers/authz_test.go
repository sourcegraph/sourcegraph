package providers

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	jsoniter "github.com/json-iterator/go"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/authz/providers/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

const bogusKey = `LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlCUEFJQkFBSkJBUEpIaWprdG1UMUlLYUd0YTVFZXAzQVo5Q2VPZUw4alBESUZUN3dRZ0tabXQzRUZxRGhCCk93bitRVUhKdUs5Zm92UkROSmVWTDJvWTVCT0l6NHJ3L0cwQ0F3RUFBUUpCQU1BK0o5Mks0d2NQVllsbWMrM28KcHU5NmlKTkNwMmp5Nm5hK1pEQlQzK0VvSUo1VFJGdnN3R2kvTHUzZThYUWwxTDNTM21ub0xPSlZNcTF0bUxOMgpIY0VDSVFEK3daeS83RlYxUEFtdmlXeWlYVklETzJnNWJOaUJlbmdKQ3hFa3Nia1VtUUloQVBOMlZaczN6UFFwCk1EVG9vTlJXcnl0RW1URERkamdiOFpzTldYL1JPRGIxQWlCZWNKblNVQ05TQllLMXJ5VTFmNURTbitoQU9ZaDkKWDFBMlVnTDE3bWhsS1FJaEFPK2JMNmRDWktpTGZORWxmVnRkTUtxQnFjNlBIK01heFU2VzlkVlFvR1dkQWlFQQptdGZ5cE9zYTFiS2hFTDg0blovaXZFYkJyaVJHalAya3lERHYzUlg0V0JrPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=`

type gitlabAuthzProviderParams struct {
	OAuthOp gitlab.OAuthProviderOp
	SudoOp  gitlab.SudoProviderOp
}

func (m gitlabAuthzProviderParams) Repos(ctx context.Context, repos []*types.Repo) (mine []*types.Repo, others []*types.Repo) {
	panic("should never be called")
}

func (m gitlabAuthzProviderParams) FetchAccount(ctx context.Context, user *types.User, current []*extsvc.Account, verifiedEmails []string) (mine *extsvc.Account, err error) {
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

func (m gitlabAuthzProviderParams) ValidateConnection(context.Context) error { return nil }

func (m gitlabAuthzProviderParams) FetchUserPerms(context.Context, *extsvc.Account, authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	panic("should never be called")
}

func (m gitlabAuthzProviderParams) FetchUserPermsByToken(context.Context, string, authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	panic("should never be called")
}

func (m gitlabAuthzProviderParams) FetchRepoPerms(context.Context, *extsvc.Repository, authz.FetchPermsOptions) ([]extsvc.AccountID, error) {
	panic("should never be called")
}

func TestAuthzProvidersFromConfig(t *testing.T) {
	t.Cleanup(licensing.TestingSkipFeatureChecks())
	gitlab.NewOAuthProvider = func(op gitlab.OAuthProviderOp) authz.Provider {
		return gitlabAuthzProviderParams{OAuthOp: op}
	}
	gitlab.NewSudoProvider = func(op gitlab.SudoProviderOp) authz.Provider {
		return gitlabAuthzProviderParams{SudoOp: op}
	}

	providersEqual := func(want ...authz.Provider) func(*testing.T, []authz.Provider) {
		return func(t *testing.T, have []authz.Provider) {
			if diff := cmp.Diff(want, have, cmpopts.IgnoreInterfaces(struct{ database.DB }{})); diff != "" {
				t.Errorf("authzProviders mismatch (-want +got):\n%s", diff)
			}
		}
	}

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
						URN:                         "extsvc:gitlab:0",
						BaseURL:                     mustURLParse(t, "https://gitlab.mine"),
						Token:                       "asdf",
						SyncInternalRepoPermissions: true,
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
			expSeriousProblems:           []string{"Did not find authentication provider matching \"https://gitlab.mine\". Check the [**site configuration**](/site-admin/configuration) to verify an entry in [`auth.providers`](https://sourcegraph.com/docs/admin/auth) exists for https://gitlab.mine."},
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
			expSeriousProblems:           []string{"Did not find authentication provider matching \"https://gitlab.mine\". Check the [**site configuration**](/site-admin/configuration) to verify an entry in [`auth.providers`](https://sourcegraph.com/docs/admin/auth) exists for https://gitlab.mine."},
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
						URN:                         "extsvc:gitlab:0",
						BaseURL:                     mustURLParse(t, "https://gitlab.mine"),
						Token:                       "asdf",
						SyncInternalRepoPermissions: true,
					},
				},
				gitlabAuthzProviderParams{
					OAuthOp: gitlab.OAuthProviderOp{
						URN:                         "extsvc:gitlab:0",
						BaseURL:                     mustURLParse(t, "https://gitlab.com"),
						Token:                       "asdf",
						SyncInternalRepoPermissions: true,
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
						GitLabProvider:              "my-external",
						SudoToken:                   "asdf",
						UseNativeUsername:           false,
						SyncInternalRepoPermissions: true,
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
						URN:                         "extsvc:gitlab:0",
						BaseURL:                     mustURLParse(t, "https://gitlab.mine"),
						SudoToken:                   "asdf",
						UseNativeUsername:           true,
						SyncInternalRepoPermissions: true,
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
			expSeriousProblems:           []string{"authorization.oauth.signingKey: illegal base64 data at input byte 7"},
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
			description: "Explicit permissions can be enabled alongside synced permissions",
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
			expAuthzAllowAccessByDefault: true,
			expAuthzProviders: providersEqual(
				gitlabAuthzProviderParams{
					OAuthOp: gitlab.OAuthProviderOp{
						URN:                         "extsvc:gitlab:0",
						BaseURL:                     mustURLParse(t, "https://gitlab.mine"),
						Token:                       "asdf",
						SyncInternalRepoPermissions: true,
					},
				},
			),
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			db := dbmocks.NewMockDB()
			externalServices := dbmocks.NewMockExternalServiceStore()
			db.ExternalServicesFunc.SetDefaultReturn(externalServices)
			externalServices.ListFunc.SetDefaultHook(func(ctx context.Context, opt database.ExternalServicesListOptions) ([]*types.ExternalService, error) {
				mustMarshalJSONString := func(v any) string {
					str, err := jsoniter.MarshalToString(v)
					require.NoError(t, err)
					return str
				}

				var svcs []*types.ExternalService
				for _, kind := range opt.Kinds {
					switch kind {
					case extsvc.KindGitLab:
						for _, gl := range test.gitlabConnections {
							svcs = append(svcs, &types.ExternalService{
								Kind:   kind,
								Config: extsvc.NewUnencryptedConfig(mustMarshalJSONString(gl)),
							})
						}
					case extsvc.KindBitbucketServer:
						for _, bbs := range test.bitbucketServerConnections {
							svcs = append(svcs, &types.ExternalService{
								Kind:   kind,
								Config: extsvc.NewUnencryptedConfig(mustMarshalJSONString(bbs)),
							})
						}
					case extsvc.KindGitHub, extsvc.KindPerforce, extsvc.KindBitbucketCloud, extsvc.KindGerrit, extsvc.KindAzureDevOps:
					default:
						return nil, errors.Errorf("unexpected kind: %s", kind)
					}
				}
				return svcs, nil
			})
			allowAccessByDefault, authzProviders, seriousProblems, _, _ := ProvidersFromConfig(
				context.Background(),
				staticConfig(test.cfg.SiteConfiguration),
				db,
			)
			assert.Equal(t, test.expAuthzAllowAccessByDefault, allowAccessByDefault)
			if test.expAuthzProviders != nil {
				test.expAuthzProviders(t, authzProviders)
			}

			assert.Equal(t, test.expSeriousProblems, seriousProblems)
		})
	}
}

func TestAuthzProvidersEnabledACLsDisabled(t *testing.T) {
	t.Cleanup(licensing.MockCheckFeatureError("failed"))
	tests := []struct {
		description                string
		cfg                        conf.Unified
		azureDevOpsConnections     []*schema.AzureDevOpsConnection
		gitlabConnections          []*schema.GitLabConnection
		bitbucketServerConnections []*schema.BitbucketServerConnection
		githubConnections          []*schema.GitHubConnection
		perforceConnections        []*schema.PerforceConnection
		bitbucketCloudConnections  []*schema.BitbucketCloudConnection
		gerritConnections          []*schema.GerritConnection

		expInvalidConnections []string
		expSeriousProblems    []string
	}{
		{
			description: "Azure DevOps connection with enforce permissions enabled but missing license for ACLs",
			cfg: conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthProviders: []schema.AuthProviders{{
						AzureDevOps: &schema.AzureDevOpsAuthProvider{
							ClientID:     "clientID",
							ClientSecret: "clientSecret",
							DisplayName:  "Azure DevOps",
							Type:         extsvc.TypeAzureDevOps,
						},
					}},
				},
			},
			azureDevOpsConnections: []*schema.AzureDevOpsConnection{
				{
					EnforcePermissions: true,
					Url:                "https://dev.azure.com",
				},
			},
			expSeriousProblems:    []string{"failed"},
			expInvalidConnections: []string{"azuredevops"},
		},
		{
			description: "GitHub connection with authz enabled but missing license for ACLs",
			cfg: conf.Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthProviders: []schema.AuthProviders{{
						Github: &schema.GitHubAuthProvider{
							ClientID:     "clientID",
							ClientSecret: "clientSecret",
							DisplayName:  "GitHub",
							Type:         extsvc.TypeGitHub,
							Url:          "https://github.mine",
						},
					}},
				},
			},
			githubConnections: []*schema.GitHubConnection{
				{
					Authorization: &schema.GitHubAuthorization{},
					Url:           "https://github.com/my-org",
				},
			},
			expSeriousProblems:    []string{"failed"},
			expInvalidConnections: []string{"github"},
		},
		{
			description: "GitLab connection with authz enabled but missing license for ACLs",
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
			expSeriousProblems:    []string{"failed"},
			expInvalidConnections: []string{"gitlab"},
		},
		{
			description: "Bitbucket Server connection with authz enabled but missing license for ACLs",
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
			expSeriousProblems:    []string{"failed"},
			expInvalidConnections: []string{"bitbucketServer"},
		},
		{
			description: "Bitbucket Cloud connection with authz enabled but missing license for ACLs",
			cfg:         conf.Unified{},
			bitbucketCloudConnections: []*schema.BitbucketCloudConnection{
				{
					Authorization: &schema.BitbucketCloudAuthorization{},
					Url:           "https://bitbucket.org",
					Username:      "admin",
					AppPassword:   "secret-password",
				},
			},
			expSeriousProblems:    []string{"failed"},
			expInvalidConnections: []string{"bitbucketCloud"},
		},
		{
			description: "Gerrit connection with authz enabled but missing license for ACLs",
			cfg:         conf.Unified{},
			gerritConnections: []*schema.GerritConnection{
				{
					Authorization: &schema.GerritAuthorization{},
					Url:           "https://gerrit.sgdev.org",
					Username:      "admin",
					Password:      "secret-password",
				},
			},
			expSeriousProblems:    []string{"failed"},
			expInvalidConnections: []string{"gerrit"},
		},
		{
			description: "Perforce connection with authz enabled but missing license for ACLs",
			cfg:         conf.Unified{},
			perforceConnections: []*schema.PerforceConnection{
				{
					Authorization: &schema.PerforceAuthorization{},
					P4Port:        "ssl:111.222.333.444:1666",
					P4User:        "admin",
					P4Passwd:      "pa$$word",
					Depots: []string{
						"//Sourcegraph",
						"//Engineering/Cloud",
					},
				},
			},
			expSeriousProblems:    []string{"failed"},
			expInvalidConnections: []string{"perforce"},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			db := dbmocks.NewMockDB()
			externalServices := dbmocks.NewMockExternalServiceStore()
			db.ExternalServicesFunc.SetDefaultReturn(externalServices)
			externalServices.ListFunc.SetDefaultHook(func(ctx context.Context, opt database.ExternalServicesListOptions) ([]*types.ExternalService, error) {
				mustMarshalJSONString := func(v any) string {
					str, err := jsoniter.MarshalToString(v)
					require.NoError(t, err)
					return str
				}

				var svcs []*types.ExternalService
				for _, kind := range opt.Kinds {
					switch kind {
					case extsvc.KindAzureDevOps:
						for _, ado := range test.azureDevOpsConnections {
							svcs = append(svcs, &types.ExternalService{
								Kind:   kind,
								Config: extsvc.NewUnencryptedConfig(mustMarshalJSONString(ado)),
							})
						}
					case extsvc.KindGitLab:
						for _, gl := range test.gitlabConnections {
							svcs = append(svcs, &types.ExternalService{
								Kind:   kind,
								Config: extsvc.NewUnencryptedConfig(mustMarshalJSONString(gl)),
							})
						}
					case extsvc.KindBitbucketServer:
						for _, bbs := range test.bitbucketServerConnections {
							svcs = append(svcs, &types.ExternalService{
								Kind:   kind,
								Config: extsvc.NewUnencryptedConfig(mustMarshalJSONString(bbs)),
							})
						}
					case extsvc.KindGitHub:
						for _, gh := range test.githubConnections {
							svcs = append(svcs, &types.ExternalService{
								Kind:   kind,
								Config: extsvc.NewUnencryptedConfig(mustMarshalJSONString(gh)),
							})
						}
					case extsvc.KindBitbucketCloud:
						for _, bbcloud := range test.bitbucketCloudConnections {
							svcs = append(svcs, &types.ExternalService{
								Kind:   kind,
								Config: extsvc.NewUnencryptedConfig(mustMarshalJSONString(bbcloud)),
							})
						}
					case extsvc.KindGerrit:
						for _, g := range test.gerritConnections {
							svcs = append(svcs, &types.ExternalService{
								Kind:   kind,
								Config: extsvc.NewUnencryptedConfig(mustMarshalJSONString(g)),
							})
						}
					case extsvc.KindPerforce:
						for _, pf := range test.perforceConnections {
							svcs = append(svcs, &types.ExternalService{
								Kind:   kind,
								Config: extsvc.NewUnencryptedConfig(mustMarshalJSONString(pf)),
							})
						}
					}
				}
				return svcs, nil
			})

			_, _, seriousProblems, _, invalidConnections := ProvidersFromConfig(
				context.Background(),
				staticConfig(test.cfg.SiteConfiguration),
				db,
			)

			assert.Equal(t, test.expSeriousProblems, seriousProblems)
			assert.Equal(t, test.expInvalidConnections, invalidConnections)
		})
	}
}

type staticConfig schema.SiteConfiguration

func (s staticConfig) SiteConfig() schema.SiteConfiguration {
	return schema.SiteConfiguration(s)
}

func mustURLParse(t *testing.T, u string) *url.URL {
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

type mockProvider struct {
	codeHost *extsvc.CodeHost
	extAcct  *extsvc.Account
}

func (p *mockProvider) FetchAccount(context.Context, *types.User, []*extsvc.Account, []string) (mine *extsvc.Account, err error) {
	return p.extAcct, nil
}

func (p *mockProvider) ServiceType() string { return p.codeHost.ServiceType }
func (p *mockProvider) ServiceID() string   { return p.codeHost.ServiceID }
func (p *mockProvider) URN() string         { return extsvc.URN(p.codeHost.ServiceType, 0) }

func (p *mockProvider) ValidateConnection(context.Context) error { return nil }

func (p *mockProvider) FetchUserPerms(context.Context, *extsvc.Account, authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	return nil, nil
}

func (p *mockProvider) FetchUserPermsByToken(context.Context, string, authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	return nil, nil
}

func (p *mockProvider) FetchRepoPerms(context.Context, *extsvc.Repository, authz.FetchPermsOptions) ([]extsvc.AccountID, error) {
	return nil, nil
}

func mockExplicitPermissions(enabled bool) func() {
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

func TestPermissionSyncingDisabled(t *testing.T) {
	authz.SetProviders(true, []authz.Provider{&mockProvider{}})
	cleanupLicense := licensing.MockCheckFeatureError("")

	t.Cleanup(func() {
		authz.SetProviders(true, nil)
		cleanupLicense()
	})

	t.Run("no authz providers", func(t *testing.T) {
		authz.SetProviders(true, nil)
		t.Cleanup(func() {
			authz.SetProviders(true, []authz.Provider{&mockProvider{}})
		})

		assert.True(t, PermissionSyncingDisabled())
	})

	t.Run("permissions user mapping enabled", func(t *testing.T) {
		cleanup := mockExplicitPermissions(true)
		t.Cleanup(func() {
			cleanup()
			conf.Mock(nil)
		})

		assert.False(t, PermissionSyncingDisabled())
	})

	t.Run("license does not have acls feature", func(t *testing.T) {
		licensing.MockCheckFeatureError("failed")
		t.Cleanup(func() {
			licensing.MockCheckFeatureError("")
		})
		assert.True(t, PermissionSyncingDisabled())
	})

	t.Run("Auto code host syncs disabled", func(t *testing.T) {
		conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{DisableAutoCodeHostSyncs: true}})
		t.Cleanup(func() {
			conf.Mock(nil)
		})
		assert.True(t, PermissionSyncingDisabled())
	})

	t.Run("Auto code host syncs enabled", func(t *testing.T) {
		conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{DisableAutoCodeHostSyncs: false}})
		t.Cleanup(func() {
			conf.Mock(nil)
		})
		assert.False(t, PermissionSyncingDisabled())
	})
}

func TestValidateExternalServiceConfig(t *testing.T) {
	t.Parallel()
	t.Cleanup(licensing.TestingSkipFeatureChecks())

	// Assertion helpers
	equals := func(want ...string) func(testing.TB, []string) {
		sort.Strings(want)
		return func(t testing.TB, have []string) {
			t.Helper()
			sort.Strings(have)
			if !reflect.DeepEqual(have, want) {
				t.Error(cmp.Diff(have, want))
			}
		}
	}

	// Set difference: a - b
	diff := func(a, b []string) (difference []string) {
		set := make(map[string]struct{}, len(b))
		for _, err := range b {
			set[err] = struct{}{}
		}
		for _, err := range a {
			if _, ok := set[err]; !ok {
				difference = append(difference, err)
			}
		}
		return
	}

	includes := func(want ...string) func(testing.TB, []string) {
		return func(t testing.TB, have []string) {
			t.Helper()
			for _, err := range diff(want, have) {
				t.Errorf("%q not found in set:\n%s", err, pretty.Sprint(have))
			}
		}
	}

	excludes := func(want ...string) func(testing.TB, []string) {
		return func(t testing.TB, have []string) {
			t.Helper()
			for _, err := range diff(want, diff(want, have)) {
				t.Errorf("%q found in set:\n%s", err, pretty.Sprint(have))
			}
		}
	}

	const bogusPrivateKey = `LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlCUEFJQkFBSkJBUEpIaWprdG1UMUlLYUd0YTVFZXAzQVo5Q2VPZUw4alBESUZUN3dRZ0tabXQzRUZxRGhCCk93bitRVUhKdUs5Zm92UkROSmVWTDJvWTVCT0l6NHJ3L0cwQ0F3RUFBUUpCQU1BK0o5Mks0d2NQVllsbWMrM28KcHU5NmlKTkNwMmp5Nm5hK1pEQlQzK0VvSUo1VFJGdnN3R2kvTHUzZThYUWwxTDNTM21ub0xPSlZNcTF0bUxOMgpIY0VDSVFEK3daeS83RlYxUEFtdmlXeWlYVklETzJnNWJOaUJlbmdKQ3hFa3Nia1VtUUloQVBOMlZaczN6UFFwCk1EVG9vTlJXcnl0RW1URERkamdiOFpzTldYL1JPRGIxQWlCZWNKblNVQ05TQllLMXJ5VTFmNURTbitoQU9ZaDkKWDFBMlVnTDE3bWhsS1FJaEFPK2JMNmRDWktpTGZORWxmVnRkTUtxQnFjNlBIK01heFU2VzlkVlFvR1dkQWlFQQptdGZ5cE9zYTFiS2hFTDg0blovaXZFYkJyaVJHalAya3lERHYzUlg0V0JrPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=`

	// Test table
	for _, tc := range []struct {
		kind   string
		desc   string
		config string
		ps     []schema.AuthProviders
		assert func(testing.TB, []string)
	}{
		{
			kind:   extsvc.KindAWSCodeCommit,
			desc:   "without region, accessKeyID, secretAccessKey, gitCredentials",
			config: `{}`,
			assert: includes(
				"region is required",
				"accessKeyID is required",
				"secretAccessKey is required",
				"gitCredentials is required",
			),
		},
		{
			kind:   extsvc.KindAWSCodeCommit,
			desc:   "invalid region",
			config: `{"region": "foo", "accessKeyID": "bar", "secretAccessKey": "baz", "gitCredentials": {"username": "user", "password": "pw"}}`,
			assert: includes(
				`region: region must be one of the following: "ap-northeast-1", "ap-northeast-2", "ap-south-1", "ap-southeast-1", "ap-southeast-2", "ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2", "eu-west-3", "sa-east-1", "us-east-1", "us-east-2", "us-west-1", "us-west-2"`,
			),
		},
		{
			kind:   extsvc.KindAWSCodeCommit,
			desc:   "invalid gitCredentials",
			config: `{"region": "eu-west-2", "accessKeyID": "bar", "secretAccessKey": "baz", "gitCredentials": {"username": "", "password": ""}}`,
			assert: includes(
				`gitCredentials.username: String length must be greater than or equal to 1`,
				`gitCredentials.password: String length must be greater than or equal to 1`,
			),
		},
		{
			kind:   extsvc.KindAWSCodeCommit,
			desc:   "valid",
			config: `{"region": "eu-west-2", "accessKeyID": "bar", "secretAccessKey": "baz", "gitCredentials": {"username": "user", "password": "pw"}}`,
			assert: equals("<nil>"),
		},
		{
			kind: extsvc.KindAWSCodeCommit,
			desc: "valid exclude",
			config: `
			{
				"region": "eu-west-1",
				"accessKeyID": "bar",
				"secretAccessKey": "baz",
				"gitCredentials": {"username": "user", "password": "pw"},
				"exclude": [
					{"name": "foobar-barfoo_bazbar"},
					{"id": "d111baff-3450-46fd-b7d2-a0ae41f1c5bb"},
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind:   extsvc.KindAWSCodeCommit,
			desc:   "invalid empty exclude",
			config: `{"exclude": []}`,
			assert: includes(`exclude: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindAWSCodeCommit,
			desc:   "invalid empty exclude item",
			config: `{"exclude": [{}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindAWSCodeCommit,
			desc:   "invalid exclude item",
			config: `{"exclude": [{"foo": "bar"}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindAWSCodeCommit,
			desc:   "invalid exclude item name",
			config: `{"exclude": [{"name": "f o o b a r"}]}`,
			assert: includes(`exclude.0.name: Does not match pattern '^[\w.-]+$'`),
		},
		{
			kind:   extsvc.KindAWSCodeCommit,
			desc:   "invalid exclude item id",
			config: `{"exclude": [{"id": "b$a$r"}]}`,
			assert: includes(`exclude.0.id: Does not match pattern '^[\w-]+$'`),
		},
		{
			kind: extsvc.KindAWSCodeCommit,
			desc: "invalid additional exclude item properties",
			config: `{"exclude": [{
				"id": "d111baff-3450-46fd-b7d2-a0ae41f1c5bb",
				"bar": "baz"
			}]}`,
			assert: includes(`exclude.0: Additional property bar is not allowed`),
		},
		{
			kind: extsvc.KindAWSCodeCommit,
			desc: "both name and id can be specified in exclude",
			config: `
			{
				"region": "eu-west-1",
				"accessKeyID": "bar",
				"secretAccessKey": "baz",
				"gitCredentials": {"username": "user", "password": "pw"},
				"exclude": [
					{
					  "name": "foobar",
					  "id": "f000ba44-3450-46fd-b7d2-a0ae41f1c5bb"
					},
					{
					  "name": "barfoo",
					  "id": "13337a11-3450-46fd-b7d2-a0ae41f1c5bb"
					},
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind:   extsvc.KindGitolite,
			desc:   "without prefix nor host",
			config: `{}`,
			assert: includes(
				"prefix is required",
				"host is required",
			),
		},
		{
			kind:   extsvc.KindGitolite,
			desc:   "with example.com defaults",
			config: `{"prefix": "gitolite.example.com/", "host": "git@gitolite.example.com"}`,
			assert: includes(
				"prefix: Must not validate the schema (not)",
				"host: Must not validate the schema (not)",
			),
		},
		{
			kind:   extsvc.KindGitolite,
			desc:   "witout prefix nor host",
			config: `{}`,
			assert: includes(
				"prefix is required",
				"host is required",
			),
		},
		{
			kind:   extsvc.KindGitolite,
			desc:   "invalid empty exclude",
			config: `{"exclude": []}`,
			assert: includes(`exclude: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindGitolite,
			desc:   "invalid empty exclude item",
			config: `{"exclude": [{}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindGitolite,
			desc:   "invalid exclude item",
			config: `{"exclude": [{"foo": "bar"}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindGitolite,
			desc:   "invalid exclude item name",
			config: `{"exclude": [{"name": ""}]}`,
			assert: includes(`exclude.0.name: String length must be greater than or equal to 1`),
		},
		{
			kind:   extsvc.KindGitolite,
			desc:   "invalid additional exclude item properties",
			config: `{"exclude": [{"name": "foo", "bar": "baz"}]}`,
			assert: includes(`exclude.0: Additional property bar is not allowed`),
		},
		{
			kind: extsvc.KindGitolite,
			desc: "name can be specified in exclude",
			config: `
			{
				"prefix": "/",
				"host": "gitolite.mycorp.com",
				"exclude": [
					{"name": "bar"},
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind: extsvc.KindBitbucketCloud,
			desc: "valid with url, username, appPassword",
			config: `
			{
				"url": "https://bitbucket.org/",
				"username": "admin",
				"appPassword": "app-password"
			}`,
			assert: equals("<nil>"),
		},
		{
			kind: extsvc.KindBitbucketCloud,
			desc: "valid with url, accessToken",
			config: `
			{
				"url": "https://bitbucket.org/",
				"accessToken": "access-token"
			}`,
			assert: equals("<nil>"),
		},
		{
			kind: extsvc.KindBitbucketCloud,
			desc: "valid with url, username, appPassword, teams",
			config: `
			{
				"url": "https://bitbucket.org/",
				"username": "admin",
				"appPassword": "app-password",
				"teams": ["sglocal", "sg_local", "--a-team----name-"]
			}`,
			assert: equals("<nil>"),
		},
		{
			kind:   extsvc.KindBitbucketCloud,
			desc:   "without url",
			config: `{}`,
			assert: includes(
				"url is required",
			),
		},
		{
			kind:   extsvc.KindBitbucketCloud,
			desc:   "bad url scheme",
			config: `{"url": "badscheme://bitbucket.org"}`,
			assert: includes("url: Does not match pattern '^https?://'"),
		},
		{
			kind:   extsvc.KindBitbucketCloud,
			desc:   "bad apiURL scheme",
			config: `{"apiURL": "badscheme://api.bitbucket.org"}`,
			assert: includes("apiURL: Does not match pattern '^https?://'"),
		},
		{
			kind:   extsvc.KindBitbucketCloud,
			desc:   "invalid gitURLType",
			config: `{"gitURLType": "bad"}`,
			assert: includes(`gitURLType: gitURLType must be one of the following: "http", "ssh"`),
		},
		{
			kind:   extsvc.KindBitbucketCloud,
			desc:   "invalid team name",
			config: `{"teams": ["sg local"]}`,
			assert: includes(
				`teams.0: Does not match pattern '^[\w-]+$'`,
			),
		},
		{
			kind: extsvc.KindBitbucketCloud,
			desc: "empty exclude",
			config: `
			{
				"url": "https://bitbucket.org/",
				"username": "admin",
				"appPassword": "app-password",
				"exclude": []
			}`,
			assert: equals("<nil>"),
		},
		{
			kind:   extsvc.KindBitbucketCloud,
			desc:   "invalid empty exclude item",
			config: `{"exclude": [{}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindBitbucketCloud,
			desc:   "invalid exclude item",
			config: `{"exclude": [{"foo": "bar"}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindBitbucketCloud,
			desc:   "invalid exclude item name",
			config: `{"exclude": [{"name": "bar"}]}`,
			assert: includes(`exclude.0.name: Does not match pattern '^[\w-]+/[\w.-]+$'`),
		},
		{
			kind:   extsvc.KindBitbucketCloud,
			desc:   "invalid additional exclude item properties",
			config: `{"exclude": [{"id": 1234, "bar": "baz"}]}`,
			assert: includes(`exclude.0: Additional property bar is not allowed`),
		},
		{
			kind: extsvc.KindBitbucketCloud,
			desc: "both name and uuid can be specified in exclude",
			config: `
			{
				"url": "https://bitbucket.org/",
				"username": "admin",
				"appPassword": "app-password",
				"exclude": [
					{"name": "foo/bar", "uuid": "{fceb73c7-cef6-4abe-956d-e471281126bc}"}
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind:   extsvc.KindBitbucketCloud,
			desc:   "invalid exclude pattern",
			config: `{"exclude": [{"pattern": "["}]}`,
			assert: includes(`exclude.0.pattern: Does not match format 'regex'`),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "valid with url, username, token, repositoryQuery",
			config: `
			{
				"url": "https://bitbucket.org/",
				"username": "admin",
				"token": "secret-token",
				"repositoryQuery": ["none"]
			}`,
			assert: equals("<nil>"),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "valid with url, username, token, repos",
			config: `
			{
				"url": "https://bitbucket.org/",
				"username": "admin",
				"token": "secret-token",
				"repos": ["sourcegraph/sourcegraph"]
			}`,
			assert: equals("<nil>"),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "without url, username, repositoryQuery nor repos",
			config: `{}`,
			assert: includes(
				"url is required",
				"username is required",
				"at least one of: repositoryQuery, projectKeys, or repos must be set",
			),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "without username",
			config: `{}`,
			assert: includes("username is required"),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "example url",
			config: `{"url": "https://bitbucket.example.com"}`,
			assert: includes("url: Must not validate the schema (not)"),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "bad url scheme",
			config: `{"url": "badscheme://bitbucket.org"}`,
			assert: includes("url: Does not match pattern '^https?://'"),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "with token AND password",
			config: `{"token": "foo", "password": "bar"}`,
			assert: includes(
				"Must validate one and only one schema (oneOf)",
				"password: Invalid type. Expected: null, given: string",
			),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "invalid token",
			config: `{"token": ""}`,
			assert: includes(`token: String length must be greater than or equal to 1`),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "invalid git url type",
			config: `{"gitURLType": "bad"}`,
			assert: includes(`gitURLType: gitURLType must be one of the following: "http", "ssh"`),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "invalid certificate",
			config: `{"certificate": ""}`,
			assert: includes("certificate: Does not match pattern '^-----BEGIN CERTIFICATE-----\n'"),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "empty repositoryQuery",
			config: `{"repositoryQuery": []}`,
			assert: includes(`repositoryQuery: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "empty repositoryQuery item",
			config: `{"repositoryQuery": [""]}`,
			assert: includes(`repositoryQuery.0: String length must be greater than or equal to 1`),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "invalid empty exclude",
			config: `{"exclude": []}`,
			assert: includes(`exclude: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "invalid empty exclude item",
			config: `{"exclude": [{}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "invalid exclude item",
			config: `{"exclude": [{"foo": "bar"}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "invalid exclude item name",
			config: `{"exclude": [{"name": "bar"}]}`,
			assert: includes(`exclude.0.name: Does not match pattern '^~?[\w-]+/[\w.-]+$'`),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "invalid additional exclude item properties",
			config: `{"exclude": [{"id": 1234, "bar": "baz"}]}`,
			assert: includes(`exclude.0: Additional property bar is not allowed`),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "both name and id can be specified in exclude",
			config: `
			{
				"url": "https://bitbucketserver.corp.com",
				"username": "admin",
				"token": "very-secret-token",
				"repositoryQuery": ["none"],
				"exclude": [
					{"name": "foo/bar", "id": 1234},
					{"pattern": "^private/.*"}
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "personal repos may be excluded",
			config: `
			{
				"url": "https://bitbucketserver.corp.com",
				"username": "admin",
				"token": "very-secret-token",
				"repositoryQuery": ["none"],
				"exclude": [
					{"name": "~FOO/bar", "id": 1234},
					{"pattern": "^private/.*"}
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "invalid empty repos",
			config: `{"repos": []}`,
			assert: includes(`repos: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindBitbucketServer,
			desc:   "invalid empty repos item",
			config: `{"repos": [""]}`,
			assert: includes(`repos.0: Does not match pattern '^~?[\w-]+/[\w.-]+$'`),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "invalid exclude pattern",
			config: `
			{
				"url": "https://bitbucketserver.corp.com",
				"username": "admin",
				"token": "very-secret-token",
				"repositoryQuery": ["none"],
				"exclude": [
					{"pattern": "["}
				]
			}`,
			assert: includes(`exclude.0.pattern: Does not match format 'regex'`),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "valid repos",
			config: `
			{
				"url": "https://bitbucketserver.corp.com",
				"username": "admin",
				"token": "very-secret-token",
				"repositoryQuery": ["none"],
				"repos": [
					"foo/bar",
					"bar/baz"
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "valid personal repos",
			config: `
			{
				"url": "https://bitbucketserver.corp.com",
				"username": "admin",
				"token": "very-secret-token",
				"repositoryQuery": ["none"],
				"repos": [
					"~FOO/bar",
					"~FOO/baz"
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "missing oauth in authorization",
			config: `
			{
				"authorization": {}
			}
			`,
			assert: includes("authorization: oauth is required"),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "missing oauth fields",
			config: `
			{
				"authorization": {
					"oauth": {},
				}
			}
			`,
			assert: includes(
				"authorization.oauth: consumerKey is required",
				"authorization.oauth: signingKey is required",
			),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "invalid oauth fields",
			config: `
			{
				"authorization": {
					"oauth": {
						"consumerKey": "",
						"signingKey": ""
					},
				}
			}
			`,
			assert: includes(
				"authorization.oauth.consumerKey: String length must be greater than or equal to 1",
				"authorization.oauth.signingKey: String length must be greater than or equal to 1",
			),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "invalid oauth signingKey",
			config: `
			{
				"authorization": {
					"oauth": {
						"consumerKey": "sourcegraph",
						"signingKey": "not-base-64-encoded"
					},
				}
			}
			`,
			assert: includes("authorization.oauth.signingKey: illegal base64 data at input byte 3"),
		},
		{
			kind: extsvc.KindBitbucketServer,
			desc: "username identity provider",
			config: fmt.Sprintf(`
			{
				"url": "https://bitbucketserver.corp.com",
				"username": "admin",
				"token": "super-secret-token",
				"repositoryQuery": ["none"],
				"authorization": {
					"identityProvider": { "type": "username" },
					"oauth": {
						"consumerKey": "sourcegraph",
						"signingKey": %q,
					},
				}
			}
			`, bogusPrivateKey),
			assert: equals("<nil>"),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "without url, token, repositoryQuery, repos nor orgs",
			config: `{}`,
			assert: includes(
				"url is required",
				"either token or GitHub App Details must be set",
				"at least one of repositoryQuery, repos, orgs, or gitHubAppDetails.cloneAllRepositories must be set",
			),
		},
		{
			kind: extsvc.KindGitHub,
			desc: "with url, token, repositoryQuery",
			config: `
			{
				"url": "https://github.corp.com",
				"token": "very-secret-token",
				"repositoryQuery": ["none"],
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind: extsvc.KindGitHub,
			desc: "with url, token, repos",
			config: `
			{
				"url": "https://github.corp.com",
				"token": "very-secret-token",
				"repos": ["sourcegraph/sourcegraph"],
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind: extsvc.KindGitHub,
			desc: "with url, token, orgs",
			config: `
			{
				"url": "https://github.corp.com",
				"token": "very-secret-token",
				"orgs": ["sourcegraph"],
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "with example.com url and badscheme",
			config: `{"url": "badscheme://github-enterprise.example.com"}`,
			assert: includes(
				"url: Must not validate the schema (not)",
				"url: Does not match pattern '^https?://'",
			),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "with invalid gitURLType",
			config: `{"gitURLType": "git"}`,
			assert: includes(`gitURLType: gitURLType must be one of the following: "http", "ssh"`),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "invalid token",
			config: `{"token": ""}`,
			assert: includes(`token: String length must be greater than or equal to 1`),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "invalid certificate",
			config: `{"certificate": ""}`,
			assert: includes("certificate: Does not match pattern '^-----BEGIN CERTIFICATE-----\n'"),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "empty repositoryQuery",
			config: `{"repositoryQuery": []}`,
			assert: includes(`repositoryQuery: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "empty repositoryQuery item",
			config: `{"repositoryQuery": [""]}`,
			assert: includes(`repositoryQuery.0: String length must be greater than or equal to 1`),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "invalid repos",
			config: `{"repos": [""]}`,
			assert: includes(`repos.0: Does not match pattern '^[\w-]+/[\w.-]+$'`),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "invalid empty exclude",
			config: `{"exclude": []}`,
			assert: includes(`exclude: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "invalid empty exclude item",
			config: `{"exclude": [{}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "invalid exclude item",
			config: `{"exclude": [{"foo": "bar"}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "invalid exclude item name",
			config: `{"exclude": [{"name": "bar"}]}`,
			assert: includes(`exclude.0.name: Does not match pattern '^[\w-]+/[\w.-]+$'`),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "invalid empty exclude item id",
			config: `{"exclude": [{"id": ""}]}`,
			assert: includes(`exclude.0.id: String length must be greater than or equal to 1`),
		},
		{
			kind:   extsvc.KindGitHub,
			desc:   "invalid additional exclude item properties",
			config: `{"exclude": [{"id": "foo", "bar": "baz"}]}`,
			assert: includes(`exclude.0: Additional property bar is not allowed`),
		},
		{
			kind: extsvc.KindGitHub,
			desc: "both name and id can be specified in exclude",
			config: `
			{
				"url": "https://github.corp.com",
				"token": "very-secret-token",
				"repositoryQuery": ["none"],
				"exclude": [
					{"name": "foo/bar", "id": "AAAAA="}
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "empty projectQuery",
			config: `{"projectQuery": []}`,
			assert: includes(`projectQuery: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "empty projectQuery item",
			config: `{"projectQuery": [""]}`,
			assert: includes(`projectQuery.0: String length must be greater than or equal to 1`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "invalid empty exclude item",
			config: `{"exclude": [{}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "invalid exclude item",
			config: `{"exclude": [{"foo": "bar"}]}`,
			assert: includes(`exclude.0: Must validate at least one schema (anyOf)`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "invalid exclude item name",
			config: `{"exclude": [{"name": "bar"}]}`,
			assert: includes(`exclude.0.name: Does not match pattern '^[\w.-]+(/[\w.-]+)+$'`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "invalid additional exclude item properties",
			config: `{"exclude": [{"id": 1234, "bar": "baz"}]}`,
			assert: includes(`exclude.0: Additional property bar is not allowed`),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "both name and id can be specified in exclude",
			config: `
			{
				"url": "https://gitlab.corp.com",
				"token": "very-secret-token",
				"projectQuery": ["none"],
				"exclude": [
					{"name": "foo/bar", "id": 1234}
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "subgroup paths are valid for exclude",
			config: `
			{
				"url": "https://gitlab.corp.com",
				"token": "very-secret-token",
				"projectQuery": ["none"],
				"exclude": [
					{"name": "foo/bar/baz", "id": 1234}
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "paths containing . in the first part of the path are valid for exclude",
			config: `
			{
				"url": "https://gitlab.corp.com",
				"token": "very-secret-token",
				"projectQuery": ["none"],
				"exclude": [
					{"name": "foo.bar/baz", "id": 1234}
				]
			}`,
			assert: equals(`<nil>`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "invalid empty projects",
			config: `{"projects": []}`,
			assert: includes(`projects: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "invalid empty projects item",
			config: `{"projects": [{}]}`,
			assert: includes(`projects.0: Must validate one and only one schema (oneOf)`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "invalid projects item",
			config: `{"projects": [{"foo": "bar"}]}`,
			assert: includes(`projects.0: Must validate one and only one schema (oneOf)`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "invalid projects item name",
			config: `{"projects": [{"name": "bar"}]}`,
			assert: includes(`projects.0.name: Does not match pattern '^[\w.-]+(/[\w.-]+)+$'`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "invalid additional projects item properties",
			config: `{"projects": [{"id": 1234, "bar": "baz"}]}`,
			assert: includes(`projects.0: Additional property bar is not allowed`),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "both name and id cannot be specified in projects",
			config: `
			{
				"url": "https://gitlab.corp.com",
				"token": "very-secret-token",
				"projectQuery": ["none"],
				"projects": [
					{"name": "foo/bar", "id": 1234}
				]
			}`,
			assert: includes(`projects.0: Must validate one and only one schema (oneOf)`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "without url, token nor projectQuery",
			config: `{}`,
			assert: includes(
				"url is required",
				"token is required",
				"projectQuery is required",
			),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "with example.com url and badscheme",
			config: `{"url": "badscheme://github-enterprise.example.com"}`,
			assert: includes(
				"url: Must not validate the schema (not)",
				"url: Does not match pattern '^https?://'",
			),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "with invalid gitURLType",
			config: `{"gitURLType": "git"}`,
			assert: includes(`gitURLType: gitURLType must be one of the following: "http", "ssh"`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "invalid token",
			config: `{"token": ""}`,
			assert: includes(`token: String length must be greater than or equal to 1`),
		},
		{
			kind:   extsvc.KindGitLab,
			desc:   "invalid certificate",
			config: `{"certificate": ""}`,
			assert: includes("certificate: Does not match pattern '^-----BEGIN CERTIFICATE-----\n'"),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "missing oauth provider",
			config: `
			{
				"url": "https://gitlab.foo.bar",
				"authorization": { "identityProvider": { "type": "oauth" } }
			}
			`,
			assert: includes("Did not find authentication provider matching \"https://gitlab.foo.bar\". Check the [**site configuration**](/site-admin/configuration) to verify an entry in [`auth.providers`](https://sourcegraph.com/docs/admin/auth) exists for https://gitlab.foo.bar."),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "valid oauth provider",
			config: `
			{
				"url": "https://gitlab.foo.bar",
				"authorization": { "identityProvider": { "type": "oauth" } }
			}
			`,
			ps: []schema.AuthProviders{
				{Gitlab: &schema.GitLabAuthProvider{Url: "https://gitlab.foo.bar"}},
			},
			assert: excludes("Did not find authentication provider matching \"https://gitlab.foo.bar\". Check the [**site configuration**](/site-admin/configuration) to verify an entry in [`auth.providers`](https://sourcegraph.com/docs/admin/auth) exists for https://gitlab.foo.bar."),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "missing external provider",
			config: `
			{
				"url": "https://gitlab.foo.bar",
				"authorization": {
					"identityProvider": {
						"type": "external",
						"authProviderID": "foo",
						"authProviderType": "bar",
						"gitlabProvider": "baz"
					}
				}
			}
			`,
			assert: includes("Did not find authentication provider matching type bar and configID foo. Check the [**site configuration**](/site-admin/configuration) to verify that an entry in [`auth.providers`](https://sourcegraph.com/docs/admin/auth) matches the type and configID."),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "valid external provider with SAML",
			config: `
			{
				"url": "https://gitlab.foo.bar",
				"authorization": {
					"identityProvider": {
						"type": "external",
						"authProviderID": "foo",
						"authProviderType": "bar",
						"gitlabProvider": "baz"
					}
				}
			}
			`,
			ps: []schema.AuthProviders{
				{
					Saml: &schema.SAMLAuthProvider{
						ConfigID: "foo",
						Type:     "bar",
					},
				},
			},
			assert: excludes("Did not find authentication provider matching type bar and configID foo. Check the [**site configuration**](/site-admin/configuration) to verify that an entry in [`auth.providers`](https://sourcegraph.com/docs/admin/auth) matches the type and configID."),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "valid external provider with OIDC",
			config: `
			{
				"url": "https://gitlab.foo.bar",
				"authorization": {
					"identityProvider": {
						"type": "external",
						"authProviderID": "foo",
						"authProviderType": "bar",
						"gitlabProvider": "baz"
					}
				}
			}
			`,
			ps: []schema.AuthProviders{
				{
					Openidconnect: &schema.OpenIDConnectAuthProvider{
						ConfigID: "foo",
						Type:     "bar",
					},
				},
			},
			assert: excludes("Did not find authentication provider matching type bar and configID foo. Check the [**site configuration**](/site-admin/configuration) to verify that an entry in [`auth.providers`](https://sourcegraph.com/docs/admin/auth) matches the type and configID."),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "username identity provider",
			config: `
			{
				"url": "https://gitlab.foo.bar",
				"token": "super-secret-token",
				"projectQuery": ["none"],
				"authorization": {
					"identityProvider": {
						"type": "username",
					}
				}
			}
			`,
			assert: equals("<nil>"),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "missing properties in name transformations",
			config: `
			{
				"nameTransformations": [
					{
						"re": "regex",
						"repl": "replacement"
					}
				]
			}
			`,
			assert: includes(
				`nameTransformations.0: regex is required`,
				`nameTransformations.0: replacement is required`,
			),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "invalid properties in name transformations",
			config: `
			{
				"nameTransformations": [
					{
						"regex": "[",
						"replacement": ""
					}
				]
			}
			`,
			assert: includes(`nameTransformations.0.regex: Does not match format 'regex'`),
		},
		{
			kind: extsvc.KindGitLab,
			desc: "valid name transformations",
			config: `
			{
				"url": "https://gitlab.foo.bar",
				"token": "super-secret-token",
				"projectQuery": ["none"],
				"nameTransformations": [
					{
						"regex": "\\.d/",
						"replacement": "/"
					},
					{
						"regex": "-git$",
						"replacement": ""
					}
				]
			}
			`,
			assert: equals("<nil>"),
		},
		{
			kind:   extsvc.KindPerforce,
			desc:   "without p4.port, p4.user, p4.passwd",
			config: `{}`,
			assert: includes(
				`p4.port is required`,
				`p4.user is required`,
				`p4.passwd is required`,
			),
		},
		{
			kind: extsvc.KindPerforce,
			desc: "invalid depot path",
			config: `
			{
				"p4.port": "ssl:111.222.333.444:1666",
				"p4.user": "admin",
				"p4.passwd": "<secure password>",
				"depots": ["//abc", "abc/", "//abc/"]
			}
`,
			assert: includes(
				`depots.0: Does not match pattern '^\/[\/\S]+\/$'`,
				`depots.1: Does not match pattern '^\/[\/\S]+\/$'`,
			),
		},
		{
			kind: extsvc.KindPerforce,
			desc: "invalid ticket",
			config: `
			{
				"p4.port": "ssl:111.222.333.444:1666",
				"p4.user": "admin",
				"p4.passwd": "perforce-server:1666=admin:6211C5E719EDE6925855039E8F5CC3D2",
				"depots": []
			}
`,
			assert: includes(
				"p4.passwd must not contain a colon. It must be the ticket generated by `p4 login -p`, not a full ticket from the `.p4tickets` file.",
			),
		},
		{
			kind:   extsvc.KindPhabricator,
			desc:   "without repos nor token",
			config: `{}`,
			assert: includes(
				`Must validate at least one schema (anyOf)`,
				`token is required`,
			),
		},
		{
			kind:   extsvc.KindPhabricator,
			desc:   "with empty repos",
			config: `{"repos": []}`,
			assert: includes(`repos: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindPhabricator,
			desc:   "with repos",
			config: `{"repos": [{"path": "gitolite/my/repo", "callsign": "MUX"}]}`,
			assert: equals(`<nil>`),
		},
		{
			kind:   extsvc.KindPhabricator,
			desc:   "invalid token",
			config: `{"token": ""}`,
			assert: includes(`token: String length must be greater than or equal to 1`),
		},
		{
			kind:   extsvc.KindPhabricator,
			desc:   "with token",
			config: `{"token": "a given token"}`,
			assert: equals(`<nil>`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "without url nor repos array",
			config: `{}`,
			assert: includes(`repos is required`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "without URL but with null repos array",
			config: `{"repos": null}`,
			assert: includes(`repos: Invalid type. Expected: array, given: null`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "without URL but with empty repos array",
			config: `{"repos": []}`,
			assert: excludes(`repos: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "without URL and empty repo array item",
			config: `{"repos": [""]}`,
			assert: includes(`repos.0: String length must be greater than or equal to 1`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "without URL and invalid repo array item",
			config: `{"repos": ["https://github.com/%%%%malformed"]}`,
			assert: includes(`repos.0: Does not match format 'uri-reference'`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "without URL and invalid scheme in repo array item",
			config: `{"repos": ["badscheme://github.com/my/repo"]}`,
			assert: includes(`repos.0: scheme "badscheme" not one of git, http, https or ssh`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "without URL and valid repos",
			config: `{"repos": ["http://git.hub/repo", "https://git.hub/repo", "git://user@hub.com:3233/repo.git/", "ssh://user@hub.com/repo.git/"]}`,
			assert: equals("<nil>"),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "with URL but null repos array",
			config: `{"url": "http://github.com/", "repos": null}`,
			assert: includes(`repos: Invalid type. Expected: array, given: null`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "with URL but empty repos array",
			config: `{"url": "http://github.com/", "repos": []}`,
			assert: excludes(`repos: Array must have at least 1 items`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "with URL and empty repo array item",
			config: `{"url": "http://github.com/", "repos": [""]}`,
			assert: includes(`repos.0: String length must be greater than or equal to 1`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "with URL and invalid repo array item",
			config: `{"url": "https://github.com/", "repos": ["foo/%%%%malformed"]}`,
			assert: includes(`repos.0: Does not match format 'uri-reference'`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "with invalid scheme URL",
			config: `{"url": "badscheme://github.com/", "repos": ["my/repo"]}`,
			assert: includes(`url: Does not match pattern '^(git|ssh|https?)://'`),
		},
		{
			kind:   extsvc.KindOther,
			desc:   "with URL and valid repos",
			config: `{"url": "https://github.com/", "repos": ["foo/", "bar", "/baz", "bam.git"]}`,
			assert: equals("<nil>"),
		},
	} {
		tc := tc
		t.Run(tc.kind+"/"+tc.desc, func(t *testing.T) {
			var have []string
			if tc.ps == nil {
				tc.ps = conf.Get().AuthProviders
			}

			_, err := ValidateExternalServiceConfig(context.Background(), dbmocks.NewMockDB(), database.ValidateExternalServiceConfigOptions{
				Kind:          tc.kind,
				Config:        tc.config,
				AuthProviders: tc.ps,
			})
			if err == nil {
				have = append(have, "<nil>")
			} else {
				var errs errors.MultiError
				if errors.As(err, &errs) {
					for _, err := range errs.Errors() {
						have = append(have, err.Error())
					}
				} else {
					have = append(have, err.Error())
				}
			}

			tc.assert(t, have)
		})
	}
}
