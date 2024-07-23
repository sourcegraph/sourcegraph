package subscriptions

import (
	"context"

	"github.com/jackc/pgx/v5"
	"gorm.io/gorm"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/utctime"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// Subscription is an Enterprise subscription condition record.
type SubscriptionCondition struct {
	// SubscriptionID is the internal unprefixed UUID of the related subscription.
	SubscriptionID string `gorm:"type:uuid;not null"`
	// Status is the type of status corresponding to this condition, corresponding
	// to the API 'EnterpriseSubscriptionCondition.Status'.
	Status string `gorm:"not null"`
	// Message is a human-readable message associated with the condition.
	Message *string `gorm:"size:256"`
	// TransitionTime is the time at which the condition was created, i.e. when
	// the subscription transitioned into this status.
	TransitionTime utctime.Time `gorm:"not null;default:current_timestamp"`
}

func (s *SubscriptionCondition) TableName() string {
	return "enterprise_portal_subscription_conditions"
}

func (t *SubscriptionCondition) RunCustomMigrations(migrator gorm.Migrator) error {
	// Drop the old, generated conditions -> subscription constraint
	if c := "fk_enterprise_portal_subscription_conditions_subscription"; migrator.HasConstraint(t, c) {
		if err := migrator.DropConstraint(t, c); err != nil {
			return err
		}
	}
	return nil
}

// subscriptionConditionJSONBAgg must be used with:
//
//	LEFT JOIN
//		enterprise_portal_subscription_conditions subscription_condition
//		ON subscription_condition.subscription_id = id
//	GROUP BY
//		id
//
// The conditions are aggregated in JSON to 'conditions', which can be directly
// unmarshaled into the 'SubscriptionCondition' type using 'pgx'.
func subscriptionConditionJSONBAgg() string {
	return `
jsonb_agg(
	jsonb_build_object(
		'Status', subscription_condition.status,
		'Message', subscription_condition.message,
		'TransitionTime', subscription_condition.transition_time
	)
	ORDER BY subscription_condition.transition_time DESC
) AS conditions`
}

type subscriptionConditionsStore struct{ tx pgx.Tx }

// newSubscriptionConditionsStore is meant to be used exclusively in the context of
// a transaction, where the parent subscription is being updated at the same time.
//
// The caller owns the transaction lifecycle.
func newSubscriptionConditionsStore(tx pgx.Tx) *subscriptionConditionsStore {
	return &subscriptionConditionsStore{tx: tx}
}

type CreateSubscriptionConditionOptions struct {
	Status         subscriptionsv1.EnterpriseSubscriptionCondition_Status
	Message        string
	TransitionTime utctime.Time
}

func (s *subscriptionConditionsStore) createSubscriptionCondition(ctx context.Context, subscriptionID string, opts CreateSubscriptionConditionOptions) error {
	if opts.TransitionTime.GetTime().IsZero() {
		return errors.New("transition time is required")
	}
	_, err := s.tx.Exec(ctx, `
INSERT INTO enterprise_portal_subscription_conditions (
	subscription_id,
	status,
	message,
	transition_time
)
VALUES (
	@subscriptionID,
	@status,
	@message,
	@transitionTime
)`, pgx.NamedArgs{
		"subscriptionID": subscriptionID,
		// Convert to string representation of EnterpriseSubscriptionCondition
		"status":         opts.Status.String(),
		"message":        pointers.NilIfZero(opts.Message),
		"transitionTime": opts.TransitionTime,
	})
	return err
}
