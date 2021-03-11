// Package auth is imported for side-effects to enable enterprise-only SSO.
package auth

import (
	"net/http"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/app"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/githuboauth"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/gitlaboauth"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/httpheader"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/openidconnect"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/saml"
	"github.com/sourcegraph/sourcegraph/internal/conf"
)

func init() {
	// Register enterprise auth middleware
	auth.RegisterMiddlewares(
		openidconnect.Middleware,
		saml.Middleware,
		httpheader.Middleware,
		githuboauth.Middleware,
		gitlaboauth.Middleware,
	)
	// Register app-level sign-out handler
	app.RegisterSSOSignOutHandler(ssoSignOutHandler)
}

func ssoSignOutHandler(w http.ResponseWriter, r *http.Request) {
	for _, p := range conf.Get().AuthProviders {
		var err error
		switch {
		case p.Openidconnect != nil:
			_, err = openidconnect.SignOut(w, r)
		case p.Saml != nil:
			_, err = saml.SignOut(w, r)
		}
		if err != nil {
			log15.Error("Error clearing auth provider session data.", "err", err)
		}
	}
}
