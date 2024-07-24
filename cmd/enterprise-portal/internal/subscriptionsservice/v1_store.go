package subscriptionsservice

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	sams "github.com/sourcegraph/sourcegraph-accounts-sdk-go"
	clientsv1 "github.com/sourcegraph/sourcegraph-accounts-sdk-go/clients/v1"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/subscriptions"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/utctime"
	"github.com/sourcegraph/sourcegraph/internal/license"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/iam"
)

// StoreV1 is the data layer carrier for subscriptions service v1. This interface
// is meant to abstract away and limit the exposure of the underlying data layer
// to the handler through a thin-wrapper.
type StoreV1 interface {
	// Now provides the current time. It should always be used instead of
	// utctime.Now() or time.Now() for ease of mocking in tests.
	Now() utctime.Time

	// GenerateSubscriptionID generates a new subscription ID for subscription
	// creation.
	GenerateSubscriptionID() (string, error)
	// UpsertEnterpriseSubscription upserts a enterprise subscription record based
	// on the given options.
	UpsertEnterpriseSubscription(ctx context.Context, subscriptionID string, opts subscriptions.UpsertSubscriptionOptions, conditions ...subscriptions.CreateSubscriptionConditionOptions) (*subscriptions.SubscriptionWithConditions, error)
	// ListEnterpriseSubscriptions returns a list of enterprise subscriptions based
	// on the given options.
	ListEnterpriseSubscriptions(ctx context.Context, opts subscriptions.ListEnterpriseSubscriptionsOptions) ([]*subscriptions.SubscriptionWithConditions, error)
	// GetEnterpriseSubscriptions returns a specific enterprise subscription.
	//
	// Returns subscriptions.ErrSubscriptionNotFound if the subscription does
	// not exist.
	GetEnterpriseSubscription(ctx context.Context, subscriptionID string) (*subscriptions.SubscriptionWithConditions, error)

	// ListDotcomEnterpriseSubscriptionLicenses returns a list of enterprise
	// subscription license attributes with the given filters. It silently ignores
	// any non-matching filters. The caller should check the length of the returned
	// slice to ensure all requested licenses were found.
	ListEnterpriseSubscriptionLicenses(ctx context.Context, opts subscriptions.ListLicensesOpts) ([]*subscriptions.LicenseWithConditions, error)
	// RevokeEnterpriseSubscriptionLicense premanently revokes a license.
	RevokeEnterpriseSubscriptionLicense(ctx context.Context, licenseID string, opts subscriptions.RevokeLicenseOpts) (*subscriptions.LicenseWithConditions, error)

	// Interfaces specific to 'ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_KEY', grouped
	// to clarify their purpose for future license key types.
	licenseKeysStore

	// IntrospectSAMSToken takes a SAMS access token and returns relevant metadata.
	//
	// 🚨SECURITY: SAMS will return a successful result if the token is valid, but
	// is no longer active. It is critical that the caller not honor tokens where
	// `.Active == false`.
	IntrospectSAMSToken(ctx context.Context, token string) (*sams.IntrospectTokenResponse, error)
	// GetSAMSUserByID returns the SAMS user with the given ID. It returns
	// sams.ErrNotFound if no such user exists.
	//
	// Required scope: profile
	GetSAMSUserByID(ctx context.Context, id string) (*clientsv1.User, error)

	// IAMListObjects returns a list of object IDs that satisfy the given options.
	IAMListObjects(ctx context.Context, opts iam.ListObjectsOptions) ([]string, error)
	// IAMWrite adds, updates, and/or deletes the IAM relation tuples.
	IAMWrite(ctx context.Context, opts iam.WriteOptions) error
	// IAMCheck checks whether a relationship exists (thus permission allowed) using
	// the given tuple key as the check condition.
	IAMCheck(ctx context.Context, opts iam.CheckOptions) (allowed bool, _ error)
}

// licenseKeysStore groups mechanisms specific to the license type
// 'ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_KEY'
type licenseKeysStore interface {
	// GetRequiredEnterpriseSubscriptionLicenseKeyTags returns the license tags
	// that must be included on all generated license keys.
	GetRequiredEnterpriseSubscriptionLicenseKeyTags() []string
	// SignEnterpriseSubscriptionLicenseKey signs a new license key for
	// creation of 'ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_KEY' licenses.
	//
	// Returns errStoreUnimplemented if key signing is not configured.
	SignEnterpriseSubscriptionLicenseKey(license.Info) (string, error)
	// CreateLicense creates a new classic offline license for the given subscription.
	CreateEnterpriseSubscriptionLicenseKey(ctx context.Context, subscriptionID string, license *subscriptions.DataLicenseKey, opts subscriptions.CreateLicenseOpts) (*subscriptions.LicenseWithConditions, error)
}

type storeV1 struct {
	db         *database.DB
	SAMSClient *sams.ClientV1
	IAMClient  *iam.ClientV1
	// LicenseKeySigner may be nil if not configured for key signing.
	LicenseKeySigner       ssh.Signer
	LicenseKeyRequiredTags []string
}

type NewStoreV1Options struct {
	DB         *database.DB
	SAMSClient *sams.ClientV1
	IAMClient  *iam.ClientV1

	LicenseKeySigner       ssh.Signer
	LicenseKeyRequiredTags []string
}

var errStoreUnimplemented = errors.New("unimplemented")

// NewStoreV1 returns a new StoreV1 using the given resource handles.
func NewStoreV1(opts NewStoreV1Options) StoreV1 {
	return &storeV1{
		db:         opts.DB,
		SAMSClient: opts.SAMSClient,
		IAMClient:  opts.IAMClient,

		LicenseKeySigner:       opts.LicenseKeySigner,
		LicenseKeyRequiredTags: opts.LicenseKeyRequiredTags,
	}
}

func (s *storeV1) Now() utctime.Time { return utctime.Now() }

func (s *storeV1) GenerateSubscriptionID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", errors.Wrap(err, "uuid")
	}
	return id.String(), nil
}

func (s *storeV1) UpsertEnterpriseSubscription(ctx context.Context, subscriptionID string, opts subscriptions.UpsertSubscriptionOptions, conditions ...subscriptions.CreateSubscriptionConditionOptions) (*subscriptions.SubscriptionWithConditions, error) {
	return s.db.Subscriptions().Upsert(
		ctx,
		strings.TrimPrefix(subscriptionID, subscriptionsv1.EnterpriseSubscriptionIDPrefix),
		opts,
		conditions...,
	)
}

func (s *storeV1) ListEnterpriseSubscriptions(ctx context.Context, opts subscriptions.ListEnterpriseSubscriptionsOptions) ([]*subscriptions.SubscriptionWithConditions, error) {
	for idx := range opts.IDs {
		opts.IDs[idx] = strings.TrimPrefix(opts.IDs[idx], subscriptionsv1.EnterpriseSubscriptionIDPrefix)
	}
	return s.db.Subscriptions().List(ctx, opts)
}

func (s *storeV1) GetEnterpriseSubscription(ctx context.Context, subscriptionID string) (*subscriptions.SubscriptionWithConditions, error) {
	return s.db.Subscriptions().Get(ctx,
		strings.TrimPrefix(subscriptionID, subscriptionsv1.EnterpriseSubscriptionIDPrefix))
}

func (s *storeV1) ListEnterpriseSubscriptionLicenses(ctx context.Context, opts subscriptions.ListLicensesOpts) ([]*subscriptions.LicenseWithConditions, error) {
	opts.SubscriptionID = strings.TrimPrefix(opts.SubscriptionID, subscriptionsv1.EnterpriseSubscriptionIDPrefix)
	return s.db.Subscriptions().Licenses().List(ctx, opts)
}

func (s *storeV1) GetRequiredEnterpriseSubscriptionLicenseKeyTags() []string {
	return s.LicenseKeyRequiredTags
}

func (s *storeV1) SignEnterpriseSubscriptionLicenseKey(info license.Info) (string, error) {
	if s.LicenseKeySigner == nil {
		return "", errStoreUnimplemented
	}
	signedKey, _, err := license.GenerateSignedKey(info, s.LicenseKeySigner)
	if err != nil {
		return "", errors.Wrap(err, "generating signed key")
	}
	return signedKey, nil
}

func (s *storeV1) CreateEnterpriseSubscriptionLicenseKey(ctx context.Context, subscriptionID string, license *subscriptions.DataLicenseKey, opts subscriptions.CreateLicenseOpts) (*subscriptions.LicenseWithConditions, error) {
	if opts.ImportLicenseID != "" {
		return nil, errors.New("import license ID not allowed via API")
	}
	return s.db.Subscriptions().Licenses().CreateLicenseKey(
		ctx,
		strings.TrimPrefix(subscriptionID, subscriptionsv1.EnterpriseSubscriptionIDPrefix),
		license,
		opts,
	)
}

func (s *storeV1) RevokeEnterpriseSubscriptionLicense(ctx context.Context, licenseID string, opts subscriptions.RevokeLicenseOpts) (*subscriptions.LicenseWithConditions, error) {
	return s.db.Subscriptions().Licenses().Revoke(
		ctx,
		strings.TrimPrefix(licenseID, subscriptionsv1.EnterpriseSubscriptionLicenseIDPrefix),
		opts,
	)
}

func (s *storeV1) IntrospectSAMSToken(ctx context.Context, token string) (*sams.IntrospectTokenResponse, error) {
	return s.SAMSClient.Tokens().IntrospectToken(ctx, token)
}

func (s *storeV1) GetSAMSUserByID(ctx context.Context, id string) (*clientsv1.User, error) {
	return s.SAMSClient.Users().GetUserByID(ctx, id)
}

func (s *storeV1) IAMListObjects(ctx context.Context, opts iam.ListObjectsOptions) ([]string, error) {
	return s.IAMClient.ListObjects(ctx, opts)
}

func (s *storeV1) IAMWrite(ctx context.Context, opts iam.WriteOptions) error {
	return s.IAMClient.Write(ctx, opts)
}

func (s *storeV1) IAMCheck(ctx context.Context, opts iam.CheckOptions) (allowed bool, _ error) {
	return s.IAMClient.Check(ctx, opts)
}
