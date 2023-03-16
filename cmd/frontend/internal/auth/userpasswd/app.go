package userpasswd

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"sync"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/session"
	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf/deploy"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const appUsername = "admin"

// AppNonce stores the nonce used by Sourcegraph App to enable passworldless
// login from the console.
var AppNonce Nonce

// Nonce is a base64 URL encoded string which can only be verified once.
type Nonce struct {
	mu    sync.Mutex
	value string
}

// Value returns the current nonce value, or generates one if it has not yet
// been generated. An error can be returned if generation fails.
func (n *Nonce) Value() (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.value != "" {
		return n.value, nil
	}

	value, err := randBase64(32)
	if err != nil {
		return "", errors.Wrap(err, "failed to generate nonce from crypto/rand")
	}
	n.value = value

	return n.value, nil
}

// Verify returns true if clientNonce matches the current nonce value. If
// matching the nonce value is cleared out to prevent re-use.
func (n *Nonce) Verify(clientNonce string) bool {
	// We hold the lock the entire verify period to ensure we do not have
	// any replay attacks.
	n.mu.Lock()
	defer n.mu.Unlock()

	// We have already accepted the nonce or the nonce was never generated.
	if n.value == "" {
		return false
	}

	if subtle.ConstantTimeCompare([]byte(n.value), []byte(clientNonce)) != 1 {
		return false
	}

	// Success. Clear out the nonce to ensure it is only used once. If we
	// issue a new nonce it will get generated by Value.
	n.value = ""

	return true
}

// AppSignInMiddleware will intercept any request containing a nonce query
// parameter. If it is the correct nonce it will sign in and redirect to
// search. Otherwise it will call the wrapped handler.
func AppSignInMiddleware(db database.DB, handler func(w http.ResponseWriter, r *http.Request) error) func(w http.ResponseWriter, r *http.Request) error {
	// This handler should only be used in App. Extra precaution to enforce
	// that here.
	if !deploy.IsApp() {
		return handler
	}

	return func(w http.ResponseWriter, r *http.Request) error {
		nonce := r.URL.Query().Get("nonce")
		if nonce == "" {
			return handler(w, r)
		}

		if !AppNonce.Verify(nonce) {
			return errors.New("Authentication failed")
		}

		// Admin should always be UID=0, but just in case we query it.
		user, err := getByEmailOrUsername(r.Context(), db, appUsername)
		if err != nil {
			return errors.Wrap(err, "Failed to find admin account")
		}

		// Write the session cookie
		actor := sgactor.Actor{
			UID: user.ID,
		}
		if err := session.SetActor(w, r, &actor, 0, user.CreatedAt); err != nil {
			return errors.Wrap(err, "Could not create new user session")
		}

		// Success. Redirect to search
		url := r.URL
		url.RawQuery = ""
		url.Path = "/search"
		http.Redirect(w, r, url.String(), http.StatusTemporaryRedirect)
		return nil
	}
}

// AppSiteInit is called in the case of Sourcegraph App to create the initial site admin account.
//
// Returns a sign-in URL which will automatically sign in the user. This URL
// can only be used once.
//
// Returns a nil error if the admin account already exists, or if it was created.
func AppSiteInit(ctx context.Context, logger log.Logger, db database.DB) (string, error) {
	password, err := generatePassword()
	if err != nil {
		return "", errors.Wrap(err, "failed to generate site admin password")
	}

	failIfNewUserIsNotInitialSiteAdmin := true
	err, _, _ = unsafeSignUp(ctx, logger, db, credentials{
		Email:    "app@sourcegraph.com",
		Username: appUsername,
		Password: password,
	}, failIfNewUserIsNotInitialSiteAdmin)
	if err != nil {
		return "", errors.Wrap(err, "failed to create site admin account")
	}

	// We have an account, return a sign in URL.
	return appSignInURL(), nil
}

func generatePassword() (string, error) {
	pw, err := randBase64(64)
	if err != nil {
		return "", err
	}
	if len(pw) > 72 {
		return pw[:72], nil
	}
	return pw, nil
}

func appSignInURL() string {
	externalURL := globals.ExternalURL().String()
	u, err := url.Parse(externalURL)
	if err != nil {
		return externalURL
	}
	nonce, err := AppNonce.Value()
	if err != nil {
		return externalURL
	}
	u.Path = "/sign-in"
	query := u.Query()
	query.Set("nonce", nonce)
	u.RawQuery = query.Encode()
	return u.String()
}

func randBase64(dataLen int) (string, error) {
	data := make([]byte, dataLen)
	_, err := rand.Read(data)
	if err != nil {
		return "", err
	}
	// Our nonces end up in URLs, so use URLEncoding.
	return base64.URLEncoding.EncodeToString(data), nil
}
