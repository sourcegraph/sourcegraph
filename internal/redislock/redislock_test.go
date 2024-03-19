package redislock

import (
	"fmt"
	"testing"
	"time"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/redispool"
)

func TestTryAcquire(t *testing.T) {
	t.Run("acquire and release", func(t *testing.T) {
		rs := redispool.NewMockKeyValue()
		rs.SetNxFunc.SetDefaultReturn(true, nil)
		acquired, release, err := TryAcquire(rs, "chicken-dinner", time.Minute)
		require.NoError(t, err)
		require.NotNil(t, release)
		assert.True(t, acquired)
		release()
	})

	t.Run("acquire and give up", func(t *testing.T) {
		rs := redispool.NewMockKeyValue()
		rs.SetNxFunc.PushReturn(true, nil)
		aliceAcquired, aliceRelease, err := TryAcquire(rs, "chicken-dinner", time.Minute)
		require.NoError(t, err)
		require.NotNil(t, aliceRelease)
		assert.True(t, aliceAcquired)
		defer aliceRelease()

		rs.SetNxFunc.PushReturn(false, nil)
		rs.GetFunc.PushReturn(redispool.NewValue(fmt.Sprintf("%d,8527", time.Now().Add(time.Minute).UnixNano()), nil))
		bobAcquired, _, err := TryAcquire(rs, "chicken-dinner", time.Minute)
		require.NoError(t, err)
		assert.False(t, bobAcquired)
		mockrequire.CalledN(t, rs.SetNxFunc, 2)
		mockrequire.Called(t, rs.GetFunc)
	})

	t.Run("acquire an expired lock", func(t *testing.T) {
		rs := redispool.NewMockKeyValue()
		rs.SetNxFunc.PushReturn(true, nil)
		aliceAcquired, aliceRelease, err := TryAcquire(rs, "chicken-dinner", time.Minute)
		require.NoError(t, err)
		require.NotNil(t, aliceRelease)
		assert.True(t, aliceAcquired)
		defer aliceRelease()

		mockCurrentLockToken := fmt.Sprintf("%d,8527", time.Now().Add(-time.Minute).UnixNano())
		rs.SetNxFunc.PushReturn(false, nil)
		rs.GetFunc.PushReturn(redispool.NewValue(mockCurrentLockToken, nil))
		rs.GetSetFunc.PushReturn(redispool.NewValue(mockCurrentLockToken, nil))
		bobAcquired, bobRelease, err := TryAcquire(rs, "chicken-dinner", time.Minute)
		require.NoError(t, err)
		require.NotNil(t, bobRelease)
		assert.True(t, bobAcquired)
		mockrequire.CalledN(t, rs.SetNxFunc, 2)
		mockrequire.Called(t, rs.GetFunc)
		mockrequire.Called(t, rs.GetSetFunc)
		bobRelease()
	})

	t.Run("acquire an expired lock but act too slow", func(t *testing.T) {
		rs := redispool.NewMockKeyValue()
		rs.SetNxFunc.PushReturn(true, nil)
		aliceAcquired, aliceRelease, err := TryAcquire(rs, "chicken-dinner", time.Minute)
		require.NoError(t, err)
		require.NotNil(t, aliceRelease)
		assert.True(t, aliceAcquired)
		defer aliceRelease()

		mockCurrentLockToken := fmt.Sprintf("%d,8527", time.Now().Add(-time.Minute).UnixNano())
		rs.SetNxFunc.PushReturn(false, nil)
		rs.GetFunc.PushReturn(redispool.NewValue(mockCurrentLockToken, nil))
		rs.GetSetFunc.PushHook(func(_ string, value any) redispool.Value {
			return redispool.NewValue(value, nil) // Return anything that's not mockCurrentLockToken
		})
		bobAcquired, _, err := TryAcquire(rs, "chicken-dinner", time.Minute)
		require.NoError(t, err)
		assert.False(t, bobAcquired)
		mockrequire.CalledN(t, rs.SetNxFunc, 2)
		mockrequire.Called(t, rs.GetFunc)
		mockrequire.Called(t, rs.GetSetFunc)
	})
}
