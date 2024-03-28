package httpapi

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/featureflag"

	"github.com/gomodule/redigo/redis"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/internal/requestclient"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type RateLimiter interface {
	TryAcquire(ctx context.Context) error
}

type RateLimitExceededError struct {
	Scope      types.CompletionsFeature
	Limit      int
	Used       int
	RetryAfter time.Time
}

func (e RateLimitExceededError) Error() string {
	return fmt.Sprintf("you exceeded the rate limit for %s, only %d requests are allowed per day at the moment to ensure the service stays functional. Current usage: %d. Retry after %s", e.Scope, e.Limit, e.Used, e.RetryAfter.Truncate(time.Second))
}

func NewRateLimiter(db database.DB, rstore redispool.KeyValue, scope types.CompletionsFeature) RateLimiter {
	return &rateLimiter{db: db, rstore: rstore, scope: scope}
}

type rateLimiter struct {
	scope  types.CompletionsFeature
	rstore redispool.KeyValue
	db     database.DB
}

func (r *rateLimiter) TryAcquire(ctx context.Context) (err error) {
	limit, err := getConfiguredLimit(ctx, r.db, r.scope)
	if err != nil {
		return errors.Wrap(err, "failed to read configured rate limit")
	}

	if limit <= 0 {
		// Rate limiting disabled.
		return nil
	}

	// Check that the user is authenticated.
	a := actor.FromContext(ctx)
	if a.IsInternal() {
		return nil
	}
	key := userKey(a.UID, r.scope)
	if !a.IsAuthenticated() {
		// Fall back to the IP address, if provided in context (ie. this is a request handler).
		req := requestclient.FromContext(ctx)
		if req == nil || req.RealIP == "" {
			return errors.Wrap(auth.ErrNotAuthenticated, "cannot claim rate limit for unauthenticated user without request context")
		}
		key = anonymousKey(req.RealIP, r.scope)
	}

	rstore := r.rstore.WithContext(ctx)

	// Check the current usage. If
	// no record exists, redis will return 0 and ErrNil.
	currentUsage, err := rstore.Get(key).Int()
	if err != nil && err != redis.ErrNil {
		return errors.Wrap(err, "failed to read rate limit counter")
	}

	// If the usage exceeds the maximum, we return an error. Consumers can check if
	// the error is of type RateLimitExceededError and extract additional information
	// like the limit and the time by when they should retry.
	if currentUsage >= limit || featureflag.FromContext(ctx).GetBoolOr("rate-limits-exceeded-for-testing", false) {
		// Read TTL to compute the RetryAfter time.
		ttl, err := rstore.TTL(key)
		if err != nil {
			return errors.Wrap(err, "failed to get TTL for rate limit counter")
		}
		return RateLimitExceededError{
			Scope: r.scope,
			Limit: limit,
			// Return the minimum value of currentUsage and limit to not return
			// confusing values when the limit was exceeded. This method increases
			// on every check, even if the limit was reached.
			Used:       min(currentUsage, limit),
			RetryAfter: time.Now().Add(time.Duration(ttl) * time.Second),
		}
	}

	// Now that we know that we want to let the user pass, let's increment the rate
	// limit counter for the user.
	// Note that the rate limiter _may_ allow slightly more requests than the configured
	// limit, incrementing the rate limit counter and reading the usage futher up are currently
	// not an atomic operation, because there is no good way to read the TTL in a transaction
	// without a lua script.
	// This approach could also slightly overcount the usage if redis requests after
	// the INCR fail, but it will always recover safely.
	// If Incr works but then everything else fails (eg ctx cancelled) the user spent
	// a token without getting anything for it. This seems pretty rare and a fine trade-off
	// since its just one token. The most likely reason this would happen is user cancelling
	// the request and at that point its more likely to happen while the LLM is running than
	// in this quick redis block.
	// On the first request in the current time block, if the requests past Incr fail we don't
	// yet have a deadline set. This means if the user comes back later we wouldn't of expired
	// just one token. This seems fine. Note: this isn't an issue on subsequent requests in the
	// same time block since the TTL would have been set.

	if _, err := rstore.Incr(key); err != nil {
		return errors.Wrap(err, "failed to increment rate limit counter")
	}

	// Set expiry on the key. If the key didn't exist prior to the previous INCR,
	// it will set the expiry of the key to one day.
	// If it did exist before, it should have an expiry set already, so the TTL >= 0
	// makes sure that we don't overwrite it and restart the 1h bucket.
	ttl, err := rstore.TTL(key)
	if err != nil {
		return errors.Wrap(err, "failed to get TTL for rate limit counter")
	}
	if ttl < 0 {
		if err := rstore.Expire(key, int(24*time.Hour/time.Second)); err != nil {
			return errors.Wrap(err, "failed to set expiry for rate limit counter")
		}
	}

	return nil
}

func userKey(userID int32, scope types.CompletionsFeature) string {
	return fmt.Sprintf("user:%d:%s_requests", userID, scope)
}

func anonymousKey(ip string, scope types.CompletionsFeature) string {
	return fmt.Sprintf("anon:%s:%s_requests", ip, scope)
}

func getConfiguredLimit(ctx context.Context, db database.DB, scope types.CompletionsFeature) (int, error) {
	a := actor.FromContext(ctx)
	if a.IsAuthenticated() && !a.IsInternal() {
		var limit *int
		var err error

		// If an authenticated user exists, check if an override exists.
		switch scope {
		case types.CompletionsFeatureChat:
			limit, err = db.Users().GetChatCompletionsQuota(ctx, a.UID)
		case types.CompletionsFeatureCode:
			limit, err = db.Users().GetCodeCompletionsQuota(ctx, a.UID)
		default:
			return 0, errors.Newf("unknown scope: %s", scope)
		}
		if err != nil {
			return 0, err
		}
		if limit != nil {
			return *limit, err
		}
	}

	// Otherwise, fall back to the global limit.
	cfg := conf.GetCompletionsConfig(conf.Get().SiteConfig())
	switch scope {
	case types.CompletionsFeatureChat:
		if cfg != nil && cfg.PerUserDailyLimit > 0 {
			return cfg.PerUserDailyLimit, nil
		}
	case types.CompletionsFeatureCode:
		if cfg != nil && cfg.PerUserCodeCompletionsDailyLimit > 0 {
			return cfg.PerUserCodeCompletionsDailyLimit, nil
		}
	default:
		return 0, errors.Newf("unknown scope: %s", scope)
	}

	return 0, nil
}
