package enforcement

import (
	"context"
	"fmt"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/cloud"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NewBeforeCreateUserHook returns a BeforeCreateUserHook closure with the given UsersStore
// that determines whether new user is allowed to be created.
func NewBeforeCreateUserHook() func(context.Context, database.DB, *extsvc.AccountSpec) error {
	return func(ctx context.Context, db database.DB, spec *extsvc.AccountSpec) error {
		// Exempt user accounts that are created by the Sourcegraph Operator
		// authentication provider.
		//
		// NOTE: It is important to make sure the Sourcegraph Operator authentication
		// provider is actually enabled.
		if spec != nil && spec.ServiceType == auth.SourcegraphOperatorProviderType &&
			cloud.SiteConfig().SourcegraphOperatorAuthProviderEnabled() {
			return nil
		}

		info, err := licensing.GetConfiguredProductLicenseInfo()
		if err != nil {
			return err
		}
		var licensedUserCount int32
		if info != nil {
			// We prevent creating new users when the license is expired because we do not want
			// all new users to be promoted as site admins automatically until the customer
			// decides to downgrade to Free tier.
			if info.IsExpired() {
				return errcode.NewPresentationError("Unable to create user account: Sourcegraph license expired! No new users can be created. Update the license key in the [**site configuration**](/site-admin/configuration) or downgrade to only using Sourcegraph Free features.")
			}
			licensedUserCount = int32(info.UserCount)
		} else {
			licensedUserCount = licensing.NoLicenseMaximumAllowedUserCount
		}

		// Block creation of a new user beyond the licensed user count (unless true-up is allowed).
		userCount, err := db.Users().Count(ctx, &database.UsersListOptions{
			ExcludeSourcegraphOperators: true,
		})
		if err != nil {
			return err
		}
		// Be conservative and treat 0 as unlimited. We don't plan to intentionally generate
		// licenses with UserCount == 0, but that might result from a bug in license decoding, and
		// we don't want that to immediately disable Sourcegraph instances.
		if licensedUserCount > 0 && int32(userCount) >= licensedUserCount {
			if info != nil && info.HasTag(licensing.TrueUpUserCountTag) {
				log15.Info("Licensed user count exceeded, but license supports true-up and will not block creation of new user. The new user will be retroactively charged for in the next billing period. Contact sales@sourcegraph.com for help.", "activeUserCount", userCount, "licensedUserCount", licensedUserCount)
			} else {
				message := "Unable to create user account: "
				if info == nil {
					message += fmt.Sprintf("a Sourcegraph subscription is required to exceed %d users (this instance now has %d users). Contact Sourcegraph to learn more at https://sourcegraph.com/contact/sales.", licensing.NoLicenseMaximumAllowedUserCount, userCount)
				} else {
					message += "the Sourcegraph subscription's maximum user count has been reached. A site admin must upgrade the Sourcegraph subscription to allow for more users. Contact Sourcegraph at https://sourcegraph.com/contact/sales."
				}
				return errcode.NewPresentationError(message)
			}
		}

		return nil
	}
}

// NewAfterCreateUserHook returns a AfterCreateUserHook closure that determines whether
// a new user should be promoted to site admin based on the product license.
func NewAfterCreateUserHook() func(context.Context, database.DB, *types.User) error {
	return func(ctx context.Context, tx database.DB, user *types.User) error {
		// 🚨 SECURITY: To be extra safe that we never promote any new user to be site admin on Sourcegraph Cloud.
		if dotcom.SourcegraphDotComMode() {
			return nil
		}
		info, err := licensing.GetConfiguredProductLicenseInfo()
		if err != nil {
			return err
		}

		if info.Plan().IsFreePlan() {
			store := tx.Users()
			user.SiteAdmin = true
			if err := store.SetIsSiteAdmin(ctx, user.ID, user.SiteAdmin); err != nil {
				return err
			}
		}

		return nil
	}
}

// NewBeforeSetUserIsSiteAdmin returns a BeforeSetUserIsSiteAdmin closure that determines whether
// the creation or removal of site admins are allowed.
func NewBeforeSetUserIsSiteAdmin() func(ctx context.Context, isSiteAdmin bool) error {
	return func(ctx context.Context, isSiteAdmin bool) error {
		// Exempt user accounts that are created by the Sourcegraph Operator
		// authentication provider.
		//
		// NOTE: It is important to make sure the Sourcegraph Operator authentication
		// provider is actually enabled.
		if cloud.SiteConfig().SourcegraphOperatorAuthProviderEnabled() && actor.FromContext(ctx).SourcegraphOperator {
			return nil
		}

		info, err := licensing.GetConfiguredProductLicenseInfo()
		if err != nil {
			return err
		}

		if info != nil {
			if info.IsExpired() {
				return errors.New("The Sourcegraph license has expired. No site-admins can be created until the license is updated.")
			}
			if !info.Plan().IsFreePlan() {
				return nil
			}

			// Allow users to be promoted to site admins on the Free plan.
			if info.Plan().IsFreePlan() && isSiteAdmin {
				return nil
			}
		}

		return licensing.NewFeatureNotActivatedError(fmt.Sprintf("The feature %q is not activated because it requires a valid Sourcegraph license. Purchase a Sourcegraph subscription to activate this feature.", "non-site admin roles"))
	}
}
