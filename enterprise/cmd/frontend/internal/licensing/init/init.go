package init

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/app"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	_ "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/dotcom/productsubscription"
	enterpriseGraphQL "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/licensing/enforcement"
	_ "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/registry"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/licensing"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/licensing/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
)

// TODO(efritz) - de-globalize assignments in this function
// TODO(efritz) - refactor licensing packages - this is a huge mess!
func Init(ctx context.Context, db dbutil.DB, outOfBandMigrationRunner *oobmigration.Runner, enterpriseServices *enterprise.Services) error {
	// Enforce the license's max user count by preventing the creation of new users when the max is
	// reached.
	database.GlobalUsers.BeforeCreateUser = enforcement.NewBeforeCreateUserHook(&usersStore{})

	// Enforce non-site admin roles in Free tier.
	database.GlobalUsers.AfterCreateUser = enforcement.NewAfterCreateUserHook()
	database.GlobalUsers.BeforeSetUserIsSiteAdmin = enforcement.NewBeforeSetUserIsSiteAdmin()

	// Enforce the license's max external service count by preventing the creation of new external
	// services when the max is reached.
	database.GlobalExternalServices.PreCreateExternalService = enforcement.NewPreCreateExternalServiceHook(&externalServicesStore{})

	// Enforce the license's feature check for monitoring. If the license does not support the monitoring
	// feature, then alternative debug handlers will be invoked.
	app.SetPreMountGrafanaHook(enforcement.NewPreMountGrafanaHook())

	// Make the Site.productSubscription.productNameWithBrand GraphQL field (and other places) use the
	// proper product name.
	graphqlbackend.GetProductNameWithBrand = licensing.ProductNameWithBrand

	enterpriseGraphQL.InitDotcom(db)

	globals.WatchBranding(func() error {
		if !licensing.EnforceTiers {
			return nil
		}
		return licensing.Check(licensing.FeatureBranding)
	})

	graphqlbackend.AlertFuncs = append(graphqlbackend.AlertFuncs, func(args graphqlbackend.AlertFuncArgs) []*graphqlbackend.Alert {
		if !licensing.EnforceTiers {
			return nil
		}

		// Only site admins can act on this alert, so only show it to site admins.
		if !args.IsSiteAdmin {
			return nil
		}

		if licensing.IsFeatureEnabledLenient(licensing.FeatureBranding) {
			return nil
		}

		if conf.Get().Branding == nil {
			return nil
		}

		return []*graphqlbackend.Alert{{
			TypeValue:    graphqlbackend.AlertTypeError,
			MessageValue: "A Sourcegraph license is required to custom branding for the instance. [**Get a license.**](/site-admin/license)",
		}}
	})

	// Make the Site.productSubscription.actualUserCount and Site.productSubscription.actualUserCountDate
	// GraphQL fields return the proper max user count and timestamp on the current license.
	graphqlbackend.ActualUserCount = licensing.ActualUserCount
	graphqlbackend.ActualUserCountDate = licensing.ActualUserCountDate

	noLicenseMaximumAllowedUserCount := licensing.NoLicenseMaximumAllowedUserCount
	graphqlbackend.NoLicenseMaximumAllowedUserCount = &noLicenseMaximumAllowedUserCount

	noLicenseWarningUserCount := licensing.NoLicenseWarningUserCount
	graphqlbackend.NoLicenseWarningUserCount = &noLicenseWarningUserCount

	// Make the Site.productSubscription GraphQL field return the actual info about the product license,
	// if any.
	graphqlbackend.GetConfiguredProductLicenseInfo = func() (*graphqlbackend.ProductLicenseInfo, error) {
		info, err := licensing.GetConfiguredProductLicenseInfo()
		if info == nil || err != nil {
			return nil, err
		}
		return &graphqlbackend.ProductLicenseInfo{
			TagsValue:      info.Tags,
			UserCountValue: info.UserCount,
			ExpiresAtValue: info.ExpiresAt,
		}, nil
	}

	enterpriseServices.LicenseResolver = resolvers.LicenseResolver{}

	goroutine.Go(func() {
		licensing.StartMaxUserCount(&usersStore{})
	})
	if envvar.SourcegraphDotComMode() {
		goroutine.Go(productsubscription.StartCheckForUpcomingLicenseExpirations)
	}

	return nil
}

type usersStore struct{}

func (usersStore) Count(ctx context.Context) (int, error) {
	return database.GlobalUsers.Count(ctx, nil)
}

type externalServicesStore struct{}

func (externalServicesStore) Count(ctx context.Context, opts database.ExternalServicesListOptions) (int, error) {
	return database.GlobalExternalServices.Count(ctx, opts)
}
