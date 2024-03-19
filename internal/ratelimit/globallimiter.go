package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// tokenBucketGlobalPrefix is the prefix used for all global rate limiter configurations in Redis,
// it is overwritten in SetupForTest to allow unique namespacing.
var tokenBucketGlobalPrefix = "v2:rate_limiters"

const (
	GitRPSLimiterBucketName                   = "git-rps"
	bucketLastReplenishmentTimestampKeySuffix = "last_replenishment_timestamp"
	bucketAllowedBurstKeySuffix               = "allowed_burst"
	bucketRateConfigKeySuffix                 = "config:bucket_rate"
	bucketReplenishmentConfigKeySuffix        = "config:bucket_replenishment_interval_seconds"
	defaultBurst                              = 10
)

// GlobalLimiter is a Redis-backed rate limiter that implements the token bucket
// algorithm.
// NOTE: This limiter needs to be backed by a syncer that will dump its configurations into Redis.
// See cmd/worker/internal/ratelimit/job.go for an example.
type GlobalLimiter interface {
	// Wait is a shorthand for WaitN(ctx, 1).
	Wait(ctx context.Context) error
	// WaitN gets N tokens from the specified rate limit token bucket. It is a synchronous operation
	// and will wait until the tokens are permitted to be used or context is canceled before returning.
	WaitN(ctx context.Context, n int) error

	// SetTokenBucketConfig sets the configuration for the specified token bucket.
	// bucketName: the name of the bucket where the tokens are, e.g. github.com:api_tokens
	// bucketQuota: the number of tokens to replenish over a bucketReplenishIntervalSeconds interval of time.
	// bucketReplenishIntervalSeconds: how often (in seconds) the bucket should replenish bucketQuota tokens.
	SetTokenBucketConfig(ctx context.Context, bucketQuota int32, bucketReplenishInterval time.Duration) error
}

type globalRateLimiter struct {
	prefix     string
	bucketName string
	pool       *redis.Pool
	logger     log.Logger

	// optionally used for tests
	nowFunc func() time.Time
	// optionally used for tests
	timerFunc func(d time.Duration) (<-chan time.Time, func() bool)
}

func NewGlobalRateLimiter(logger log.Logger, bucketName string) GlobalLimiter {
	logger = logger.Scoped(fmt.Sprintf("GlobalRateLimiter.%s", bucketName))

	return &globalRateLimiter{
		prefix:     tokenBucketGlobalPrefix,
		bucketName: bucketName,
		pool:       kv().Pool(),
		logger:     logger,
	}
}

func NewTestGlobalRateLimiter(pool *redis.Pool, prefix, bucketName string) GlobalLimiter {
	return &globalRateLimiter{
		prefix:     prefix,
		bucketName: bucketName,
		pool:       pool,
	}
}

func (r *globalRateLimiter) Wait(ctx context.Context) error {
	return r.WaitN(ctx, 1)
}

func (r *globalRateLimiter) WaitN(ctx context.Context, n int) (err error) {
	now := r.now()

	// Check if ctx is already cancelled.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Determine wait limit.
	waitLimit := time.Duration(-1)
	if deadline, ok := ctx.Deadline(); ok {
		waitLimit = deadline.Sub(now)
	}

	// Reserve a token from the bucket.
	timeToWait, err := r.waitn(ctx, n, now, waitLimit)
	if err != nil {
		return err
	}

	// If no need to wait, return immediately.
	if timeToWait == 0 {
		return nil
	}

	// Wait for the required time before the token can be used.
	ch, stop := r.newTimer(timeToWait)
	defer stop()
	select {
	case <-ch:
		// We can proceed.
		return nil
	case <-ctx.Done():
		// Note: rate.Limiter would return the tokens to the bucket
		// here, we don't do that for simplicity.
		return ctx.Err()
	}
}

func (r *globalRateLimiter) now() time.Time {
	if r.nowFunc != nil {
		return r.nowFunc()
	}
	return time.Now()
}

func (r *globalRateLimiter) newTimer(d time.Duration) (<-chan time.Time, func() bool) {
	if r.timerFunc != nil {
		return r.timerFunc(d)
	}

	timer := time.NewTimer(d)
	return timer.C, timer.Stop
}

func (r *globalRateLimiter) waitn(ctx context.Context, n int, requestTime time.Time, maxTimeToWait time.Duration) (timeToWait time.Duration, err error) {
	metricLimiterAttempts.WithLabelValues(r.bucketName).Inc()
	metricLimiterWaiting.WithLabelValues(r.bucketName).Inc()
	defer metricLimiterWaiting.WithLabelValues(r.bucketName).Dec()
	keys := getRateLimiterKeys(r.prefix, r.bucketName)
	connection := r.pool.Get()
	defer connection.Close()

	fallbackRateLimit := -1 // equivalent of rate.Inf
	// the rate limit in the config is in requests per hour, whereas rate.Limit is in
	// requests per second.
	if rate := conf.Get().DefaultRateLimit; rate != nil {
		fallbackRateLimit = *rate
	}

	maxWaitTime := int32(-1)
	if maxTimeToWait != -1 {
		maxWaitTime = int32(maxTimeToWait.Seconds())
	}
	result, err := invokeScriptWithRetries(
		ctx,
		getTokensScript,
		connection,
		keys.BucketKey, keys.LastReplenishmentTimestampKey, keys.RateKey, keys.ReplenishmentIntervalSecondsKey, keys.BurstKey,
		requestTime.Unix(),
		maxWaitTime,
		int32(fallbackRateLimit),
		int32(time.Hour/time.Second),
		defaultBurst,
		n,
	)
	if err != nil {
		metricLimiterFailedAcquire.WithLabelValues(r.bucketName).Inc()
		r.logger.Error("failed to acquire global rate limiter, falling back to default in-memory limiter", log.Error(err))
		// If using the real global limiter fails, we fall back to the in-memory registry
		// of rate limiters. This rate limiter is NOT synced across services, so when these
		// errors occur, admins should fix their redis connection stability! Since these
		// rate limiters are not configured by the worker job, the default rate limit will
		// be used, which can be configured using site config under `.defaultRateLimit`.

		defaultRateLimit := 3600 // Allow 1 request / s per code host in fallback mode, if defaultRateLimit is not configured.
		if rate := conf.Get().DefaultRateLimit; rate != nil {
			defaultRateLimit = *rate
		}
		rl := getInMemoryLimiter(r.bucketName, defaultRateLimit)
		return 0, rl.WaitN(ctx, n)
	}

	scriptResponse, ok := result.([]interface{})
	if !ok || len(scriptResponse) != 2 {
		return 0, errors.Newf("unexpected response from Redis when getting tokens from bucket: %s, response: %+v", keys.BucketKey)
	}

	allowedInt, ok := scriptResponse[0].(int64)
	if !ok {
		return 0, errors.Newf("unexpected response for allowed, expected int64 but got %T", allowedInt)
	}

	timeToWaitSeconds, ok := scriptResponse[1].(int64)
	if !ok {
		return 0, errors.Newf("unexpected response for timeToWait, expected int64, got %T", timeToWaitSeconds)
	}

	timeToWait = time.Duration(timeToWaitSeconds) * time.Second
	return timeToWait, getTokenBucketError(keys.BucketKey, getTokenGrantType(allowedInt), timeToWait)
}

const (
	scriptInvocationMaxRetries      = 8
	scriptInvocationMinRetryDelayMs = 50
	scriptInvocationMaxRetryDelayMs = 250
)

func invokeScriptWithRetries(ctx context.Context, script *redis.Script, c redis.Conn, keysAndArgs ...any) (result any, err error) {
	for range scriptInvocationMaxRetries {
		result, err = script.DoContext(ctx, c, keysAndArgs...)
		if err == nil {
			// If no error, return the result.
			return result, nil
		}

		delayMs := rand.Intn(scriptInvocationMaxRetryDelayMs-scriptInvocationMinRetryDelayMs) + scriptInvocationMinRetryDelayMs
		sleepDelay := time.Duration(delayMs) * time.Millisecond
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(sleepDelay):
			// Continue.
		}
	}

	return nil, err
}

func getTokenBucketError(bucketKey string, allowed getTokenGrantType, timeToWait time.Duration) error {
	switch allowed {
	case tokenGranted:
		return nil
	case waitTimeExceedsDeadline:
		return WaitTimeExceedsDeadlineError{
			timeToWait:     timeToWait,
			tokenBucketKey: bucketKey,
		}
	case negativeTimeDifference:
		return NegativeTimeDifferenceError{tokenBucketKey: bucketKey}
	case allBlocked:
		return AllBlockedError{tokenBucketKey: bucketKey}
	default:
		return UnexpectedRateLimitReturnError{tokenBucketKey: bucketKey}
	}
}

func (r *globalRateLimiter) SetTokenBucketConfig(ctx context.Context, bucketQuota int32, bucketReplenishInterval time.Duration) error {
	keys := getRateLimiterKeys(r.prefix, r.bucketName)
	connection := r.pool.Get()
	defer connection.Close()

	_, err := setReplenishmentScript.DoContext(ctx, connection, keys.RateKey, keys.ReplenishmentIntervalSecondsKey, keys.BurstKey, bucketQuota, bucketReplenishInterval.Seconds(), defaultBurst)
	return errors.Wrapf(err, "error while setting token bucket replenishment for bucket %s", r.bucketName)
}

func getRateLimiterKeys(prefix, bucketName string) rateLimitBucketConfigKeys {
	var keys rateLimitBucketConfigKeys
	// e.g. v2:rate_limiters:github.com
	keys.BucketKey = fmt.Sprintf("%s:%s", prefix, bucketName)
	// e.g. v2:rate_limiters:github.com:config:bucket_rate
	keys.RateKey = fmt.Sprintf("%s:%s", keys.BucketKey, bucketRateConfigKeySuffix)
	// e.g.. v2:rate_limiters:github.com:config:bucket_replenishment_interval_seconds
	keys.ReplenishmentIntervalSecondsKey = fmt.Sprintf("%s:%s", keys.BucketKey, bucketReplenishmentConfigKeySuffix)
	// e.g.. v2:rate_limiters:github.com:last_replenishment_timestamp
	keys.LastReplenishmentTimestampKey = fmt.Sprintf("%s:%s", keys.BucketKey, bucketLastReplenishmentTimestampKeySuffix)
	// e.g.. v2:rate_limiters:github.com:allowed_burst
	keys.BurstKey = fmt.Sprintf("%s:%s", keys.BucketKey, bucketAllowedBurstKeySuffix)

	return keys
}

var (
	getTokensScript        = redis.NewScript(5, getTokensFromBucketLuaScript)
	setReplenishmentScript = redis.NewScript(3, setTokenBucketReplenishmentLuaScript)
)

// getTokensFromBucketLuaScript gets a single token from the specified bucket.
// bucket_key: the key in Redis that stores the bucket, under which all the bucket's tokens and rate limit configs are found, e.g. v2:rate_limiters:github.com:api_tokens.
// last_replenishment_timestamp_key: the key in Redis that stores the timestamp (seconds since epoch) of the last bucket replenishment, e.g. v2:rate_limiters:github.com:api_tokens:last_replenishment_timestamp.
// bucket_quota_key: the key in Redis that stores how many tokens the bucket should refill in a `bucket_replenishment_interval` period of time, e.g. v2:rate_limiters:github.com:api_tokens:config:bucket_quota.
// bucket_replenishment_interval_key: the key in Redis that stores how often (in seconds), the bucket should be replenished bucket_quota tokens, e.g. v2:rate_limiters:github.com:api_tokens:config:bucket_replenishment_interval_seconds.
// burst: the amount of tokens the bucket can hold, always bucketMaxCapacity right now.
// current_time: current time (seconds since epoch).
// max_time_to_wait_for_token: the maximum amount of time (in seconds) the requester is willing to wait before acquiring/using a token.
//
//go:embed globallimitergettokens.lua
var getTokensFromBucketLuaScript string

//go:embed globallimitersettokenbucket.lua
var setTokenBucketReplenishmentLuaScript string

type getTokenGrantType int64

const (
	tokenGranted            getTokenGrantType = 1
	waitTimeExceedsDeadline getTokenGrantType = -1
	negativeTimeDifference  getTokenGrantType = -2
	allBlocked              getTokenGrantType = -3
)

type rateLimitBucketConfigKeys struct {
	BucketKey                       string
	RateKey                         string
	ReplenishmentIntervalSecondsKey string
	LastReplenishmentTimestampKey   string
	BurstKey                        string
}

type WaitTimeExceedsDeadlineError struct {
	tokenBucketKey string
	timeToWait     time.Duration
}

func (e WaitTimeExceedsDeadlineError) Error() string {
	return fmt.Sprintf("bucket:%s, acquiring token would require a wait of %s which exceeds the context deadline", e.tokenBucketKey, e.timeToWait.String())
}

type NegativeTimeDifferenceError struct {
	tokenBucketKey string
}

func (e NegativeTimeDifferenceError) Error() string {
	return fmt.Sprintf("bucket:%s, time difference between now and the last replenishment is negative", e.tokenBucketKey)
}

type AllBlockedError struct {
	tokenBucketKey string
}

func (e AllBlockedError) Error() string {
	return fmt.Sprintf("bucket:%s, rate is 0, no requests permitted", e.tokenBucketKey)
}

type UnexpectedRateLimitReturnError struct {
	tokenBucketKey string
}

func (e UnexpectedRateLimitReturnError) Error() string {
	return fmt.Sprintf("bucket:%s, unexpected return code from rate limit script", e.tokenBucketKey)
}

type GlobalLimiterInfo struct {
	// CurrentCapacity is the current number of tokens in the bucket.
	CurrentCapacity int
	// Burst is the maximum number of allowed burst.
	Burst int
	// Limit is the number of maximum allowed requests per interval. If the limit is
	// infinite, Limit will be -1 and Infinite will be true.
	Limit int
	// Interval is the interval over which the number of requests can be made.
	// For example: Limit: 3600, Interval: hour means 3600 requests per hour,
	// expressed internally as 1/s.
	Interval time.Duration
	// LastReplenishment is the time the bucket has been last replenished. Replenishment
	// only happens when borrowed from the bucket.
	LastReplenishment time.Time
	// Infinite is true if Limit is infinite. This is required since infinity cannot
	// be marshalled in JSON.
	Infinite bool
}

// GetGlobalLimiterState reports how all the existing rate limiters are configured,
// keyed by bucket name.
func GetGlobalLimiterState(ctx context.Context) (map[string]GlobalLimiterInfo, error) {
	return GetGlobalLimiterStateFromPool(ctx, kv().Pool(), tokenBucketGlobalPrefix)
}

func GetGlobalLimiterStateFromPool(ctx context.Context, pool *redis.Pool, prefix string) (map[string]GlobalLimiterInfo, error) {
	conn, err := pool.GetContext(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get connection")
	}
	defer conn.Close()

	// First, find all known limiters in redis.
	resp, err := conn.Do("KEYS", fmt.Sprintf("%s:*:%s", prefix, bucketAllowedBurstKeySuffix))
	if err != nil {
		return nil, errors.Wrap(err, "failed to list keys")
	}
	keys, ok := resp.([]interface{})
	if !ok {
		return nil, errors.Newf("invalid response from redis keys command, expected []interface{}, got %T", resp)
	}

	m := make(map[string]GlobalLimiterInfo, len(keys))
	for _, k := range keys {
		kchars, ok := k.([]uint8)
		if !ok {
			return nil, errors.Newf("invalid response from redis keys command, expected string, got %T", k)
		}
		key := string(kchars)
		limiterName := strings.TrimSuffix(strings.TrimPrefix(key, prefix+":"), ":"+bucketAllowedBurstKeySuffix)
		rlKeys := getRateLimiterKeys(prefix, limiterName)

		rstore := redispool.RedisKeyValue(pool)

		currentCapacity, err := rstore.Get(rlKeys.BucketKey).Int()
		if err != nil && err != redis.ErrNil {
			return nil, errors.Wrap(err, "failed to read current capacity")
		}

		burst, err := rstore.Get(rlKeys.BurstKey).Int()
		if err != nil && err != redis.ErrNil {
			return nil, errors.Wrap(err, "failed to read burst config")
		}

		rate, err := rstore.Get(rlKeys.RateKey).Int()
		if err != nil && err != redis.ErrNil {
			return nil, errors.Wrap(err, "failed to read rate config")
		}

		intervalSeconds, err := rstore.Get(rlKeys.ReplenishmentIntervalSecondsKey).Int()
		if err != nil && err != redis.ErrNil {
			return nil, errors.Wrap(err, "failed to read interval config")
		}

		lastReplenishment, err := rstore.Get(rlKeys.LastReplenishmentTimestampKey).Int()
		if err != nil && err != redis.ErrNil {
			return nil, errors.Wrap(err, "failed to read last replenishment")
		}

		info := GlobalLimiterInfo{
			CurrentCapacity:   currentCapacity,
			Burst:             burst,
			Limit:             rate,
			LastReplenishment: time.Unix(int64(lastReplenishment), 0),
			Interval:          time.Duration(intervalSeconds) * time.Second,
		}
		if rate == -1 {
			info.Limit = 0
			info.Infinite = true
		}
		m[limiterName] = info
	}

	return m, nil
}

var (
	// inMemoryLimitersMapMu protects access to inMemoryLimitersMap.
	inMemoryLimitersMapMu sync.Mutex
	// inMemoryLimitersMap contains all the in-memory rate limiters keyed by name.
	inMemoryLimitersMap = make(map[string]GlobalLimiter)
)

// getInMemoryLimiter in app mode, we don't have a working redis, so our limiters
// are in memory instead. Since we only have a single binary in app, this is actually
// just as global as it is in multi-container deployments with redis as the backing
// store. When used as the fallback limiter for a failing redis-backed limiter, it
// is a best-effort limiter and not actually configured with code-host rate limits.
func getInMemoryLimiter(name string, defaultPerHour int) GlobalLimiter {
	inMemoryLimitersMapMu.Lock()
	l, ok := inMemoryLimitersMap[name]
	if !ok {
		r := rate.Limit(defaultPerHour / 3600)
		if defaultPerHour < 0 {
			r = rate.Inf
		}
		l = &inMemoryLimiter{rl: rate.NewLimiter(r, defaultBurst)}
		inMemoryLimitersMap[name] = l
	}
	inMemoryLimitersMapMu.Unlock()
	return l
}

type inMemoryLimiter struct {
	rl *rate.Limiter
}

func (rl *inMemoryLimiter) Wait(ctx context.Context) error {
	return rl.rl.Wait(ctx)
}

func (rl *inMemoryLimiter) WaitN(ctx context.Context, n int) error {
	return rl.rl.WaitN(ctx, n)
}

func (rl *inMemoryLimiter) SetTokenBucketConfig(ctx context.Context, bucketQuota int32, bucketReplenishInterval time.Duration) error {
	rate := rate.Limit(bucketQuota) / rate.Limit(bucketReplenishInterval.Seconds())
	rl.rl.SetLimit(rate)
	rl.rl.SetBurst(defaultBurst)

	return nil
}

// Below is setup code for testing:

// TB is a subset of testing.TB
type TB interface {
	Name() string
	Skip(args ...any)
	Helper()
	Fatalf(string, ...any)
}

// SetupForTest adjusts the tokenBucketGlobalPrefix and clears it out. You will have
// conflicts if you do `t.Parallel()`.
func SetupForTest(t TB) {
	t.Helper()

	pool := &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", "127.0.0.1:6379")
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
	kvMock = redispool.RedisKeyValue(pool)

	tokenBucketGlobalPrefix = "__test__" + t.Name()
	c := pool.Get()
	defer c.Close()

	// If we are not on CI, skip the test if our redis connection fails.
	if os.Getenv("CI") == "" {
		_, err := c.Do("PING")
		if err != nil {
			t.Skip("could not connect to redis", err)
		}
	}

	err := redispool.DeleteAllKeysWithPrefix(c, tokenBucketGlobalPrefix)
	if err != nil {
		t.Fatalf("cold not clear test prefix: &v", err)
	}
}

var kvMock redispool.KeyValue

func kv() redispool.KeyValue {
	if kvMock != nil {
		return kvMock
	}
	return redispool.Store
}

// metrics.
var (
	metricLimiterAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_globallimiter_attempts",
		Help: "Incremented each time we request a token from a rate limiter.",
	}, []string{"bucket"})
	metricLimiterWaiting = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_globallimiter_waiting",
		Help: "Number of rate limiter requests that are pending.",
	}, []string{"bucket"})
	// TODO: Once we add Grafana dashboards, add an alert on this metric.
	metricLimiterFailedAcquire = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_globallimiter_failed_acquire",
		Help: "Incremented each time requesting a token from a rate limiter fails after retries.",
	}, []string{"bucket"})
)
