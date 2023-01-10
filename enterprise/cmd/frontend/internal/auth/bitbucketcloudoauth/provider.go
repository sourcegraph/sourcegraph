package bitbucketcloudoauth

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/dghubble/gologin"
	"github.com/dghubble/gologin/bitbucket"
	goauth2 "github.com/dghubble/gologin/oauth2"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/schema"
)

const sessionKey = "bitbucketcloudoauth@0"

func parseProvider(logger log.Logger, p *schema.BitbucketCloudAuthProvider, db database.DB, sourceCfg schema.AuthProviders) (provider *oauth.Provider, messages []string) {
	rawURL := p.Url
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		messages = append(messages, fmt.Sprintf("Could not parse Bitbucket Cloud URL %q. You will not be able to login via Bitbucket Cloud.", rawURL))
		return nil, messages
	}
	if !validateClientKeyOrSecret(p.ClientKey) {
		messages = append(messages, "Bitbucket Cloud key contains unexpected characters, possibly hidden")
	}
	if !validateClientKeyOrSecret(p.ClientSecret) {
		messages = append(messages, "Bitbucket Cloud secret contains unexpected characters, possibly hidden")
	}
	codeHost := extsvc.NewCodeHost(parsedURL, extsvc.TypeBitbucketCloud)

	return oauth.NewProvider(oauth.ProviderOp{
		AuthPrefix: authPrefix,
		OAuth2Config: func() oauth2.Config {
			return oauth2.Config{
				ClientID:     p.ClientKey,
				ClientSecret: p.ClientSecret,
				Scopes:       requestedScopes(),
				Endpoint: oauth2.Endpoint{
					AuthURL:  codeHost.BaseURL.ResolveReference(&url.URL{Path: "/site/oauth2/authorize"}).String(),
					TokenURL: codeHost.BaseURL.ResolveReference(&url.URL{Path: "/site/oauth2/access_token"}).String(),
				},
			}
		},
		SourceConfig: sourceCfg,
		StateConfig:  getStateConfig(),
		ServiceID:    codeHost.ServiceID,
		ServiceType:  codeHost.ServiceType,
		Login: func(oauth2Cfg oauth2.Config) http.Handler {
			return bitbucket.LoginHandler(&oauth2Cfg, nil)
		},
		Callback: func(oauth2Cfg oauth2.Config) http.Handler {
			return bitbucket.CallbackHandler(
				&oauth2Cfg,
				oauth.SessionIssuer(logger, db, &sessionIssuerHelper{
					CodeHost:    codeHost,
					db:          db,
					clientKey:   p.ClientKey,
					allowSignup: p.AllowSignup,
				}, sessionKey),
				http.HandlerFunc(failureHandler),
			)
		},
	}), messages
}

func failureHandler(w http.ResponseWriter, r *http.Request) {
	// As a special case wa want to handle `access_denied` errors by redirecting
	// back. This case arises when the user decides not to proceed by clicking `cancel`.
	if err := r.URL.Query().Get("error"); err != "access_denied" {
		// Fall back to default failure handler
		gologin.DefaultFailureHandler.ServeHTTP(w, r)
		return
	}

	ctx := r.Context()
	encodedState, err := goauth2.StateFromContext(ctx)
	if err != nil {
		http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: could not get OAuth state from context.", http.StatusInternalServerError)
		return
	}
	state, err := oauth.DecodeState(encodedState)
	if err != nil {
		http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: could not get decode OAuth state.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, auth.SafeRedirectURL(state.Redirect), http.StatusFound)
}

var clientKeySecretValidator = lazyregexp.New("^[a-zA-Z0-9.]*$")

func validateClientKeyOrSecret(clientKeyOrSecret string) (valid bool) {
	return clientKeySecretValidator.MatchString(clientKeyOrSecret)
}

func requestedScopes() []string {
	scopes := []string{"account", "email"}
	if !envvar.SourcegraphDotComMode() {
		scopes = append(scopes, "repository")
	}

	return scopes
}
