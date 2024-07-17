package gitcli

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
)

var (
	globalCache   *caches
	globalCacheMu sync.Mutex
)

func makeGlobalCache() *caches {
	globalCacheMu.Lock()
	if globalCache == nil {
		var c caches
		c.revAtTimeCache, _ = lru.New[revAtTimeCacheKey, gitdomain.OID](15_000)
		globalCache = &c
	}
	globalCacheMu.Unlock()
	return globalCache
}

type caches struct {
	// A simple in-process cache to mitigate the cost of any regular
	// rev:at.time() queries.
	//
	// We assume a repo name is ~34 bytes long. The OID takes 20 bytes, and the time.Time
	// takes 16 bytes. That is ~70 bytes per record.
	// 15_000 entries should keep this cache around 1MB.
	revAtTimeCache *lru.Cache[revAtTimeCacheKey, gitdomain.OID]
}

func cacheStats() map[string]any {
	var c caches
	return map[string]any{
		"globalRevAtTimeCache": map[string]any{
			"cached":   c.revAtTimeCache.Len(),
			"capacity": c.revAtTimeCache.Cap(),
		},
	}
}

type revAtTimeCacheKey struct {
	repoName api.RepoName
	sha      gitdomain.OID
	t        time.Time
}
