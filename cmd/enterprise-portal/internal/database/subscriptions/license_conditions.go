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

type SubscriptionLicenseCondition struct {
	// SubscriptionID is the internal unprefixed UUID of the related license.
	LicenseID string `gorm:"type:uuid;not null"`
	// Status is the type of status corresponding to this condition, corresponding
	// to the API 'EnterpriseSubscriptionLicenseCondition.Status'.
	Status string `gorm:"not null"`
	// Message is a human-readable message associated with the condition.
	Message *string `gorm:"size:256"`
	// TransitionTime is the time at which the condition was created, i.e. when
	// the license transitioned into this status.
	TransitionTime utctime.Time `gorm:"not null;default:current_timestamp"`
}

func (*SubscriptionLicenseCondition) TableName() string {
	return "enterprise_portal_subscription_license_conditions"
}

func (t *SubscriptionLicenseCondition) RunCustomMigrations(migrator gorm.Migrator) error {
	// Drop the old, generated conditions -> license constraint
	if c := "fk_enterprise_portal_subscription_license_conditions_license"; migrator.HasConstraint(t, c) {
		if err := migrator.DropConstraint(t, c); err != nil {
			return err
		}
	}
	return nil
}

// subscriptionLicenseConditionJSONBAgg must be used with:
//
//	LEFT JOIN
//		enterprise_portal_subscription_license_conditions license_condition
//		ON license_condition.license_id = id
//	GROUP BY
//		id
//
// The conditions are aggregated in JSON to 'conditions', which can be directly
// unmarshaled into the 'SubscriptionLicenseCondition' type using 'pgx'.
func subscriptionLicenseConditionJSONBAgg() string {
	return `
jsonb_agg(
	jsonb_build_object(
		'Status', license_condition.status,
		'Message', license_condition.message,
		'TransitionTime', license_condition.transition_time
	)
	ORDER BY license_condition.transition_time DESC
) AS conditions`
}

type licenseConditionsStore struct{ tx pgx.Tx }

// newLicenseConditionsStore is meant to be used exclusively in the context of
// a transaction, where the parent license is being updated at the same time.
//
// The caller owns the transaction lifecycle.
func newLicenseConditionsStore(tx pgx.Tx) *licenseConditionsStore {
	return &licenseConditionsStore{tx: tx}
}

type createLicenseConditionOpts struct {
	Status         subscriptionsv1.EnterpriseSubscriptionLicenseCondition_Status
	Message        string
	TransitionTime utctime.Time
}

func (s *licenseConditionsStore) createLicenseCondition(ctx context.Context, licenseID string, opts createLicenseConditionOpts) error {
	if opts.TransitionTime.GetTime().IsZero() {
		return errors.New("transition time is required")
	}
	_, err := s.tx.Exec(ctx, `
INSERT INTO enterprise_portal_subscription_license_conditions (
	license_id,
	status,
	message,
	transition_time
)
VALUES (
	@licenseID,
	@status,
	@message,
	@transitionTime
)`, pgx.NamedArgs{
		"licenseID": licenseID,
		// Convert to string representation of EnterpriseSubscriptionLicenseCondition
		"status":         opts.Status.String(),
		"message":        pointers.NilIfZero(opts.Message),
		"transitionTime": opts.TransitionTime,
	})
	return err
}
