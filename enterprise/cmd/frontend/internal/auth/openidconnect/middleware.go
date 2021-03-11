// Package openidconnect implements auth via OIDC.
package openidconnect

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/csrf"
	"github.com/inconshreveable/log15"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/session"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"

	"github.com/coreos/go-oidc"
)

const stateCookieName = "sg-oidc-state"

// All OpenID Connect endpoints are under this path prefix.
const authPrefix = auth.AuthURLPrefix + "/openidconnect"

type userClaims struct {
	Name              string `json:"name"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	PreferredUsername string `json:"preferred_username"`
	Picture           string `json:"picture"`
	EmailVerified     *bool  `json:"email_verified"`
}

// Middleware is middleware for OpenID Connect (OIDC) authentication, adding endpoints under the
// auth path prefix ("/.auth") to enable the login flow and requiring login for all other endpoints.
//
// The OIDC spec (http://openid.net/specs/openid-connect-core-1_0.html) describes an authentication protocol
// that involves 3 parties: the Relying Party (e.g., Sourcegraph), the OpenID Provider (e.g., Okta, OneLogin,
// or another SSO provider), and the End User (e.g., a user's web browser).
//
// This middleware implements two things: (1) the OIDC Authorization Code Flow
// (http://openid.net/specs/openid-connect-core-1_0.html#CodeFlowAuth) and (2) Sourcegraph-specific session management
// (outside the scope of the OIDC spec). Upon successful completion of the OIDC login flow, the handler will create
// a new session and session cookie. The expiration of the session is the expiration of the OIDC ID Token.
//
// 🚨 SECURITY
var Middleware = &auth.Middleware{
	API: func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handleOpenIDConnectAuth(w, r, next, true)
		})
	},
	App: func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handleOpenIDConnectAuth(w, r, next, false)
		})
	},
}

// handleOpenIDConnectAuth performs OpenID Connect authentication (if configured) for HTTP requests,
// both API requests and non-API requests.
func handleOpenIDConnectAuth(w http.ResponseWriter, r *http.Request, next http.Handler, isAPIRequest bool) {
	// Fixup URL path. We use "/.auth/callback" as the redirect URI for OpenID Connect, but the rest
	// of this middleware's handlers expect paths of "/.auth/openidconnect/...", so add the
	// "openidconnect" path component. We can't change the redirect URI because it is hardcoded in
	// instances' external auth providers.
	if r.URL.Path == auth.AuthURLPrefix+"/callback" {
		// Rewrite "/.auth/callback" -> "/.auth/openidconnect/callback".
		r.URL.Path = authPrefix + "/callback"
	}

	// Delegate to the OpenID Connect auth handler.
	if !isAPIRequest && strings.HasPrefix(r.URL.Path, authPrefix+"/") {
		authHandler(w, r)
		return
	}

	// If the actor is authenticated and not performing an OpenID Connect flow, then proceed to
	// next.
	if actor.FromContext(r.Context()).IsAuthenticated() {
		next.ServeHTTP(w, r)
		return
	}

	// If there is only one auth provider configured, the single auth provider is OpenID Connect,
	// and it's an app request, redirect to signin immediately. The user wouldn't be able to do
	// anything else anyway; there's no point in showing them a signin screen with just a single
	// signin option.
	if ps := providers.Providers(); len(ps) == 1 && ps[0].Config().Openidconnect != nil && !isAPIRequest {
		p, handled := handleGetProvider(r.Context(), w, ps[0].ConfigID().ID)
		if handled {
			return
		}
		redirectToAuthRequest(w, r, p, auth.SafeRedirectURL(r.URL.String()))
		return
	}

	next.ServeHTTP(w, r)
}

// mockVerifyIDToken mocks the OIDC ID Token verification step. It should only be set in tests.
var mockVerifyIDToken func(rawIDToken string) *oidc.IDToken

// authHandler handles the OIDC Authentication Code Flow
// (http://openid.net/specs/openid-connect-core-1_0.html#CodeFlowAuth) on the Relying Party's end.
//
// 🚨 SECURITY
func authHandler(w http.ResponseWriter, r *http.Request) {
	switch strings.TrimPrefix(r.URL.Path, authPrefix) {
	case "/login":
		// Endpoint that starts the Authentication Request Code Flow.
		p, handled := handleGetProvider(r.Context(), w, r.URL.Query().Get("pc"))
		if handled {
			return
		}
		redirectToAuthRequest(w, r, p, r.URL.Query().Get("redirect"))
		return

	case "/callback":
		// Endpoint for the OIDC Authorization Response. See http://openid.net/specs/openid-connect-core-1_0.html#AuthResponse.
		ctx := r.Context()
		if authError := r.URL.Query().Get("error"); authError != "" {
			errorDesc := r.URL.Query().Get("error_description")
			log15.Error("OpenID Connect auth provider returned error to callback.", "error", authError, "description", errorDesc)
			http.Error(w, fmt.Sprintf("Authentication failed. Try signing in again (and clearing cookies for the current site). The authentication provider reported the following problems.\n\n%s\n\n%s", authError, errorDesc), http.StatusUnauthorized)
			return
		}

		// Validate state parameter to prevent CSRF attacks
		stateParam := r.URL.Query().Get("state")
		if stateParam == "" {
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). No OpenID Connect state query parameter specified.", http.StatusBadRequest)
			return
		}
		stateCookie, err := r.Cookie(stateCookieName)
		if err == http.ErrNoCookie {
			log15.Error("OpenID Connect auth failed: no state cookie found (possible request forgery).")
			http.Error(w, fmt.Sprintf("Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: no OpenID Connect state cookie found (possible request forgery, or more than %s elapsed since you started the authentication process).", stateCookieTimeout), http.StatusBadRequest)
			return
		} else if err != nil {
			log15.Error("OpenID Connect auth failed: could not read state cookie (possible request forgery).", "error", err)
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: invalid OpenID Connect state cookie.", http.StatusInternalServerError)
			return
		}
		if stateCookie.Value != stateParam {
			log15.Error("OpenID Connect auth failed: state cookie mismatch (possible request forgery).")
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: OpenID Connect state parameter did not match the expected value (possible request forgery).", http.StatusBadRequest)
			return
		}

		// Decode state param value
		var state authnState
		if err := state.Decode(stateParam); err != nil {
			log15.Error("OpenID Connect auth failed: state parameter was malformed.", "error", err)
			http.Error(w, "Authentication failed. OpenID Connect state parameter was malformed.", http.StatusBadRequest)
			return
		}
		// 🚨 SECURITY: TODO(sqs): Do we need to check state.CSRFToken?

		p, handled := handleGetProvider(r.Context(), w, state.ProviderID)
		if handled {
			return
		}
		verifier := p.oidc.Verifier(&oidc.Config{ClientID: p.config.ClientID})

		// Exchange the code for an access token. See http://openid.net/specs/openid-connect-core-1_0.html#TokenRequest.
		oauth2Token, err := p.oauth2Config().Exchange(ctx, r.URL.Query().Get("code"))
		if err != nil {
			log15.Error("OpenID Connect auth failed: failed to obtain access token from OP.", "error", err)
			http.Error(w, "Authentication failed. Try signing in again. The error was: unable to obtain access token from issuer.", http.StatusUnauthorized)
			return
		}

		// Extract the ID Token from the Access Token. See http://openid.net/specs/openid-connect-core-1_0.html#TokenResponse.
		rawIDToken, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			log15.Error("OpenID Connect auth failed: the issuer's authorization response did not contain an ID token.")
			http.Error(w, "Authentication failed. Try signing in again. The error was: the issuer's authorization response did not contain an ID token.", http.StatusUnauthorized)
			return
		}

		// Parse and verify ID Token payload. See http://openid.net/specs/openid-connect-core-1_0.html#TokenResponseValidation.
		var idToken *oidc.IDToken
		if mockVerifyIDToken != nil {
			idToken = mockVerifyIDToken(rawIDToken)
		} else {
			idToken, err = verifier.Verify(ctx, rawIDToken)
			if err != nil {
				log15.Error("OpenID Connect auth failed: the ID token verification failed.", "error", err)
				http.Error(w, "Authentication failed. Try signing in again. The error was: OpenID Connect ID token could not be verified.", http.StatusUnauthorized)
				return
			}
		}

		// Validate the nonce. The Verify method explicitly doesn't handle nonce validation, so we do that here.
		// We set the nonce to be the same as the state in the Authentication Request state, so we check for equality
		// here.
		if idToken.Nonce != stateParam {
			log15.Error("OpenID Connect auth failed: nonce is incorrect (possible replay attach).")
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: OpenID Connect nonce is incorrect (possible replay attack).", http.StatusUnauthorized)
			return
		}

		userInfo, err := p.oidc.UserInfo(ctx, oauth2.StaticTokenSource(oauth2Token))
		if err != nil {
			log15.Error("Failed to get userinfo", "error", err)
			http.Error(w, "Failed to get userinfo: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if p.config.RequireEmailDomain != "" && !strings.HasSuffix(userInfo.Email, "@"+p.config.RequireEmailDomain) {
			log15.Error("OpenID Connect auth failed: user's email is not from allowed domain.", "userEmail", userInfo.Email, "requireEmailDomain", p.config.RequireEmailDomain)
			http.Error(w, fmt.Sprintf("Authentication failed. Only users in %q are allowed.", p.config.RequireEmailDomain), http.StatusUnauthorized)
			return
		}

		var claims userClaims
		if err := userInfo.Claims(&claims); err != nil {
			log15.Warn("OpenID Connect auth: could not parse userInfo claims.", "error", err)
		}
		actr, safeErrMsg, err := getOrCreateUser(ctx, p, idToken, userInfo, &claims)
		if err != nil {
			log15.Error("OpenID Connect auth failed: error looking up OpenID-authenticated user.", "error", err, "userErr", safeErrMsg)
			http.Error(w, safeErrMsg, http.StatusInternalServerError)
			return
		}

		user, err := database.GlobalUsers.GetByID(r.Context(), actr.UID)
		if err != nil {
			log15.Error("OpenID Connect auth failed: error retrieving user from database.", "error", err)
			http.Error(w, "Failed to retrieve user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var exp time.Duration
		// 🚨 SECURITY: TODO(sqs): We *should* uncomment the lines below to make our own sessions
		// only last for as long as the OP said the access token is active for. Unfortunately,
		// until we support refreshing access tokens in the background
		// (https://github.com/sourcegraph/sourcegraph/issues/11340), this provides a bad user
		// experience because users need to re-authenticate via OIDC every minute or so
		// (assuming their OIDC OP, like many, has a 1-minute access token validity period).
		//
		// if !idToken.Expiry.IsZero() {
		// 	exp = time.Until(idToken.Expiry)
		// }
		if err := session.SetActor(w, r, actr, exp, user.CreatedAt); err != nil {
			log15.Error("OpenID Connect auth failed: could not initiate session.", "error", err)
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: could not initiate session.", http.StatusInternalServerError)
			return
		}

		data := sessionData{
			ID:          p.ConfigID(),
			AccessToken: oauth2Token.AccessToken,
			TokenType:   oauth2Token.TokenType,
		}
		if err := session.SetData(w, r, sessionKey, data); err != nil {
			// It's not fatal if this fails. It just means we won't be able to sign the user out of
			// the OP.
			log15.Warn("Failed to set OpenID Connect session data. The session is still secure, but Sourcegraph will be unable to revoke the user's token or redirect the user to the end-session endpoint after the user signs out of Sourcegraph.", "error", err)
		}

		// 🚨 SECURITY: Call auth.SafeRedirectURL to avoid an open-redirect vuln.
		http.Redirect(w, r, auth.SafeRedirectURL(state.Redirect), http.StatusFound)

	default:
		http.Error(w, "", http.StatusNotFound)
	}
}

// authnState is the state parameter passed to the Authn request and returned in the Authn response callback.
type authnState struct {
	CSRFToken string `json:"csrfToken"`
	Redirect  string `json:"redirect"`

	// Allow /.auth/callback to demux callbacks from multiple OpenID Connect OPs.
	ProviderID string `json:"p"`
}

// Encode returns the base64-encoded JSON representation of the authn state.
func (s *authnState) Encode() string {
	b, _ := json.Marshal(s)
	return base64.StdEncoding.EncodeToString(b)
}

// Decode decodes the base64-encoded JSON representation of the authn state into the receiver.
func (s *authnState) Decode(encoded string) error {
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, s)
}

const stateCookieTimeout = time.Minute * 15

func redirectToAuthRequest(w http.ResponseWriter, r *http.Request, p *provider, returnToURL string) {
	// The state parameter is an opaque value used to maintain state between the original Authentication Request
	// and the callback. We do not record any state beyond a CSRF token used to defend against CSRF attacks against the callback.
	// We use the CSRF token created by gorilla/csrf that is used for other app endpoints as the OIDC state parameter.
	//
	// See http://openid.net/specs/openid-connect-core-1_0.html#AuthRequest of the OIDC spec.
	state := (&authnState{
		CSRFToken:  csrf.Token(r),
		Redirect:   returnToURL,
		ProviderID: p.ConfigID().ID,
	}).Encode()
	http.SetCookie(w, &http.Cookie{
		Name:    stateCookieName,
		Value:   state,
		Path:    auth.AuthURLPrefix + "/", // include the OIDC redirect URI (/.auth/callback not /.auth/openidconnect/callback for BACKCOMPAT)
		Expires: time.Now().Add(stateCookieTimeout),
	})

	// Redirect to the OP's Authorization Endpoint for authentication. The nonce is an optional
	// string value used to associate a Client session with an ID Token and to mitigate replay attacks.
	// Whereas the state parameter is used in validating the Authentication Request
	// callback, the nonce is used in validating the response to the ID Token request.
	// We re-use the Authn request state as the nonce.
	//
	// See http://openid.net/specs/openid-connect-core-1_0.html#AuthRequest of the OIDC spec.
	http.Redirect(w, r, p.oauth2Config().AuthCodeURL(state, oidc.Nonce(state)), http.StatusFound)
}
