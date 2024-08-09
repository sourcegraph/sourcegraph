package productsubscription

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hexops/autogold/v2"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/sourcegraph/sourcegraph/internal/codygateway/codygatewayactor"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	codyaccessv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/codyaccess/v1"
)

func TestNewActor(t *testing.T) {
	type args struct {
		access *codyaccessv1.CodyGatewayAccess
	}
	tests := []struct {
		name      string
		args      args
		wantActor autogold.Value
	}{
		{
			name: "enabled, no embeddings",
			args: args{
				&codyaccessv1.CodyGatewayAccess{
					SubscriptionId:          "es_1234uuid",
					SubscriptionDisplayName: "My Subscription",
					Enabled:                 true,
					ChatCompletionsRateLimit: &codyaccessv1.CodyGatewayRateLimit{
						Limit:            10,
						IntervalDuration: durationpb.New(10 * time.Second),
					},
					CodeCompletionsRateLimit: &codyaccessv1.CodyGatewayRateLimit{
						Limit:            10,
						IntervalDuration: durationpb.New(10 * time.Second),
					},
				},
			},
			wantActor: autogold.Expect(`{
  "key": "sekret_token",
  "id": "1234uuid",
  "name": "My Subscription",
  "accessEnabled": true,
  "endpointAccess": {
    "/v1/attribution": true
  },
  "rateLimits": {
    "chat_completions": {
      "allowedModels": [
        "*"
      ],
      "limit": 10,
      "interval": 10000000000,
      "concurrentRequests": 4320000,
      "concurrentRequestsInterval": 86400000000000
    },
    "code_completions": {
      "allowedModels": [
        "*"
      ],
      "limit": 10,
      "interval": 10000000000,
      "concurrentRequests": 4320000,
      "concurrentRequestsInterval": 86400000000000
    }
  },
  "lastUpdated": "2024-06-03T20:03:07-07:00"
}`),
		},
		{
			name: "enabled, only embeddings",
			args: args{
				&codyaccessv1.CodyGatewayAccess{
					SubscriptionId:          "es_1234uuid",
					SubscriptionDisplayName: "My Subscription",
					Enabled:                 true,
					EmbeddingsRateLimit: &codyaccessv1.CodyGatewayRateLimit{
						Limit:            10,
						IntervalDuration: durationpb.New(10 * time.Second),
					},
				},
			},
			wantActor: autogold.Expect(`{
  "key": "sekret_token",
  "id": "1234uuid",
  "name": "My Subscription",
  "accessEnabled": true,
  "endpointAccess": {
    "/v1/attribution": true
  },
  "rateLimits": {
    "embeddings": {
      "allowedModels": [
        "*"
      ],
      "limit": 10,
      "interval": 10000000000,
      "concurrentRequests": 4320000,
      "concurrentRequestsInterval": 86400000000000
    }
  },
  "lastUpdated": "2024-06-03T20:03:07-07:00"
}`),
		},
		{
			name: "enabled, rate limit has invalid duration",
			args: args{
				&codyaccessv1.CodyGatewayAccess{
					SubscriptionId:          "es_1234uuid",
					SubscriptionDisplayName: "My Subscription",
					Enabled:                 true,
					EmbeddingsRateLimit: &codyaccessv1.CodyGatewayRateLimit{
						Limit:            10,
						IntervalDuration: &durationpb.Duration{},
					},
				},
			},
			wantActor: autogold.Expect(`{
  "key": "sekret_token",
  "id": "1234uuid",
  "name": "My Subscription",
  "accessEnabled": true,
  "endpointAccess": {
    "/v1/attribution": true
  },
  "rateLimits": {},
  "lastUpdated": "2024-06-03T20:03:07-07:00"
}`),
		},
		{
			name: "disabled",
			args: args{
				&codyaccessv1.CodyGatewayAccess{
					SubscriptionId:          "es_1234uuid",
					SubscriptionDisplayName: "My Subscription",
					Enabled:                 false,
				},
			},
			wantActor: autogold.Expect(`{
  "key": "sekret_token",
  "id": "1234uuid",
  "name": "My Subscription",
  "accessEnabled": false,
  "endpointAccess": {
    "/v1/attribution": true
  },
  "rateLimits": {},
  "lastUpdated": "2024-06-03T20:03:07-07:00"
}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2024, 6, 3, 20, 3, 7, 0, time.FixedZone("PDT", -25200))
			act := newActor(&Source{
				concurrencyConfig: codygatewayactor.ActorConcurrencyLimitConfig{
					Percentage: 50,
					Interval:   24 * time.Hour,
				},
			}, "sekret_token", tt.args.access, now)
			// Assert against JSON representation, because that's what we end
			// up caching.
			actData, err := json.MarshalIndent(act, "", "  ")
			require.NoError(t, err)
			tt.wantActor.Equal(t, string(actData))
		})
	}
}

func TestGetSubscriptionAccountName(t *testing.T) {
	tests := []struct {
		name     string
		mockTags []string
		wantName string
	}{
		{
			name:     "has special license tag",
			mockTags: []string{"trial", "customer:acme"},
			wantName: "acme",
		},
		{
			name:     "no data",
			wantName: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := getSubscriptionAccountName(test.mockTags)
			assert.Equal(t, test.wantName, got)
		})
	}
}

func TestRemoveUnseenTokens(t *testing.T) {
	ctx := context.Background()

	t.Run("removes unseen tokens", func(t *testing.T) {
		seen := collections.NewSet("slk_token1")

		cache := &fakeListingCache{
			state: map[string][]byte{"v2:product-subscription:v1:slk_token1": nil, "v2:product-subscription:v2:slk_token3": nil},
		}

		removeUnseenTokens(ctx, logtest.Scoped(t), seen, cache)
		assert.Equal(t, cache.calls, []struct{ call, key string }{{"ListAllKeys", ""}, {"Delete", "slk_token3"}})
	})

	t.Run("ignores malformed keys ", func(t *testing.T) {
		seen := collections.NewSet[string]()

		cache := &fakeListingCache{
			state: map[string][]byte{"v2:product-subscription:": nil},
		}

		removeUnseenTokens(ctx, logtest.Scoped(t), seen, cache)
		assert.Equal(t, cache.calls, []struct{ call, key string }{{"ListAllKeys", ""}})
	})
	t.Run("ignores malformed keys ", func(t *testing.T) {
		seen := collections.NewSet[string]()

		cache := &fakeListingCache{
			state: map[string][]byte{"v2:product-subscription:v2:sgp_dotcom": nil},
		}

		removeUnseenTokens(ctx, logtest.Scoped(t), seen, cache)
		assert.Equal(t, cache.calls, []struct{ call, key string }{{"ListAllKeys", ""}})
	})
}

type fakeListingCache struct {
	state map[string][]byte
	calls []struct{ call, key string }
}

var _ ListingCache = &fakeListingCache{}

func (m *fakeListingCache) Set(key string, responseBytes []byte) {
	m.state[key] = responseBytes
	m.calls = append(m.calls, struct{ call, key string }{"Set", key})
}

func (m *fakeListingCache) ListAllKeys() []string {
	m.calls = append(m.calls, struct{ call, key string }{"ListAllKeys", ""})
	return maps.Keys(m.state)
}

func (m *fakeListingCache) Get(key string) ([]byte, bool) {
	m.calls = append(m.calls, struct{ call, key string }{"Get", key})
	if v, ok := m.state[key]; ok {
		return v, ok
	}
	return nil, false
}

func (m *fakeListingCache) Delete(key string) {
	m.calls = append(m.calls, struct{ call, key string }{"Delete", key})
	delete(m.state, key)
}
