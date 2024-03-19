package gitlaboauth

import (
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestParseConfig(t *testing.T) {
	logger := logtest.Scoped(t)
	spew.Config.DisablePointerAddresses = true
	spew.Config.SortKeys = true
	spew.Config.SpewKeys = true

	db := database.NewDB(logger, dbtest.NewDB(t))

	type args struct {
		cfg *conf.Unified
	}
	tests := []struct {
		name          string
		args          args
		dotcom        bool
		wantProviders []Provider
		wantProblems  []string
	}{
		{
			name:          "No configs",
			args:          args{cfg: &conf.Unified{}},
			wantProviders: []Provider(nil),
		},
		{
			name: "1 GitLab.com config",
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				ExternalURL: "https://sourcegraph.example.com",
				AuthProviders: []schema.AuthProviders{{
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
						AllowGroups:  []string{"mygroup"},
					},
				}},
			}}},
			wantProviders: []Provider{
				{
					GitLabAuthProvider: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
						AllowGroups:  []string{"mygroup"},
					},
					Provider: provider("https://gitlab.com/", oauth2.Config{
						RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://gitlab.com/oauth/authorize",
							TokenURL: "https://gitlab.com/oauth/token",
						},
						Scopes: []string{"read_user", "api"},
					}),
				},
			},
		},
		{
			name: "1 GitLab.com config with scope override",
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				ExternalURL: "https://sourcegraph.example.com",
				AuthProviders: []schema.AuthProviders{{
					Gitlab: &schema.GitLabAuthProvider{
						ApiScope:     "read_api",
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
				}},
			}}},
			wantProviders: []Provider{
				{
					GitLabAuthProvider: &schema.GitLabAuthProvider{
						ApiScope:     "read_api",
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
					Provider: provider("https://gitlab.com/", oauth2.Config{
						RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://gitlab.com/oauth/authorize",
							TokenURL: "https://gitlab.com/oauth/token",
						},
						Scopes: []string{"read_user", "read_api"},
					}),
				},
			},
		},
		{
			name:   "1 GitLab.com config, Sourcegraph.com",
			dotcom: true,
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				ExternalURL: "https://sourcegraph.example.com",
				AuthProviders: []schema.AuthProviders{{
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
				}},
			}}},
			wantProviders: []Provider{
				{
					GitLabAuthProvider: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
					Provider: provider("https://gitlab.com/", oauth2.Config{
						RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://gitlab.com/oauth/authorize",
							TokenURL: "https://gitlab.com/oauth/token",
						},
						Scopes: []string{"read_user", "api"},
					}),
				},
			},
		},
		{
			name: "2 GitLab configs",
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				ExternalURL: "https://sourcegraph.example.com",
				AuthProviders: []schema.AuthProviders{{
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
				}, {
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id-2",
						ClientSecret: "my-client-secret-2",
						DisplayName:  "GitLab Enterprise",
						Type:         extsvc.TypeGitLab,
						Url:          "https://mycompany.com",
					},
				}},
			}}},
			wantProviders: []Provider{
				{
					GitLabAuthProvider: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
					Provider: provider("https://gitlab.com/", oauth2.Config{
						RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://gitlab.com/oauth/authorize",
							TokenURL: "https://gitlab.com/oauth/token",
						},
						Scopes: []string{"read_user", "api"},
					}),
				},
				{
					GitLabAuthProvider: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id-2",
						ClientSecret: "my-client-secret-2",
						DisplayName:  "GitLab Enterprise",
						Type:         extsvc.TypeGitLab,
						Url:          "https://mycompany.com",
					},
					Provider: provider("https://mycompany.com/", oauth2.Config{
						RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
						ClientID:     "my-client-id-2",
						ClientSecret: "my-client-secret-2",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://mycompany.com/oauth/authorize",
							TokenURL: "https://mycompany.com/oauth/token",
						},
						Scopes: []string{"read_user", "api"},
					}),
				},
			},
		},
		{
			name: "2 GitLab configs with the same URL and client ID",
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				ExternalURL: "https://sourcegraph.example.com",
				AuthProviders: []schema.AuthProviders{{
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
				}, {
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret-2",
						DisplayName:  "GitLab Duplicate",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
				}},
			}}},
			wantProviders: []Provider{
				{
					GitLabAuthProvider: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
					Provider: provider("https://gitlab.com/", oauth2.Config{
						RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://gitlab.com/oauth/authorize",
							TokenURL: "https://gitlab.com/oauth/token",
						},
						Scopes: []string{"read_user", "api"},
					}),
				},
			},
			wantProblems: []string{
				`Cannot have more than one GitLab auth provider with url "https://gitlab.com/" and client ID "my-client-id", only the first one will be used`,
			},
		},
		{
			name: "2 GitLab configs with the same URL but different client IDs",
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				ExternalURL: "https://sourcegraph.example.com",
				AuthProviders: []schema.AuthProviders{{
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
				}, {
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id-2",
						ClientSecret: "my-client-secret-2",
						DisplayName:  "GitLab Duplicate",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
				}},
			}}},
			wantProviders: []Provider{
				{
					GitLabAuthProvider: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
					Provider: provider("https://gitlab.com/", oauth2.Config{
						RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://gitlab.com/oauth/authorize",
							TokenURL: "https://gitlab.com/oauth/token",
						},
						Scopes: []string{"read_user", "api"},
					}),
				},
				{
					GitLabAuthProvider: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id-2",
						ClientSecret: "my-client-secret-2",
						DisplayName:  "GitLab Duplicate",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
					Provider: provider("https://gitlab.com/", oauth2.Config{
						RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
						ClientID:     "my-client-id-2",
						ClientSecret: "my-client-secret-2",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://gitlab.com/oauth/authorize",
							TokenURL: "https://gitlab.com/oauth/token",
						},
						Scopes: []string{"read_user", "api"},
					}),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dotcom.MockSourcegraphDotComMode(t, tt.dotcom)

			gotProviders, gotProblems := parseConfig(logtest.Scoped(t), tt.args.cfg, db)
			gotConfigs := make([]oauth2.Config, len(gotProviders))
			for k, p := range gotProviders {
				if p, ok := p.Provider.(*oauth.Provider); ok {
					p.Login, p.Callback = nil, nil
					gotConfigs[k] = p.OAuth2Config()
					p.OAuth2Config = nil
					p.ProviderOp.Login, p.ProviderOp.Callback = nil, nil
				}
			}
			wantConfigs := make([]oauth2.Config, len(tt.wantProviders))
			for k, p := range tt.wantProviders {
				k := k
				if q, ok := p.Provider.(*oauth.Provider); ok {
					q.SourceConfig = schema.AuthProviders{Gitlab: p.GitLabAuthProvider}
					wantConfigs[k] = q.OAuth2Config()
					q.OAuth2Config = nil
				}
			}
			if !reflect.DeepEqual(gotProviders, tt.wantProviders) {
				dmp := diffmatchpatch.New()
				t.Errorf("parseConfig() gotProviders != tt.wantProviders, diff:\n%s",
					dmp.DiffPrettyText(dmp.DiffMain(spew.Sdump(tt.wantProviders), spew.Sdump(gotProviders), false)),
				)
			}
			if !reflect.DeepEqual(gotProblems.Messages(), tt.wantProblems) {
				t.Errorf("parseConfig() gotProblems = %v, want %v", gotProblems, tt.wantProblems)
			}

			if !reflect.DeepEqual(gotConfigs, wantConfigs) {
				dmp := diffmatchpatch.New()
				t.Errorf("parseConfig() gotConfigs != wantConfigs, diff:\n%s",
					dmp.DiffPrettyText(dmp.DiffMain(spew.Sdump(gotConfigs), spew.Sdump(wantConfigs), false)),
				)
			}
		})
	}
}

func provider(serviceID string, oauth2Config oauth2.Config) *oauth.Provider {
	op := oauth.ProviderOp{
		AuthPrefix:   authPrefix,
		OAuth2Config: func() oauth2.Config { return oauth2Config },
		ServiceID:    serviceID,
		ServiceType:  extsvc.TypeGitLab,
	}
	return &oauth.Provider{ProviderOp: op}
}
