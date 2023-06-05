package actor

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/enterprise/cmd/cody-gateway/internal/limiter"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type RateLimit struct {
	AllowedModels []string      `json:"allowedModels"`
	Limit         int           `json:"limit"`
	Interval      time.Duration `json:"interval"`
}

func (r *RateLimit) IsValid() bool {
	return r != nil && r.Interval > 0 && r.Limit > 0 && len(r.AllowedModels) > 0
}

type Actor struct {
	// Key is the original key used to identify the actor. It may be a sensitive value
	// so use with care!
	//
	// For example, for product subscriptions this is the license-based access token.
	Key string `json:"key"`
	// ID is the identifier for this actor's rate-limiting pool. It is not a sensitive
	// value.
	//
	// For example, for product subscriptions this is the subscription UUID. For
	// Sourcegraph.com users, this is the string representation of the user ID.
	ID string `json:"id"`
	// AccessEnabled is an evaluated field that summarizes whether or not Cody Gateway access
	// is enabled.
	//
	// For example, for product subscriptions it is based on whether the subscription is
	// archived, if access is enabled, and if any rate limits are set.
	AccessEnabled bool `json:"accessEnabled"`
	// RateLimits holds the rate limits for Cody Gateway access for this actor.
	RateLimits map[types.CompletionsFeature]RateLimit `json:"rateLimits"`
	// LastUpdated indicates when this actor's state was last updated.
	LastUpdated *time.Time `json:"lastUpdated"`
	// Source is a reference to the source of this actor's state.
	Source Source `json:"-"`
}

type contextKey int

const actorKey contextKey = iota

// FromContext returns a new Actor instance from a given context. It always
// returns a non-nil actor.
func FromContext(ctx context.Context) *Actor {
	a, ok := ctx.Value(actorKey).(*Actor)
	if !ok || a == nil {
		return &Actor{}
	}
	return a
}

// Logger returns a logger that has metadata about the actor attached to it.
func (a *Actor) Logger(logger log.Logger) log.Logger {
	if a == nil {
		return logger
	}
	return logger.With(
		log.String("actor.ID", a.ID),
		log.String("actor.Source", a.Source.Name()),
		log.Bool("actor.AccessEnabled", a.AccessEnabled),
		log.Timep("actor.LastUpdated", a.LastUpdated),
	)
}

// Update updates the given actor's state using the actor's originating source
// if it implements SourceUpdater.
//
// The source may define additional conditions for updates, such that an update
// does not necessarily occur on every call.
//
// If the actor has no source, this is a no-op.
func (a *Actor) Update(ctx context.Context) {
	if su, ok := a.Source.(SourceUpdater); ok && su != nil {
		su.Update(ctx, a)
	}
}

// WithActor returns a new context with the given Actor instance.
func WithActor(ctx context.Context, a *Actor) context.Context {
	return context.WithValue(ctx, actorKey, a)
}

func (a *Actor) Limiter(
	redis limiter.RedisStore,
	feature types.CompletionsFeature,
	concurrentLimitConfig codygateway.ConcurrentLimitConfig,
) (limiter.Limiter, bool) {
	if a == nil {
		// Not logged in, no limit applicable.
		return nil, false
	}
	limit, ok := a.RateLimits[feature]
	if !ok {
		return nil, false
	}

	// The actual type of time.Duration is int64, so we can use it to compute the
	// ratio of the rate limit interval to a day (24 hours).
	ratioToDay := float32(limit.Interval / (24 * time.Hour))
	// Then use the ratio to compute the rate limit for a day.
	dailyLimit := float32(limit.Limit) / ratioToDay
	// Finally, compute the concurrent limit with the given percentage of the daily limit.
	concurrentLimit := int(dailyLimit * concurrentLimitConfig.Percentage)

	// The redis store has to use a prefix for the given feature because we need to
	// rate limit by feature.
	featurePrefix := fmt.Sprintf("%s:", feature)
	concurrentLimiter := &concurrentLimiter{
		actor:   a,
		feature: feature,
		redis:   limiter.NewPrefixRedisStore(fmt.Sprintf("concurrent:%s", featurePrefix), redis),
		rateLimit: RateLimit{
			Limit:    concurrentLimit,
			Interval: concurrentLimitConfig.Interval,
		},
		featureLimiter: updateOnFailureLimiter{
			Redis:     limiter.NewPrefixRedisStore(featurePrefix, redis),
			RateLimit: limit,
			Actor:     a,
		},
	}
	return concurrentLimiter, true
}

type concurrentLimiter struct {
	actor          *Actor
	feature        types.CompletionsFeature
	redis          limiter.RedisStore
	rateLimit      RateLimit
	featureLimiter limiter.Limiter

	// Optional stub for current time. Used for testing.
	nowFunc func() time.Time
}

func (l *concurrentLimiter) TryAcquire(ctx context.Context) (func() error, error) {
	commit, err := (limiter.StaticLimiter{
		Identifier: l.actor.ID,
		Redis:      l.redis,
		Limit:      l.rateLimit.Limit,
		Interval:   l.rateLimit.Interval,
		// Only update rate limit TTL if the actor has been updated recently.
		UpdateRateLimitTTL: l.actor.LastUpdated != nil && time.Since(*l.actor.LastUpdated) < 5*time.Minute,
	}).TryAcquire(ctx)
	if err != nil {
		if errors.As(err, &limiter.NoAccessError{}) || errors.As(err, &limiter.RateLimitExceededError{}) {
			retryAfter, err := limiter.RetryAfterWithTTL(l.redis, l.nowFunc, l.actor.ID)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get TTL for rate limit counter")
			}
			return nil, ErrConcurrentLimitExceeded{
				feature:    l.feature,
				limit:      l.rateLimit.Limit,
				retryAfter: retryAfter,
			}
		}
		return nil, errors.Wrap(err, "check concurrent limit")
	}

	featureCommit, err := l.featureLimiter.TryAcquire(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "check feature rate limit")
	}
	return func() error {
		if err := featureCommit(); err != nil {
			return err
		}
		return commit()
	}, nil
}

type ErrConcurrentLimitExceeded struct {
	feature    types.CompletionsFeature
	limit      int
	retryAfter time.Time
}

func (e ErrConcurrentLimitExceeded) Error() string {
	return fmt.Sprintf("you exceeded the concurrency limit of %d requests for %q. Retry after %s",
		e.limit, e.feature, e.retryAfter.Truncate(time.Second))
}

type updateOnFailureLimiter struct {
	Redis     limiter.RedisStore
	RateLimit RateLimit
	*Actor
}

func (u updateOnFailureLimiter) TryAcquire(ctx context.Context) (func() error, error) {
	commit, err := (limiter.StaticLimiter{
		Identifier: u.ID,
		Redis:      u.Redis,
		Limit:      u.RateLimit.Limit,
		Interval:   u.RateLimit.Interval,
		// Only update rate limit TTL if the actor has been updated recently.
		UpdateRateLimitTTL: u.LastUpdated != nil && time.Since(*u.LastUpdated) < 5*time.Minute,
	}).TryAcquire(ctx)

	if errors.As(err, &limiter.NoAccessError{}) || errors.As(err, &limiter.RateLimitExceededError{}) {
		u.Actor.Update(ctx) // TODO: run this in goroutine+background context maybe?
	}

	return commit, err
}

// ErrAccessTokenDenied is returned when the access token is denied due to the
// reason.
type ErrAccessTokenDenied struct {
	Reason string
}

func (e ErrAccessTokenDenied) Error() string {
	return fmt.Sprintf("access token denied: %s", e.Reason)
}
