package limits

import (
	"math"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
)

const (
	DefaultMaxSearchResults          = 30
	DefaultMaxSearchResultsStreaming = 10000

	// The default timeout to use for queries.
	DefaultTimeout = 20 * time.Second

	// We still set limits for exhaustive to prevent runaway jobs.
	DefaultMaxSearchResultsExhaustive = 1_000_000
	DefaultTimeoutExhaustive          = time.Hour
)

func SearchLimits(c *conf.Unified) schema.SearchLimits {
	// Our configuration reader does not set defaults from schema. So we rely
	// on Go default values to mean defaults.
	withDefault := func(x *int, def int) {
		if *x <= 0 {
			*x = def
		}
	}

	var limits schema.SearchLimits
	if c.SearchLimits != nil {
		limits = *c.SearchLimits
	}

	// If MaxRepos unset use deprecated value
	if limits.MaxRepos == 0 {
		limits.MaxRepos = c.MaxReposToSearch
	}

	withDefault(&limits.MaxRepos, math.MaxInt32>>1)
	withDefault(&limits.CommitDiffMaxRepos, 50)
	withDefault(&limits.CommitDiffWithTimeFilterMaxRepos, 10000)
	withDefault(&limits.DisplayLimit, 1500)
	withDefault(&limits.MaxTimeoutSeconds, 60)

	return limits
}
