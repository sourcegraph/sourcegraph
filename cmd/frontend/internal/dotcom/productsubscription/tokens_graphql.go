package productsubscription

import (
	"context"
	"fmt"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	internalproductsubscription "github.com/sourcegraph/sourcegraph/internal/productsubscription"
)

type ErrProductSubscriptionNotFound struct {
	err error
}

func (e ErrProductSubscriptionNotFound) Error() string {
	if e.err == nil {
		return "product subscription not found"
	}
	return fmt.Sprintf("product subscription not found: %v", e.err)
}

func (e ErrProductSubscriptionNotFound) Extensions() map[string]any {
	return map[string]any{"code": internalproductsubscription.GQLErrCodeProductSubscriptionNotFound}
}

// ProductSubscriptionByAccessToken retrieves the subscription corresponding to the
// given access token.
func (r ProductSubscriptionLicensingResolver) ProductSubscriptionByAccessToken(ctx context.Context, args *graphqlbackend.ProductSubscriptionByAccessTokenArgs) (graphqlbackend.ProductSubscription, error) {
	// 🚨 SECURITY: Only specific entities may use this functionality.
	if _, err := serviceAccountOrLicenseManager(ctx, r.DB, false); err != nil {
		return nil, err
	}

	subID, err := newDBTokens(r.DB).LookupProductSubscriptionIDByAccessToken(ctx, args.AccessToken)
	if err != nil {
		if errcode.IsNotFound(err) {
			return nil, ErrProductSubscriptionNotFound{err}
		}
		return nil, err
	}

	v, err := dbSubscriptions{db: r.DB}.GetByID(ctx, subID)
	if err != nil {
		if err == errSubscriptionNotFound {
			return nil, ErrProductSubscriptionNotFound{err}
		}
		return nil, err
	}
	return &productSubscription{logger: r.Logger, v: v, db: r.DB}, nil
}
