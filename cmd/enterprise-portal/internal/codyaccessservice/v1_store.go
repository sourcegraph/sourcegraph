package codyaccessservice

import (
	"context"
	"encoding/hex"
	"strings"

	"github.com/sourcegraph/conc/pool"

	sams "github.com/sourcegraph/sourcegraph-accounts-sdk-go"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/codyaccess"
	"github.com/sourcegraph/sourcegraph/internal/codygateway/codygatewayactor"
	"github.com/sourcegraph/sourcegraph/internal/codygateway/codygatewayevents"
	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/license"
	"github.com/sourcegraph/sourcegraph/internal/productsubscription"
	codyaccessv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/codyaccess/v1"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// StoreV1 is the data layer carrier for Cody access service v1. This interface
// is meant to abstract away and limit the exposure of the underlying data layer
// to the handler through a thin-wrapper.
type StoreV1 interface {
	// IntrospectSAMSToken takes a SAMS access token and returns relevant metadata.
	//
	// 🚨SECURITY: SAMS will return a successful result if the token is valid, but
	// is no longer active. It is critical that the caller not honor tokens where
	// `.Active == false`.
	IntrospectSAMSToken(ctx context.Context, token string) (*sams.IntrospectTokenResponse, error)

	// GetCodyGatewayUsage retrieves recent Cody Gateway usage data.
	// The subscriptionID should not be prefixed.
	//
	// Returns errStoreUnimplemented if the data source not configured.
	GetCodyGatewayUsage(ctx context.Context, subscriptionID string) (*codyaccessv1.CodyGatewayUsage, error)

	// GetCodyGatewayAccessBySubscription retrieves Cody Gateway access by
	// subscription ID.
	//
	// If the subscription does not exist, then ErrSubscriptionDoesNotExist is
	// returned.
	GetCodyGatewayAccessBySubscription(ctx context.Context, subscriptionID string) (*codyaccess.CodyGatewayAccessWithSubscriptionDetails, error)

	// GetCodyGatewayAccessByAccessToken retrieves Cody Gateway access details
	// associated with the given access token.
	GetCodyGatewayAccessByAccessToken(ctx context.Context, token string) (*codyaccess.CodyGatewayAccessWithSubscriptionDetails, error)

	// ListCodyGatewayAccesses retrieves all Cody Gateway accesses with their
	// associated subscription details.
	ListCodyGatewayAccesses(ctx context.Context) ([]*codyaccess.CodyGatewayAccessWithSubscriptionDetails, error)

	// UpsertCodyGatewayAccess creates or updates a Cody Gateway access record
	// for the given subscription ID.
	//
	// If the subscription does not exist, then ErrSubscriptionDoesNotExist is
	// returned.
	UpsertCodyGatewayAccess(ctx context.Context, subscriptionID string, opts codyaccess.UpsertCodyGatewayAccessOptions) (*codyaccess.CodyGatewayAccessWithSubscriptionDetails, error)
}

type storeV1 struct {
	SAMSClient        *sams.ClientV1
	CodyAccess        *codyaccess.Store
	CodyGatewayEvents *codygatewayevents.Service
}

type StoreV1Options struct {
	SAMSClient *sams.ClientV1
	DB         *database.DB
	// Optional.
	CodyGatewayEvents *codygatewayevents.Service
}

var errStoreUnimplemented = errors.New("unimplemented")

// NewStoreV1 returns a new StoreV1 using the given resource handles.
func NewStoreV1(opts StoreV1Options) StoreV1 {
	return &storeV1{
		SAMSClient:        opts.SAMSClient,
		CodyAccess:        opts.DB.CodyAccess(),
		CodyGatewayEvents: opts.CodyGatewayEvents,
	}
}

func (s *storeV1) IntrospectSAMSToken(ctx context.Context, token string) (*sams.IntrospectTokenResponse, error) {
	return s.SAMSClient.Tokens().IntrospectToken(ctx, token)
}

func (s *storeV1) GetCodyGatewayUsage(ctx context.Context, subscriptionID string) (*codyaccessv1.CodyGatewayUsage, error) {
	if s.CodyGatewayEvents == nil {
		return nil, errStoreUnimplemented
	}

	subscriptionID = strings.TrimPrefix(subscriptionID, subscriptionsv1.EnterpriseSubscriptionIDPrefix)

	// Collect results concurrently, cancelling on the first error result.
	p := pool.NewWithResults[func(*codyaccessv1.CodyGatewayUsage)]().
		WithContext(ctx).
		WithCancelOnError()

	// Get code completions usage.
	p.Go(func(ctx context.Context) (func(*codyaccessv1.CodyGatewayUsage), error) {
		codeCompletions, err := s.CodyGatewayEvents.CompletionsUsageForActor(
			ctx,
			types.CompletionsFeatureCode,
			codygatewayactor.ActorSourceEnterpriseSubscription,
			subscriptionID,
		)
		if err != nil {
			return nil, errors.Wrap(err, "get code completions usage")
		}
		return func(u *codyaccessv1.CodyGatewayUsage) {
			u.CodeCompletionsUsage = convertCodyGatewayUsageDatapoints(codeCompletions)
		}, nil
	})

	// Get chat completions usage.
	p.Go(func(ctx context.Context) (func(*codyaccessv1.CodyGatewayUsage), error) {
		chatCompletions, err := s.CodyGatewayEvents.CompletionsUsageForActor(
			ctx,
			types.CompletionsFeatureChat,
			codygatewayactor.ActorSourceEnterpriseSubscription,
			subscriptionID,
		)
		if err != nil {
			return nil, errors.Wrap(err, "get chat completions usage")
		}
		return func(u *codyaccessv1.CodyGatewayUsage) {
			u.ChatCompletionsUsage = convertCodyGatewayUsageDatapoints(chatCompletions)
		}, nil
	})

	// Get embeddings usage
	p.Go(func(ctx context.Context) (func(*codyaccessv1.CodyGatewayUsage), error) {
		embeddings, err := s.CodyGatewayEvents.EmbeddingsUsageForActor(
			ctx,
			codygatewayactor.ActorSourceEnterpriseSubscription,
			subscriptionID,
		)
		if err != nil {
			return nil, errors.Wrap(err, "get embeddings usage")
		}
		return func(u *codyaccessv1.CodyGatewayUsage) {
			u.EmbeddingsUsage = convertCodyGatewayUsageDatapoints(embeddings)
		}, nil
	})

	// Collect all results
	results, err := p.Wait()
	if err != nil {
		return nil, err
	}

	// Apply all results synchronously
	usage := &codyaccessv1.CodyGatewayUsage{
		// Use public format ID
		SubscriptionId: subscriptionsv1.EnterpriseSubscriptionIDPrefix + subscriptionID,
	}
	for _, applyResult := range results {
		applyResult(usage)
	}
	return usage, nil

}

func (s *storeV1) GetCodyGatewayAccessBySubscription(ctx context.Context, subscriptionID string) (*codyaccess.CodyGatewayAccessWithSubscriptionDetails, error) {
	return s.CodyAccess.CodyGateway().Get(ctx, codyaccess.GetCodyGatewayAccessOptions{
		SubscriptionID: strings.TrimPrefix(subscriptionID, subscriptionsv1.EnterpriseSubscriptionIDPrefix),
	})
}

func (s *storeV1) GetCodyGatewayAccessByAccessToken(ctx context.Context, token string) (*codyaccess.CodyGatewayAccessWithSubscriptionDetails, error) {
	// Below is copied from 'func (t dbTokens) LookupProductSubscriptionIDByAccessToken'
	// in 'cmd/frontend/internal/dotcom/productsubscription'.
	if !strings.HasPrefix(token, productsubscription.AccessTokenPrefix) &&
		!strings.HasPrefix(token, license.LicenseKeyBasedAccessTokenPrefix) {
		return nil, errors.WithSafeDetails(codyaccess.ErrSubscriptionNotFound, "invalid token with unknown prefix")
	}
	tokenSansPrefix := token[len(license.LicenseKeyBasedAccessTokenPrefix):]
	decoded, err := hex.DecodeString(tokenSansPrefix)
	if err != nil {
		return nil, errors.WithSafeDetails(codyaccess.ErrSubscriptionNotFound, "invalid token with unknown encoding")
	}
	// End copied code.

	return s.CodyAccess.CodyGateway().Get(ctx, codyaccess.GetCodyGatewayAccessOptions{
		LicenseKeyHash: decoded,
	})
}

func (s *storeV1) ListCodyGatewayAccesses(ctx context.Context) ([]*codyaccess.CodyGatewayAccessWithSubscriptionDetails, error) {
	return s.CodyAccess.CodyGateway().List(ctx)
}

func (s *storeV1) UpsertCodyGatewayAccess(ctx context.Context, subscriptionID string, opts codyaccess.UpsertCodyGatewayAccessOptions) (*codyaccess.CodyGatewayAccessWithSubscriptionDetails, error) {
	return s.CodyAccess.CodyGateway().Upsert(
		ctx,
		strings.TrimPrefix(subscriptionID, subscriptionsv1.EnterpriseSubscriptionIDPrefix),
		opts)
}
