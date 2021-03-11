package search

import (
	"fmt"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	sgapi "github.com/sourcegraph/sourcegraph/internal/api"
	searchshared "github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming/api"
)

type progressAggregator struct {
	Start      time.Time
	MatchCount int
	Stats      streaming.Stats
	Limit      int

	// Dirty is true if p has changed since the last call to Current.
	Dirty bool
}

func (p *progressAggregator) Update(event graphqlbackend.SearchEvent) {
	if len(event.Results) == 0 && event.Stats.Zero() {
		return
	}

	p.Dirty = true
	p.Stats.Update(&event.Stats)
	for _, result := range event.Results {
		// We use a different result count in streaming than graphql. We don't
		// want to break existing graphql clients like saved searches.
		if crs, ok := result.ToCommitSearchResult(); ok {
			p.MatchCount += crs.CommitMatch.ResultCount()
			continue
		}
		p.MatchCount += int(result.ResultCount())
	}

	if p.MatchCount > p.Limit {
		p.MatchCount = p.Limit
		p.Stats.IsLimitHit = true
	}
}

func (p *progressAggregator) currentStats() api.ProgressStats {
	// Suggest the next 1000 after rounding off.
	suggestedLimit := (p.Limit + 1500) / 1000 * 1000

	return api.ProgressStats{
		MatchCount:          p.MatchCount,
		ElapsedMilliseconds: int(time.Since(p.Start).Milliseconds()),
		ExcludedArchived:    p.Stats.ExcludedArchived,
		ExcludedForks:       p.Stats.ExcludedForks,
		Timedout:            getNames(p.Stats, searchshared.RepoStatusTimedout),
		Missing:             getNames(p.Stats, searchshared.RepoStatusMissing),
		Cloning:             getNames(p.Stats, searchshared.RepoStatusCloning),
		LimitHit:            p.Stats.IsLimitHit,
		SuggestedLimit:      suggestedLimit,
	}
}

// Current returns the current progress event.
func (p *progressAggregator) Current() api.Progress {
	p.Dirty = false

	return api.BuildProgressEvent(p.currentStats())
}

// Final returns the current progress event, but with final fields set to
// indicate it is the last progress event.
func (p *progressAggregator) Final() api.Progress {
	p.Dirty = false

	s := p.currentStats()

	// We only send RepositoriesCount at the end because the number is
	// confusing to users to see while searching.
	s.RepositoriesCount = intPtr(len(p.Stats.Repos))

	event := api.BuildProgressEvent(s)
	event.Done = true
	return event
}

type namerFunc string

func (n namerFunc) Name() string {
	return string(n)
}

func getNames(stats streaming.Stats, status searchshared.RepoStatus) []api.Namer {
	var names []api.Namer
	stats.Status.Filter(status, func(id sgapi.RepoID) {
		if name, ok := stats.Repos[id]; ok {
			names = append(names, namerFunc(name.Name))
		} else {
			names = append(names, namerFunc(fmt.Sprintf("UNKNOWN{ID=%d}", id)))
		}
	})
	return names
}

func intPtr(i int) *int {
	return &i
}
