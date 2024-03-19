package featureflag

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/gomodule/redigo/redis"
	"github.com/rafaeljusto/redigomock/v3"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
)

func TestMiddleware(t *testing.T) {
	// Create a request with an actor on its context
	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	require.NoError(t, err)
	req = req.WithContext(actor.WithActor(context.Background(), actor.FromUser(1)))

	mockStore := NewMockStore()
	mockStore.GetUserFlagsFunc.SetDefaultReturn(map[string]bool{"user1": true}, nil)

	handler := http.Handler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// After going through the middleware, a request with an actor should
		// also have feature flags available.
		v, ok := FromContext(r.Context()).GetBool("user1")
		require.True(t, v && ok)
	}))
	handler = Middleware(mockStore, handler)

	handler.ServeHTTP(httptest.NewRecorder(), req)
}

func TestContextFlags_GetBool(t *testing.T) {
	setupRedisTest(t)
	mockStore := NewMockStore()
	mockStore.GetUserFlagsFunc.SetDefaultHook(func(_ context.Context, uid int32) (map[string]bool, error) {
		switch uid {
		case 1:
			return map[string]bool{"user1": true}, nil
		case 2:
			return map[string]bool{"user2": true}, nil
		default:
			return map[string]bool{}, nil
		}
	})

	actor1 := actor.FromUser(1)
	actor2 := actor.FromUser(2)

	ctx := context.Background()
	ctx = WithFlags(ctx, mockStore)

	t.Run("Make sure user1 flags are set", func(t *testing.T) {
		ctx = actor.WithActor(ctx, actor1)
		flags := FromContext(ctx)
		require.Equal(t, EvaluatedFlagSet{}, GetEvaluatedFlagSet(ctx))

		v, ok := flags.GetBool("user1")
		require.True(t, v && ok)

		require.Equal(t, EvaluatedFlagSet{"user1": true}, GetEvaluatedFlagSet(ctx))

		mockrequire.CalledN(t, mockStore.GetUserFlagsFunc, 1)
	})

	t.Run("With a new actor, the flag fetcher should re-fetch", func(t *testing.T) {
		ctx = actor.WithActor(ctx, actor2)
		flags := FromContext(ctx)
		require.Equal(t, EvaluatedFlagSet{}, GetEvaluatedFlagSet(ctx))

		v, ok := flags.GetBool("user1")
		require.False(t, v || ok)
		v, ok = flags.GetBool("user2")
		require.True(t, v && ok)

		require.Equal(t, EvaluatedFlagSet{"user2": true}, GetEvaluatedFlagSet(ctx))

		mockrequire.CalledN(t, mockStore.GetUserFlagsFunc, 4)
	})

	t.Run("With the first actor, we should return flags for the first actor and we should not call GetUserFlags again because the flags should be cached.", func(t *testing.T) {
		ctx = actor.WithActor(ctx, actor1)
		flags := FromContext(ctx)
		require.Equal(t, EvaluatedFlagSet{"user1": true}, GetEvaluatedFlagSet(ctx))

		v, ok := flags.GetBool("user1")
		require.True(t, v && ok)
		v, ok = flags.GetBool("user2")
		require.False(t, v || ok)

		mockrequire.CalledN(t, mockStore.GetUserFlagsFunc, 4)
	})

	t.Run("Clears Redis", func(t *testing.T) {
		require.Equal(t, EvaluatedFlagSet{"user1": true}, GetEvaluatedFlagSet(ctx))
		ClearEvaluatedFlagFromCache("user1")
		require.Equal(t, EvaluatedFlagSet{}, GetEvaluatedFlagSet(ctx))
	})
}

func TestContextFlags_GetBoolOr(t *testing.T) {
	setupRedisTest(t)
	mockStore := NewMockStore()
	mockStore.GetUserFlagsFunc.SetDefaultHook(func(_ context.Context, uid int32) (map[string]bool, error) {
		switch uid {
		case 1:
			return map[string]bool{"user1": true}, nil
		case 2:
			return map[string]bool{"user2": true}, nil
		default:
			return map[string]bool{}, nil
		}
	})

	actor1 := actor.FromUser(1)
	actor2 := actor.FromUser(2)

	ctx := context.Background()
	ctx = WithFlags(ctx, mockStore)

	t.Run("Make sure user1 flags are set", func(t *testing.T) {
		ctx = actor.WithActor(ctx, actor1)
		flags := FromContext(ctx)
		require.Equal(t, EvaluatedFlagSet{}, GetEvaluatedFlagSet(ctx))

		require.True(t, flags.GetBoolOr("user1", false))

		require.Equal(t, EvaluatedFlagSet{"user1": true}, GetEvaluatedFlagSet(ctx))

		mockrequire.CalledN(t, mockStore.GetUserFlagsFunc, 1)
	})

	t.Run("With a new actor, the flag fetcher should re-fetch", func(t *testing.T) {
		ctx = actor.WithActor(ctx, actor2)
		flags := FromContext(ctx)
		require.Equal(t, EvaluatedFlagSet{}, GetEvaluatedFlagSet(ctx))

		require.False(t, flags.GetBoolOr("user1", false))
		require.True(t, flags.GetBoolOr("user2", false))

		require.Equal(t, EvaluatedFlagSet{"user2": true}, GetEvaluatedFlagSet(ctx))

		mockrequire.CalledN(t, mockStore.GetUserFlagsFunc, 4)
	})

	t.Run("With the first actor, we should return flags for the first actor and we should not call GetUserFlags again because the flags should be cached.", func(t *testing.T) {
		ctx = actor.WithActor(ctx, actor1)
		flags := FromContext(ctx)
		require.Equal(t, EvaluatedFlagSet{"user1": true}, GetEvaluatedFlagSet(ctx))

		require.True(t, flags.GetBoolOr("user1", false))
		require.False(t, flags.GetBoolOr("user2", false))

		mockrequire.CalledN(t, mockStore.GetUserFlagsFunc, 4)
	})

	t.Run("Clears Redis", func(t *testing.T) {
		require.Equal(t, EvaluatedFlagSet{"user1": true}, GetEvaluatedFlagSet(ctx))
		ClearEvaluatedFlagFromCache("user1")
		require.Equal(t, EvaluatedFlagSet{}, GetEvaluatedFlagSet(ctx))
	})
}

func setupRedisTest(t *testing.T) {
	cache := map[string][]byte{}

	mockConn := redigomock.NewConn()

	t.Cleanup(func() { mockConn.Clear(); mockConn.Close() })

	mockConn.GenericCommand("HSET").Handle(func(args []interface{}) (interface{}, error) {
		cache[args[0].(string)] = []byte(args[2].(string))
		return nil, nil
	})

	mockConn.GenericCommand("HGET").Handle(func(args []interface{}) (interface{}, error) {
		return cache[args[0].(string)], nil
	})

	mockConn.GenericCommand("DEL").Handle(func(args []interface{}) (interface{}, error) {
		delete(cache, args[0].(string))
		return nil, nil
	})

	evalStore = redispool.RedisKeyValue(&redis.Pool{Dial: func() (redis.Conn, error) { return mockConn, nil }, MaxIdle: 10})
}
