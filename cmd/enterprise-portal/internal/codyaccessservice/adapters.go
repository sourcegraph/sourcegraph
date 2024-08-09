package codyaccessservice

import (
	"encoding/hex"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/codyaccess"
	"github.com/sourcegraph/sourcegraph/internal/codygateway/codygatewayevents"
	"github.com/sourcegraph/sourcegraph/internal/license"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	codyaccessv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/codyaccess/v1"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func convertAccessAttrsToProto(access *codyaccess.CodyGatewayAccessWithSubscriptionDetails) *codyaccessv1.CodyGatewayAccess {
	// Provide ID in prefixed format.
	subscriptionID := subscriptionsv1.EnterpriseSubscriptionIDPrefix + access.SubscriptionID

	// Always try to return the full response, since even when disabled, some
	// features may be allowed via Cody Gateway (notably attributions). This
	// also allows Cody Gateway to cache the state of actors that are disabled.
	limits := evaluateCodyGatewayAccessRateLimits(access)
	return &codyaccessv1.CodyGatewayAccess{
		SubscriptionId:          subscriptionID,
		SubscriptionDisplayName: access.DisplayName,
		Enabled:                 access.Enabled,
		// Rate limits return nil if not enabled, per API spec
		ChatCompletionsRateLimit: nilIfNotEnabled(access.Enabled, &codyaccessv1.CodyGatewayRateLimit{
			Source:           limits.ChatSource,
			Limit:            uint64(limits.Chat.Limit),
			IntervalDuration: durationpb.New(limits.Chat.IntervalDuration()),
		}),
		CodeCompletionsRateLimit: nilIfNotEnabled(access.Enabled, &codyaccessv1.CodyGatewayRateLimit{
			Source:           limits.CodeSource,
			Limit:            uint64(limits.Code.Limit),
			IntervalDuration: durationpb.New(limits.Code.IntervalDuration()),
		}),
		EmbeddingsRateLimit: nilIfNotEnabled(access.Enabled, &codyaccessv1.CodyGatewayRateLimit{
			Source:           limits.EmbeddingsSource,
			Limit:            uint64(limits.Embeddings.Limit),
			IntervalDuration: durationpb.New(limits.Embeddings.IntervalDuration()),
		}),
		// This is always provided, even if access is disabled
		AccessTokens: func() []*codyaccessv1.CodyGatewayAccessToken {
			accessTokens := generateCodyGatewayAccessTokens(access)
			if len(accessTokens) == 0 {
				return []*codyaccessv1.CodyGatewayAccessToken{}
			}

			results := make([]*codyaccessv1.CodyGatewayAccessToken, len(accessTokens))
			for i, token := range accessTokens {
				results[i] = &codyaccessv1.CodyGatewayAccessToken{
					Token: token,
				}
			}
			return results
		}(),
	}
}

func generateCodyGatewayAccessTokens(access *codyaccess.CodyGatewayAccessWithSubscriptionDetails) []string {
	accessTokens := make([]string, 0, len(access.LicenseKeyHashes))
	for _, t := range access.LicenseKeyHashes {
		if len(t) == 0 { // query can return empty hashes, ignore these
			continue
		}
		// See license.GenerateLicenseKeyBasedAccessToken
		accessTokens = append(accessTokens, license.LicenseKeyBasedAccessTokenPrefix+hex.EncodeToString(t))
	}
	return accessTokens
}

func nilIfNotEnabled[T any](enabled bool, value *T) *T {
	if !enabled {
		return nil
	}
	return value
}

type CodyGatewayRateLimits struct {
	ChatSource codyaccessv1.CodyGatewayRateLimitSource
	Chat       licensing.CodyGatewayRateLimit

	CodeSource codyaccessv1.CodyGatewayRateLimitSource
	Code       licensing.CodyGatewayRateLimit

	EmbeddingsSource codyaccessv1.CodyGatewayRateLimitSource
	Embeddings       licensing.CodyGatewayRateLimit
}

func maybeApplyOverride[T ~int32 | ~int64](limit *T, overrideValue T, overrideValid bool) codyaccessv1.CodyGatewayRateLimitSource {
	if overrideValid {
		*limit = overrideValue
		return codyaccessv1.CodyGatewayRateLimitSource_CODY_GATEWAY_RATE_LIMIT_SOURCE_OVERRIDE
	}
	// No override
	return codyaccessv1.CodyGatewayRateLimitSource_CODY_GATEWAY_RATE_LIMIT_SOURCE_PLAN
}

// evaluateCodyGatewayAccessRateLimits returns the current CodyGatewayRateLimits based on the
// plan and applying known overrides on top. This closely models the existing
// codyGatewayAccessResolver in 'cmd/frontend/internal/dotcom/productsubscription'.
func evaluateCodyGatewayAccessRateLimits(access *codyaccess.CodyGatewayAccessWithSubscriptionDetails) CodyGatewayRateLimits {
	// Set defaults for everything based on active license plan and user count.
	// If there isn't one, zero values apply.
	limits := CodyGatewayRateLimits{}
	if access.ActiveLicenseInfo != nil {
		// Infer defaults for the active license
		var (
			activeLicense = *access.ActiveLicenseInfo
			plan          = licensing.PlanFromTags(activeLicense.Tags)
			userCount     = pointers.Ptr(int(activeLicense.UserCount))
		)
		limits = CodyGatewayRateLimits{
			ChatSource: codyaccessv1.CodyGatewayRateLimitSource_CODY_GATEWAY_RATE_LIMIT_SOURCE_PLAN,
			Chat:       licensing.NewCodyGatewayChatRateLimit(plan, userCount),

			CodeSource: codyaccessv1.CodyGatewayRateLimitSource_CODY_GATEWAY_RATE_LIMIT_SOURCE_PLAN,
			Code:       licensing.NewCodyGatewayCodeRateLimit(plan, userCount),

			EmbeddingsSource: codyaccessv1.CodyGatewayRateLimitSource_CODY_GATEWAY_RATE_LIMIT_SOURCE_PLAN,
			Embeddings:       licensing.NewCodyGatewayEmbeddingsRateLimit(plan, userCount),
		}
	}

	// Chat
	limits.ChatSource = maybeApplyOverride(&limits.Chat.Limit,
		access.ChatCompletionsRateLimit.Int64, access.ChatCompletionsRateLimit.Valid)
	limits.ChatSource = maybeApplyOverride(&limits.Chat.IntervalSeconds,
		access.ChatCompletionsRateLimitIntervalSeconds.Int32, access.ChatCompletionsRateLimitIntervalSeconds.Valid)

	// Code
	limits.CodeSource = maybeApplyOverride(&limits.Code.Limit,
		access.CodeCompletionsRateLimit.Int64, access.CodeCompletionsRateLimit.Valid)
	limits.CodeSource = maybeApplyOverride(&limits.Code.IntervalSeconds,
		access.CodeCompletionsRateLimitIntervalSeconds.Int32, access.CodeCompletionsRateLimitIntervalSeconds.Valid)

	// Embeddings
	limits.EmbeddingsSource = maybeApplyOverride(&limits.Embeddings.Limit,
		access.EmbeddingsRateLimit.Int64, access.EmbeddingsRateLimit.Valid)
	limits.EmbeddingsSource = maybeApplyOverride(&limits.Embeddings.IntervalSeconds,
		access.EmbeddingsRateLimitIntervalSeconds.Int32, access.EmbeddingsRateLimitIntervalSeconds.Valid)

	return limits
}

func convertCodyGatewayUsageDatapoints(usage []codygatewayevents.SubscriptionUsage) []*codyaccessv1.CodyGatewayUsage_UsageDatapoint {
	results := make([]*codyaccessv1.CodyGatewayUsage_UsageDatapoint, len(usage))
	for i, datapoint := range usage {
		results[i] = &codyaccessv1.CodyGatewayUsage_UsageDatapoint{
			Time:  timestamppb.New(datapoint.Date),
			Usage: uint64(datapoint.Count),
			Model: datapoint.Model,
		}
	}
	return results
}
