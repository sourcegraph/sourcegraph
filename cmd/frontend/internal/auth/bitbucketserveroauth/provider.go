package bitbucketserveroauth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/dghubble/gologin/v2"
	"github.com/dghubble/gologin/v2/bitbucket"
	oauth2Login "github.com/dghubble/gologin/v2/oauth2"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/session"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	iauth "github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/schema"
)

const (
	sessionKey = "bitbucketserveroauth@0"
)

func parseProvider(logger log.Logger, p *schema.BitbucketServerAuthProvider, db database.DB, sourceCfg schema.AuthProviders) (provider *oauth.Provider, messages []string) {
	rawURL := p.Url
	if rawURL == "" {
		messages = append(messages, "Bitbucket Server URL must not be empty")
		return nil, messages
	}
	parsedURL, err := url.Parse(rawURL)
	parsedURL = extsvc.NormalizeBaseURL(parsedURL)
	if err != nil {
		messages = append(messages, fmt.Sprintf("Could not parse Bitbucket Server URL %q. You will not be able to login via Bitbucket Server.", rawURL))
		return nil, messages
	}

	if !validateClientKeyOrSecret(p.ClientKey) {
		messages = append(messages, "Bitbucket Server key contains unexpected characters, possibly hidden")
	}
	if !validateClientKeyOrSecret(p.ClientSecret) {
		messages = append(messages, "Bitbucket Server secret contains unexpected characters, possibly hidden")
	}

	cli, err := bitbucketserver.NewClient("BitbucketServerOAuth", &schema.BitbucketServerConnection{Url: p.Url}, nil)
	if err != nil {
		messages = append(messages, "Unable to initialize Bitbucket Server client. You will not be able to login via Bitbucket Server.")
		return nil, messages
	}

	return oauth.NewProvider(oauth.ProviderOp{
		AuthPrefix: authPrefix,
		OAuth2Config: func() oauth2.Config {
			return oauth2.Config{
				ClientID:     p.ClientKey,
				ClientSecret: p.ClientSecret,
				Scopes:       requestedScopes(p.ApiScope),
				RedirectURL:  globals.ExternalURL().ResolveReference(&url.URL{Path: "/.auth/bitbucketserver/callback"}).String(),
				Endpoint: oauth2.Endpoint{
					AuthURL:  parsedURL.ResolveReference(&url.URL{Path: "/rest/oauth2/latest/authorize"}).String(),
					TokenURL: parsedURL.ResolveReference(&url.URL{Path: "/rest/oauth2/latest/token"}).String(),
				},
			}
		},
		SourceConfig: sourceCfg,
		ServiceID:    parsedURL.String(),
		ServiceType:  extsvc.TypeBitbucketServer,
		Login: func(oauth2Cfg oauth2.Config) http.Handler {
			return bitbucket.LoginHandler(&oauth2Cfg, nil)
		},
		Callback: func(oauth2Cfg oauth2.Config) http.Handler {
			return CallbackHandler(
				&oauth2Cfg,
				cli,
				oauth.SessionIssuer(logger, db, &sessionIssuerHelper{
					baseURL:     parsedURL,
					db:          db,
					clientKey:   p.ClientKey,
					allowSignup: p.AllowSignup,
				}, sessionKey),
				http.HandlerFunc(failureHandler),
			)
		},
	}), messages
}

// CallbackHandler handles Bitbucket redirection URI requests and adds the
// Bitbucket access token and User to the ctx. If authentication succeeds,
// handling delegates to the success handler, otherwise to the failure
// handler.
func CallbackHandler(config *oauth2.Config, client *bitbucketserver.Client, success, failure http.Handler) http.Handler {
	success = bitbucketHandler(config, client, success, failure)
	return oauth2Login.CallbackHandler(config, success, failure)
}

// bitbucketHandler is a http.Handler that gets the OAuth2 Token from the ctx
// to get the corresponding Bitbucket User. If successful, the User is added to
// the ctx and the success handler is called. Otherwise, the failure handler is
// called.
func bitbucketHandler(config *oauth2.Config, client *bitbucketserver.Client, success, failure http.Handler) http.Handler {
	if failure == nil {
		failure = gologin.DefaultFailureHandler
	}
	fn := func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		token, err := oauth2Login.TokenFromContext(ctx)
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}

		oauthToken := &iauth.OAuthBearerToken{
			Token:              token.AccessToken,
			RefreshToken:       token.RefreshToken,
			Expiry:             token.Expiry,
			NeedsRefreshBuffer: 5,
		}

		// TODO: Needed? We just authd.
		// oauthToken.RefreshFunc = oauthtoken.GetAccountRefreshAndStoreOAuthTokenFunc(db.UserExternalAccounts(), account.ID, bitbucketserver.GetOAuthContext(p.codeHost.BaseURL.String()))

		client := client.WithAuthenticator(oauthToken)
		_, err = client.AuthenticatedUsername(ctx)
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}
		// TODO: Needed?
		// ctx = WithUser(ctx, user)
		success.ServeHTTP(w, req.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
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

var clientKeySecretValidator = lazyregexp.New("^[a-zA-Z0-9.]*$")

func validateClientKeyOrSecret(clientKeyOrSecret string) (valid bool) {
	return clientKeySecretValidator.MatchString(clientKeyOrSecret)
}

func requestedScopes(apiScopes string) []string {
	scopes := []string{"REPO_READ"}
	if apiScopes != "" {
		scopes = strings.Split(apiScopes, ",")
	}
	return scopes
}
