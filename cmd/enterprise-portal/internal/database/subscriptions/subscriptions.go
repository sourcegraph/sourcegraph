package subscriptions

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/internal/pgxerrors"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/internal/upsert"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/utctime"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// ⚠️ DO NOT USE: This type is only used for creating foreign key constraints
// and initializing tables with gorm.
type TableSubscription struct {
	// Each Subscription has many Licenses.
	Licenses []*TableSubscriptionLicense `gorm:"foreignKey:SubscriptionID"`

	// Each Subscription has many Conditions.
	Conditions []*SubscriptionCondition `gorm:"foreignKey:SubscriptionID"`

	Subscription
}

func (*TableSubscription) TableName() string {
	return "enterprise_portal_subscriptions"
}

// Subscription is an Enterprise subscription record.
type Subscription struct {
	// ID is the internal (unprefixed) UUID-format identifier for the subscription.
	ID string `gorm:"type:uuid;primaryKey"`
	// InstanceDomain is the instance domain associated with the subscription, e.g.
	// "acme.sourcegraphcloud.com". This is set explicitly.
	//
	// It must be unique across all currently un-archived subscriptions.
	InstanceDomain *string `gorm:"uniqueIndex:,where:archived_at IS NULL"`
	// InstanceType is the category of the instance associated with this
	// subscription, e.g. 'TYPE_PRIMARY' or 'TYPE_SECONDARY'.
	InstanceType *string

	// WARNING: The below fields are not yet used in production.

	// DisplayName is the human-friendly name of this subscription, e.g. "Acme, Inc."
	//
	// It must be unique across all currently un-archived subscriptions, unless
	// it is not set.
	DisplayName *string `gorm:"size:256;uniqueIndex:,where:archived_at IS NULL"`

	// Timestamps representing the latest timestamps of key conditions related
	// to this subscription.
	//
	// Condition transition details are tracked in 'enterprise_portal_subscription_conditions'.
	CreatedAt  utctime.Time  `gorm:"not null;default:current_timestamp"`
	UpdatedAt  utctime.Time  `gorm:"not null;default:current_timestamp"`
	ArchivedAt *utctime.Time // Null indicates the subscription is not archived.

	// SalesforceSubscriptionID associated with this Enterprise subscription.
	SalesforceSubscriptionID *string
}

type SubscriptionWithConditions struct {
	Subscription
	Conditions []SubscriptionCondition
}

// subscriptionTableColumns must match scanSubscription() values.
func subscriptionTableColumns() []string {
	return []string{
		"id",
		"instance_domain",
		"instance_type",
		"display_name",
		"created_at",
		"updated_at",
		"archived_at",
		"salesforce_subscription_id",

		subscriptionConditionJSONBAgg(),
	}
}

// scanSubscription matches subscriptionTableColumns() values.
func scanSubscription(row pgx.Row) (*SubscriptionWithConditions, error) {
	var s SubscriptionWithConditions
	err := row.Scan(
		&s.ID,
		&s.InstanceDomain,
		&s.InstanceType,
		&s.DisplayName,
		&s.CreatedAt,
		&s.UpdatedAt,
		&s.ArchivedAt,
		&s.SalesforceSubscriptionID,
		&s.Conditions,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Store is the storage layer for Enterprise subscriptions.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{
		db: db,
	}
}

// Licenses returns a new LicensesStore instance associated with the current
// Store's database connection.
func (s *Store) Licenses() *LicensesStore { return NewLicensesStore(s.db) }

// ListEnterpriseSubscriptionsOptions is the set of options to filter subscriptions.
// Non-empty fields are treated as AND-concatenated.
type ListEnterpriseSubscriptionsOptions struct {
	// IDs is a list of subscription IDs to filter by.
	IDs []string
	// InstanceDomains is a list of instance domains to filter by.
	InstanceDomains []string
	// IsArchived indicates whether to only list archived subscriptions, or only
	// non-archived subscriptions.
	IsArchived *bool
	// DisplayNameSubstring is a substring match on display name.
	DisplayNameSubstring string
	// SalesforceSubscriptionIDs are exact matches on the Salesforce subscription
	// ID.
	SalesforceSubscriptionIDs []string

	// PageSize is the maximum number of subscriptions to return.
	PageSize int
}

func (opts ListEnterpriseSubscriptionsOptions) toQueryConditions() (where, limit string, _ pgx.NamedArgs) {
	whereConds := []string{"TRUE"}
	namedArgs := pgx.NamedArgs{}
	if len(opts.IDs) > 0 {
		whereConds = append(whereConds, "id = ANY(@ids)")
		namedArgs["ids"] = opts.IDs
	}
	if len(opts.InstanceDomains) > 0 {
		whereConds = append(whereConds, "instance_domain = ANY(@instanceDomains)")
		namedArgs["instanceDomains"] = opts.InstanceDomains
	}
	if opts.IsArchived != nil {
		if *opts.IsArchived {
			whereConds = append(whereConds, "archived_at IS NOT NULL")
		} else {
			whereConds = append(whereConds, "archived_at IS NUlL")
		}
	}
	if len(opts.DisplayNameSubstring) > 0 {
		whereConds = append(whereConds,
			"LOWER(display_name) LIKE '%' || LOWER(@displayName) || '%'")
		namedArgs["displayName"] = opts.DisplayNameSubstring
	}
	if len(opts.SalesforceSubscriptionIDs) > 0 {
		whereConds = append(whereConds,
			"salesforce_subscription_id = ANY(@salesforceSubscriptionIDs)")
		namedArgs["salesforceSubscriptionIDs"] = opts.SalesforceSubscriptionIDs
	}

	where = strings.Join(whereConds, " AND ")

	if opts.PageSize > 0 {
		limit = "LIMIT @pageSize"
		namedArgs["pageSize"] = opts.PageSize
	}
	return where, limit, namedArgs
}

// List returns a list of subscriptions based on the given options.
func (s *Store) List(ctx context.Context, opts ListEnterpriseSubscriptionsOptions) ([]*SubscriptionWithConditions, error) {
	where, limit, namedArgs := opts.toQueryConditions()
	query := fmt.Sprintf(`
SELECT
	%s
FROM enterprise_portal_subscriptions
LEFT JOIN
    enterprise_portal_subscription_conditions subscription_condition
    ON subscription_condition.subscription_id = id
WHERE
	%s
GROUP BY
    id
ORDER BY
	created_at DESC -- TODO: parameterize order-by
%s`,
		strings.Join(subscriptionTableColumns(), ", "),
		where, limit,
	)
	rows, err := s.db.Query(ctx, query, namedArgs)
	if err != nil {
		return nil, errors.Wrap(err, "query rows")
	}
	defer rows.Close()

	var subscriptions []*SubscriptionWithConditions
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, errors.Wrap(err, "scan row")
		}
		subscriptions = append(subscriptions, sub)
	}
	if errors.Is(rows.Err(), pgx.ErrNoRows) {
		return subscriptions, nil
	}
	return subscriptions, rows.Err()
}

type UpsertSubscriptionOptions struct {
	InstanceDomain *sql.NullString
	DisplayName    *sql.NullString

	CreatedAt  utctime.Time
	ArchivedAt *utctime.Time

	SalesforceSubscriptionID *sql.NullString

	InstanceType *sql.NullString

	// ForceUpdate indicates whether to force update all fields of the subscription
	// record.
	ForceUpdate bool
}

// toQuery returns the query based on the options. It returns an empty query if
// nothing to update.
func (opts UpsertSubscriptionOptions) apply(ctx context.Context, db upsert.Execer, id string) error {
	b := upsert.New("enterprise_portal_subscriptions", "id", opts.ForceUpdate)
	upsert.Field(b, "id", id)

	upsert.Field(b, "instance_domain", opts.InstanceDomain)
	upsert.Field(b, "instance_type", opts.InstanceType)

	upsert.Field(b, "display_name", opts.DisplayName)

	upsert.Field(b, "created_at", opts.CreatedAt,
		upsert.WithColumnDefault(),
		// Can only be set explicitly (creation)
		upsert.WithIgnoreZeroOnForceUpdate())
	upsert.Field(b, "updated_at", time.Now()) // always updated now
	upsert.Field(b, "archived_at", opts.ArchivedAt,
		// Can only be set explicitly (archival)
		upsert.WithIgnoreZeroOnForceUpdate())

	upsert.Field(b, "salesforce_subscription_id", opts.SalesforceSubscriptionID)

	return b.Exec(ctx, db)
}

var ErrInvalidArgument = errors.New("invalid argument")

// Upsert upserts a subscription record based on the given options. If the
// operation has additional application meaning, conditions can be provided
// for insert as well.
//
// Constraint errors are returned as a human-friendly error that wraps
// ErrInvalidArgument.
func (s *Store) Upsert(
	ctx context.Context,
	subscriptionID string,
	opts UpsertSubscriptionOptions,
	conditions ...CreateSubscriptionConditionOptions,
) (*SubscriptionWithConditions, error) {
	if len(conditions) == 0 {
		// No conditions to add, do a simple update
		if err := opts.apply(ctx, s.db, subscriptionID); err != nil {
			return nil, err
		}

		return s.Get(ctx, subscriptionID)
	}

	// Do update and conditions insert in same transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "begin transaction")
	}
	defer func() {
		if rollbackErr := tx.Rollback(context.Background()); rollbackErr != nil {
			err = errors.Append(err, errors.Wrap(err, "rollback"))
		}
	}()

	if err := opts.apply(ctx, tx, subscriptionID); err != nil {
		if pgxerrors.IsContraintError(err, "idx_enterprise_portal_subscriptions_display_name") {
			return nil, errors.WithSafeDetails(
				errors.Wrapf(ErrInvalidArgument, "display_name %q is already in use", opts.DisplayName.String),
				"%+v", err)
		}
		if pgxerrors.IsContraintError(err, "idx_enterprise_portal_subscriptions_instance_domain") {
			return nil, errors.WithSafeDetails(
				errors.Wrapf(ErrInvalidArgument, "instance_domain %q is assigned to another subscription", opts.DisplayName.String),
				"%+v", err)
		}
		return nil, errors.Wrap(err, "upsert")
	}
	for _, condition := range conditions {
		err := newSubscriptionConditionsStore(tx).
			createSubscriptionCondition(ctx, subscriptionID, condition)
		if err != nil {
			return nil, errors.Wrapf(err, "set condition %q", condition.Status.String())
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, "commit upsert and conditions")
	}

	return s.Get(ctx, subscriptionID)
}

var ErrSubscriptionNotFound = errors.New("subscription not found")

// Get returns a subscription record with the given subscription ID. It returns
// ErrSubscriptionNotFound if no such subscription exists.
func (s *Store) Get(ctx context.Context, subscriptionID string) (*SubscriptionWithConditions, error) {
	query := fmt.Sprintf(`
SELECT
	%s
FROM
	enterprise_portal_subscriptions
LEFT JOIN
    enterprise_portal_subscription_conditions subscription_condition
    ON subscription_condition.subscription_id = id
WHERE
	id = @id
GROUP BY
    id`,
		strings.Join(subscriptionTableColumns(), ", "))
	namedArgs := pgx.NamedArgs{"id": subscriptionID}

	sub, err := scanSubscription(s.db.QueryRow(ctx, query, namedArgs))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.WithStack(ErrSubscriptionNotFound)
		}
		return nil, err
	}
	return sub, nil
}
