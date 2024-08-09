package codyaccessservice

import (
	"fmt"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/codyaccess"
	"github.com/sourcegraph/sourcegraph/internal/license"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	codyaccessv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/codyaccess/v1"
)

func TestConvertAccessAttrsToProto(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		proto := convertAccessAttrsToProto(&codyaccess.CodyGatewayAccessWithSubscriptionDetails{})
		assert.False(t, proto.Enabled)
	})

	t.Run("disabled returns access tokens", func(t *testing.T) {
		proto := convertAccessAttrsToProto(&codyaccess.CodyGatewayAccessWithSubscriptionDetails{
			CodyGatewayAccess: codyaccess.CodyGatewayAccess{
				Enabled: false,
			},
			LicenseKeyHashes: [][]byte{[]byte("abc"), []byte("efg")},
		})
		assert.False(t, proto.Enabled)
		// NOTE: These are not real access tokens
		autogold.Expect([]string{`token:"slk_616263"`, `token:"slk_656667"`}).Equal(t, toStrings(proto.GetAccessTokens()))
		// Returns nil rate limits
		assert.Nil(t, proto.ChatCompletionsRateLimit)
		assert.Nil(t, proto.CodeCompletionsRateLimit)
		assert.Nil(t, proto.EmbeddingsRateLimit)
	})

	t.Run("enabled with empty access token", func(t *testing.T) {
		proto := convertAccessAttrsToProto(&codyaccess.CodyGatewayAccessWithSubscriptionDetails{
			CodyGatewayAccess: codyaccess.CodyGatewayAccess{
				Enabled: true,
			},
			LicenseKeyHashes: [][]byte{[]byte(""), nil},
		})
		assert.True(t, proto.Enabled)
		assert.Empty(t, proto.GetAccessTokens())
	})

	t.Run("enabled WITHOUT active license returns nothing", func(t *testing.T) {
		proto := convertAccessAttrsToProto(&codyaccess.CodyGatewayAccessWithSubscriptionDetails{
			CodyGatewayAccess: codyaccess.CodyGatewayAccess{
				Enabled: true,
				// no overrides
			},
			ActiveLicenseInfo: nil, // no active license
		})
		assert.True(t, proto.Enabled)
		// Returns non-nil rate limits
		autogold.Expect(&codyaccessv1.CodyGatewayRateLimit{
			Source:           codyaccessv1.CodyGatewayRateLimitSource_CODY_GATEWAY_RATE_LIMIT_SOURCE_PLAN,
			IntervalDuration: &durationpb.Duration{},
		}).Equal(t, proto.ChatCompletionsRateLimit)
		autogold.Expect(&codyaccessv1.CodyGatewayRateLimit{
			Source:           codyaccessv1.CodyGatewayRateLimitSource_CODY_GATEWAY_RATE_LIMIT_SOURCE_PLAN,
			IntervalDuration: &durationpb.Duration{},
		}).Equal(t, proto.CodeCompletionsRateLimit)
		autogold.Expect(&codyaccessv1.CodyGatewayRateLimit{
			Source:           codyaccessv1.CodyGatewayRateLimitSource_CODY_GATEWAY_RATE_LIMIT_SOURCE_PLAN,
			IntervalDuration: &durationpb.Duration{},
		}).Equal(t, proto.EmbeddingsRateLimit)
	})

	t.Run("enabled with active license returns plan defaults", func(t *testing.T) {
		proto := convertAccessAttrsToProto(&codyaccess.CodyGatewayAccessWithSubscriptionDetails{
			CodyGatewayAccess: codyaccess.CodyGatewayAccess{
				Enabled: true,
				// no overrides
			},
			ActiveLicenseInfo: &license.Info{
				UserCount: 10,
				Tags:      []string{licensing.PlanCodyEnterprise.Tag()},
			},
			LicenseKeyHashes: [][]byte{[]byte("abc"), []byte("efg")},
		})
		assert.True(t, proto.Enabled)
		// NOTE: These are not real access tokens
		autogold.Expect([]string{`token:"slk_616263"`, `token:"slk_656667"`}).Equal(t, toStrings(proto.GetAccessTokens()))
		// Returns non-nil rate limits
		autogold.Expect(&codyaccessv1.CodyGatewayRateLimit{
			Source: codyaccessv1.CodyGatewayRateLimitSource_CODY_GATEWAY_RATE_LIMIT_SOURCE_PLAN,
			Limit:  100,
			IntervalDuration: &durationpb.Duration{
				Seconds: 86400,
			},
		}).Equal(t, proto.ChatCompletionsRateLimit)
		autogold.Expect(&codyaccessv1.CodyGatewayRateLimit{
			Source: codyaccessv1.CodyGatewayRateLimitSource_CODY_GATEWAY_RATE_LIMIT_SOURCE_PLAN,
			Limit:  10000,
			IntervalDuration: &durationpb.Duration{
				Seconds: 86400,
			},
		}).Equal(t, proto.CodeCompletionsRateLimit)
		autogold.Expect(&codyaccessv1.CodyGatewayRateLimit{
			Source: codyaccessv1.CodyGatewayRateLimitSource_CODY_GATEWAY_RATE_LIMIT_SOURCE_PLAN,
			Limit:  33333333,
			IntervalDuration: &durationpb.Duration{
				Seconds: 86400,
			},
		}).Equal(t, proto.EmbeddingsRateLimit)
	})
}

func toStrings[T fmt.Stringer](stringers []T) []string {
	strs := make([]string, len(stringers))
	for i, s := range stringers {
		strs[i] = s.String()
	}
	return strs
}
