package bitbucketserveroauth

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestParseConfig(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))

	type args struct {
		cfg *conf.Unified
	}
	tests := []struct {
		name          string
		args          args
		wantProviders []Provider
		wantProblems  []string
	}{
		{
			name:          "No configs",
			args:          args{cfg: &conf.Unified{}},
			wantProviders: []Provider(nil),
		},
		{
			name: "1 Bitbucket Cloud config",
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				AuthProviders: []schema.AuthProviders{{
					Bitbucketcloud: &schema.BitbucketCloudAuthProvider{
						ClientKey:    "myclientid",
						ClientSecret: "myclientsecret",
						DisplayName:  "Bitbucket Cloud",
						Type:         extsvc.TypeBitbucketCloud,
						Url:          "https://bitbucket.org",
						ApiScope:     "account,email",
					},
				}},
			}}},
			wantProviders: []Provider{
				{
					BitbucketCloudAuthProvider: &schema.BitbucketCloudAuthProvider{
						ClientKey:    "myclientid",
						ClientSecret: "myclientsecret",
						DisplayName:  "Bitbucket Cloud",
						Type:         extsvc.TypeBitbucketCloud,
						Url:          "https://bitbucket.org",
						ApiScope:     "account,email",
					},
					Provider: provider("https://bitbucket.org/", oauth2.Config{
						ClientID:     "myclientid",
						ClientSecret: "myclientsecret",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://bitbucket.org/site/oauth2/authorize",
							TokenURL: "https://bitbucket.org/site/oauth2/access_token",
						},
						Scopes: []string{"account", "email"},
					}),
				},
			},
		},
		{
			name: "2 Bitbucket Cloud configs with the same Url and client IDs",
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				AuthProviders: []schema.AuthProviders{{
					Bitbucketcloud: &schema.BitbucketCloudAuthProvider{
						ClientKey:    "myclientid",
						ClientSecret: "myclientsecret",
						DisplayName:  "Bitbucket Cloud",
						Type:         extsvc.TypeBitbucketCloud,
						Url:          "https://bitbucket.org",
						ApiScope:     "account,email",
					},
				}, {
					Bitbucketcloud: &schema.BitbucketCloudAuthProvider{
						ClientKey:    "myclientid",
						ClientSecret: "myclientsecret2",
						DisplayName:  "Bitbucket Cloud Duplicate",
						Type:         extsvc.TypeBitbucketCloud,
						Url:          "https://bitbucket.org",
						ApiScope:     "account,email",
					},
				}},
			}}},
			wantProviders: []Provider{
				{
					BitbucketCloudAuthProvider: &schema.BitbucketCloudAuthProvider{
						ClientKey:    "myclientid",
						ClientSecret: "myclientsecret",
						DisplayName:  "Bitbucket Cloud",
						Type:         extsvc.TypeBitbucketCloud,
						Url:          "https://bitbucket.org",
						ApiScope:     "account,email",
					},
					Provider: provider("https://bitbucket.org/", oauth2.Config{
						ClientID:     "myclientid",
						ClientSecret: "myclientsecret",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://bitbucket.org/site/oauth2/authorize",
							TokenURL: "https://bitbucket.org/site/oauth2/access_token",
						},
						Scopes: []string{"account", "email"},
					}),
				},
			},
			wantProblems: []string{
				`Cannot have more than one Bitbucket Cloud auth provider with url "https://bitbucket.org/" and client ID "myclientid", only the first one will be used`,
			},
		},
		{
			name: "2 Bitbucket Cloud configs with the same Url but different client IDs",
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				AuthProviders: []schema.AuthProviders{{
					Bitbucketcloud: &schema.BitbucketCloudAuthProvider{
						ClientKey:    "myclientid",
						ClientSecret: "myclientsecret",
						DisplayName:  "Bitbucket Cloud",
						Type:         extsvc.TypeBitbucketCloud,
						Url:          "https://bitbucket.org",
						ApiScope:     "account,email",
					},
				}, {
					Bitbucketcloud: &schema.BitbucketCloudAuthProvider{
						ClientKey:    "myclientid2",
						ClientSecret: "myclientsecret2",
						DisplayName:  "Bitbucket Cloud Duplicate",
						Type:         extsvc.TypeBitbucketCloud,
						Url:          "https://bitbucket.org",
						ApiScope:     "account,email",
					},
				}},
			}}},
			wantProviders: []Provider{
				{
					BitbucketCloudAuthProvider: &schema.BitbucketCloudAuthProvider{
						ClientKey:    "myclientid",
						ClientSecret: "myclientsecret",
						DisplayName:  "Bitbucket Cloud",
						Type:         extsvc.TypeBitbucketCloud,
						Url:          "https://bitbucket.org",
						ApiScope:     "account,email",
					},
					Provider: provider("https://bitbucket.org/", oauth2.Config{
						ClientID:     "myclientid",
						ClientSecret: "myclientsecret",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://bitbucket.org/site/oauth2/authorize",
							TokenURL: "https://bitbucket.org/site/oauth2/access_token",
						},
						Scopes: []string{"account", "email"},
					}),
				},
				{
					BitbucketCloudAuthProvider: &schema.BitbucketCloudAuthProvider{
						ClientKey:    "myclientid2",
						ClientSecret: "myclientsecret2",
						DisplayName:  "Bitbucket Cloud Duplicate",
						Type:         extsvc.TypeBitbucketCloud,
						Url:          "https://bitbucket.org",
						ApiScope:     "account,email",
					},
					Provider: provider("https://bitbucket.org/", oauth2.Config{
						ClientID:     "myclientid2",
						ClientSecret: "myclientsecret2",
						Endpoint: oauth2.Endpoint{
							AuthURL:  "https://bitbucket.org/site/oauth2/authorize",
							TokenURL: "https://bitbucket.org/site/oauth2/access_token",
						},
						Scopes: []string{"account", "email"},
					}),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
					q.SourceConfig = schema.AuthProviders{Bitbucketcloud: p.BitbucketCloudAuthProvider}
					wantConfigs[k] = q.OAuth2Config()
					q.OAuth2Config = nil
				}
			}
			if diff := cmp.Diff(tt.wantProviders, gotProviders); diff != "" {
				t.Errorf("providers: %s", diff)
			}
			if diff := cmp.Diff(tt.wantProblems, gotProblems.Messages()); diff != "" {
				t.Errorf("problems: %s", diff)
			}
			if diff := cmp.Diff(wantConfigs, gotConfigs, cmpopts.IgnoreUnexported(oauth2.Config{})); diff != "" {
				t.Errorf("problems: %s", diff)
			}
		})
	}
}

func provider(serviceID string, oauth2Config oauth2.Config) *oauth.Provider {
	op := oauth.ProviderOp{
		AuthPrefix:   authPrefix,
		OAuth2Config: func() oauth2.Config { return oauth2Config },
		ServiceID:    serviceID,
		ServiceType:  extsvc.TypeBitbucketCloud,
	}
	return &oauth.Provider{ProviderOp: op}
}
