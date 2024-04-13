package graphqlbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/log"

	searchlogs "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search/logs"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/honey"
	searchhoney "github.com/sourcegraph/sourcegraph/internal/honey/search"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/search"
	searchclient "github.com/sourcegraph/sourcegraph/internal/search/client"
	"github.com/sourcegraph/sourcegraph/internal/search/job/jobutil"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// SearchResultsResolver is a resolver for the GraphQL type `SearchResults`
type SearchResultsResolver struct {
	db          database.DB
	Matches     result.Matches
	Stats       streaming.Stats
	SearchAlert *search.Alert

	// The time it took to compute all results.
	elapsed time.Duration
}

func (c *SearchResultsResolver) LimitHit() bool {
	return c.Stats.IsLimitHit
}

func (c *SearchResultsResolver) matchesRepoIDs() map[api.RepoID]struct{} {
	m := map[api.RepoID]struct{}{}
	for _, id := range c.Matches {
		m[id.RepoName().ID] = struct{}{}
	}
	return m
}

func (c *SearchResultsResolver) Repositories(ctx context.Context) ([]*RepositoryResolver, error) {
	// c.Stats.Repos does not necessarily respect limits that are applied in
	// our graphql layers. Instead we generate the list from the matches.
	m := c.matchesRepoIDs()
	ids := make([]api.RepoID, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return c.repositoryResolvers(ctx, ids)
}

func (c *SearchResultsResolver) RepositoriesCount() int32 {
	return int32(len(c.matchesRepoIDs()))
}

func (c *SearchResultsResolver) repositoryResolvers(ctx context.Context, ids []api.RepoID) ([]*RepositoryResolver, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	gsClient := gitserver.NewClient("graphql.search.results.repositories")
	resolvers := make([]*RepositoryResolver, 0, len(ids))
	err := c.db.Repos().StreamMinimalRepos(ctx, database.ReposListOptions{
		IDs: ids,
	}, func(repo *types.MinimalRepo) {
		resolvers = append(resolvers, NewMinimalRepositoryResolver(c.db, gsClient, repo.ID, repo.Name))
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(resolvers, func(a, b int) bool {
		return resolvers[a].ID() < resolvers[b].ID()
	})
	return resolvers, nil
}

func (c *SearchResultsResolver) repoIDsByStatus(mask search.RepoStatus) []api.RepoID {
	var ids []api.RepoID
	c.Stats.Status.Filter(mask, func(id api.RepoID) {
		ids = append(ids, id)
	})
	return ids
}

func (c *SearchResultsResolver) Cloning(ctx context.Context) ([]*RepositoryResolver, error) {
	return c.repositoryResolvers(ctx, c.repoIDsByStatus(search.RepoStatusCloning))
}

func (c *SearchResultsResolver) Missing(ctx context.Context) ([]*RepositoryResolver, error) {
	return c.repositoryResolvers(ctx, c.repoIDsByStatus(search.RepoStatusMissing))
}

func (c *SearchResultsResolver) Timedout(ctx context.Context) ([]*RepositoryResolver, error) {
	return c.repositoryResolvers(ctx, c.repoIDsByStatus(search.RepoStatusTimedOut))
}

func (c *SearchResultsResolver) IndexUnavailable() bool {
	// This used to return c.Stats.IsIndexUnavailable, but it was never set,
	// so would always return false
	return false
}

// Results are the results found by the search. It respects the limits set. To
// access all results directly access the SearchResults field.
func (sr *SearchResultsResolver) Results() []SearchResultResolver {
	return matchesToResolvers(sr.db, sr.Matches)
}

func matchesToResolvers(db database.DB, matches []result.Match) []SearchResultResolver {
	repoResolvers := make(map[types.MinimalRepo]*RepositoryResolver, 10)
	gsClient := gitserver.NewClient("graphql.search.results")
	getRepoResolver := func(repoName types.MinimalRepo) *RepositoryResolver {
		if existing, ok := repoResolvers[repoName]; ok {
			return existing
		}
		resolver := NewMinimalRepositoryResolver(db, gsClient, repoName.ID, repoName.Name)
		repoResolvers[repoName] = resolver
		return resolver
	}

	resolvers := make([]SearchResultResolver, 0, len(matches))
	for _, match := range matches {
		switch v := match.(type) {
		case *result.FileMatch:
			resolvers = append(resolvers, &FileMatchResolver{
				db:           db,
				FileMatch:    *v,
				RepoResolver: getRepoResolver(v.Repo),
			})
		case *result.RepoMatch:
			resolvers = append(resolvers, getRepoResolver(v.RepoName()))
		case *result.CommitMatch:
			resolvers = append(resolvers, &CommitSearchResultResolver{
				db:          db,
				CommitMatch: *v,
			})
		case *result.OwnerMatch:
			// todo(own): add OwnerSearchResultResolver
		}
	}
	return resolvers
}

func (sr *SearchResultsResolver) MatchCount() int32 {
	return int32(sr.Matches.ResultCount())
}

// Deprecated. Prefer MatchCount.
func (sr *SearchResultsResolver) ResultCount() int32 { return sr.MatchCount() }

func (sr *SearchResultsResolver) ApproximateResultCount() string {
	count := sr.MatchCount()
	if sr.LimitHit() || sr.Stats.Status.Any(search.RepoStatusCloning|search.RepoStatusTimedOut) {
		return fmt.Sprintf("%d+", count)
	}
	return strconv.Itoa(int(count))
}

func (sr *SearchResultsResolver) Alert() *searchAlertResolver {
	return NewSearchAlertResolver(sr.SearchAlert)
}

func (sr *SearchResultsResolver) ElapsedMilliseconds() int32 {
	return int32(sr.elapsed.Milliseconds())
}

func (sr *SearchResultsResolver) DynamicFilters(ctx context.Context) []*searchFilterResolver {
	tr, _ := trace.New(ctx, "DynamicFilters", attribute.String("resolver", "SearchResultsResolver"))
	defer tr.End()

	var filters streaming.SearchFilters
	filters.Update(streaming.SearchEvent{
		Results: sr.Matches,
		Stats:   sr.Stats,
	})

	var resolvers []*searchFilterResolver
	for _, f := range filters.Compute() {
		resolvers = append(resolvers, &searchFilterResolver{filter: *f})
	}
	return resolvers
}

type searchFilterResolver struct {
	filter streaming.Filter
}

func (sf *searchFilterResolver) Value() string {
	return sf.filter.Value
}

func (sf *searchFilterResolver) Label() string {
	return sf.filter.Label
}

func (sf *searchFilterResolver) Count() int32 {
	return int32(sf.filter.Count)
}

func (sf *searchFilterResolver) LimitHit() bool {
	// TODO(camdencheek): calculate exhaustiveness correctly
	return true
}

func (sf *searchFilterResolver) Kind() string {
	return string(sf.filter.Kind)
}

// blameFileMatch blames the specified file match to produce the time at which
// the first line match inside of it was authored.
func (sr *SearchResultsResolver) blameFileMatch(ctx context.Context, fm *result.FileMatch) (t time.Time, err error) {
	tr, ctx := trace.New(ctx, "SearchResultsResolver.blameFileMatch")
	defer tr.EndWithErr(&err)

	// Blame the first line match.
	if len(fm.ChunkMatches) == 0 {
		// No line match
		return time.Time{}, nil
	}

	hm := fm.ChunkMatches[0]
	hr, err := gitserver.NewClient("graphql.search.results.blame").StreamBlameFile(ctx, fm.Repo.ID, fm.Path, &gitserver.BlameOptions{
		NewestCommit: fm.CommitID,
		Range: &gitserver.BlameRange{
			StartLine: hm.Ranges[0].Start.Line,
			EndLine:   hm.Ranges[0].Start.Line,
		},
	})
	if err != nil {
		return time.Time{}, err
	}
	defer hr.Close()

	// We are only interested in the first hunk, so we consume one and then return
	// which calls hr.Close above.
	hunk, err := hr.Read()
	if err != nil {
		return time.Time{}, err
	}

	return hunk.Author.Date, nil
}

func (sr *SearchResultsResolver) Sparkline(ctx context.Context) (sparkline []int32, err error) {
	var (
		days     = 30  // number of days the sparkline represents
		maxBlame = 100 // maximum number of file results to blame for date/time information.
		p        = pool.New().WithMaxGoroutines(8)
	)

	var (
		sparklineMu sync.Mutex
		blameOps    = 0
	)
	sparkline = make([]int32, days)
	addPoint := func(t time.Time) {
		// Check if the author date of the search result is inside of our sparkline
		// timerange.
		now := time.Now()
		if t.Before(now.Add(-time.Duration(len(sparkline)) * 24 * time.Hour)) {
			// Outside the range of the sparkline.
			return
		}
		sparklineMu.Lock()
		defer sparklineMu.Unlock()
		for n := range sparkline {
			d1 := now.Add(-time.Duration(n) * 24 * time.Hour)
			d2 := now.Add(-time.Duration(n-1) * 24 * time.Hour)
			if t.After(d1) && t.Before(d2) {
				sparkline[n]++ // on the nth day
			}
		}
	}

	// Consider all of our search results as a potential data point in our
	// sparkline.
loop:
	for _, r := range sr.Matches {
		r := r // shadow so it doesn't change in the goroutine
		switch m := r.(type) {
		case *result.RepoMatch, *result.OwnerMatch:
			// We don't care about repo or owner results here.
			continue
		case *result.CommitMatch:
			// Diff searches are cheap, because we implicitly have author date info.
			addPoint(m.Commit.Author.Date)
		case *result.FileMatch:
			// File match searches are more expensive, because we must blame the
			// (first) line in order to know its placement in our sparkline.
			blameOps++
			if blameOps > maxBlame {
				// We have exceeded our budget of blame operations for
				// calculating this sparkline, so don't do any more file match
				// blaming.
				continue loop
			}

			p.Go(func() {
				// Blame the file match in order to retrieve date informatino.
				t, err := sr.blameFileMatch(ctx, m)
				if err != nil {
					log15.Warn("failed to blame fileMatch during sparkline generation", "error", err)
					return
				}
				addPoint(t)
			})
		default:
			panic("SearchResults.Sparkline unexpected union type state")
		}
	}
	p.Wait()
	return sparkline, nil
}

var (
	searchResponseCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_graphql_search_response",
		Help: "Number of searches that have ended in the given status (success, error, timeout, partial_timeout).",
	}, []string{"status", "alert_type", "source", "request_name"})

	searchLatencyHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_search_response_latency_seconds",
		Help:    "Search response latencies in seconds that have ended in the given status (success, error, timeout, partial_timeout).",
		Buckets: []float64{0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 15, 20, 30},
	}, []string{"status", "alert_type", "source", "request_name"})
)

func logPrometheusBatch(status, alertType, requestSource, requestName string, elapsed time.Duration) {
	searchResponseCounter.WithLabelValues(
		status,
		alertType,
		requestSource,
		requestName,
	).Inc()

	searchLatencyHistogram.WithLabelValues(
		status,
		alertType,
		requestSource,
		requestName,
	).Observe(elapsed.Seconds())
}

func logBatch(ctx context.Context, searchInputs *search.Inputs, srr *SearchResultsResolver, err error) {
	var status, alertType string
	status = searchclient.DetermineStatusForLogs(srr.SearchAlert, srr.Stats, err)
	if srr.SearchAlert != nil {
		alertType = srr.SearchAlert.PrometheusType
	}
	requestSource := string(trace.RequestSource(ctx))
	requestName := trace.GraphQLRequestName(ctx)
	logPrometheusBatch(status, alertType, requestSource, requestName, srr.elapsed)

	isSlow := srr.elapsed > searchlogs.LogSlowSearchesThreshold()
	if honey.Enabled() || isSlow {
		var n int
		if srr != nil {
			n = len(srr.Matches)
		}
		ev := searchhoney.SearchEvent(ctx, searchhoney.SearchEventArgs{
			OriginalQuery: searchInputs.OriginalQuery,
			Typ:           requestName,
			Source:        requestSource,
			Status:        status,
			AlertType:     alertType,
			DurationMs:    srr.elapsed.Milliseconds(),
			LatencyMs:     nil, // no latency for batch requests
			ResultSize:    n,
			Error:         err,
		})

		_ = ev.Send()

		if isSlow {
			log15.Warn("slow search request", "query", searchInputs.OriginalQuery, "type", requestName, "source", requestSource, "status", status, "alertType", alertType, "durationMs", srr.elapsed.Milliseconds(), "resultSize", n, "error", err)
		}
	}
}

func (r *searchResolver) Results(ctx context.Context) (*SearchResultsResolver, error) {
	start := time.Now()
	agg := streaming.NewAggregatingStream()
	alert, err := r.client.Execute(ctx, agg, r.SearchInputs)
	srr := r.resultsToResolver(agg.Results, alert, agg.Stats)
	srr.elapsed = time.Since(start)
	logBatch(ctx, r.SearchInputs, srr, err)
	return srr, err
}

func (r *searchResolver) resultsToResolver(matches result.Matches, alert *search.Alert, stats streaming.Stats) *SearchResultsResolver {
	return &SearchResultsResolver{
		Matches:     matches,
		SearchAlert: alert,
		Stats:       stats,
		db:          r.db,
	}
}

type searchResultsStats struct {
	logger                  log.Logger
	JApproximateResultCount string
	JSparkline              []int32

	sr *searchResolver

	// These items are lazily populated by getResults
	once    sync.Once
	results result.Matches
	err     error
}

func (srs *searchResultsStats) ApproximateResultCount() string { return srs.JApproximateResultCount }
func (srs *searchResultsStats) Sparkline() []int32             { return srs.JSparkline }

var (
	searchResultsStatsCache   = rcache.NewWithTTL("search_results_stats", 3600) // 1h
	searchResultsStatsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_graphql_search_results_stats_cache_hit",
		Help: "Counts cache hits and misses for search results stats (e.g. sparklines).",
	}, []string{"type"})
)

func (r *searchResolver) Stats(ctx context.Context) (stats *searchResultsStats, err error) {
	cacheKey := r.SearchInputs.OriginalQuery
	// Check if value is in the cache.
	jsonRes, ok := searchResultsStatsCache.Get(cacheKey)
	if ok {
		searchResultsStatsCounter.WithLabelValues("hit").Inc()
		if err := json.Unmarshal(jsonRes, &stats); err != nil {
			return nil, err
		}
		stats.logger = r.logger.Scoped("searchResultsStats")
		stats.sr = r
		return stats, nil
	}

	// Calculate value from scratch.
	searchResultsStatsCounter.WithLabelValues("miss").Inc()
	attempts := 0
	var v *SearchResultsResolver

	for {
		// Query search results.
		b, err := query.ToBasicQuery(r.SearchInputs.Query)
		if err != nil {
			return nil, err
		}
		j, err := jobutil.NewBasicJob(r.SearchInputs, b)
		if err != nil {
			return nil, err
		}
		j = jobutil.NewLogJob(r.SearchInputs, j)
		agg := streaming.NewAggregatingStream()
		alert, err := j.Run(ctx, r.client.JobClients(), agg)
		if err != nil {
			return nil, err // do not cache errors.
		}
		v = r.resultsToResolver(agg.Results, alert, agg.Stats)
		if v.MatchCount() > 0 {
			break
		}

		status := v.Stats.Status
		if !status.Any(search.RepoStatusCloning) && !status.Any(search.RepoStatusTimedOut) {
			break // zero results, but no cloning or timed out repos. No point in retrying.
		}

		var cloning, timedout int
		status.Filter(search.RepoStatusCloning, func(api.RepoID) {
			cloning++
		})
		status.Filter(search.RepoStatusTimedOut, func(api.RepoID) {
			timedout++
		})

		if attempts > 5 {
			log15.Error("failed to generate sparkline due to cloning or timed out repos", "cloning", cloning, "timedout", timedout)
			return nil, errors.Errorf("failed to generate sparkline due to %d cloning %d timedout repos", cloning, timedout)
		}

		// We didn't find any search results. Some repos are cloning or timed
		// out, so try again in a few seconds.
		attempts++
		log15.Warn("sparkline generation found 0 search results due to cloning or timed out repos (retrying in 5s)", "cloning", cloning, "timedout", timedout)
		time.Sleep(5 * time.Second)
	}

	sparkline, err := v.Sparkline(ctx)
	if err != nil {
		return nil, err // sparkline generation failed, so don't cache.
	}
	stats = &searchResultsStats{
		logger:                  r.logger.Scoped("searchResultsStats"),
		JApproximateResultCount: v.ApproximateResultCount(),
		JSparkline:              sparkline,
		sr:                      r,
	}

	// Store in the cache if we got non-zero results. If we got zero results,
	// it should be quick and caching is not desired because e.g. it could be
	// a query for a repo that has not been added by the user yet.
	if v.ResultCount() > 0 {
		jsonRes, err = json.Marshal(stats)
		if err != nil {
			return nil, err
		}
		searchResultsStatsCache.Set(cacheKey, jsonRes)
	}
	return stats, nil
}

// SearchResultResolver is a resolver for the GraphQL union type `SearchResult`.
//
// Supported types:
//
//   - *RepositoryResolver         // repo name match
//   - *fileMatchResolver          // text match
//   - *commitSearchResultResolver // diff or commit match
//
// Note: Any new result types added here also need to be handled properly in search_results.go:301 (sparklines)
type SearchResultResolver interface {
	ToRepository() (*RepositoryResolver, bool)
	ToFileMatch() (*FileMatchResolver, bool)
	ToCommitSearchResult() (*CommitSearchResultResolver, bool)
}
