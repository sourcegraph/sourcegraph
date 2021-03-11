package gitlaboauth

import (
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestParseConfig(t *testing.T) {
	spew.Config.DisablePointerAddresses = true
	spew.Config.SortKeys = true
	spew.Config.SpewKeys = true

	type args struct {
		cfg *conf.Unified
	}
	tests := []struct {
		name          string
		args          args
		wantProviders map[schema.GitLabAuthProvider]providers.Provider
		wantProblems  []string
	}{
		{
			name:          "No configs",
			args:          args{cfg: &conf.Unified{}},
			wantProviders: map[schema.GitLabAuthProvider]providers.Provider{},
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
					},
				}},
			}}},
			wantProviders: map[schema.GitLabAuthProvider]providers.Provider{
				{
					ClientID:     "my-client-id",
					ClientSecret: "my-client-secret",
					DisplayName:  "GitLab",
					Type:         extsvc.TypeGitLab,
					Url:          "https://gitlab.com",
				}: provider("https://gitlab.com/", oauth2.Config{
					RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
					ClientID:     "my-client-id",
					ClientSecret: "my-client-secret",
					Endpoint: oauth2.Endpoint{
						AuthURL:  "https://gitlab.com/oauth/authorize",
						TokenURL: "https://gitlab.com/oauth/token",
					},
					Scopes: []string{"api", "read_user"},
				}),
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
			wantProviders: map[schema.GitLabAuthProvider]providers.Provider{
				{
					ClientID:     "my-client-id",
					ClientSecret: "my-client-secret",
					DisplayName:  "GitLab",
					Type:         extsvc.TypeGitLab,
					Url:          "https://gitlab.com",
				}: provider("https://gitlab.com/", oauth2.Config{
					RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
					ClientID:     "my-client-id",
					ClientSecret: "my-client-secret",
					Endpoint: oauth2.Endpoint{
						AuthURL:  "https://gitlab.com/oauth/authorize",
						TokenURL: "https://gitlab.com/oauth/token",
					},
					Scopes: []string{"api", "read_user"},
				}),
				{
					ClientID:     "my-client-id-2",
					ClientSecret: "my-client-secret-2",
					DisplayName:  "GitLab Enterprise",
					Type:         extsvc.TypeGitLab,
					Url:          "https://mycompany.com",
				}: provider("https://mycompany.com/", oauth2.Config{
					RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
					ClientID:     "my-client-id-2",
					ClientSecret: "my-client-secret-2",
					Endpoint: oauth2.Endpoint{
						AuthURL:  "https://mycompany.com/oauth/authorize",
						TokenURL: "https://mycompany.com/oauth/token",
					},
					Scopes: []string{"api", "read_user"},
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProviders, gotProblems := parseConfig(tt.args.cfg)
			for _, p := range gotProviders {
				if p, ok := p.(*oauth.Provider); ok {
					p.Login, p.Callback = nil, nil
					p.ProviderOp.Login, p.ProviderOp.Callback = nil, nil
				}
			}
			for k, p := range tt.wantProviders {
				k := k
				if q, ok := p.(*oauth.Provider); ok {
					q.SourceConfig = schema.AuthProviders{Gitlab: &k}
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
		})
	}
}

func provider(serviceID string, oauth2Config oauth2.Config) *oauth.Provider {
	op := oauth.ProviderOp{
		AuthPrefix:   authPrefix,
		OAuth2Config: oauth2Config,
		StateConfig:  getStateConfig(),
		ServiceID:    serviceID,
		ServiceType:  extsvc.TypeGitLab,
	}
	return &oauth.Provider{ProviderOp: op}
}
