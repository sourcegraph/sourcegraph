package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/derision-test/glock"
	"github.com/gomodule/redigo/redis"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

var testBucketName = "extsvc:999:github"

func TestGlobalRateLimiter(t *testing.T) {
	// This test is verifying the basic functionality of the rate limiter.
	// We should be able to get a token once the token bucket config is set.
	prefix := "__test__" + t.Name()
	pool := redisPoolForTest(t, prefix)
	rl := getTestRateLimiter(prefix, pool, testBucketName)

	clock := glock.NewMockClock()
	rl.nowFunc = clock.Now
	timerFuncCalled := false
	tickers := make(chan *glock.MockTicker)
	t.Cleanup(func() { close(tickers) })
	rl.timerFunc = func(d time.Duration) (<-chan time.Time, func() bool) {
		timerFuncCalled = true
		ticker := glock.NewMockTickerAt(clock.Now(), d)
		tickers <- ticker
		return ticker.Chan(), func() bool {
			ticker.Stop()
			return true
		}
	}

	// Set up the test by initializing the bucket with some initial quota and replenishment interval.
	ctx := context.Background()
	bucketQuota := int32(100)
	bucketReplenishInterval := 100 * time.Second
	// Rate is 100 / 100s, so 1/s.
	err := rl.SetTokenBucketConfig(ctx, bucketQuota, bucketReplenishInterval)
	assert.Nil(t, err)

	// Get a token from the bucket.
	{
		require.NoError(t, rl.Wait(ctx))
		assert.False(t, timerFuncCalled, "timerFunc should not be called when bucket is full")
	}
	// Exhaust the burst of the bucket entirely.
	{
		for range defaultBurst - 1 {
			require.NoError(t, rl.Wait(ctx))
			assert.False(t, timerFuncCalled, "timerFunc should not be called when bucket is full")
		}
	}

	// The time has not advanced yet, so getting another token should make us wait for the
	// replenishment timer to trigger.
	// Spawn the wait in the background, it will be blocking.
	{
		waitReturn := make(chan struct{})
		t.Cleanup(func() { close(waitReturn) })
		go func() {
			require.NoError(t, rl.Wait(ctx))
			waitReturn <- struct{}{}
		}()
		select {
		case ticker := <-tickers:
			// After 500 milliseconds, nothing should happen yet.
			ticker.Advance(500 * time.Millisecond)
			// After another 500ms, 1s total has passed, and the replenishment should
			// happen now.
			select {
			case <-waitReturn:
				t.Fatal("returned too early")
			default:
			}
			ticker.Advance(500 * time.Millisecond)
			select {
			case <-waitReturn:
			case <-time.After(100 * time.Millisecond):
				t.Fatal("timed out waiting for return")
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for wait function")
		}
	}

	// Move a second forward so the bucket capacity is 0 after replenishment again.
	// It's -1 before this.
	clock.Advance(time.Second)

	// Now we claim multiple tokens, and need to wait longer for that.
	{
		waitReturn := make(chan struct{})
		t.Cleanup(func() { close(waitReturn) })
		go func() {
			require.NoError(t, rl.WaitN(ctx, 5))
			waitReturn <- struct{}{}
		}()
		select {
		case ticker := <-tickers:
			// After 4999 milliseconds, nothing should happen yet. We need to wait
			// 5s.
			ticker.Advance(4999 * time.Millisecond)
			// After another 500ms, 1s total has passed, and the replenishment should
			// happen now.
			select {
			case <-waitReturn:
				t.Fatal("returned too early")
			default:
			}
			ticker.Advance(1 * time.Millisecond)
			select {
			case <-waitReturn:
			case <-time.After(100 * time.Millisecond):
				t.Fatal("timed out waiting for return")
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for wait function")
		}
	}
}

func TestGlobalRateLimiter_TimeToWaitExceedsLimit(t *testing.T) {
	// This test is verifying that if the amount of time needed to wait for a token
	// exceeds the context deadline, a TokenGrantExceedsLimitError is returned.
	prefix := "__test__" + t.Name()
	pool := redisPoolForTest(t, prefix)
	rl := getTestRateLimiter(prefix, pool, testBucketName)

	clock := glock.NewMockClock()
	rl.nowFunc = clock.Now
	timerFuncCalled := false
	rl.timerFunc = func(d time.Duration) (<-chan time.Time, func() bool) {
		timerFuncCalled = true
		ticker := glock.NewMockTickerAt(clock.Now(), d)
		return ticker.Chan(), func() bool {
			ticker.Stop()
			return true
		}
	}

	// Set up the test by initializing the bucket with some initial quota and replenishment interval
	ctx := context.Background()
	bucketQuota := int32(10)
	bucketReplenishInterval := 100 * time.Second
	// Rate 10/100 -> 0.1 tokens/second.
	err := rl.SetTokenBucketConfig(ctx, bucketQuota, bucketReplenishInterval)
	assert.Nil(t, err)

	// Ser a context with a deadline of 5s from now so that it can't wait the 10s to use the token.
	ctxWithDeadline, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
	t.Cleanup(cancel)

	// First, deplete the burst.
	require.NoError(t, rl.WaitN(ctx, 10))

	// Get a token from the bucket, expect that the deadline is hit.
	err = rl.Wait(ctxWithDeadline)
	var expectedErr WaitTimeExceedsDeadlineError
	assert.NotNil(t, err)
	assert.True(t, errors.As(err, &expectedErr))
	assert.False(t, timerFuncCalled, "expected no timer to be spawned")
}

func TestGlobalRateLimiter_AllBlockedError(t *testing.T) {
	// Verify that a limit of 0 means "block all".
	prefix := "__test__" + t.Name()
	pool := redisPoolForTest(t, prefix)
	rl := getTestRateLimiter(prefix, pool, testBucketName)

	clock := glock.NewMockClock()
	rl.nowFunc = clock.Now
	timerFuncCalled := false
	rl.timerFunc = func(d time.Duration) (<-chan time.Time, func() bool) {
		timerFuncCalled = true
		ticker := glock.NewMockTickerAt(clock.Now(), d)
		return ticker.Chan(), func() bool {
			ticker.Stop()
			return true
		}
	}

	// Set up the test by initializing the bucket with some initial quota and replenishment interval
	// that doesn't ever allow anything.
	ctx := context.Background()
	err := rl.SetTokenBucketConfig(ctx, 0, time.Minute)
	assert.Nil(t, err)

	// Try to get a token, it should fail.
	require.Error(t, rl.Wait(ctx), AllBlockedError{})
	assert.False(t, timerFuncCalled, "expected no timer to be spawned")
}

func TestGlobalRateLimiter_Inf(t *testing.T) {
	// Verify that a rate of -1 means inf.
	prefix := "__test__" + t.Name()
	pool := redisPoolForTest(t, prefix)
	rl := getTestRateLimiter(prefix, pool, testBucketName)

	clock := glock.NewMockClock()
	rl.nowFunc = clock.Now
	timerFuncCalled := false
	rl.timerFunc = func(d time.Duration) (<-chan time.Time, func() bool) {
		timerFuncCalled = true
		ticker := glock.NewMockTickerAt(clock.Now(), d)
		return ticker.Chan(), func() bool {
			ticker.Stop()
			return true
		}
	}

	// Set up the test by initializing the bucket with some initial quota and replenishment interval.
	ctx := context.Background()
	// Allow infinite requests.
	err := rl.SetTokenBucketConfig(ctx, -1, time.Minute)
	assert.Nil(t, err)

	// Get many from the bucket.
	require.NoError(t, rl.WaitN(ctx, 999999))
	assert.False(t, timerFuncCalled, "timerFunc should not be called when bucket is full")
}

func TestGlobalRateLimiter_UnconfiguredLimiter(t *testing.T) {
	// This test is verifying the basic functionality of the rate limiter.
	// We should be able to get a token once the token bucket config is set.
	prefix := "__test__" + t.Name()
	pool := redisPoolForTest(t, prefix)
	rl := getTestRateLimiter(prefix, pool, testBucketName)

	clock := glock.NewMockClock()
	rl.nowFunc = clock.Now
	timerFuncCalled := false
	waitDurations := make(chan time.Duration)
	t.Cleanup(func() { close(waitDurations) })
	rl.timerFunc = func(d time.Duration) (<-chan time.Time, func() bool) {
		timerFuncCalled = true
		waitDurations <- d
		ticker := glock.NewMockTickerAt(clock.Now(), d)
		return ticker.Chan(), func() bool {
			ticker.Stop()
			return true
		}
	}

	// Set up the test by initializing the bucket with some initial quota and replenishment interval.
	ctx := context.Background()
	// We do NOT call SetTokenBucketConfig here. Instead, we set a sane default.
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			DefaultRateLimit: pointers.Ptr(10),
		},
	})
	defer conf.Mock(nil)

	// First, deplete the burst.
	require.NoError(t, rl.WaitN(ctx, 10))
	assert.False(t, timerFuncCalled, "timerFunc should not be called when bucket is full")

	// Next, try to get another token. The bucket should be at capacity 0, and the
	// rate is 10/h -> 360s for one token.
	{
		go func() {
			require.NoError(t, rl.Wait(ctx))
		}()
		select {
		case duration := <-waitDurations:
			require.Equal(t, 360*time.Second, duration)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for wait function")
		}
	}
}

func Test_GetToken_CanceledContext(t *testing.T) {
	// This test is verifying that if the context we give to GetToken is
	// already canceled, then we get back a context.Canceled error.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rl := globalRateLimiter{}
	err := rl.Wait(ctx)
	assert.NotNil(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func getTestRateLimiter(prefix string, pool *redis.Pool, bucketName string) globalRateLimiter {
	return globalRateLimiter{
		pool:       pool,
		prefix:     prefix,
		bucketName: bucketName,
	}
}

// Mostly copy-pasta from rache. Will clean up later as the relationship
// between the two packages becomes cleaner.
func redisPoolForTest(t *testing.T, prefix string) *redis.Pool {
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

	c := pool.Get()
	t.Cleanup(func() {
		_ = c.Close()
	})

	if err := redispool.DeleteAllKeysWithPrefix(c, prefix); err != nil {
		t.Logf("Could not clear test prefix name=%q prefix=%q error=%v", t.Name(), prefix, err)
	}

	return pool
}

func TestLimitInfo(t *testing.T) {
	ctx := context.Background()
	prefix := "__test__" + t.Name()
	pool := redisPoolForTest(t, prefix)

	r1 := getTestRateLimiter(prefix, pool, "extsvc:github:1")
	// 1/s allowed.
	require.NoError(t, r1.SetTokenBucketConfig(ctx, 3600, time.Hour))
	r2 := getTestRateLimiter(prefix, pool, "extsvc:github:2")
	// Infinite.
	require.NoError(t, r2.SetTokenBucketConfig(ctx, -1, time.Hour))
	r3 := getTestRateLimiter(prefix, pool, "extsvc:github:3")
	// No requests allowed.
	require.NoError(t, r3.SetTokenBucketConfig(ctx, 0, time.Hour))

	info, err := GetGlobalLimiterStateFromPool(ctx, pool, prefix)
	require.NoError(t, err)

	if diff := cmp.Diff(map[string]GlobalLimiterInfo{
		"extsvc:github:1": {
			Burst:             10,
			Limit:             3600,
			Interval:          time.Hour,
			LastReplenishment: time.Unix(0, 0),
		},
		"extsvc:github:2": {
			Burst:             10,
			Limit:             0,
			Infinite:          true,
			Interval:          time.Hour,
			LastReplenishment: time.Unix(0, 0),
		},
		"extsvc:github:3": {
			Burst:             10,
			Limit:             0,
			Interval:          time.Hour,
			LastReplenishment: time.Unix(0, 0),
		},
	}, info); diff != "" {
		t.Fatal(diff)
	}

	now := time.Now().Truncate(time.Second)
	r1.nowFunc = func() time.Time { return now }
	// Now claim 3 tokens from the limiter.
	require.NoError(t, r1.WaitN(ctx, 3))

	info, err = GetGlobalLimiterStateFromPool(ctx, pool, prefix)
	require.NoError(t, err)

	if diff := cmp.Diff(map[string]GlobalLimiterInfo{
		"extsvc:github:1": {
			// Used 3 tokens, so not at full burst anymore!
			CurrentCapacity:   defaultBurst - 3,
			Burst:             10,
			Limit:             3600,
			Interval:          time.Hour,
			LastReplenishment: now,
		},
		"extsvc:github:2": {
			Burst:             10,
			Limit:             0,
			Infinite:          true,
			Interval:          time.Hour,
			LastReplenishment: time.Unix(0, 0),
		},
		"extsvc:github:3": {
			Burst:             10,
			Limit:             0,
			Interval:          time.Hour,
			LastReplenishment: time.Unix(0, 0),
		},
	}, info); diff != "" {
		t.Fatal(diff)
	}
}
