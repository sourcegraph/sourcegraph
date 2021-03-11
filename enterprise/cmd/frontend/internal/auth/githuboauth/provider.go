package githuboauth

import (
	"fmt"
	"net/url"

	"github.com/dghubble/gologin/github"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/schema"
)

const sessionKey = "githuboauth@0"

func parseProvider(p *schema.GitHubAuthProvider, sourceCfg schema.AuthProviders) (provider *oauth.Provider, messages []string) {
	rawURL := p.Url
	if rawURL == "" {
		rawURL = "https://github.com/"
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		messages = append(messages, fmt.Sprintf("Could not parse GitHub URL %q. You will not be able to login via this GitHub instance.", rawURL))
		return nil, messages
	}
	if !validateClientIDAndSecret(p.ClientID) {
		messages = append(messages, "GitHub client ID contains unexpected characters, possibly hidden")
	}
	if !validateClientIDAndSecret(p.ClientSecret) {
		messages = append(messages, "GitHub client secret contains unexpected characters, possibly hidden")
	}
	codeHost := extsvc.NewCodeHost(parsedURL, extsvc.TypeGitHub)
	oauth2Cfg := oauth2.Config{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		Scopes:       requestedScopes(p),
		Endpoint: oauth2.Endpoint{
			AuthURL:  codeHost.BaseURL.ResolveReference(&url.URL{Path: "/login/oauth/authorize"}).String(),
			TokenURL: codeHost.BaseURL.ResolveReference(&url.URL{Path: "/login/oauth/access_token"}).String(),
		},
	}
	return oauth.NewProvider(oauth.ProviderOp{
		AuthPrefix:   authPrefix,
		OAuth2Config: oauth2Cfg,
		SourceConfig: sourceCfg,
		StateConfig:  getStateConfig(),
		ServiceID:    codeHost.ServiceID,
		ServiceType:  codeHost.ServiceType,
		Login:        github.LoginHandler(&oauth2Cfg, nil),
		Callback: github.CallbackHandler(
			&oauth2Cfg,
			oauth.SessionIssuer(&sessionIssuerHelper{
				CodeHost:    codeHost,
				clientID:    p.ClientID,
				allowSignup: p.AllowSignup,
				allowOrgs:   p.AllowOrgs,
			}, sessionKey),
			nil,
		),
	}), messages
}

var clientIDSecretValidator = lazyregexp.New("^[a-z0-9]*$")

func validateClientIDAndSecret(clientIDOrSecret string) (valid bool) {
	return clientIDSecretValidator.MatchString(clientIDOrSecret)
}

func requestedScopes(p *schema.GitHubAuthProvider) []string {
	scopes := []string{"user:email"}
	if !envvar.SourcegraphDotComMode() {
		scopes = append(scopes, "repo")
	}

	// Needs extra scope to check organization membership
	if len(p.AllowOrgs) > 0 {
		scopes = append(scopes, "read:org")
	}

	return scopes
}
