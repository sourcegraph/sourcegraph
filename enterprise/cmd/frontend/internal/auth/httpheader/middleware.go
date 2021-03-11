// Package httpheader implements auth via HTTP Headers.
package httpheader

import (
	"net/http"
	"strings"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
)

const providerType = "http-header"

// Middleware is the same for both the app and API because the HTTP proxy is assumed to wrap
// requests to both the app and API and add headers.
//
// See the "func middleware" docs for more information.
var Middleware = &auth.Middleware{
	API: middleware,
	App: middleware,
}

// middleware is middleware that checks for an HTTP header from an auth proxy that specifies the
// client's authenticated username. It's for use with auth proxies like
// https://github.com/bitly/oauth2_proxy and is configured with the http-header auth provider in
// site config.
//
// TESTING: Use the testproxy test program to test HTTP auth proxy behavior. For example, run `go
// run cmd/frontend/auth/httpheader/testproxy.go -username=alice` then go to
// http://localhost:4080. See `-h` for flag help.
//
// 🚨 SECURITY
func middleware(next http.Handler) http.Handler {
	const misconfiguredMessage = "Misconfigured http-header auth provider."
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authProvider, multiple := getProviderConfig()
		if multiple {
			log15.Error("At most 1 HTTP header auth provider may be set in site config.")
			http.Error(w, misconfiguredMessage, http.StatusInternalServerError)
			return
		}
		if authProvider == nil {
			next.ServeHTTP(w, r)
			return
		}
		if authProvider.UsernameHeader == "" {
			log15.Error("No HTTP header set for username (set the http-header auth provider's usernameHeader property).")
			http.Error(w, "misconfigured http-header auth provider", http.StatusInternalServerError)
			return
		}

		rawUsername := strings.TrimPrefix(r.Header.Get(authProvider.UsernameHeader), authProvider.StripUsernameHeaderPrefix)

		// Continue onto next auth provider if no header is set (in case the auth proxy allows
		// unauthenticated users to bypass it, which some do). Also respect already authenticated
		// actors (e.g., via access token).
		//
		// It would NOT add any additional security to return an error here, because a user who can
		// access this HTTP endpoint directly can just as easily supply a fake username whose
		// identity to assume.
		if rawUsername == "" || actor.FromContext(r.Context()).IsAuthenticated() {
			next.ServeHTTP(w, r)
			return
		}

		// Otherwise, get or create the user and proceed with the authenticated request.
		username, err := auth.NormalizeUsername(rawUsername)
		if err != nil {
			log15.Error("Error normalizing username from HTTP auth proxy.", "username", rawUsername, "err", err)
			http.Error(w, "unable to normalize username", http.StatusInternalServerError)
			return
		}
		userID, safeErrMsg, err := auth.GetAndSaveUser(r.Context(), auth.GetAndSaveUserOp{
			UserProps: database.NewUser{Username: username},
			ExternalAccount: extsvc.AccountSpec{
				ServiceType: providerType,
				// Store rawUsername, not normalized username, to prevent two users with distinct
				// pre-normalization usernames from being merged into the same normalized username
				// (and therefore letting them each impersonate the other).
				AccountID: rawUsername,
			},
			CreateIfNotExist: true,
			LookUpByUsername: true,
		})
		if err != nil {
			log15.Error("unable to get/create user from SSO header", "header", authProvider.UsernameHeader, "rawUsername", rawUsername, "err", err, "userErr", safeErrMsg)
			http.Error(w, safeErrMsg, http.StatusInternalServerError)
			return
		}

		r = r.WithContext(actor.WithActor(r.Context(), &actor.Actor{UID: userID}))
		next.ServeHTTP(w, r)
	})
}
