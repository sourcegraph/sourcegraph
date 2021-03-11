package oauth

import (
	"context"
	"net/http"
	"time"

	goauth2 "github.com/dghubble/gologin/oauth2"
	"github.com/inconshreveable/log15"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/session"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

type SessionData struct {
	ID providers.ConfigID

	// Store only the oauth2.Token fields we need, to avoid hitting the ~4096-byte session data
	// limit.
	AccessToken string
	TokenType   string
}

type SessionIssuerHelper interface {
	GetOrCreateUser(ctx context.Context, token *oauth2.Token, anonymousUserID, firstSourceURL string) (actr *actor.Actor, safeErrMsg string, err error)
	CreateCodeHostConnection(ctx context.Context, token *oauth2.Token, providerID string) (safeErrMsg string, err error)
	DeleteStateCookie(w http.ResponseWriter)
	SessionData(token *oauth2.Token) SessionData
}

func SessionIssuer(s SessionIssuerHelper, sessionKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		token, err := goauth2.TokenFromContext(ctx)
		if err != nil {
			log15.Error("OAuth failed: could not read token from context", "error", err)
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: could not read token from callback request.", http.StatusInternalServerError)
			return
		}

		expiryDuration := time.Duration(0)
		if token.Expiry != (time.Time{}) {
			expiryDuration = time.Until(token.Expiry)
		}
		if expiryDuration < 0 {
			log15.Error("OAuth failed: token was expired.")
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: OAuth token was expired.", http.StatusInternalServerError)
			return
		}

		encodedState, err := goauth2.StateFromContext(ctx)
		if err != nil {
			log15.Error("OAuth failed: could not get state from context.", "error", err)
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: could not get OAuth state from context.", http.StatusInternalServerError)
			return
		}
		state, err := DecodeState(encodedState)
		if err != nil {
			log15.Error("OAuth failed: could not decode state.", "error", err)
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: could not get decode OAuth state.", http.StatusInternalServerError)
			return
		}

		// Delete state cookie (no longer needed, while be stale if user logs out and logs back in within 120s)
		defer s.DeleteStateCookie(w)

		if state.Op == LoginStateOpCreateCodeHostConnection {
			safeErrMsg, err := s.CreateCodeHostConnection(ctx, token, state.ProviderID)
			if err != nil {
				log15.Error("OAuth failed: error upserting code host connection from OAuth token.", "error", err, "userErr", safeErrMsg)
				http.Error(w, safeErrMsg, http.StatusInternalServerError)
				return
			}

			http.Redirect(w, r, auth.SafeRedirectURL(state.Redirect), http.StatusFound)
			return
		}

		getCookie := func(name string) string {
			c, err := r.Cookie(name)
			if err != nil {
				return ""
			}
			return c.Value
		}

		actr, safeErrMsg, err := s.GetOrCreateUser(ctx, token, getCookie("sourcegraphAnonymousUid"), getCookie("sourcegraphSourceUrl"))
		if err != nil {
			log15.Error("OAuth failed: error looking up or creating user from OAuth token.", "error", err, "userErr", safeErrMsg)
			http.Error(w, safeErrMsg, http.StatusInternalServerError)
			return
		}

		user, err := database.GlobalUsers.GetByID(r.Context(), actr.UID)
		if err != nil {
			log15.Error("OAuth failed: error retrieving user from database.", "error", err)
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: could not initiate session.", http.StatusInternalServerError)
			return
		}

		if err := session.SetActor(w, r, actr, expiryDuration, user.CreatedAt); err != nil { // TODO: test session expiration
			log15.Error("OAuth failed: could not initiate session.", "error", err)
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: could not initiate session.", http.StatusInternalServerError)
			return
		}

		if err := session.SetData(w, r, sessionKey, s.SessionData(token)); err != nil {
			// It's not fatal if this fails. It just means we won't be able to sign the user out of
			// the OP.
			log15.Warn("Failed to set OAuth session data. The session is still secure, but Sourcegraph will be unable to revoke the user's token or redirect the user to the end-session endpoint after the user signs out of Sourcegraph.", "error", err)
		}

		http.Redirect(w, r, auth.SafeRedirectURL(state.Redirect), http.StatusFound)
	})
}
