package authz

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/hooks"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/authz/resolvers"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/licensing/enforcement"
	eauthz "github.com/sourcegraph/sourcegraph/enterprise/internal/authz"
	eiauthz "github.com/sourcegraph/sourcegraph/enterprise/internal/authz"
	edb "github.com/sourcegraph/sourcegraph/enterprise/internal/database"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

var clock = timeutil.Now

func Init(ctx context.Context, db dbutil.DB, outOfBandMigrationRunner *oobmigration.Runner, enterpriseServices *enterprise.Services) error {
	// TODO(efritz) - de-globalize assignments in this function
	database.GlobalExternalServices = edb.NewExternalServicesStore(db)
	database.GlobalAuthz = edb.NewAuthzStore(db, clock)

	// Warn about usage of auth providers that are not enabled by the license.
	graphqlbackend.AlertFuncs = append(graphqlbackend.AlertFuncs, func(args graphqlbackend.AlertFuncArgs) []*graphqlbackend.Alert {
		// Only site admins can act on this alert, so only show it to site admins.
		if !args.IsSiteAdmin {
			return nil
		}

		if licensing.IsFeatureEnabledLenient(licensing.FeatureACLs) {
			return nil
		}

		// We can ignore problems returned here because they would have been surfaced in other places.
		_, providers, _, _ := eauthz.ProvidersFromConfig(context.Background(), conf.Get(), database.GlobalExternalServices)
		if len(providers) == 0 {
			return nil
		}

		// We currently support three types of authz providers: GitHub, GitLab and Bitbucket Server.
		authzTypes := make(map[string]struct{}, 3)
		for _, p := range providers {
			authzTypes[p.ServiceType()] = struct{}{}
		}

		authzNames := make([]string, 0, len(authzTypes))
		for t := range authzTypes {
			switch t {
			case extsvc.TypeGitHub:
				authzNames = append(authzNames, "GitHub")
			case extsvc.TypeGitLab:
				authzNames = append(authzNames, "GitLab")
			case extsvc.TypeBitbucketServer:
				authzNames = append(authzNames, "Bitbucket Server")
			default:
				authzNames = append(authzNames, t)
			}
		}
		return []*graphqlbackend.Alert{{
			TypeValue:    graphqlbackend.AlertTypeError,
			MessageValue: fmt.Sprintf("A Sourcegraph license is required to enable repository permissions for the following code hosts: %s. [**Get a license.**](/site-admin/license)", strings.Join(authzNames, ", ")),
		}}
	})

	graphqlbackend.AlertFuncs = append(graphqlbackend.AlertFuncs, func(args graphqlbackend.AlertFuncArgs) []*graphqlbackend.Alert {
		// 🚨 SECURITY: Only the site admin should ever see this (all other users will see a hard-block
		// license expiration screen) about this. Leaking this wouldn't be a security vulnerability, but
		// just in case this method is changed to return more information, we lock it down.
		if !args.IsSiteAdmin {
			return nil
		}

		info, err := licensing.GetConfiguredProductLicenseInfo()
		if err != nil {
			log15.Error("Error reading license key for Sourcegraph subscription.", "err", err)
			return []*graphqlbackend.Alert{{
				TypeValue:    graphqlbackend.AlertTypeError,
				MessageValue: "Error reading Sourcegraph license key. Check the logs for more information, or update the license key in the [**site configuration**](/site-admin/configuration).",
			}}
		}
		if info != nil && info.IsExpiredWithGracePeriod() {
			return []*graphqlbackend.Alert{{
				TypeValue:    graphqlbackend.AlertTypeError,
				MessageValue: "Sourcegraph license expired! All non-admin users are locked out of Sourcegraph. Update the license key in the [**site configuration**](/site-admin/configuration) or downgrade to only using Sourcegraph Free features.",
			}}
		}
		return nil
	})

	// Enforce the use of a valid license key by preventing all HTTP requests if the license is invalid
	// (due to an error in parsing or verification, or because the license has expired).
	hooks.PostAuthMiddleware = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Site admins are exempt from license enforcement screens so that they can
			// easily update the license key. Also ignore backend.ErrNotAuthenticated
			// because we need to allow site admins to sign in.
			err := backend.CheckCurrentUserIsSiteAdmin(r.Context())
			if err == nil || err == backend.ErrNotAuthenticated {
				next.ServeHTTP(w, r)
				return
			} else if err != backend.ErrMustBeSiteAdmin {
				log15.Error("Error checking current user is site admin", "err", err)
				http.Error(w, "Error checking current user is site admin. Site admins may check the logs for more information.", http.StatusInternalServerError)
				return
			}

			info, err := licensing.GetConfiguredProductLicenseInfo()
			if err != nil {
				log15.Error("Error reading license key for Sourcegraph subscription.", "err", err)
				enforcement.WriteSubscriptionErrorResponse(w, http.StatusInternalServerError, "Error reading Sourcegraph license key", "Site admins may check the logs for more information. Update the license key in the [**site configuration**](/site-admin/configuration).")
				return
			}
			if info != nil && info.IsExpiredWithGracePeriod() {
				enforcement.WriteSubscriptionErrorResponse(w, http.StatusForbidden, "Sourcegraph license expired", "To continue using Sourcegraph, a site admin must renew the Sourcegraph license (or downgrade to only using Sourcegraph Free features). Update the license key in the [**site configuration**](/site-admin/configuration).")
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	go func() {
		t := time.NewTicker(5 * time.Second)
		for range t.C {
			allowAccessByDefault, authzProviders, _, _ :=
				eiauthz.ProvidersFromConfig(ctx, conf.Get(), database.GlobalExternalServices)
			authz.SetProviders(allowAccessByDefault, authzProviders)
		}
	}()

	enterpriseServices.AuthzResolver = resolvers.NewResolver(db, timeutil.Now)
	return nil
}

func init() {
	// Report any authz provider problems in external configs.
	conf.ContributeWarning(func(cfg conf.Unified) (problems conf.Problems) {
		_, _, seriousProblems, warnings :=
			eauthz.ProvidersFromConfig(context.Background(), &cfg, database.GlobalExternalServices)
		problems = append(problems, conf.NewExternalServiceProblems(seriousProblems...)...)
		problems = append(problems, conf.NewExternalServiceProblems(warnings...)...)
		return problems
	})
}
