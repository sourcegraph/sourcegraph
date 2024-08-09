package subscriptions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/internal/pgxerrors"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/utctime"
	internallicense "github.com/sourcegraph/sourcegraph/internal/license"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/pointers"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// ⚠️ DO NOT USE: This type is only used for creating foreign key constraints
// and initializing tables with gorm.
type TableSubscriptionLicense struct {
	// ⚠️ DO NOT USE: This field is only used for creating foreign key constraint.
	Conditions []*SubscriptionLicenseCondition `gorm:"foreignKey:LicenseID"`

	SubscriptionLicense
}

func (*TableSubscriptionLicense) TableName() string {
	return "enterprise_portal_subscription_licenses"
}

// Implement tables.CustomMigrator
func (t *TableSubscriptionLicense) RunCustomMigrations(migrator gorm.Migrator) error {
	if migrator.HasColumn(t, "license_kind") {
		if err := migrator.DropColumn(t, "license_kind"); err != nil {
			return err
		}
	}
	// Drop the old, generated license -> subscription constraint
	if c := "fk_enterprise_portal_subscription_licenses_subscription"; migrator.HasConstraint(t, c) {
		if err := migrator.DropConstraint(t, c); err != nil {
			return err
		}
	}
	return nil
}

type SubscriptionLicense struct {
	// SubscriptionID is the internal unprefixed UUID of the related subscription.
	SubscriptionID string `gorm:"type:uuid;not null"`
	// ID is the internal unprefixed UUID of this license.
	ID string `gorm:"type:uuid;primaryKey"`

	// Timestamps representing the latest timestamps of key conditions related
	// to this subscription.
	//
	// Condition transition details are tracked in 'enterprise_portal_subscription_license_conditions'.
	CreatedAt utctime.Time  `gorm:"not null;default:current_timestamp"`
	RevokedAt *utctime.Time // Null indicates the license is not revoked.

	// ExpireAt is the time at which the license should expire. Expiration does
	// NOT get a corresponding condition entry in 'enterprise_portal_subscription_license_conditions'.
	ExpireAt utctime.Time `gorm:"not null"`

	// LicenseType is the kind of license stored in LicenseData, corresponding
	// to the API 'EnterpriseSubscriptionLicenseType'.
	LicenseType string `gorm:"not null"`
	// LicenseData is the license data stored in JSON format. It is read-only
	// and generally never queried in conditions - properties that are should
	// be stored at the subscription or license level.
	//
	// Value shapes correspond to API types appropriate for each
	// 'EnterpriseSubscriptionLicenseType'.
	LicenseData json.RawMessage `gorm:"type:jsonb"`
}

// subscriptionLicenseWithConditionsColumns must match scanSubscriptionLicense()
// values.
func subscriptionLicenseWithConditionsColumns() []string {
	return []string{
		"subscription_id",
		"id",

		"created_at",
		"revoked_at",
		"expire_at",

		"license_type",
		"license_data",

		subscriptionLicenseConditionJSONBAgg(),
	}
}

type LicenseWithConditions struct {
	SubscriptionLicense
	Conditions []SubscriptionLicenseCondition
}

// scanSubscription matches subscriptionTableColumns() values.
func scanSubscriptionLicenseWithConditions(row pgx.Row) (*LicenseWithConditions, error) {
	var l LicenseWithConditions
	err := row.Scan(
		&l.SubscriptionID,
		&l.ID,
		&l.CreatedAt,
		&l.RevokedAt,
		&l.ExpireAt,
		&l.LicenseType,
		&l.LicenseData,
		&l.Conditions, // see subscriptionLicenseConditionJSONBAgg docstring
	)
	return &l, err
}

// LicensesStore manages licenses belonging to Enterprise subscriptions.
//
// Licenses can only be created and revoked - they can never be updated.
type LicensesStore struct {
	db *pgxpool.Pool
}

func NewLicensesStore(db *pgxpool.Pool) *LicensesStore {
	return &LicensesStore{
		db: db,
	}
}

type ListLicensesOpts struct {
	SubscriptionID          string
	LicenseType             subscriptionsv1.EnterpriseSubscriptionLicenseType
	LicenseKeySubstring     string
	SalesforceOpportunityID string
	// PageSize is the maximum number of licenses to return.
	PageSize int
}

func (opts ListLicensesOpts) toQueryConditions() (where, limitClause string, _ pgx.NamedArgs) {
	whereConds := []string{"TRUE"}
	namedArgs := pgx.NamedArgs{}
	if opts.SubscriptionID != "" {
		whereConds = append(whereConds, "subscription_id = @subscriptionID")
		namedArgs["subscriptionID"] = opts.SubscriptionID
	}
	if opts.LicenseType > 0 {
		whereConds = append(whereConds,
			"license_type = @licenseType")
		namedArgs["licenseType"] = opts.LicenseType.String()
	}

	switch opts.LicenseType {
	case subscriptionsv1.EnterpriseSubscriptionLicenseType_ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_KEY:
		if opts.LicenseKeySubstring != "" {
			whereConds = append(whereConds,
				"license_data->>'SignedKey' LIKE  '%' || @licenseKeySubstring || '%'")
			namedArgs["licenseKeySubstring"] = opts.LicenseKeySubstring
		}
		if opts.SalesforceOpportunityID != "" {
			whereConds = append(whereConds,
				"license_data->'Info'->>'sf_opp_id' = @salesforceOpportunityID")
			namedArgs["salesforceOpportunityID"] = opts.SalesforceOpportunityID
		}
	}

	where = strings.Join(whereConds, " AND ")

	if opts.PageSize > 0 {
		limitClause = "LIMIT @pageSize"
		namedArgs["pageSize"] = opts.PageSize
	}
	return where, limitClause, namedArgs
}

func (s *LicensesStore) List(ctx context.Context, opts ListLicensesOpts) ([]*LicenseWithConditions, error) {
	where, limitClause, namedArgs := opts.toQueryConditions()
	query := fmt.Sprintf(`
SELECT
	%s
FROM
	enterprise_portal_subscription_licenses
LEFT JOIN
	enterprise_portal_subscription_license_conditions license_condition
	ON license_condition.license_id = id
WHERE
	%s
GROUP BY
	id
ORDER BY
	created_at DESC
%s`,
		strings.Join(subscriptionLicenseWithConditionsColumns(), ", "),
		where, limitClause)

	rows, err := s.db.Query(ctx, query, namedArgs)
	if err != nil {
		return nil, errors.Wrap(err, "query rows")
	}
	defer rows.Close()

	var licenses []*LicenseWithConditions
	for rows.Next() {
		license, err := scanSubscriptionLicenseWithConditions(rows)
		if err != nil {
			return nil, errors.Wrap(err, "scan row")
		}
		licenses = append(licenses, license)
	}
	return licenses, rows.Err()
}

var ErrSubscriptionLicenseNotFound = errors.New("subscription license not found")

func (s *LicensesStore) Get(ctx context.Context, licenseID string) (*LicenseWithConditions, error) {
	query := fmt.Sprintf(`
SELECT
	%s
FROM
	enterprise_portal_subscription_licenses
LEFT JOIN
	enterprise_portal_subscription_license_conditions license_condition
	ON license_condition.license_id = id
WHERE
	id = @licenseID
GROUP BY
	id`,
		strings.Join(subscriptionLicenseWithConditionsColumns(), ", "))

	license, err := scanSubscriptionLicenseWithConditions(
		s.db.QueryRow(ctx, query, pgx.NamedArgs{
			"licenseID": licenseID,
		}),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.WithStack(ErrSubscriptionLicenseNotFound)
		}
		return nil, errors.Wrap(err, "query rows")
	}
	return license, nil
}

type CreateLicenseOpts struct {
	Message string
	// If nil, the creation time will be set to the current time.
	Time *utctime.Time
	// Expiration time of the license.
	ExpireTime utctime.Time

	// ImportLicenseID can be provided to avoid generating a new license ID. Should
	// only be used on import.
	ImportLicenseID string
}

func (c CreateLicenseOpts) getLicenseID() (string, error) {
	if c.ImportLicenseID != "" {
		if _, err := uuid.Parse(c.ImportLicenseID); err != nil {
			return "", errors.Wrap(err, "invalid license ID")
		}
		return c.ImportLicenseID, nil
	}
	licenseID, err := uuid.NewV7()
	if err != nil {
		return "", errors.Wrap(err, "generate uuid")
	}
	return licenseID.String(), nil
}

// DataLicenseKey corresponds to *subscriptionsv1.EnterpriseSubscriptionLicenseKey
// and the 'ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_KEY' license type.
type DataLicenseKey struct {
	Info internallicense.Info
	// Signed license key with the license information in Info.
	SignedKey string
}

// CreateLicense creates a new classic offline license for the given subscription.
func (s *LicensesStore) CreateLicenseKey(
	ctx context.Context,
	subscriptionID string,
	license *DataLicenseKey,
	opts CreateLicenseOpts,
) (_ *LicenseWithConditions, err error) {
	// Special behaviour: the license key embeds the creation time, and it must
	// match the time provided in the options.
	if opts.Time == nil {
		return nil, errors.New("creation time must be specified for licensekeys")
	} else if !opts.Time.GetTime().Equal(utctime.FromTime(license.Info.CreatedAt).AsTime()) {
		return nil, errors.New("creation time must match the license key information")
	}
	if !opts.ExpireTime.GetTime().Equal(utctime.FromTime(license.Info.ExpiresAt).AsTime()) {
		return nil, errors.New("expiration time must match the license key information")
	}

	return s.create(
		ctx,
		subscriptionID,
		subscriptionsv1.EnterpriseSubscriptionLicenseType_ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_KEY,
		license,
		opts,
	)
}

func (s *LicensesStore) create(
	ctx context.Context,
	subscriptionID string,
	licenseType subscriptionsv1.EnterpriseSubscriptionLicenseType,
	license any,
	opts CreateLicenseOpts,
) (_ *LicenseWithConditions, err error) {
	if subscriptionID == "" {
		return nil, errors.New("subscription ID must be specified")
	}
	if opts.Time == nil {
		opts.Time = pointers.Ptr(utctime.Now())
	} else if opts.Time.GetTime().After(time.Now()) {
		return nil, errors.New("creation time cannot be in the future")
	}
	if licenseType == subscriptionsv1.EnterpriseSubscriptionLicenseType_ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_UNSPECIFIED {
		return nil, errors.New("license type must be specified")
	}

	licenseID, err := opts.getLicenseID()
	if err != nil {
		return nil, err
	}
	licenseData, err := json.Marshal(license)
	if err != nil {
		return nil, errors.Wrap(err, "marshal license data")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "begin transaction")
	}
	defer func() {
		if rollbackErr := tx.Rollback(context.Background()); rollbackErr != nil {
			err = errors.Append(err, errors.Wrap(err, "rollback"))
		}
	}()

	if _, err := tx.Exec(ctx, `
INSERT INTO enterprise_portal_subscription_licenses (
	id,
	subscription_id,
	license_type,
	license_data,
	created_at,
	expire_at
)
VALUES (
	@licenseID,
	@subscriptionID,
	@licenseType,
	@licenseData,
	@createdAt,
	@expireAt
)
`, pgx.NamedArgs{
		"licenseID":      licenseID,
		"subscriptionID": subscriptionID,
		"licenseType":    licenseType.String(),
		"licenseData":    licenseData,
		"createdAt":      opts.Time,
		"expireAt":       opts.ExpireTime,
	}); err != nil {
		if pgxerrors.IsContraintError(err, "fk_enterprise_portal_subscriptions_licenses") {
			return nil, errors.WithSafeDetails(
				errors.WithStack(ErrSubscriptionNotFound),
				"subscription %s: %+v", subscriptionID, err)
		}
		return nil, errors.Wrap(err, "create license")
	}

	if err := newLicenseConditionsStore(tx).createLicenseCondition(ctx, licenseID, createLicenseConditionOpts{
		Status:         subscriptionsv1.EnterpriseSubscriptionLicenseCondition_STATUS_CREATED,
		Message:        opts.Message,
		TransitionTime: *opts.Time,
	}); err != nil {
		return nil, errors.Wrap(err, "create license condition")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, "commit transaction")
	}

	return s.Get(ctx, licenseID)
}

type RevokeLicenseOpts struct {
	Message string
	// If nil, the revocation time will be set to the current time.
	Time *utctime.Time
}

// Revoke marks the given license as revoked.
func (s *LicensesStore) Revoke(ctx context.Context, licenseID string, opts RevokeLicenseOpts) (*LicenseWithConditions, error) {
	if opts.Time == nil {
		opts.Time = pointers.Ptr(utctime.Now())
	} else if opts.Time.GetTime().After(time.Now()) {
		return nil, errors.New("revocation time cannot be in the future")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "begin transaction")
	}
	defer func() {
		if rollbackErr := tx.Rollback(context.Background()); rollbackErr != nil {
			err = errors.Append(err, rollbackErr)
		}
	}()

	if _, err := tx.Exec(ctx, `
UPDATE enterprise_portal_subscription_licenses
SET revoked_at = COALESCE(revoked_at, @revokedAt) -- use existing revoke time if already revoked
WHERE id = @licenseID
`, pgx.NamedArgs{
		"revokedAt": opts.Time,
		"licenseID": licenseID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSubscriptionLicenseNotFound
		}
		return nil, errors.Wrap(err, "revoke license")
	}

	if err := newLicenseConditionsStore(tx).createLicenseCondition(ctx, licenseID, createLicenseConditionOpts{
		Status:         subscriptionsv1.EnterpriseSubscriptionLicenseCondition_STATUS_REVOKED,
		Message:        opts.Message,
		TransitionTime: *opts.Time,
	}); err != nil {
		return nil, errors.Wrap(err, "create license condition")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, "commit transaction")
	}

	return s.Get(ctx, licenseID)
}
