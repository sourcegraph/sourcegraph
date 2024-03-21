package saml

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/session"
	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

// All SAML endpoints are under this path prefix.
const authPrefix = auth.AuthURLPrefix + "/saml"

// Middleware is middleware for SAML authentication, adding endpoints under the auth path prefix to
// enable the login flow an requiring login for all other endpoints.
//
// 🚨 SECURITY
func Middleware(db database.DB) *auth.Middleware {
	return &auth.Middleware{
		API: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authHandler(db, w, r, next, true)
			})
		},
		App: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authHandler(db, w, r, next, false)
			})
		},
	}
}

// authHandler is the new SAML HTTP auth handler.
//
// It uses github.com/russelhaering/gosaml2 and (unlike authHandler1) makes it possible to support
// multiple auth providers with SAML and expose more SAML functionality.
func authHandler(db database.DB, w http.ResponseWriter, r *http.Request, next http.Handler, isAPIRequest bool) {
	// Delegate to SAML ACS and metadata endpoint handlers.
	if !isAPIRequest && strings.HasPrefix(r.URL.Path, auth.AuthURLPrefix+"/saml/") {
		samlSPHandler(db)(w, r)
		return
	}

	// If the actor is authenticated and not performing a SAML operation, then proceed to next.
	if sgactor.FromContext(r.Context()).IsAuthenticated() {
		next.ServeHTTP(w, r)
		return
	}

	// If there is only one auth provider configured, the single auth provider is SAML, it's an
	// app request, and the sign-out cookie is not present, redirect to the sso sign-in immediately.
	//
	// For sign-out requests (sign-out cookie is  present), the user will be redirected to the Sourcegraph login page.
	ps := providers.SignInProviders()
	if len(ps) == 1 && ps[0].Config().Saml != nil && !auth.HasSignOutCookie(r) && !isAPIRequest {
		p, handled := handleGetProvider(r.Context(), w, ps[0].ConfigID().ID)
		if handled {
			return
		}
		redirectToAuthURL(w, r, p, auth.SafeRedirectURL(r.URL.String()))
		return
	}

	next.ServeHTTP(w, r)
}

func samlSPHandler(db database.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		requestPath := strings.TrimPrefix(r.URL.Path, authPrefix)

		// Handle GET endpoints.
		if r.Method == "GET" {
			// All of these endpoints expect the provider ID in the URL query.
			p, handled := handleGetProvider(r.Context(), w, r.URL.Query().Get("pc"))
			if handled {
				return
			}

			switch requestPath {
			case "/metadata":
				metadata, err := p.samlSP.Metadata()
				if err != nil {
					log15.Error("Error generating SAML service provider metadata.", "err", err)
					http.Error(w, "", http.StatusInternalServerError)
					return
				}

				buf, err := xml.MarshalIndent(metadata, "", "  ")
				if err != nil {
					log15.Error("Error encoding SAML service provider metadata.", "err", err)
					http.Error(w, "", http.StatusInternalServerError)
					return
				}
				traceLog(fmt.Sprintf("Service Provider metadata: %s", p.ConfigID().ID), string(buf))
				w.Header().Set("Content-Type", "application/samlmetadata+xml; charset=utf-8")
				_, _ = w.Write(buf)
				return

			case "/login":
				// It is safe to use r.Referer() because the redirect-to URL will be checked later,
				// before the client is actually instructed to navigate there.
				redirectToAuthURL(w, r, p, r.Referer())
				return
			}
		}

		if r.Method != "POST" {
			http.Error(w, "", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		// The remaining endpoints all expect the provider ID in the POST data's RelayState.
		traceLog("SAML RelayState", r.FormValue("RelayState"))
		var relayState relayState
		relayState.decode(r.FormValue("RelayState"))

		p, handled := handleGetProvider(r.Context(), w, relayState.ProviderID)
		if handled {
			return
		}

		// Handle POST endpoints.
		switch requestPath {
		case "/acs":
			info, err := readAuthnResponse(p, r.FormValue("SAMLResponse"))
			if err != nil {
				log15.Error("Error validating SAML assertions. Set the env var INSECURE_SAML_LOG_TRACES=1 to log all SAML requests and responses.", "err", err)
				http.Error(w, "Error validating SAML assertions. Try signing in again. If the problem persists, a site admin must check the configuration.", http.StatusForbidden)
				return
			}

			if !allowSignin(p, info.groups) {
				log15.Warn("Error authorizing SAML-authenticated user.", "AccountID", info.spec.AccountID, "Expected groups", p.config.AllowGroups, "Got", info.groups)
				http.Error(w, "Error authorizing SAML-authenticated user. The user does not belong to one of the configured groups.", http.StatusForbidden)
				return
			}
			allowSignup := p.config.AllowSignup == nil || *p.config.AllowSignup
			newUserCreated, actor, safeErrMsg, err := getOrCreateUser(r.Context(), db, allowSignup, info)
			if err != nil {
				log15.Error("Error looking up SAML-authenticated user.", "err", err, "userErr", safeErrMsg)
				http.Error(w, safeErrMsg, http.StatusInternalServerError)
				return
			}

			user, err := db.Users().GetByID(r.Context(), actor.UID)
			if err != nil {
				log15.Error("Error retrieving SAML-authenticated user from database.", "error", err)
				http.Error(w, "Failed to retrieve user: "+err.Error(), http.StatusInternalServerError)
				return
			}

			var exp time.Duration
			// 🚨 SECURITY: TODO(sqs): We *should* uncomment the line below to make our own sessions
			// only last for as long as the IdP said the authn grant is active for. Unfortunately,
			// until we support refreshing SAML authn in the background
			// (https://github.com/sourcegraph/sourcegraph/issues/11340), this provides a bad user
			// experience because users need to re-authenticate via SAML every minute or so
			// (assuming their SAML IdP, like many, has a 1-minute access token validity period).
			//
			// if info.SessionNotOnOrAfter != nil {
			// 	exp = time.Until(*info.SessionNotOnOrAfter)
			// }
			if _, err := session.SetActorFromUser(r.Context(), w, r, user, exp); err != nil {
				log15.Error("Error setting SAML-authenticated actor in session.", "err", err)
				http.Error(w, "Error starting SAML-authenticated session. Try signing in again.", http.StatusInternalServerError)
				return
			}

			// Add a ?signup= or ?signin= parameter to the redirect URL.
			redirectURL := auth.AddPostAuthRedirectParametersToString(relayState.ReturnToURL, newUserCreated, "SAML")

			// 🚨 SECURITY: Call auth.SafeRedirectURL to avoid an open-redirect vuln.
			http.Redirect(w, r, auth.SafeRedirectURL(redirectURL), http.StatusFound)

		case "/logout":
			encodedResp := r.FormValue("SAMLResponse")

			{
				if raw, err := base64.StdEncoding.DecodeString(encodedResp); err == nil {
					traceLog(fmt.Sprintf("LogoutResponse: %s", p.ConfigID().ID), string(raw))
				}
			}

			// TODO(sqs): Fully validate the LogoutResponse here (i.e., also validate that the document
			// is a valid LogoutResponse). It is possible that this request is being spoofed, but it
			// doesn't let an attacker do very much (just log a user out and redirect).
			//
			// 🚨 SECURITY: If this logout handler starts to do anything more advanced, it probably must
			// validate the LogoutResponse to avoid being vulnerable to spoofing.
			_, err := p.samlSP.ValidateEncodedResponse(encodedResp)
			if err != nil && !strings.HasPrefix(err.Error(), "unable to unmarshal response:") {
				log15.Error("Error validating SAML logout response.", "err", err)
				http.Error(w, "Error validating SAML logout response.", http.StatusForbidden)
				return
			}

			// If this is an SP-initiated logout, then the actor has already been cleared from the
			// session (but there's no harm in clearing it again). If it's an IdP-initiated logout,
			// then it hasn't, and we must clear it here.
			if err := session.SetActor(w, r, nil, 0, time.Time{}); err != nil {
				log15.Error("Error clearing actor from session in SAML logout handler.", "err", err)
				http.Error(w, "Error signing out of SAML-authenticated session.", http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, "/", http.StatusFound)

		default:
			http.Error(w, "", http.StatusNotFound)
		}
	}
}

func redirectToAuthURL(w http.ResponseWriter, r *http.Request, p *provider, returnToURL string) {
	authURL, err := buildAuthURLRedirect(p, relayState{
		ProviderID:  p.ConfigID().ID,
		ReturnToURL: auth.SafeRedirectURL(returnToURL),
	})
	if err != nil {
		log15.Error("Failed to build SAML auth URL.", "err", err)
		http.Error(w, "Unexpected error in SAML authentication provider.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func buildAuthURLRedirect(p *provider, relayState relayState) (string, error) {
	doc, err := p.samlSP.BuildAuthRequestDocument()
	if err != nil {
		return "", err
	}
	{
		if data, err := doc.WriteToString(); err == nil {
			traceLog(fmt.Sprintf("AuthnRequest: %s", p.ConfigID().ID), data)
		}
	}
	return p.samlSP.BuildAuthURLRedirect(relayState.encode(), doc)
}

// relayState represents the decoded RelayState value in both the IdP-initiated and SP-initiated
// login flows.
//
// SAML overloads the term "RelayState".
//   - In the SP-initiated login flow, it is an opaque value originated from the SP and reflected
//     back in the AuthnResponse. The Sourcegraph SP uses the base64-encoded JSON of this struct as
//     the RelayState.
//   - In the IdP-initiated login flow, the RelayState can be any arbitrary hint, but in practice
//     is the desired post-login redirect URL in plain text.
type relayState struct {
	ProviderID  string `json:"k"`
	ReturnToURL string `json:"r"`
}

// encode returns the base64-encoded JSON representation of the relay state.
func (s *relayState) encode() string {
	b, _ := json.Marshal(s)
	return base64.StdEncoding.EncodeToString(b)
}

// Decode decodes the base64-encoded JSON representation of the relay state into the receiver.
func (s *relayState) decode(encoded string) {
	if strings.HasPrefix(encoded, "http://") || strings.HasPrefix(encoded, "https://") || encoded == "" {
		s.ProviderID, s.ReturnToURL = "", encoded
		return
	}

	if b, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		if err := json.Unmarshal(b, s); err == nil {
			return
		}
	}

	s.ProviderID, s.ReturnToURL = "", ""
}

func allowSignin(p *provider, groups map[string]bool) bool {
	if p.config.AllowGroups == nil {
		return true
	}

	for _, group := range p.config.AllowGroups {
		if groups[group] {
			return true
		}
	}
	return false
}
