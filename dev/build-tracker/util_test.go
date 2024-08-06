package main

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/exp/maps"

	"github.com/sourcegraph/sourcegraph/dev/build-tracker/build"
)

func mockRedisClient() *build.MockRedisClient {
	bjsons := make(map[string][]byte)
	failureCount := 0
	rclient := build.NewMockRedisClient()
	rclient.SetFunc.SetDefaultHook(func(ctx context.Context, s string, i interface{}, d time.Duration) *redis.StatusCmd {
		if strings.HasPrefix(s, "build/") {
			bjsons[s] = i.([]byte)
		} else {
			failureCount = 0
		}
		return redis.NewStatusCmd(context.Background())
	})
	rclient.KeysFunc.SetDefaultHook(func(ctx context.Context, s string) *redis.StringSliceCmd {
		return redis.NewStringSliceResult(maps.Keys(bjsons), nil)
	})
	rclient.MGetFunc.SetDefaultHook(func(ctx context.Context, s ...string) *redis.SliceCmd {
		var result []interface{}
		for _, key := range s {
			result = append(result, string(bjsons[key]))
		}
		return redis.NewSliceResult(result, nil)
	})
	rclient.DelFunc.PushHook(func(ctx context.Context, s ...string) *redis.IntCmd {
		for _, key := range s {
			delete(bjsons, key)
		}
		return redis.NewIntCmd(ctx)
	})
	rclient.GetFunc.SetDefaultHook(func(ctx context.Context, s string) *redis.StringCmd {
		if b, ok := bjsons[s]; strings.HasPrefix(s, "build/") && ok {
			return redis.NewStringResult(string(b), nil)
		}
		return redis.NewStringResult("", redis.Nil)
	})
	rclient.IncrFunc.PushHook(func(ctx context.Context, s string) *redis.IntCmd {
		failureCount++
		return redis.NewIntResult(int64(failureCount), nil)
	})
	return rclient
}
