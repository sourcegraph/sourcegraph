package streaming

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
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
	Limit      int
	Used       int
	RetryAfter time.Time
}

func (e RateLimitExceededError) Error() string {
	return fmt.Sprintf("you exceeded the rate limit for completions, only %d requests are allowed per day at the moment to ensure the service stays functional. Current usage: %d. Retry after %s. \n\n Interested in using Cody with higher rates and priority support? Cody now comes with Sourcegraph Enterprise, <a href='https://about.sourcegraph.com/contact/request-info'>contact our sales team</a> to discuss customized plans for your business. ", e.Limit, e.Used, e.RetryAfter.Truncate(time.Second))
}

func NewRateLimiter(db database.DB, rstore redispool.KeyValue) RateLimiter {
	return &rateLimiter{db: db, rstore: rstore}
}

type rateLimiter struct {
	rstore redispool.KeyValue
	db     database.DB
}

func (r *rateLimiter) TryAcquire(ctx context.Context) (err error) {
	limit, err := getConfiguredLimit(ctx, r.db)
	if err != nil {
		return errors.Wrap(err, "failed to read configured rate limit")
	}

	if limit == 0 {
		// Rate limiting disabled.
		return nil
	}

	// Check that the user is authenticated.
	a := actor.FromContext(ctx)
	if a.IsInternal() {
		return nil
	}
	key := userKey(a.UID)
	if !a.IsAuthenticated() {
		// Fall back to the IP address, if provided in context (ie. this is a request handler).
		req := requestclient.FromContext(ctx)
		var ip string
		if req != nil {
			ip = req.IP
			// Note: ForwardedFor header in general can be spoofed. For
			// Sourcegraph.com we use a trusted value for this so this is a
			// reliable value to rate limit with.
			if req.ForwardedFor != "" {
				ip = req.ForwardedFor
			}
		}
		if ip == "" {
			return errors.Wrap(auth.ErrNotAuthenticated, "cannot claim rate limit for unauthenticated user without request context")
		}
		key = anonymousKey(ip)
	}

	rstore := r.rstore.WithContext(ctx)

	// Let's increment the rate limit counter for the user.
	// Note that the rate limiter _may_ allow slightly less requests than the configured
	// limit, incrementing the rate limit counter and expiry operations are currently not
	// an atomic operation.
	// We also don't use a transaction in here, because there is no good way to
	// read the TTL without a lua script. This approach could slightly overcount the
	// usage if redis requests after the INCR fail, but it will always recover safely.
	// If Incr works but then everything else fails (eg ctx cancelled) the user spent
	// a token without getting anything for it. This seems pretty rare and a fine trade-off
	// since its just one token. The most likely reason this would happen is user cancelling
	// the request and at that point its more likely to happen while the LLM is running than
	// in this quick redis block.
	// On the first request in the current time block, if the requests past Incr fail we don't
	// yet have a deadline set. This means if the user comes back later we wouldn't of expired
	// just one token. This seems fine. Note: this isn't an issue on subsequent requests in the
	// same time block since the TTL would have been set.

	// Check the current usage and increment the counter for the current user. If
	// no record exists, redis will initialize it with 1.
	currentUsage, err := rstore.Incr(key)
	if err != nil {
		return errors.Wrap(err, "failed to increase rate limit counter")
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

	// If the usage exceeds the maximum, we return an error. Consumers can check if
	// the error is of type RateLimitExceededError and extract additional information
	// like the limit and the time by when they should retry.
	if currentUsage > limit {
		return RateLimitExceededError{
			Limit: limit,
			// Return the minimum value of currentUsage and limit to not return
			// confusing values when the limit was exceeded. This method increases
			// on every check, even if the limit was reached.
			Used:       min(currentUsage, limit),
			RetryAfter: time.Now().Add(time.Duration(ttl) * time.Second),
		}
	}

	return nil
}

func userKey(userID int32) string {
	return fmt.Sprintf("user:%d:completion_requests", userID)
}

func anonymousKey(ip string) string {
	return fmt.Sprintf("anon:%s:completion_requests", ip)
}

func getConfiguredLimit(ctx context.Context, db database.DB) (int, error) {
	a := actor.FromContext(ctx)
	if a.IsAuthenticated() && !a.IsInternal() {
		// If an authenticated user exists, check if an override exists.
		limit, err := db.Users().GetCompletionsQuota(ctx, a.UID)
		if err != nil {
			return 0, err
		}
		if limit != nil {
			return *limit, err
		}
	}

	// Otherwise, fall back to the global limit.
	cfg := conf.Get()
	if cfg.Completions != nil && cfg.Completions.PerUserDailyLimit > 0 {
		return cfg.Completions.PerUserDailyLimit, nil
	}

	return 0, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
