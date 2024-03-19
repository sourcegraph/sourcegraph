package productsubscription

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/rbac"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// 🚨 SECURITY: Use this to check if access to a subscription query or mutation
// is authorized for users with the correct RBAC permissions and site admins.
func hasRBACPermsOrSiteAdmin(
	ctx context.Context,
	db database.DB,
	requiresSubscriptionsWriter bool,
) (string, error) {
	return hasRBACPermsOrOwnerOrSiteAdmin(ctx, db, nil, requiresSubscriptionsWriter)
}

// 🚨 SECURITY: Use this to check if access to a subscription query or mutation
// is authorized for service accounts, the owning user, and site admins. Callers
// should record the returned grant reason in an audit log tracking the access.
func hasRBACPermsOrOwnerOrSiteAdmin(
	ctx context.Context,
	db database.DB,
	ownerUserID *int32,
	requiresSubscriptionsWriter bool,
) (string, error) {
	// 🚨 SECURITY: In all cases, being a subscription writer is sufficient.
	if rbac.CheckCurrentUserHasPermission(ctx, db,
		rbac.ProductSubscriptionsWritePermission) == nil {
		return rbac.ProductSubscriptionsWritePermission, nil
	}

	// 🚨 SECURITY: If we don't need write access, simply being a reader is sufficient.
	if !requiresSubscriptionsWriter {
		if rbac.CheckCurrentUserHasPermission(ctx, db,
			rbac.ProductSubscriptionsReadPermission) == nil {
			return rbac.ProductSubscriptionsReadPermission, nil
		}
	}

	// 🚨 SECURITY: The user does not have subscriptions permissions - but,
	// if ownerUserID is specified, we can grant access if the actor is the owner.
	if ownerUserID != nil {
		if auth.CheckSameUser(ctx, *ownerUserID) == nil {
			return "is_owner", nil
		}
	}

	// HACK: rbac.CheckCurrentUserHasPermission _should_ return true for site
	// admins, but this currently doesn't work in integration tests - for now,
	// we retain our legacy direct check on whether the user is a site admin
	// or not as a fallback, just in case.
	if auth.CheckCurrentUserIsSiteAdmin(ctx, db) == nil {
		return "site_admin", nil
	}

	// Otherwise, we are done - this user does not have access.
	return "unauthorized", errors.New("unauthorized")
}
