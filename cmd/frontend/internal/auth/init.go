// Package auth is imported for side-effects to enable enterprise-only SSO.
package auth

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/authutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/azureoauth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/bitbucketcloudoauth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/bitbucketserveroauth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/gerrit"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/githuboauth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/gitlaboauth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/httpheader"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/openidconnect"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/saml"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/sourcegraphoperator"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/userpasswd"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
)

// Init must be called by the frontend to initialize the auth middlewares.
func Init(logger log.Logger, db database.DB) {
	logger = logger.Scoped("auth")
	userpasswd.Init()
	azureoauth.Init(logger, db)
	bitbucketcloudoauth.Init(logger, db)
	bitbucketserveroauth.Init(logger, db)
	gerrit.Init()
	githuboauth.Init(logger, db)
	gitlaboauth.Init(logger, db)
	httpheader.Init()
	openidconnect.Init()
	saml.Init()
	sourcegraphoperator.Init()

	// Register enterprise auth middleware
	auth.RegisterMiddlewares(
		authutil.ConnectOrSignOutMiddleware(db),
		openidconnect.Middleware(logger, db),
		sourcegraphoperator.Middleware(db),
		saml.Middleware(db),
		httpheader.Middleware(logger, db),
		githuboauth.Middleware(db),
		gitlaboauth.Middleware(db),
		bitbucketcloudoauth.Middleware(db),
		bitbucketserveroauth.Middleware(db),
		azureoauth.Middleware(db),
	)

	// Warn about usage of auth providers that are not enabled by the license.
	graphqlbackend.AlertFuncs = append(graphqlbackend.AlertFuncs, func(args graphqlbackend.AlertFuncArgs) []*graphqlbackend.Alert {
		// Only site admins can act on this alert, so only show it to site admins.
		if !args.IsSiteAdmin {
			return nil
		}

		if licensing.IsFeatureEnabledLenient(licensing.FeatureSSO) {
			return nil
		}

		collected := make(map[string]struct{})
		var names []string
		for _, p := range conf.Get().AuthProviders {
			// Only built-in authentication provider is allowed by default.
			if p.Builtin != nil {
				continue
			}

			var name string
			switch {
			case p.Github != nil:
				name = "GitHub OAuth"
			case p.Gitlab != nil:
				name = "GitLab OAuth"
			case p.Bitbucketcloud != nil:
				name = "Bitbucket Cloud OAuth"
			case p.Bitbucketserver != nil:
				name = "Bitbucket Server OAuth"
			case p.AzureDevOps != nil:
				name = "Azure DevOps"
			case p.HttpHeader != nil:
				name = "HTTP header"
			case p.Openidconnect != nil:
				name = "OpenID Connect"
			case p.Saml != nil:
				name = "SAML"
			default:
				name = "Other"
			}

			if _, ok := collected[name]; !ok {
				collected[name] = struct{}{}
				names = append(names, name)
			}
		}
		if len(names) == 0 {
			return nil
		}

		sort.Strings(names)
		return []*graphqlbackend.Alert{{
			TypeValue:    graphqlbackend.AlertTypeError,
			MessageValue: fmt.Sprintf("A Sourcegraph license is required to enable following authentication providers: %s. [**Get a license.**](/site-admin/license)", strings.Join(names, ", ")),
		}}
	})
}
