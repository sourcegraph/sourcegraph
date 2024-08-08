package bitbucketserveroauth

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/dghubble/gologin/v2"
	"github.com/dghubble/gologin/v2/bitbucket"
	oauth2Login "github.com/dghubble/gologin/v2/oauth2"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/session"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/schema"
	"golang.org/x/oauth2"
)

const sessionKey = "bitbucketserveroauth@0"

func bitbucketServerCallbackHandler(config *oauth2.Config, success, failure http.Handler) http.Handler {
	return oauth2Login.CallbackHandler(config, success, failure)
}

func parseProvider(logger log.Logger, p *schema.BitbucketServerAuthProvider, db database.DB, sourceCfg schema.AuthProviders) (provider *oauth.Provider, messages []string) {
	rawURL := p.Url
	parsedURL, err := url.Parse(rawURL)
	parsedURL = extsvc.NormalizeBaseURL(parsedURL)
	if err != nil {
		messages = append(messages, fmt.Sprintf("Could not parse Bitbucket Server URL %q. You will not be able to login via Bitbucket Server.", rawURL))
		return nil, messages
	}

	externalURL := conf.ExternalURL()
	extURL, _ := url.Parse(externalURL)

	return oauth.NewProvider(oauth.ProviderOp{
		AuthPrefix: authPrefix,
		OAuth2Config: func() oauth2.Config {
			return oauth2.Config{
				ClientID:     p.ClientID,
				ClientSecret: p.ClientSecret,
				Endpoint: oauth2.Endpoint{
					AuthURL:  parsedURL.ResolveReference(&url.URL{Path: "/rest/oauth2/latest/authorize"}).String(),
					TokenURL: parsedURL.ResolveReference(&url.URL{Path: "/rest/oauth2/latest/token"}).String(),
				},
				Scopes:      []string{"REPO_READ"},
				RedirectURL: extURL.ResolveReference(&url.URL{Path: "/.auth/bitbucketserver/callback"}).String(),
			}
		},
		SourceConfig: sourceCfg,
		ServiceID:    parsedURL.String(),
		ServiceType:  extsvc.TypeBitbucketServer,
		Login: func(oauth2Cfg oauth2.Config) http.Handler {
			return bitbucket.LoginHandler(&oauth2Cfg, nil)
		},
		Callback: func(oauth2Cfg oauth2.Config) http.Handler {
			return bitbucketServerCallbackHandler(
				&oauth2Cfg,
				oauth.SessionIssuer(logger, db, &sessionIssuerHelper{
					logger:      logger.Scoped("sessionIssuerHelper"),
					baseURL:     parsedURL,
					db:          db,
					clientKey:   p.ClientID,
					allowSignup: p.AllowSignup,
				}, sessionKey),
				http.HandlerFunc(failureHandler),
			)
		},
	}), messages
}

func failureHandler(w http.ResponseWriter, r *http.Request) {
	// As a special case we want to handle `access_denied` errors by redirecting
	// back. This case arises when the user decides not to proceed by clicking `cancel`.
	if err := r.URL.Query().Get("error"); err != "access_denied" {
		// Fall back to default failure handler
		gologin.DefaultFailureHandler.ServeHTTP(w, r)
		return
	}

	var encodedState string
	err := session.GetData(r, "oauthState", &encodedState)
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
