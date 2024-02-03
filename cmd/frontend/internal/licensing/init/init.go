package init

import (
	"context"
	"fmt"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/hooks"
	_ "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/dotcom/productsubscription"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/licensing/enforcement"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/licensing/resolvers"
	_ "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/registry"
	"github.com/sourcegraph/sourcegraph/internal/codeintel"
	confLib "github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/usagestats"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// TODO(efritz) - de-globalize assignments in this function
// TODO(efritz) - refactor licensing packages - this is a huge mess!
func Init(
	ctx context.Context,
	observationCtx *observation.Context,
	db database.DB,
	codeIntelServices codeintel.Services,
	conf conftypes.UnifiedWatchable,
	enterpriseServices *enterprise.Services,
) error {
	// Enforce the license's max user count by preventing the creation of new users when the max is
	// reached.
	database.BeforeCreateUser = enforcement.NewBeforeCreateUserHook()

	// Enforce non-site admin roles in Free tier.
	database.AfterCreateUser = enforcement.NewAfterCreateUserHook()

	// Enforce site admin creation rules.
	database.BeforeSetUserIsSiteAdmin = enforcement.NewBeforeSetUserIsSiteAdmin()

	// Enforce the license's max external service count by preventing the creation of new external
	// services when the max is reached.
	database.BeforeCreateExternalService = enforcement.NewBeforeCreateExternalServiceHook()

	logger := log.Scoped("licensing")

	// Surface basic, non-sensitive information about the license type. This information
	// can be used to soft-gate features from the UI, and to provide info to admins from
	// site admin settings pages in the UI.
	hooks.GetLicenseInfo = func() *hooks.LicenseInfo {
		info, err := licensing.GetConfiguredProductLicenseInfo()
		if err != nil {
			logger.Error("Failed to get license info", log.Error(err))
			return nil
		}

		licenseInfo := &hooks.LicenseInfo{
			CurrentPlan: string(info.Plan()),
		}
		if info.Plan() == licensing.PlanBusiness0 {
			const codeScaleLimit = 100 * 1024 * 1024 * 1024
			licenseInfo.CodeScaleLimit = "100GiB"

			stats, err := usagestats.GetRepositories(ctx, db)
			if err != nil {
				logger.Error("Failed to get repository stats", log.Error(err))
				return nil
			}

			if stats.GitDirBytes >= codeScaleLimit {
				licenseInfo.CodeScaleExceededLimit = true
			} else if stats.GitDirBytes >= codeScaleLimit*0.9 {
				licenseInfo.CodeScaleCloseToLimit = true
			}
		}

		// returning this only makes sense on dotcom
		if envvar.SourcegraphDotComMode() {
			for _, plan := range licensing.AllPlans {
				licenseInfo.KnownLicenseTags = append(licenseInfo.KnownLicenseTags, fmt.Sprintf("plan:%s", plan))
			}
			for _, feature := range licensing.AllFeatures {
				licenseInfo.KnownLicenseTags = append(licenseInfo.KnownLicenseTags, feature.FeatureName())
			}
			licenseInfo.KnownLicenseTags = append(licenseInfo.KnownLicenseTags, licensing.MiscTags...)
		} else { // returning BC info only makes sense on non-dotcom
			bcFeature := &licensing.FeatureBatchChanges{}
			if err := licensing.Check(bcFeature); err == nil {
				if bcFeature.Unrestricted {
					licenseInfo.BatchChanges = &hooks.FeatureBatchChanges{
						Unrestricted: true,
						// Superceded by being unrestricted
						MaxNumChangesets: -1,
					}
				} else {
					max := int(bcFeature.MaxNumChangesets)
					licenseInfo.BatchChanges = &hooks.FeatureBatchChanges{
						MaxNumChangesets: max,
					}
				}
			}
		}

		licenseInfo.Features = hooks.LicenseFeatures{
			CodeSearch:     licensing.Check(licensing.FeatureCodeSearch) == nil,
			Cody:           licensing.Check(licensing.FeatureCody) == nil,
			AllowAirGapped: licensing.Check(licensing.FeatureAllowAirGapped) == nil,
		}

		return licenseInfo
	}

	// Make the Site.productSubscription.productNameWithBrand GraphQL field (and other places) use the
	// proper product name.
	graphqlbackend.GetProductNameWithBrand = licensing.ProductNameWithBrand

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
		hashedKeyValue := confLib.HashedCurrentLicenseKeyForAnalytics()
		return &graphqlbackend.ProductLicenseInfo{
			TagsValue:                    info.Tags,
			UserCountValue:               info.UserCount,
			ExpiresAtValue:               info.ExpiresAt,
			IsValidValue:                 licensing.IsLicenseValid(),
			LicenseInvalidityReasonValue: pointers.NonZeroPtr(licensing.GetLicenseInvalidReason()),
			HashedKeyValue:               &hashedKeyValue,
		}, nil
	}

	graphqlbackend.IsFreePlan = func(info *graphqlbackend.ProductLicenseInfo) bool {
		for _, tag := range info.Tags() {
			if tag == fmt.Sprintf("plan:%s", licensing.PlanFree0) || tag == fmt.Sprintf("plan:%s", licensing.PlanFree1) {
				return true
			}
		}

		return false
	}

	enterpriseServices.LicenseResolver = resolvers.LicenseResolver{}

	if envvar.SourcegraphDotComMode() {
		goroutine.Go(func() {
			productsubscription.StartCheckForUpcomingLicenseExpirations(logger, db)
		})
		goroutine.Go(func() {
			productsubscription.StartCheckForAnomalousLicenseUsage(logger, db)
		})
	}

	return nil
}
