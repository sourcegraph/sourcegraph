package github

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gregjones/httpcache"

	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
)

type cachedGroup struct {
	// Org login, required
	Org string
	// Team slug, if empty implies group is an org
	Team string

	Repositories []extsvc.RepoID
}

func (g cachedGroup) key() string {
	key := g.Org
	if g.Team != "" {
		key += "/" + g.Team
	}
	return key
}

type groupsCache struct {
	cache httpcache.Cache
}

func newGroupPermsCache(urn string, codeHost *extsvc.CodeHost, ttl time.Duration) *groupsCache {
	return &groupsCache{
		cache: rcache.NewWithTTL(fmt.Sprintf("gh_groups_perms:%s:%s", codeHost.ServiceID, urn), int(ttl/time.Second)),
	}
}

// setGroup stores the given group in the cache
func (c *groupsCache) setGroup(group cachedGroup) error {
	bytes, err := json.Marshal(&group)
	if err != nil {
		return err
	}
	c.cache.Set(group.key(), bytes)
	return nil
}

// getGroup attempts to retrive the given org, team group from cache.
//
// It always returns a valid cachedGroup even if it fails to retrieve a group from cache.
func (c *groupsCache) getGroup(org string, team string) (cachedGroup, bool) {
	rawGroup := cachedGroup{Org: org, Team: team}
	bytes, ok := c.cache.Get(rawGroup.key())
	if !ok {
		return rawGroup, ok
	}
	var cached cachedGroup
	if err := json.Unmarshal(bytes, &cached); err != nil {
		return rawGroup, false
	}
	return cached, ok
}

// deleteGroup deletes the given group from the cache.
func (c *groupsCache) deleteGroup(group cachedGroup) {
	c.cache.Delete(group.key())
}
