package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/sourcegraph/sourcegraph/internal/rcache"
)

type GitHubAuthCache struct {
	cache *rcache.Cache
}

var githubAuthCache = &GitHubAuthCache{
	cache: rcache.NewWithTTL("codeintel.github-authz:", 60 /* seconds */, false),
}

func (c *GitHubAuthCache) Get(key string) (authorized bool, _ bool) {
	b, ok := c.cache.Get(key)
	if !ok {
		return false, false
	}

	err := json.Unmarshal(b, &authorized)
	return authorized, err == nil
}

func (c *GitHubAuthCache) Set(key string, authorized bool) {
	b, _ := json.Marshal(authorized)
	c.cache.Set(key, b)
}

func makeGitHubAuthCacheKey(githubToken, repoName string) string {
	key := sha256.Sum256([]byte(githubToken + ":" + repoName))
	return hex.EncodeToString(key[:])
}
