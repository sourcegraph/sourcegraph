package gitlaboauth

import (
	"net/http"
	"net/url"

	"github.com/dghubble/gologin/v2"
	oauth2Login "github.com/dghubble/gologin/v2/oauth2"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// SSOLoginHandler is a custom implementation of github.com/dghubble/gologin/oauth2's LoginHandler method.
// It takes an extra ssoAuthURL parameter, and adds the original authURL as a redirect parameter to that
// URL.
//
// This is used in cases where customers use SAML/SSO on their GitLab configurations. The default
// way GitLab handles redirects for groups that require SSO sign-on does not work, and users
// need to sign into GitLab outside of Sourcegraph, and can only then come back and use OAuth.
//
// This implementaion allows users to be directed to their GitLab SSO sign-in page, and then
// the redirect query parameter will redirect them to the OAuth sign-in flow that Sourcegraph
// requires.
func SSOLoginHandler(config *oauth2.Config, failure http.Handler, ssoAuthURL string) http.Handler {
	if failure == nil {
		failure = gologin.DefaultFailureHandler
	}
	fn := func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		state, err := oauth2Login.StateFromContext(ctx)
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}
		authURL, err := url.Parse(config.AuthCodeURL(state))
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}

		ssoAuthURL, err := url.Parse(ssoAuthURL)
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}
		queryParams := ssoAuthURL.Query()
		queryParams.Add("redirect", authURL.Path+"?"+authURL.RawQuery)
		ssoAuthURL.RawQuery = queryParams.Encode()
		http.Redirect(w, req, ssoAuthURL.String(), http.StatusFound)
	}
	return http.HandlerFunc(fn)
}

func LoginHandler(config *oauth2.Config, failure http.Handler) http.Handler {
	return oauth2Login.LoginHandler(config, failure)
}

func CallbackHandler(config *oauth2.Config, success, failure http.Handler) http.Handler {
	success = gitlabHandler(config, success, failure)
	return oauth2Login.CallbackHandler(config, success, failure)
}

func gitlabHandler(config *oauth2.Config, success, failure http.Handler) http.Handler {
	logger := log.Scoped("GitlabOAuthHandler")

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

		gitlabClient, err := gitlabClientFromAuthURL(config.Endpoint.AuthURL, token.AccessToken)
		if err != nil {
			ctx = gologin.WithError(ctx, errors.Errorf("could not parse AuthURL %s", config.Endpoint.AuthURL))
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}
		user, err := gitlabClient.GetUser(ctx, "")
		err = validateResponse(user, err)
		if err != nil {
			// TODO: Prefer a more general purpose fix, potentially
			// https://github.com/sourcegraph/sourcegraph/pull/20000
			logger.Warn("invalid response", log.Error(err))
		}
		if err != nil {
			ctx = gologin.WithError(ctx, err)
			failure.ServeHTTP(w, req.WithContext(ctx))
			return
		}
		ctx = WithUser(ctx, user)
		success.ServeHTTP(w, req.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

// validateResponse returns an error if the given GitLab user or error are unexpected. Returns nil
// if they are valid.
func validateResponse(user *gitlab.AuthUser, err error) error {
	if err != nil {
		return errors.Wrap(err, "unable to get GitLab user")
	}
	if user == nil || user.ID == 0 {
		return errors.Errorf("unable to get GitLab user: bad user info %#+v", user)
	}
	return nil
}

func gitlabClientFromAuthURL(authURL, oauthToken string) (*gitlab.Client, error) {
	baseURL, err := url.Parse(authURL)
	if err != nil {
		return nil, err
	}
	baseURL.Path = ""
	baseURL.RawQuery = ""
	baseURL.Fragment = ""
	p, err := gitlab.NewClientProvider(extsvc.URNGitLabOAuth, baseURL, nil)
	if err != nil {
		return nil, err
	}
	return p.GetOAuthClient(oauthToken), nil
}
