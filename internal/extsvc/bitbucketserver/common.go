package bitbucketserver

import (
	"net/url"
	"strings"

	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/oauthutil"
)

var MockGetOAuthContext func() *oauthutil.OAuthContext

func GetOAuthContext(baseURL string) *oauthutil.OAuthContext {
	if MockGetOAuthContext != nil {
		return MockGetOAuthContext()
	}

	for _, authProvider := range conf.SiteConfig().AuthProviders {
		if authProvider.Bitbucketserver != nil {
			p := authProvider.Bitbucketserver
			rawURL := p.Url
			rawURL = strings.TrimSuffix(rawURL, "/")
			if !strings.HasPrefix(baseURL, rawURL) {
				continue
			}
			authURL, err := url.JoinPath(rawURL, "/rest/oauth2/latest/authorize")
			if err != nil {
				continue
			}
			tokenURL, err := url.JoinPath(rawURL, "/rest/oauth2/latest/token")
			if err != nil {
				continue
			}

			scopes := []string{"REPO_READ"}
			if p.ApiScope != "" {
				scopes = strings.Split(p.ApiScope, ",")
			}

			return &oauthutil.OAuthContext{
				ClientID:     p.ClientKey,
				ClientSecret: p.ClientSecret,
				Scopes:       scopes,
				Endpoint: oauth2.Endpoint{
					AuthURL:  authURL,
					TokenURL: tokenURL,
				},
			}
		}
	}
	return nil
}
