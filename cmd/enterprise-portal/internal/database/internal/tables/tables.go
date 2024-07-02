package tables

import (
	"gorm.io/gorm/schema"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/codyaccess"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/subscriptions"
)

// All tables provisioned for the Enterprise Portal database are defined here.
//
// ⚠️ WARNING: This list is meant to be read-only.
func All() []schema.Tabler {
	return []schema.Tabler{
		&subscriptions.Subscription{},
		&subscriptions.SubscriptionCondition{},
		&subscriptions.SubscriptionLicense{},
		&subscriptions.SubscriptionLicenseCondition{},

		&codyaccess.CodyGatewayAccess{},
	}
}
