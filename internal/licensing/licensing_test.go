package licensing

import (
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/redispool"
)

func cleanupStore(t *testing.T, store redispool.KeyValue) {
	t.Cleanup(func() {
		store.Del(licenseValidityStoreKey)
		store.Del(licenseInvalidReason)
	})
}

func TestIsLicenseValid(t *testing.T) {
	store = redispool.NewKeyValue("127.0.0.1:6379", &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 5 * time.Second,
	})
	store.Del(licenseValidityStoreKey)

	t.Run("unset key returns true", func(t *testing.T) {
		cleanupStore(t, store)
		require.True(t, IsLicenseValid())
	})

	t.Run("set false key returns false", func(t *testing.T) {
		cleanupStore(t, store)
		require.NoError(t, store.Set(licenseValidityStoreKey, false))
		require.False(t, IsLicenseValid())
	})

	t.Run("set true key returns true", func(t *testing.T) {
		cleanupStore(t, store)
		require.NoError(t, store.Set(licenseValidityStoreKey, true))
		require.True(t, IsLicenseValid())
	})
}

func TestGetLicenseInvalidReason(t *testing.T) {
	store = redispool.NewKeyValue("127.0.0.1:6379", &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 5 * time.Second,
	})
	store.Del(licenseValidityStoreKey)
	store.Del(licenseInvalidReason)

	t.Run("unset licenseValidityStoreKey returns empty string", func(t *testing.T) {
		cleanupStore(t, store)
		require.Empty(t, GetLicenseInvalidReason())
	})

	t.Run("true licenseValidityStoreKey returns empty string", func(t *testing.T) {
		cleanupStore(t, store)
		require.NoError(t, store.Set(licenseValidityStoreKey, true))
		require.Empty(t, GetLicenseInvalidReason())
	})

	t.Run("unset reason returns `unknown`", func(t *testing.T) {
		cleanupStore(t, store)
		require.NoError(t, store.Set(licenseValidityStoreKey, false))
		require.Equal(t, "unknown", GetLicenseInvalidReason())
	})

	t.Run("set reason returns the reason", func(t *testing.T) {
		cleanupStore(t, store)

		reason := "test reason"
		require.NoError(t, store.Set(licenseValidityStoreKey, false))
		require.NoError(t, store.Set(licenseInvalidReason, reason))
		require.Equal(t, reason, GetLicenseInvalidReason())
	})
}
