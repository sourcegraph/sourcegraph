package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/actor"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/actor/anonymous"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/actor/productsubscription"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/events"
	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	internalproductsubscription "github.com/sourcegraph/sourcegraph/internal/productsubscription"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// TODO(@bobheadxi): Try to rewrite this as a table-driven test for less copy-pasta.
func TestAuthenticatorMiddleware(t *testing.T) {
	logger := logtest.Scoped(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	concurrencyConfig := codygateway.ActorConcurrencyLimitConfig{Percentage: 50, Interval: time.Hour}

	t.Run("unauthenticated and allow anonymous", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		(&Authenticator{
			Logger:      logger,
			EventLogger: events.NewStdoutLogger(logger),
			Sources:     actor.NewSources(anonymous.NewSource(true, concurrencyConfig)),
		}).Middleware(next).ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("unauthenticated but disallow anonymous", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		(&Authenticator{
			Logger:      logger,
			EventLogger: events.NewStdoutLogger(logger),
			Sources:     actor.NewSources(anonymous.NewSource(false, concurrencyConfig)),
		}).Middleware(next).ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("authenticated without cache hit", func(t *testing.T) {
		cache := NewMockListingCache()
		client := dotcom.NewMockClient()
		client.MakeRequestFunc.SetDefaultHook(func(_ context.Context, _ *graphql.Request, resp *graphql.Response) error {
			resp.Data.(*dotcom.CheckAccessTokenResponse).Dotcom = dotcom.CheckAccessTokenDotcomDotcomQuery{
				ProductSubscriptionByAccessToken: dotcom.CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription{
					ProductSubscriptionState: dotcom.ProductSubscriptionState{
						Id:         "UHJvZHVjdFN1YnNjcmlwdGlvbjoiNjQ1MmE4ZmMtZTY1MC00NWE3LWEwYTItMzU3Zjc3NmIzYjQ2Ig==",
						Uuid:       "6452a8fc-e650-45a7-a0a2-357f776b3b46",
						IsArchived: false,
						CodyGatewayAccess: dotcom.ProductSubscriptionStateCodyGatewayAccess{
							CodyGatewayAccessFields: dotcom.CodyGatewayAccessFields{
								Enabled: true,
								ChatCompletionsRateLimit: &dotcom.CodyGatewayAccessFieldsChatCompletionsRateLimitCodyGatewayRateLimit{
									RateLimitFields: dotcom.RateLimitFields{
										Limit:           10,
										IntervalSeconds: 10,
									},
								},
								CodeCompletionsRateLimit: &dotcom.CodyGatewayAccessFieldsCodeCompletionsRateLimitCodyGatewayRateLimit{
									RateLimitFields: dotcom.RateLimitFields{
										Limit:           10,
										IntervalSeconds: 10,
									},
								},
							},
						},
					},
				},
			}
			return nil
		})
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NotNil(t, actor.FromContext(r.Context()))
			w.WriteHeader(http.StatusOK)
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer sgs_abc1228e23e789431f08cd15e9be20e69b8694c2dff701b81d16250a4a861f37")
		(&Authenticator{
			Logger:      logger,
			EventLogger: events.NewStdoutLogger(logger),
			Sources:     actor.NewSources(productsubscription.NewSource(logger, cache, client, false, concurrencyConfig)),
		}).Middleware(next).ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		mockrequire.Called(t, client.MakeRequestFunc)
	})

	t.Run("authenticated with cache hit", func(t *testing.T) {
		cache := NewMockListingCache()
		cache.GetFunc.SetDefaultReturn(
			[]byte(`{"id":"UHJvZHVjdFN1YnNjcmlwdGlvbjoiNjQ1MmE4ZmMtZTY1MC00NWE3LWEwYTItMzU3Zjc3NmIzYjQ2Ig==","accessEnabled":true,"rateLimit":null}`),
			true,
		)
		client := dotcom.NewMockClient()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NotNil(t, actor.FromContext(r.Context()))
			w.WriteHeader(http.StatusOK)
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer sgs_abc1228e23e789431f08cd15e9be20e69b8694c2dff701b81d16250a4a861f37")
		(&Authenticator{
			Logger:      logger,
			EventLogger: events.NewStdoutLogger(logger),
			Sources:     actor.NewSources(productsubscription.NewSource(logger, cache, client, false, concurrencyConfig)),
		}).Middleware(next).ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		mockrequire.NotCalled(t, client.MakeRequestFunc)
	})

	t.Run("authenticated but not enabled", func(t *testing.T) {
		cache := NewMockListingCache()
		cache.GetFunc.SetDefaultReturn(
			[]byte(`{"id":"UHJvZHVjdFN1YnNjcmlwdGlvbjoiNjQ1MmE4ZmMtZTY1MC00NWE3LWEwYTItMzU3Zjc3NmIzYjQ2Ig==","accessEnabled":false,"rateLimit":null}`),
			true,
		)
		client := dotcom.NewMockClient()

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer sgs_abc1228e23e789431f08cd15e9be20e69b8694c2dff701b81d16250a4a861f37")
		(&Authenticator{
			Logger:      logger,
			EventLogger: events.NewStdoutLogger(logger),
			Sources:     actor.NewSources(productsubscription.NewSource(logger, cache, client, false, concurrencyConfig)),
		}).Middleware(next).ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("authenticated bypassing attribution while not enabled", func(t *testing.T) {
		cache := NewMockListingCache()
		cache.GetFunc.SetDefaultReturn(
			[]byte(`{"id":"UHJvZHVjdFN1YnNjcmlwdGlvbjoiNjQ1MmE4ZmMtZTY1MC00NWE3LWEwYTItMzU3Zjc3NmIzYjQ2Ig==","accessEnabled":false,"endpointAccess":{"/v1/attribution":true},"rateLimit":null}`),
			true,
		)
		client := dotcom.NewMockClient()

		t.Run("bypass works for attribution", func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/v1/attribution", strings.NewReader(`{}`))
			r.Header.Set("Authorization", "Bearer sgs_abc1228e23e789431f08cd15e9be20e69b8694c2dff701b81d16250a4a861f37")
			(&Authenticator{
				Logger:      logger,
				EventLogger: events.NewStdoutLogger(logger),
				Sources:     actor.NewSources(productsubscription.NewSource(logger, cache, client, false, concurrencyConfig)),
			}).Middleware(next).ServeHTTP(w, r)
			assert.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("bypass does not work for other endpoints", func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(`{}`))
			r.Header.Set("Authorization", "Bearer sgs_abc1228e23e789431f08cd15e9be20e69b8694c2dff701b81d16250a4a861f37")
			(&Authenticator{
				Logger:      logger,
				EventLogger: events.NewStdoutLogger(logger),
				Sources:     actor.NewSources(productsubscription.NewSource(logger, cache, client, false, concurrencyConfig)),
			}).Middleware(next).ServeHTTP(w, r)
			assert.Equal(t, http.StatusForbidden, w.Code)
		})
	})

	t.Run("access token denied from sources", func(t *testing.T) {
		cache := NewMockListingCache()
		client := dotcom.NewMockClient()
		client.MakeRequestFunc.SetDefaultHook(func(_ context.Context, _ *graphql.Request, resp *graphql.Response) error {
			return gqlerror.List{
				{
					Message:    "access denied",
					Extensions: map[string]any{"code": internalproductsubscription.GQLErrCodeProductSubscriptionNotFound},
				},
			}
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer sgs_abc1228e23e789431f08cd15e9be20e69b8694c2dff701b81d16250a4a861f37")
		(&Authenticator{
			Logger:      logger,
			EventLogger: events.NewStdoutLogger(logger),
			Sources:     actor.NewSources(productsubscription.NewSource(logger, cache, client, true, concurrencyConfig)),
		}).Middleware(next).ServeHTTP(w, r)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("server error from sources", func(t *testing.T) {
		cache := NewMockListingCache()
		client := dotcom.NewMockClient()
		client.MakeRequestFunc.SetDefaultHook(func(_ context.Context, _ *graphql.Request, resp *graphql.Response) error {
			return errors.New("server error")
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer sgs_abc1228e23e789431f08cd15e9be20e69b8694c2dff701b81d16250a4a861f37")
		(&Authenticator{
			Logger:      logger,
			EventLogger: events.NewStdoutLogger(logger),
			Sources:     actor.NewSources(productsubscription.NewSource(logger, cache, client, true, concurrencyConfig)),
		}).Middleware(next).ServeHTTP(w, r)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("internal mode, authenticated but not dev license", func(t *testing.T) {
		cache := NewMockListingCache()
		client := dotcom.NewMockClient()
		client.MakeRequestFunc.SetDefaultHook(func(_ context.Context, _ *graphql.Request, resp *graphql.Response) error {
			resp.Data.(*dotcom.CheckAccessTokenResponse).Dotcom = dotcom.CheckAccessTokenDotcomDotcomQuery{
				ProductSubscriptionByAccessToken: dotcom.CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription{
					ProductSubscriptionState: dotcom.ProductSubscriptionState{
						Id:         "UHJvZHVjdFN1YnNjcmlwdGlvbjoiNjQ1MmE4ZmMtZTY1MC00NWE3LWEwYTItMzU3Zjc3NmIzYjQ2Ig==",
						Uuid:       "6452a8fc-e650-45a7-a0a2-357f776b3b46",
						IsArchived: false,
						CodyGatewayAccess: dotcom.ProductSubscriptionStateCodyGatewayAccess{
							CodyGatewayAccessFields: dotcom.CodyGatewayAccessFields{
								Enabled: true,
								ChatCompletionsRateLimit: &dotcom.CodyGatewayAccessFieldsChatCompletionsRateLimitCodyGatewayRateLimit{
									RateLimitFields: dotcom.RateLimitFields{
										Limit:           10,
										IntervalSeconds: 10,
									},
								},
								CodeCompletionsRateLimit: &dotcom.CodyGatewayAccessFieldsCodeCompletionsRateLimitCodyGatewayRateLimit{
									RateLimitFields: dotcom.RateLimitFields{
										Limit:           10,
										IntervalSeconds: 10,
									},
								},
							},
						},
						ActiveLicense: &dotcom.ProductSubscriptionStateActiveLicenseProductLicense{
							Info: &dotcom.ProductSubscriptionStateActiveLicenseProductLicenseInfo{
								Tags: []string{""},
							},
						},
					},
				},
			}
			return nil
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer sgs_abc1228e23e789431f08cd15e9be20e69b8694c2dff701b81d16250a4a861f37")
		(&Authenticator{
			Logger:      logger,
			EventLogger: events.NewStdoutLogger(logger),
			Sources:     actor.NewSources(productsubscription.NewSource(logger, cache, client, true, concurrencyConfig)),
		}).Middleware(next).ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("internal mode, authenticated dev license", func(t *testing.T) {
		cache := NewMockListingCache()
		client := dotcom.NewMockClient()
		client.MakeRequestFunc.SetDefaultHook(func(_ context.Context, _ *graphql.Request, resp *graphql.Response) error {
			resp.Data.(*dotcom.CheckAccessTokenResponse).Dotcom = dotcom.CheckAccessTokenDotcomDotcomQuery{
				ProductSubscriptionByAccessToken: dotcom.CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription{
					ProductSubscriptionState: dotcom.ProductSubscriptionState{
						Id:         "UHJvZHVjdFN1YnNjcmlwdGlvbjoiNjQ1MmE4ZmMtZTY1MC00NWE3LWEwYTItMzU3Zjc3NmIzYjQ2Ig==",
						Uuid:       "6452a8fc-e650-45a7-a0a2-357f776b3b46",
						IsArchived: false,
						CodyGatewayAccess: dotcom.ProductSubscriptionStateCodyGatewayAccess{
							CodyGatewayAccessFields: dotcom.CodyGatewayAccessFields{
								Enabled: true,
								ChatCompletionsRateLimit: &dotcom.CodyGatewayAccessFieldsChatCompletionsRateLimitCodyGatewayRateLimit{
									RateLimitFields: dotcom.RateLimitFields{
										Limit:           10,
										IntervalSeconds: 10,
									},
								},
								CodeCompletionsRateLimit: &dotcom.CodyGatewayAccessFieldsCodeCompletionsRateLimitCodyGatewayRateLimit{
									RateLimitFields: dotcom.RateLimitFields{
										Limit:           10,
										IntervalSeconds: 10,
									},
								},
							},
						},
						ActiveLicense: &dotcom.ProductSubscriptionStateActiveLicenseProductLicense{
							Info: &dotcom.ProductSubscriptionStateActiveLicenseProductLicenseInfo{
								Tags: []string{licensing.DevTag},
							},
						},
					},
				},
			}
			return nil
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer sgs_abc1228e23e789431f08cd15e9be20e69b8694c2dff701b81d16250a4a861f37")
		(&Authenticator{
			Logger:      logger,
			EventLogger: events.NewStdoutLogger(logger),
			Sources:     actor.NewSources(productsubscription.NewSource(logger, cache, client, true, concurrencyConfig)),
		}).Middleware(next).ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("internal mode, authenticated internal license", func(t *testing.T) {
		cache := NewMockListingCache()
		client := dotcom.NewMockClient()
		client.MakeRequestFunc.SetDefaultHook(func(_ context.Context, _ *graphql.Request, resp *graphql.Response) error {
			resp.Data.(*dotcom.CheckAccessTokenResponse).Dotcom = dotcom.CheckAccessTokenDotcomDotcomQuery{
				ProductSubscriptionByAccessToken: dotcom.CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription{
					ProductSubscriptionState: dotcom.ProductSubscriptionState{
						Id:         "UHJvZHVjdFN1YnNjcmlwdGlvbjoiNjQ1MmE4ZmMtZTY1MC00NWE3LWEwYTItMzU3Zjc3NmIzYjQ2Ig==",
						Uuid:       "6452a8fc-e650-45a7-a0a2-357f776b3b46",
						IsArchived: false,
						CodyGatewayAccess: dotcom.ProductSubscriptionStateCodyGatewayAccess{
							CodyGatewayAccessFields: dotcom.CodyGatewayAccessFields{
								Enabled: true,
								ChatCompletionsRateLimit: &dotcom.CodyGatewayAccessFieldsChatCompletionsRateLimitCodyGatewayRateLimit{
									RateLimitFields: dotcom.RateLimitFields{
										Limit:           10,
										IntervalSeconds: 10,
									},
								},
								CodeCompletionsRateLimit: &dotcom.CodyGatewayAccessFieldsCodeCompletionsRateLimitCodyGatewayRateLimit{
									RateLimitFields: dotcom.RateLimitFields{
										Limit:           10,
										IntervalSeconds: 10,
									},
								},
							},
						},
						ActiveLicense: &dotcom.ProductSubscriptionStateActiveLicenseProductLicense{
							Info: &dotcom.ProductSubscriptionStateActiveLicenseProductLicenseInfo{
								Tags: []string{licensing.DevTag},
							},
						},
					},
				},
			}
			return nil
		})

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer sgs_abc1228e23e789431f08cd15e9be20e69b8694c2dff701b81d16250a4a861f37")
		(&Authenticator{
			Logger:      logger,
			EventLogger: events.NewStdoutLogger(logger),
			Sources:     actor.NewSources(productsubscription.NewSource(logger, cache, client, true, concurrencyConfig)),
		}).Middleware(next).ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
