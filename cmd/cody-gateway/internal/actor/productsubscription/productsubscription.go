package productsubscription

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gregjones/httpcache"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/codygateway/codygatewayactor"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/license"
	"github.com/sourcegraph/sourcegraph/internal/productsubscription"
	sgtrace "github.com/sourcegraph/sourcegraph/internal/trace"
	codyaccessv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/codyaccess/v1"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/actor"
)

var tracer = otel.GetTracerProvider().Tracer("cody-gateway/actor/productsubscription")

// SourceVersion should be bumped whenever the format of any cached data in this
// actor source implementation is changed. This effectively expires all entries.
const SourceVersion = "v2"

// product subscription tokens are always a prefix of 4 characters (sgs_ or slk_)
// followed by a 64-character hex-encoded SHA256 hash
const tokenLength = 4 + 64

var (
	minUpdateInterval = 10 * time.Minute

	defaultUpdateInterval = 24 * time.Hour
)

type ListingCache interface {
	httpcache.Cache
	ListAllKeys() []string
}

// EnterprisePortalClient defines the RPCs implemented by the Enterprise Portal
// that this Source depends on. We declare our own interface to keep our
// generated mock surface minimal, and our dependencies explicit.
type EnterprisePortalClient interface {
	// codyaccessv1 RPCs
	GetCodyGatewayAccess(context.Context, *codyaccessv1.GetCodyGatewayAccessRequest, ...grpc.CallOption) (*codyaccessv1.GetCodyGatewayAccessResponse, error)
	ListCodyGatewayAccesses(context.Context, *codyaccessv1.ListCodyGatewayAccessesRequest, ...grpc.CallOption) (*codyaccessv1.ListCodyGatewayAccessesResponse, error)
}

type Source struct {
	log   log.Logger
	cache ListingCache // cache is expected to be something with automatic TTL

	enterprisePortal EnterprisePortalClient

	concurrencyConfig codygatewayactor.ActorConcurrencyLimitConfig
}

var _ actor.Source = &Source{}
var _ actor.SourceUpdater = &Source{}
var _ actor.SourceSyncer = &Source{}

func NewSource(
	logger log.Logger,
	cache ListingCache,
	enterprisePortal EnterprisePortalClient,
	concurrencyConfig codygatewayactor.ActorConcurrencyLimitConfig,
) *Source {
	return &Source{
		log:   logger.Scoped("productsubscriptions"),
		cache: cache,

		enterprisePortal: enterprisePortal,

		concurrencyConfig: concurrencyConfig,
	}
}

func (s *Source) Name() string { return string(codygatewayactor.ActorSourceEnterpriseSubscription) }

func (s *Source) Get(ctx context.Context, token string) (*actor.Actor, error) {
	if token == "" {
		return nil, actor.ErrNotFromSource{}
	}

	// NOTE: For back-compat, we support both the old and new token prefixes.
	// However, as we use the token as part of the cache key, we need to be
	// consistent with the prefix we use.
	token = strings.Replace(token, productsubscription.AccessTokenPrefix, license.LicenseKeyBasedAccessTokenPrefix, 1)
	if !strings.HasPrefix(token, license.LicenseKeyBasedAccessTokenPrefix) {
		return nil, actor.ErrNotFromSource{Reason: "unknown token prefix"}
	}

	if len(token) != tokenLength {
		return nil, errors.New("invalid token format")
	}

	span := trace.SpanFromContext(ctx)

	data, hit := s.cache.Get(token)
	if !hit {
		span.SetAttributes(attribute.Bool("actor-cache-miss", true))
		return s.fetchAndCache(ctx, token)
	}

	var act *actor.Actor
	if err := json.Unmarshal(data, &act); err != nil {
		span.SetAttributes(attribute.Bool("actor-corrupted", true))
		sgtrace.Logger(ctx, s.log).Error("failed to unmarshal subscription", log.Error(err))

		// Delete the corrupted record.
		s.cache.Delete(token)

		return s.fetchAndCache(ctx, token)
	}

	if act.LastUpdated != nil && time.Since(*act.LastUpdated) > defaultUpdateInterval {
		span.SetAttributes(attribute.Bool("actor-expired", true))
		return s.fetchAndCache(ctx, token)
	}

	act.Source = s
	return act, nil
}

func (s *Source) Update(ctx context.Context, act *actor.Actor) error {
	if time.Since(*act.LastUpdated) < minUpdateInterval {
		// Last update was too recent - do it later.
		return actor.ErrActorRecentlyUpdated{
			RetryAt: act.LastUpdated.Add(minUpdateInterval),
		}
	}

	_, err := s.fetchAndCache(ctx, act.Key)
	return err
}

// Sync retrieves all known actors from this source and updates its cache.
// All Sync implementations are called periodically - implementations can decide
// to skip syncs if the frequency is too high.
func (s *Source) Sync(ctx context.Context) (seen int, errs error) {
	syncLog := sgtrace.Logger(ctx, s.log)
	seenTokens := collections.NewSet[string]()

	resp, err := s.enterprisePortal.ListCodyGatewayAccesses(ctx, &codyaccessv1.ListCodyGatewayAccessesRequest{
		// TODO(https://linear.app/sourcegraph/issue/CORE-134): Once the
		// Enterprise Portal supports pagination in its API responses we need to
		// update this callsite to make repeated requests with a continuation
		// token, etc. For now, we assume that we are fetching all licenses in
		// a single call.
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			syncLog.Warn("sync context cancelled")
			return seen, nil
		}
		return seen, errors.Wrap(err, "failed to list Enterprise subscriptions")
	}

	for _, access := range resp.GetAccesses() {
		for _, token := range access.GetAccessTokens() {
			select {
			case <-ctx.Done():
				return seen, ctx.Err()
			default:
			}

			act := newActor(s, token.GetToken(), access, time.Now())
			data, err := json.Marshal(act)
			if err != nil {
				act.Logger(syncLog).Error("failed to marshal actor",
					log.Error(err))
				errs = errors.Append(errs, err)
				continue
			}
			s.cache.Set(token.GetToken(), data)
			seenTokens.Add(token.GetToken())
			seen++
		}
	}
	removeUnseenTokens(ctx, syncLog, seenTokens, s.cache)
	return seen, errs
}

func removeUnseenTokens(ctx context.Context, syncLog log.Logger, seen collections.Set[string], cache ListingCache) {
	_, span := tracer.Start(ctx, "removeUnseenTokens", // ctx is unused, linter will complain
		trace.WithAttributes(attribute.Int("seen", len(seen.Values()))))
	defer span.End()

	start := time.Now()
	// Using Redis KEYS can get slow, but we only expect NUMBER_OF_SUBSCRIPTIONS * NUMBER_OF_ACTIVE_LICENCE_KEYS here
	// Right now, listing 2000 keys takes ~3ms, and replacing this with SCAN this would require changing our Redis client to support scanning / context, so let's leave like that until we fix listing subscriptions in Q2 2024.
	keys := cache.ListAllKeys()
	span.SetAttributes(attribute.Int("allKeys", len(keys)))
	elapsed := time.Since(start)
	syncLog.Debug("removing expired/disabled tokens", log.Int("seen", len(seen.Values())), log.Int("allKeys", len(keys)), log.Duration("listLatency", elapsed))
	// We don't have instrumentation on Redis, add a span event that gives the
	// equivalent of the log above
	span.AddEvent("ListAllKeys",
		trace.WithAttributes(attribute.Int64("elapsedMs", elapsed.Milliseconds())))

	var deleted int
	for _, key := range keys {
		parts := strings.Split(key, ":")
		if len(parts) != 4 {
			// weird, we expect things like v2:product-subscriptions:v2:TOKEN (3 ":", 4 parts)
			// we can't log the TOKEN, so we log the # of parts and skip delete
			syncLog.Warn("invalid key format, expected 4 parts, got", log.Int("parts", len(parts)))
			continue
		}
		token := parts[3]
		if !strings.HasPrefix(token, license.LicenseKeyBasedAccessTokenPrefix) {
			// let's not touch other types of tokens
			continue
		}
		if !seen.Has(token) {
			deleted += 1
			cache.Delete(token)
		}
	}
	syncLog.Info("deleted expired/disabled tokens", log.Int("deleted", deleted))
	span.SetAttributes(
		attribute.Int("deleted", deleted))
}

func (s *Source) checkAccessToken(ctx context.Context, token string) (*codyaccessv1.CodyGatewayAccess, error) {
	resp, err := s.enterprisePortal.GetCodyGatewayAccess(ctx, &codyaccessv1.GetCodyGatewayAccessRequest{
		Query: &codyaccessv1.GetCodyGatewayAccessRequest_AccessToken{
			AccessToken: token,
		},
	})
	if err == nil {
		return resp.GetAccess(), nil
	}

	// Inspect the error to see if it's not-found error.
	if statusErr, ok := status.FromError(err); ok && statusErr.Code() == codes.NotFound {
		return nil, actor.ErrAccessTokenDenied{
			Source: s.Name(),
			Reason: "associated product subscription not found",
		}
	}

	return nil, errors.Wrap(err, "verifying token via Enterprise Portal")
}

func (s *Source) fetchAndCache(ctx context.Context, token string) (*actor.Actor, error) {
	var act *actor.Actor
	resp, checkErr := s.checkAccessToken(ctx, token)
	if checkErr != nil {
		// Generate a stateless actor so that we aren't constantly hitting the dotcom API
		act = newActor(s, token, &codyaccessv1.CodyGatewayAccess{}, time.Now())
	} else {
		act = newActor(
			s,
			token,
			resp,
			time.Now(),
		)
	}

	if data, err := json.Marshal(act); err != nil {
		sgtrace.Logger(ctx, s.log).Error("failed to marshal actor",
			log.Error(err))
	} else {
		s.cache.Set(token, data)
	}

	if checkErr != nil {
		return nil, errors.Wrap(checkErr, "failed to validate access token")
	}
	return act, nil
}

// getSubscriptionAccountName attempts to get the account name from the product
// subscription. It returns an empty string if no account name is available.
func getSubscriptionAccountName(activeLicenseTags []string) string {
	// Check if the special "customer:" tag is present.
	for _, tag := range activeLicenseTags {
		if strings.HasPrefix(tag, "customer:") {
			return strings.TrimPrefix(tag, "customer:")
		}
	}
	return ""
}

// newActor creates an actor from Sourcegraph.com product subscription state.
func newActor(
	source *Source,
	token string,
	s *codyaccessv1.CodyGatewayAccess,
	now time.Time,
) *actor.Actor {
	name := s.GetSubscriptionDisplayName()
	if name == "" {
		name = s.GetSubscriptionId()
	}

	a := &actor.Actor{
		Key: token,

		// Maintain consistency with existing non-prefixed IDs.
		ID: strings.TrimPrefix(s.GetSubscriptionId(), subscriptionsv1.EnterpriseSubscriptionIDPrefix),

		Name:          name,
		AccessEnabled: s.GetEnabled(),
		EndpointAccess: map[string]bool{
			// Always enabled even if !s.GetEnabled(), to allow BYOK customers.
			"/v1/attribution": true,
		},
		RateLimits:  map[codygateway.Feature]actor.RateLimit{},
		LastUpdated: &now,
		Source:      source,
	}

	if rl := s.GetChatCompletionsRateLimit(); rl != nil && rl.IntervalDuration.AsDuration() > 0 {
		a.RateLimits[codygateway.FeatureChatCompletions] = actor.NewRateLimitWithPercentageConcurrency(
			int64(rl.Limit),
			rl.IntervalDuration.AsDuration(),
			[]string{"*"}, // allow all models that are allowlisted by Cody Gateway
			source.concurrencyConfig,
		)
	}

	if rl := s.GetCodeCompletionsRateLimit(); rl != nil && rl.IntervalDuration.AsDuration() > 0 {
		a.RateLimits[codygateway.FeatureCodeCompletions] = actor.NewRateLimitWithPercentageConcurrency(
			int64(rl.Limit),
			rl.IntervalDuration.AsDuration(),
			[]string{"*"}, // allow all models that are allowlisted by Cody Gateway
			source.concurrencyConfig,
		)
	}

	if rl := s.GetEmbeddingsRateLimit(); rl != nil && rl.IntervalDuration.AsDuration() > 0 {
		a.RateLimits[codygateway.FeatureEmbeddings] = actor.NewRateLimitWithPercentageConcurrency(
			int64(rl.Limit),
			rl.IntervalDuration.AsDuration(),
			[]string{"*"}, // allow all models that are allowlisted by Cody Gateway
			// TODO: Once we split interactive and on-interactive, we want to apply
			// stricter limits here than percentage based for this heavy endpoint.
			source.concurrencyConfig,
		)
	}

	return a
}
