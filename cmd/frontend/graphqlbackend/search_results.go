package graphqlbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-enry/go-enry/v2"
	"github.com/inconshreveable/log15"
	"github.com/neelance/parallel"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	searchrepos "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search/repos"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search/searchcontexts"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/honey"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/usagestats"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
	"github.com/sourcegraph/sourcegraph/schema"
)

func (c *SearchResultsResolver) LimitHit() bool {
	return c.IsLimitHit || (c.limit > 0 && len(c.SearchResults) > c.limit)
}

func (c *SearchResultsResolver) Repositories() []*RepositoryResolver {
	repos := c.Repos
	resolvers := make([]*RepositoryResolver, 0, len(repos))
	for _, r := range repos {
		resolvers = append(resolvers, NewRepositoryResolver(c.db, r.ToRepo()))
	}
	sort.Slice(resolvers, func(a, b int) bool {
		return resolvers[a].ID() < resolvers[b].ID()
	})
	return resolvers
}

func (c *SearchResultsResolver) RepositoriesCount() int32 {
	return int32(len(c.Repos))
}

func (c *SearchResultsResolver) repositoryResolvers(mask search.RepoStatus) []*RepositoryResolver {
	var resolvers []*RepositoryResolver
	c.Status.Filter(mask, func(id api.RepoID) {
		r := c.Repos[id]
		if r != nil {
			resolvers = append(resolvers, NewRepositoryResolver(c.db, c.Repos[id].ToRepo()))
		}
	})
	sort.Slice(resolvers, func(a, b int) bool {
		return resolvers[a].ID() < resolvers[b].ID()
	})
	return resolvers
}

func (c *SearchResultsResolver) Cloning() []*RepositoryResolver {
	return c.repositoryResolvers(search.RepoStatusCloning)
}

func (c *SearchResultsResolver) Missing() []*RepositoryResolver {
	return c.repositoryResolvers(search.RepoStatusMissing)
}

func (c *SearchResultsResolver) Timedout() []*RepositoryResolver {
	return c.repositoryResolvers(search.RepoStatusTimedout)
}

func (c *SearchResultsResolver) IndexUnavailable() bool {
	return c.IsIndexUnavailable
}

func (c *SearchResultsResolver) allReposTimedout() bool {
	return c.Status.All(search.RepoStatusTimedout) && c.Status.Len() == len(c.Repos)
}

// SearchResultsResolver is a resolver for the GraphQL type `SearchResults`
type SearchResultsResolver struct {
	db dbutil.DB
	// SearchResults is the full list of results found. The method Results()
	// will return the list respecting limits.
	SearchResults []SearchResultResolver
	streaming.Stats

	// limit is the maximum number of SearchResults to send back to the user.
	limit int

	alert *searchAlert
	start time.Time // when the results started being computed

	// cursor to return for paginated search requests, or nil if the request
	// wasn't paginated.
	cursor *searchCursor

	// cache for user settings. Ideally this should be set just once in the code path
	// by an upstream resolver
	UserSettings *schema.Settings
}

// Results are the results found by the search. It respects the limits set. To
// access all results directly access the SearchResults field.
func (sr *SearchResultsResolver) Results() []SearchResultResolver {
	if sr.limit > 0 && sr.limit < len(sr.SearchResults) {
		return sr.SearchResults[:sr.limit]
	}

	return sr.SearchResults
}

func (sr *SearchResultsResolver) MatchCount() int32 {
	var totalResults int32
	for _, result := range sr.SearchResults {
		totalResults += result.ResultCount()
	}
	return totalResults
}

// Deprecated. Prefer MatchCount.
func (sr *SearchResultsResolver) ResultCount() int32 { return sr.MatchCount() }

func (sr *SearchResultsResolver) ApproximateResultCount() string {
	count := sr.MatchCount()
	if sr.LimitHit() || sr.Status.Any(search.RepoStatusCloning|search.RepoStatusTimedout) {
		return fmt.Sprintf("%d+", count)
	}
	return strconv.Itoa(int(count))
}

func (sr *SearchResultsResolver) Alert() *searchAlert { return sr.alert }

func (sr *SearchResultsResolver) ElapsedMilliseconds() int32 {
	return int32(time.Since(sr.start).Milliseconds())
}

func (sr *SearchResultsResolver) DynamicFilters(ctx context.Context) []*searchFilterResolver {
	tr, ctx := trace.New(ctx, "DynamicFilters", "", trace.Tag{Key: "resolver", Value: "SearchResultsResolver"})
	defer func() {
		tr.Finish()
	}()

	globbing := false
	// For search, sr.userSettings is set in (r *searchResolver) Results(ctx
	// context.Context). However we might regress on that or call DynamicFilters from
	// other code paths. Hence we fallback to accessing the user settings directly.
	if sr.UserSettings != nil {
		globbing = getBoolPtr(sr.UserSettings.SearchGlobbing, false)
	} else {
		settings, err := decodedViewerFinalSettings(ctx, sr.db)
		if err != nil {
			log15.Warn("DynamicFilters: could not get user settings from database")
		} else {
			globbing = getBoolPtr(settings.SearchGlobbing, false)
		}
	}
	tr.LogFields(otlog.Bool("globbing", globbing))

	filters := SearchFilters{
		Globbing: globbing,
	}
	filters.Update(SearchEvent{
		Results: sr.SearchResults,
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
	return sf.filter.IsLimitHit
}

func (sf *searchFilterResolver) Kind() string {
	return sf.filter.Kind
}

// blameFileMatch blames the specified file match to produce the time at which
// the first line match inside of it was authored.
func (sr *SearchResultsResolver) blameFileMatch(ctx context.Context, fm *FileMatchResolver) (t time.Time, err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "blameFileMatch")
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
		}
		span.Finish()
	}()

	// Blame the first line match.
	lineMatches := fm.LineMatches()
	if len(lineMatches) == 0 {
		// No line match
		return time.Time{}, nil
	}
	lm := fm.LineMatches()[0]
	hunks, err := git.BlameFile(ctx, fm.Repo.Name, fm.path(), &git.BlameOptions{
		NewestCommit: fm.CommitID,
		StartLine:    int(lm.LineNumber()),
		EndLine:      int(lm.LineNumber()),
	})
	if err != nil {
		return time.Time{}, err
	}

	return hunks[0].Author.Date, nil
}

func (sr *SearchResultsResolver) Sparkline(ctx context.Context) (sparkline []int32, err error) {
	var (
		days     = 30                 // number of days the sparkline represents
		maxBlame = 100                // maximum number of file results to blame for date/time information.
		run      = parallel.NewRun(8) // number of concurrent blame ops
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
	for _, r := range sr.SearchResults {
		r := r // shadow so it doesn't change in the goroutine
		switch m := r.(type) {
		case *RepositoryResolver:
			// We don't care about repo results here.
			continue
		case *CommitSearchResultResolver:
			// Diff searches are cheap, because we implicitly have author date info.
			addPoint(m.Commit().commit.Author.Date)
		case *FileMatchResolver:
			// File match searches are more expensive, because we must blame the
			// (first) line in order to know its placement in our sparkline.
			blameOps++
			if blameOps > maxBlame {
				// We have exceeded our budget of blame operations for
				// calculating this sparkline, so don't do any more file match
				// blaming.
				continue loop
			}

			run.Acquire()
			goroutine.Go(func() {
				defer run.Release()

				// Blame the file match in order to retrieve date informatino.
				var err error
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
	span := opentracing.SpanFromContext(ctx)
	span.SetTag("blame_ops", blameOps)
	return sparkline, nil
}

var searchResponseCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "src_graphql_search_response",
	Help: "Number of searches that have ended in the given status (success, error, timeout, partial_timeout).",
}, []string{"status", "alert_type", "source", "request_name"})

// logSearchLatency records search durations in the event database. This
// function may only be called after a search result is performed, because it
// relies on the invariant that query and pattern error checking has already
// been performed.
func (r *searchResolver) logSearchLatency(ctx context.Context, durationMs int32) {
	tr, ctx := trace.New(ctx, "logSearchLatency", "")
	defer func() {
		tr.Finish()
	}()
	var types []string
	resultTypes, _ := r.Query.StringValues(query.FieldType)
	for _, typ := range resultTypes {
		switch typ {
		case "repo", "symbol", "diff", "commit":
			types = append(types, typ)
		case "path":
			// Map type:path to file
			types = append(types, "file")
		case "file":
			switch {
			case r.PatternType == query.SearchTypeStructural:
				types = append(types, "structural")
			case r.PatternType == query.SearchTypeLiteral:
				types = append(types, "literal")
			case r.PatternType == query.SearchTypeRegex:
				types = append(types, "regexp")
			}
		}
	}

	// Don't record composite searches that specify more than one type:
	// because we can't break down the search timings into multiple
	// categories.
	if len(types) > 1 {
		return
	}

	options := &getPatternInfoOptions{}
	if r.PatternType == query.SearchTypeStructural {
		options = &getPatternInfoOptions{performStructuralSearch: true}
	}
	if r.PatternType == query.SearchTypeLiteral {
		options = &getPatternInfoOptions{performLiteralSearch: true}
	}
	p, _ := r.getPatternInfo(options)

	// If no type: was explicitly specified, infer the result type.
	if len(types) == 0 {
		// If a pattern was specified, a content search happened.
		if p.Pattern != "" {
			switch {
			case r.PatternType == query.SearchTypeStructural:
				types = append(types, "structural")
			case r.PatternType == query.SearchTypeLiteral:
				types = append(types, "literal")
			case r.PatternType == query.SearchTypeRegex:
				types = append(types, "regexp")
			}
		} else if len(r.Query.Fields()["file"]) > 0 {
			// No search pattern specified and file: is specified.
			types = append(types, "file")
		} else {
			// No search pattern or file: is specified, assume repo.
			// This includes accounting for searches of fields that
			// specify repohasfile: and repohascommitafter:.
			types = append(types, "repo")
		}
	}

	// Only log the time if we successfully resolved one search type.
	if len(types) == 1 {
		actor := actor.FromContext(ctx)
		if actor.IsAuthenticated() {
			value := fmt.Sprintf(`{"durationMs": %d}`, durationMs)
			eventName := fmt.Sprintf("search.latencies.%s", types[0])
			go func() {
				err := usagestats.LogBackendEvent(r.db, actor.UID, eventName, json.RawMessage(value))
				if err != nil {
					log15.Warn("Could not log search latency", "err", err)
				}
			}()
		}
	}
}

// evaluateLeaf performs a single search operation and corresponds to the
// evaluation of leaf expression in a query.
func (r *searchResolver) evaluateLeaf(ctx context.Context) (_ *SearchResultsResolver, err error) {
	tr, ctx := trace.New(ctx, "evaluateLeaf", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	start := time.Now()

	// If the request specifies stable:truthy, use pagination to return a stable ordering.
	if r.Query.BoolValue(query.FieldStable) {
		var stableResultCount int32 = defaultMaxSearchResults
		if count := r.Query.Count(); count != nil {
			stableResultCount = int32(*count)
			if stableResultCount > maxSearchResultsPerPaginatedRequest {
				return alertForQuery(r.db, r.rawQuery(), fmt.Errorf("Stable searches are limited to at max count:%d results. Consider removing 'stable:', narrowing the search with 'repo:', or using the paginated search API.", maxSearchResultsPerPaginatedRequest)).wrap(), nil
			}
		}

		r.Pagination = &searchPaginationInfo{
			limit: stableResultCount,
		}

		// Pagination only works for file content searches, and will
		// raise an error otherwise. If stable is explicitly set, this
		// is implied. So, force this query to only return file content
		// results.
		r.Query = query.OverrideField(r.Query, query.FieldType, "file")
		result, err := r.paginatedResults(ctx)
		if err != nil {
			return nil, err
		}
		if result == nil {
			// Panic if paginatedResults does not ensure a non-nil search result.
			panic("stable search: paginated search returned nil results")
		}
		if result.cursor == nil {
			// Perhaps an alert was raised.
			return result, err
		}
		if !result.cursor.Finished {
			// For stable result queries limitHit = true implies
			// there is a next cursor, and more results may exist.
			result.Stats.IsLimitHit = true
		}
		return result, err
	}

	// If the request is a paginated one, we handle it separately. See
	// paginatedResults for more details.
	if r.Pagination != nil {
		return r.paginatedResults(ctx)
	}

	rr, err := r.resultsWithTimeoutSuggestion(ctx)
	if rr != nil {
		r.logSearchLatency(ctx, rr.ElapsedMilliseconds())
	}

	// Record what type of response we sent back via Prometheus.
	var status, alertType string
	switch {
	case err == context.DeadlineExceeded || (err == nil && rr.allReposTimedout()):
		status = "timeout"
	case err == nil && rr.Stats.Status.Any(search.RepoStatusTimedout):
		status = "partial_timeout"
	case err == nil && rr.alert != nil:
		status = "alert"
		alertType = rr.alert.prometheusType
	case err != nil:
		status = "error"
	case err == nil:
		status = "success"
	default:
		status = "unknown"
	}
	searchResponseCounter.WithLabelValues(
		status,
		alertType,
		string(trace.RequestSource(ctx)),
		trace.GraphQLRequestName(ctx),
	).Inc()

	isSlow := time.Since(start) > logSlowSearchesThreshold()
	if honey.Enabled() || isSlow {
		var act actor.Actor
		if a := actor.FromContext(ctx); a != nil {
			act = *a
		}

		ev := honey.Event("search")
		ev.AddField("query", r.rawQuery())
		ev.AddField("actor_uid", act.UID)
		ev.AddField("actor_internal", act.Internal)
		ev.AddField("type", trace.GraphQLRequestName(ctx))
		ev.AddField("source", string(trace.RequestSource(ctx)))
		ev.AddField("status", status)
		ev.AddField("alert_type", alertType)
		ev.AddField("duration_ms", time.Since(start).Milliseconds())
		if rr != nil {
			ev.AddField("result_size", len(rr.SearchResults))
		}

		if honey.Enabled() {
			_ = ev.Send()
		}

		if isSlow {
			log15.Warn("slow search request", mapToLog15Ctx(ev.Fields())...)
		}
	}

	return rr, err
}

// unionMerge performs a merge of file match results, merging line matches when
// they occur in the same file.
func unionMerge(left, right *SearchResultsResolver) *SearchResultsResolver {
	dedup := NewDeduper()

	// Add results to maps for deduping
	for _, leftResult := range left.SearchResults {
		dedup.Add(leftResult)
	}
	for _, rightResult := range right.SearchResults {
		dedup.Add(rightResult)
	}

	left.SearchResults = dedup.Results()
	left.Stats.Update(&right.Stats)
	return left
}

// union returns the union of two sets of search results and merges common search data.
func union(left, right *SearchResultsResolver) *SearchResultsResolver {
	if right == nil {
		return left
	}
	if left == nil {
		return right
	}

	if left.SearchResults != nil && right.SearchResults != nil {
		return unionMerge(left, right)
	} else if right.SearchResults != nil {
		return right
	}
	return left
}

// intersectMerge performs a merge of file match results, merging line matches
// for files contained in both result sets, and updating counts.
func intersectMerge(left, right *SearchResultsResolver) *SearchResultsResolver {
	rightFileMatches := make(map[string]*FileMatchResolver)
	for _, r := range right.SearchResults {
		if fileMatch, ok := r.ToFileMatch(); ok {
			rightFileMatches[fileMatch.URI] = fileMatch
		}
	}

	var merged []SearchResultResolver
	for _, leftMatch := range left.SearchResults {
		leftFileMatch, ok := leftMatch.ToFileMatch()
		if !ok {
			continue
		}

		rightFileMatch := rightFileMatches[leftFileMatch.URI]
		if rightFileMatch == nil {
			continue
		}

		leftFileMatch.appendMatches(rightFileMatch)
		merged = append(merged, leftMatch)
	}
	left.SearchResults = merged
	left.Stats.Update(&right.Stats)
	return left
}

// intersect returns the intersection of two sets of search result content
// matches, based on whether a single file path contains content matches in both sets.
func intersect(left, right *SearchResultsResolver) *SearchResultsResolver {
	if left == nil || right == nil {
		return nil
	}
	return intersectMerge(left, right)
}

// evaluateAndStream is a wrapper around evaluateAnd which temporarily suspends
// streaming and waits for evaluateAnd to return before streaming results back on
// r.resultChannel.
func (r *searchResolver) evaluateAndStream(ctx context.Context, scopeParameters []query.Node, operands []query.Node) (*SearchResultsResolver, error) {
	// Streaming disabled.
	if r.stream == nil {
		return r.evaluateAnd(ctx, scopeParameters, operands)
	}
	// For streaming search we rely on batch evaluation of
	// results. Implementing true streaming on AND expressions will require
	// support in backends (eg directly using Zoekt) or ANDing per repo.
	r2 := *r
	r2.stream = nil

	result, err := r2.evaluateAnd(ctx, scopeParameters, operands)
	if err != nil {
		return nil, err
	}
	// evaluateAnd may return result, err = nil, nil because downstream calls return
	// nil, nil. See further comments in evaluateAnd.
	if result == nil {
		return &SearchResultsResolver{}, nil
	}
	r.stream.Send(SearchEvent{
		Results: result.SearchResults,
		Stats:   result.Stats,
	})
	return result, err
}

// evaluateAnd performs set intersection on result sets. It collects results for
// all expressions that are ANDed together by searching for each subexpression
// and then intersects those results that are in the same repo/file path. To
// collect N results for count:N, we need to opportunistically ask for more than
// N results for each subexpression (since intersect can never yield more than N,
// and likely yields fewer than N results). If the intersection does not yield N
// results, and is not exhaustive for every expression, we rerun the search by
// doubling count again.
func (r *searchResolver) evaluateAnd(ctx context.Context, scopeParameters []query.Node, operands []query.Node) (*SearchResultsResolver, error) {
	start := time.Now()

	if len(operands) == 0 {
		return &SearchResultsResolver{}, nil
	}

	var (
		err        error
		result     *SearchResultsResolver
		termResult *SearchResultsResolver
	)

	// The number of results we want. Note that for intersect, this number
	// corresponds to documents, not line matches. By default, we ask for at
	// least 5 documents to fill the result page.
	want := 5
	// The fraction of file matches two terms share on average
	averageIntersection := 0.05
	// When we retry, cap the max search results we request for each expression
	// if search continues to not be exhaustive. Alert if exceeded.
	maxTryCount := 40000

	// Set an overall timeout in addition to the timeouts that are set for leaf-requests.
	ctx, cancel, err := r.withTimeout(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	// Set count: if not specified.
	var countStr string
	query.VisitField(scopeParameters, "count", func(value string, _ bool, _ query.Annotation) {
		countStr = value
	})
	if countStr != "" {
		// Override "want" if count is specified.
		want, _ = strconv.Atoi(countStr) // Invariant: count is validated.
	} else {
		scopeParameters = append(scopeParameters, query.Parameter{
			Field: "count",
			Value: strconv.FormatInt(int64(want), 10),
		})
	}

	// tryCount starts small but grows exponentially with the number of operands. It is capped at maxTryCount.
	tryCount := int(math.Floor(float64(want) / math.Pow(averageIntersection, float64(len(operands)-1))))
	if tryCount > maxTryCount {
		tryCount = maxTryCount
	}

	var exhausted bool
	for {
		scopeParameters = query.MapParameter(scopeParameters, func(field, value string, negated bool, annotation query.Annotation) query.Node {
			if field == "count" {
				value = strconv.FormatInt(int64(tryCount), 10)
			}
			return query.Parameter{Field: field, Value: value, Negated: negated, Annotation: annotation}
		})

		result, err = r.evaluatePatternExpression(ctx, scopeParameters, operands[0])
		if err != nil {
			return nil, err
		}
		// We have to guard against result = nil because evaluatePatternExpression can
		// return nil, nil via evaluateOperator.
		if result == nil || len(result.SearchResults) == 0 {
			// We return result instead of nil because result might contain an alert.
			return result, nil
		}
		exhausted = !result.IsLimitHit
		for _, term := range operands[1:] {
			// check if we exceed the overall time limit before running the next query.
			select {
			case <-ctx.Done():
				usedTime := time.Since(start)
				suggestTime := longer(2, usedTime)
				return alertForTimeout(r.db, usedTime, suggestTime, r).wrap(), nil
			default:
			}

			termResult, err = r.evaluatePatternExpression(ctx, scopeParameters, term)
			if err != nil {
				return nil, err
			}
			if termResult == nil || len(termResult.SearchResults) == 0 {
				return termResult, nil
			}
			exhausted = exhausted && !termResult.IsLimitHit
			result = intersect(result, termResult)
		}
		if exhausted {
			break
		}
		if len(result.SearchResults) >= want {
			break
		}
		// If the result size set is not big enough, and we haven't
		// exhausted search on all expressions, double the tryCount and search more.
		tryCount *= 2
		if tryCount > maxTryCount {
			// We've capped out what we're willing to do, throw alert.
			return alertForCappedAndExpression(r.db).wrap(), nil
		}
	}
	result.IsLimitHit = !exhausted
	return result, nil
}

// evaluateOr performs set union on result sets. It collects results for all
// expressions that are ORed together by searching for each subexpression. If
// the maximum number of results are reached after evaluating a subexpression,
// we shortcircuit and return results immediately.
func (r *searchResolver) evaluateOr(ctx context.Context, scopeParameters []query.Node, operands []query.Node) (*SearchResultsResolver, error) {
	if len(operands) == 0 {
		return nil, nil
	}

	wantCount := defaultMaxSearchResults
	if count := query.Q(scopeParameters).Count(); count != nil {
		wantCount = *count
	}

	result := &SearchResultsResolver{}
	for _, term := range operands {
		new, err := r.evaluatePatternExpression(ctx, scopeParameters, term)
		if err != nil {
			return nil, err
		}
		if new != nil {
			result = union(result, new)
			// Do not rely on result.Stats.resultCount because it may
			// count non-content matches and there's no easy way to know.
			if len(result.SearchResults) > wantCount {
				result.SearchResults = result.SearchResults[:wantCount]
				return result, nil
			}
		}
	}
	return result, nil
}

// setQuery sets a new query in the search resolver, for potentially repeated
// calls in the search pipeline. The important part is it takes care of
// invalidating cached repo info.
func (r *searchResolver) setQuery(q []query.Node) {
	if r.invalidateRepoCache {
		r.resolved.RepoRevs = nil
		r.resolved.MissingRepoRevs = nil
		r.repoErr = nil
	}
	r.Query = q
}

// evaluatePatternExpression evaluates a search pattern containing and/or expressions.
func (r *searchResolver) evaluatePatternExpression(ctx context.Context, scopeParameters []query.Node, node query.Node) (*SearchResultsResolver, error) {
	switch term := node.(type) {
	case query.Operator:
		if len(term.Operands) == 0 {
			return nil, nil
		}

		switch term.Kind {
		case query.And:
			return r.evaluateAndStream(ctx, scopeParameters, term.Operands)
		case query.Or:
			return r.evaluateOr(ctx, scopeParameters, term.Operands)
		case query.Concat:
			r.setQuery(append(scopeParameters, term))
			return r.evaluateLeaf(ctx)
		}
	case query.Pattern:
		r.setQuery(append(scopeParameters, term))
		return r.evaluateLeaf(ctx)
	case query.Parameter:
		// evaluatePatternExpression does not process Parameter nodes.
		return nil, nil
	}
	// Unreachable.
	return nil, fmt.Errorf("unrecognized type %T in evaluatePatternExpression", node)
}

// evaluate evaluates all expressions of a search query.
func (r *searchResolver) evaluate(ctx context.Context, q query.Q) (*SearchResultsResolver, error) {
	scopeParameters, pattern, err := query.PartitionSearchPattern(q)
	if err != nil {
		return alertForQuery(r.db, "", err).wrap(), nil
	}
	if pattern == nil {
		r.setQuery(scopeParameters)
		return r.evaluateLeaf(ctx)
	}
	return r.evaluatePatternExpression(ctx, scopeParameters, pattern)
}

// invalidateRepoCache returns whether resolved repos should be invalidated when
// evaluating subexpressions. If a query contains more than one repo, revision,
// or repogroup field, we should invalidate resolved repos, since multiple
// repos, revisions, or repogroups imply that different repos may need to be
// resolved.
func invalidateRepoCache(q []query.Node) bool {
	var seenRepo, seenRevision, seenRepoGroup, seenContext int
	query.VisitField(q, query.FieldRepo, func(_ string, _ bool, _ query.Annotation) {
		seenRepo += 1
	})
	query.VisitField(q, query.FieldRev, func(_ string, _ bool, _ query.Annotation) {
		seenRevision += 1
	})
	query.VisitField(q, query.FieldRepoGroup, func(_ string, _ bool, _ query.Annotation) {
		seenRepoGroup += 1
	})
	query.VisitField(q, query.FieldContext, func(_ string, _ bool, _ query.Annotation) {
		seenContext += 1
	})
	return seenRepo+seenRepoGroup > 1 || seenRevision > 1 || seenContext > 1
}

func (r *searchResolver) Results(ctx context.Context) (srr *SearchResultsResolver, err error) {
	return r.queryResults(ctx, r.Query)
}

func (r *searchResolver) queryResults(ctx context.Context, q query.Q) (srr *SearchResultsResolver, err error) {
	tr, ctx := trace.New(ctx, "Results", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	if invalidateRepoCache(q) {
		r.invalidateRepoCache = true
	}
	r.setQuery(q)

	wantCount := defaultMaxSearchResults
	if count := r.Query.Count(); count != nil {
		wantCount = *count
	}

	expanded, err := query.MapPredicates(r.Query, func(p query.Predicate) ([]query.Node, error) {
		return r.runPredicate(ctx, p)
	})
	if err != nil && errors.Is(err, query.ErrPredicateNoResults) {
		return &SearchResultsResolver{start: time.Now()}, nil
	} else if err != nil {
		return nil, err
	}
	r.setQuery(expanded)

	for _, disjunct := range query.Dnf(r.Query) {
		disjunct = query.ConcatRevFilters(disjunct)
		newResult, err := r.evaluate(ctx, disjunct)
		if err != nil {
			// Fail if any subquery fails.
			return nil, err
		}
		if newResult != nil {
			newResult.SearchResults = selectResults(newResult.SearchResults, r.Query)
			srr = union(srr, newResult)
			if len(srr.SearchResults) > wantCount {
				srr.SearchResults = srr.SearchResults[:wantCount]
				break
			}
		}
	}

	if srr != nil {
		r.sortResults(srr.SearchResults)
	}
	// copy userSettings from searchResolver to SearchResultsResolver
	if srr != nil {
		srr.UserSettings = r.UserSettings
	}
	if srr == nil {
		srr = &SearchResultsResolver{db: r.db}
	}
	return srr, err
}

func (r *searchResolver) runPredicate(ctx context.Context, p query.Predicate) ([]query.Node, error) {
	srr, err := r.queryResults(ctx, p.Query())
	if err != nil {
		return nil, err
	}

	switch p.Field() {
	case query.FieldFile:
		return searchResultsToFileNodes(srr.SearchResults)
	}
	return nil, fmt.Errorf("unsupported predicate result type %q", p.Field())
}

func searchResultsToFileNodes(srs []SearchResultResolver) ([]query.Node, error) {
	nodes := make([]query.Node, 0, len(srs))
	for _, rs := range srs {
		fileResult, ok := rs.(*FileMatchResolver)
		if !ok {
			return nil, fmt.Errorf("expected type %T, but got %T", &FileMatchResolver{}, rs)
		}

		nodes = append(nodes, query.Parameter{
			Field: "file",
			Value: fileResult.Path,
		})
	}
	return nodes, nil
}

// resultsWithTimeoutSuggestion calls doResults, and in case of deadline
// exceeded returns a search alert with a did-you-mean link for the same
// query with a longer timeout.
func (r *searchResolver) resultsWithTimeoutSuggestion(ctx context.Context) (*SearchResultsResolver, error) {
	start := time.Now()
	rr, err := r.doResults(ctx, "")

	// If we encountered a context timeout, it indicates one of the many result
	// type searchers (file, diff, symbol, etc) completely timed out and could not
	// produce even partial results. Other searcher types may have produced results.
	//
	// In this case, or if we got a partial timeout where ALL repositories timed out,
	// we do not return partial results and instead display a timeout alert.
	shouldShowAlert := err == context.DeadlineExceeded
	if err == nil && rr.allReposTimedout() {
		shouldShowAlert = true
	}
	if shouldShowAlert {
		usedTime := time.Since(start)
		suggestTime := longer(2, usedTime)
		return alertForTimeout(r.db, usedTime, suggestTime, r).wrap(), nil
	}
	return rr, err
}

// longer returns a suggested longer time to wait if the given duration wasn't long enough.
func longer(N int, dt time.Duration) time.Duration {
	dt2 := func() time.Duration {
		Ndt := time.Duration(N) * dt
		dceil := func(x float64) time.Duration {
			return time.Duration(math.Ceil(x))
		}
		switch {
		case math.Floor(Ndt.Hours()) > 0:
			return dceil(Ndt.Hours()) * time.Hour
		case math.Floor(Ndt.Minutes()) > 0:
			return dceil(Ndt.Minutes()) * time.Minute
		case math.Floor(Ndt.Seconds()) > 0:
			return dceil(Ndt.Seconds()) * time.Second
		default:
			return 0
		}
	}()
	lowest := 2 * time.Second
	if dt2 < lowest {
		return lowest
	}
	return dt2
}

type searchResultsStats struct {
	JApproximateResultCount string
	JSparkline              []int32

	sr *searchResolver

	once   sync.Once
	srs    *SearchResultsResolver
	srsErr error
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
	// Override user context to ensure that stats for this query are cached
	// regardless of the user context's cancellation. For example, if
	// stats/sparklines are slow to load on the homepage and all users navigate
	// away from that page before they load, no user would ever see them and we
	// would never cache them. This fixes that by ensuring the first request
	// 'kicks off loading' and places the result into cache regardless of
	// whether or not the original querier of this information still wants it.
	originalCtx := ctx
	ctx = context.Background()
	ctx = opentracing.ContextWithSpan(ctx, opentracing.SpanFromContext(originalCtx))

	cacheKey := r.rawQuery()
	// Check if value is in the cache.
	jsonRes, ok := searchResultsStatsCache.Get(cacheKey)
	if ok {
		searchResultsStatsCounter.WithLabelValues("hit").Inc()
		if err := json.Unmarshal(jsonRes, &stats); err != nil {
			return nil, err
		}
		stats.sr = r
		return stats, nil
	}

	// Calculate value from scratch.
	searchResultsStatsCounter.WithLabelValues("miss").Inc()
	attempts := 0
	var v *SearchResultsResolver
	for {
		// Query search results.
		var err error
		v, err = r.doResults(ctx, "")
		if err != nil {
			return nil, err // do not cache errors.
		}
		if v.MatchCount() > 0 {
			break
		}

		cloning := len(v.Cloning())
		timedout := len(v.Timedout())
		if cloning == 0 && timedout == 0 {
			break // zero results, but no cloning or timed out repos. No point in retrying.
		}

		if attempts > 5 {
			log15.Error("failed to generate sparkline due to cloning or timed out repos", "cloning", len(v.Cloning()), "timedout", len(v.Timedout()))
			return nil, fmt.Errorf("failed to generate sparkline due to %d cloning %d timedout repos", len(v.Cloning()), len(v.Timedout()))
		}

		// We didn't find any search results. Some repos are cloning or timed
		// out, so try again in a few seconds.
		attempts++
		log15.Warn("sparkline generation found 0 search results due to cloning or timed out repos (retrying in 5s)", "cloning", len(v.Cloning()), "timedout", len(v.Timedout()))
		time.Sleep(5 * time.Second)
	}

	sparkline, err := v.Sparkline(ctx)
	if err != nil {
		return nil, err // sparkline generation failed, so don't cache.
	}
	stats = &searchResultsStats{
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

type getPatternInfoOptions struct {
	// forceFileSearch, when true, specifies that the search query should be
	// treated as if every default term had `file:` before it. This can be used
	// to allow users to jump to files by just typing their name.
	forceFileSearch         bool
	performStructuralSearch bool
	performLiteralSearch    bool

	fileMatchLimit int32
}

// getPatternInfo gets the search pattern info for the query in the resolver.
func (r *searchResolver) getPatternInfo(opts *getPatternInfoOptions) (*search.TextPatternInfo, error) {
	if opts == nil {
		opts = &getPatternInfoOptions{}
	}

	if opts.fileMatchLimit == 0 {
		opts.fileMatchLimit = int32(r.MaxResults())
	}

	return getPatternInfo(r.Query, opts)
}

func isPatternNegated(q []query.Node) bool {
	isNegated := false
	patternsFound := 0
	query.VisitPattern(q, func(_ string, negated bool, _ query.Annotation) {
		patternsFound++
		if patternsFound > 1 {
			return
		}
		isNegated = negated
	})

	// we only support negation for queries that contain exactly 1 pattern.
	if patternsFound > 1 {
		return false
	}
	return isNegated
}

// processSearchPattern processes the search pattern for a query. It handles the interpretation of search patterns
// as literal, regex, or structural patterns, and applies fuzzy regex matching if applicable.
func processSearchPattern(q query.Q, opts *getPatternInfoOptions) (string, bool, bool, bool) {
	var pattern string
	var pieces []string
	var contentFieldSet bool
	isRegExp := false
	isStructuralPat := false

	patternValues := q.Values(query.FieldDefault)
	isNegated := isPatternNegated(q)

	if overridePattern := q.Values(query.FieldContent); len(overridePattern) > 0 {
		patternValues = overridePattern
		contentFieldSet = true
	}

	if opts.performStructuralSearch {
		isStructuralPat = true
		for _, v := range patternValues {
			if piece := v.ToString(); piece != "" {
				pieces = append(pieces, piece)
			}
		}
		pattern = strings.Join(pieces, " ")
	} else if !opts.forceFileSearch {
		isRegExp = true
		for _, v := range patternValues {
			var piece string
			switch {
			case v.String != nil:
				if contentFieldSet && !opts.performLiteralSearch {
					piece = *v.String
				} else {
					// Treat quoted strings as literal
					// strings to match, not regexps.
					piece = regexp.QuoteMeta(*v.String)
				}
			case v.Regexp != nil:
				piece = v.Regexp.String()
			}
			if piece == "" {
				continue
			}
			pieces = append(pieces, piece)
		}
		pattern = orderedFuzzyRegexp(pieces)
	} else {
		// TODO: We must have some pattern that always matches here, or else
		// cmd/searcher/search/matcher.go:97 would cause a nil regexp panic
		// when not using indexed search. I am unsure what the right solution
		// is here. Would this code path go away when we switch fully to
		// indexed search @keegan? This workaround is OK for now though.
		isRegExp = true
		pattern = "."
	}

	return pattern, isRegExp, isStructuralPat, isNegated
}

// getPatternInfo gets the search pattern info for q
func getPatternInfo(q query.Q, opts *getPatternInfoOptions) (*search.TextPatternInfo, error) {
	pattern, isRegExp, isStructuralPat, isNegated := processSearchPattern(q, opts)

	// Handle file: and -file: filters.
	includePatterns, excludePatterns := q.RegexpPatterns(query.FieldFile)
	filePatternsReposMustInclude, filePatternsReposMustExclude := q.RegexpPatterns(query.FieldRepoHasFile)

	if opts.forceFileSearch {
		for _, v := range q.Values(query.FieldDefault) {
			includePatterns = append(includePatterns, v.ToString())
		}
	}

	var combyRule []string
	for _, v := range q.Values(query.FieldCombyRule) {
		combyRule = append(combyRule, v.ToString())
	}

	// Handle lang: and -lang: filters.
	langIncludePatterns, langExcludePatterns, err := langIncludeExcludePatterns(q.StringValues(query.FieldLang))
	if err != nil {
		return nil, err
	}
	includePatterns = append(includePatterns, langIncludePatterns...)
	excludePatterns = append(excludePatterns, langExcludePatterns...)

	languages, _ := q.StringValues(query.FieldLang)

	var sp filter.SelectPath
	if sf, _ := q.StringValue(query.FieldSelect); sf != "" {
		sp, err = filter.SelectPathFromString(sf)
		if err != nil {
			return nil, err
		}
	}

	patternInfo := &search.TextPatternInfo{
		IsRegExp:                     isRegExp,
		IsStructuralPat:              isStructuralPat,
		IsCaseSensitive:              q.IsCaseSensitive(),
		FileMatchLimit:               opts.fileMatchLimit,
		Pattern:                      pattern,
		IsNegated:                    isNegated,
		IncludePatterns:              includePatterns,
		FilePatternsReposMustInclude: filePatternsReposMustInclude,
		FilePatternsReposMustExclude: filePatternsReposMustExclude,
		Languages:                    languages,
		PathPatternsAreCaseSensitive: q.IsCaseSensitive(),
		CombyRule:                    strings.Join(combyRule, ""),
		Index:                        indexValue(q),
		Select:                       sp,
	}
	if len(excludePatterns) > 0 {
		patternInfo.ExcludePattern = searchrepos.UnionRegExps(excludePatterns)
	}
	return patternInfo, nil
}

// indexValue converts the query index field to one of yes (default), only, or no
// enum values.
func indexValue(q query.Q) query.YesNoOnly {
	indexParam := query.Yes
	if index := q.Values(query.FieldIndex); len(index) > 0 {
		indexParam = query.ParseYesNoOnly(index[0].ToString())
	}
	return indexParam
}

// langIncludeExcludePatterns returns regexps for the include/exclude path patterns given the lang:
// and -lang: filter values in a search query. For example, a query containing "lang:go" should
// include files whose paths match /\.go$/.
func langIncludeExcludePatterns(values, negatedValues []string) (includePatterns, excludePatterns []string, err error) {
	do := func(values []string, patterns *[]string) error {
		for _, value := range values {
			lang, ok := enry.GetLanguageByAlias(value)
			if !ok {
				return fmt.Errorf("unknown language: %q", value)
			}
			exts := enry.GetLanguageExtensions(lang)
			extPatterns := make([]string, len(exts))
			for i, ext := range exts {
				// Add `\.ext$` pattern to match files with the given extension.
				extPatterns[i] = regexp.QuoteMeta(ext) + "$"
			}
			*patterns = append(*patterns, searchrepos.UnionRegExps(extPatterns))
		}
		return nil
	}

	if err := do(values, &includePatterns); err != nil {
		return nil, nil, err
	}
	if err := do(negatedValues, &excludePatterns); err != nil {
		return nil, nil, err
	}
	return includePatterns, excludePatterns, nil
}

var (
	// The default timeout to use for queries.
	defaultTimeout = 20 * time.Second
)

func (r *searchResolver) searchTimeoutFieldSet() bool {
	timeout := r.Query.Timeout()
	return timeout != nil || r.countIsSet()
}

func (r *searchResolver) withTimeout(ctx context.Context) (context.Context, context.CancelFunc, error) {
	d := defaultTimeout
	maxTimeout := time.Duration(searchrepos.SearchLimits().MaxTimeoutSeconds) * time.Second
	timeout := r.Query.Timeout()
	if timeout != nil {
		d = *timeout
	} else if r.countIsSet() {
		// If `count:` is set but `timeout:` is not explicitly set, use the max timeout
		d = maxTimeout
	}
	if d > maxTimeout {
		d = maxTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, d)
	return ctx, cancel, nil
}

func (r *searchResolver) determineResultTypes(args search.TextParameters, forceOnlyResultType string) (resultTypes []string) {
	// Determine which types of results to return.
	if forceOnlyResultType != "" {
		resultTypes = []string{forceOnlyResultType}
	} else {
		resultTypes, _ = r.Query.StringValues(query.FieldType)
		if len(resultTypes) == 0 {
			resultTypes = []string{"file", "path", "repo"}
		}
	}
	for _, resultType := range resultTypes {
		if resultType == "file" {
			args.PatternInfo.PatternMatchesContent = true
		} else if resultType == "path" {
			args.PatternInfo.PatternMatchesPath = true
		}
	}
	return resultTypes
}

func (r *searchResolver) determineRepos(ctx context.Context, tr *trace.Trace, start time.Time) (resolved searchrepos.Resolved, res *SearchResultsResolver, err error) {
	resolved, err = r.resolveRepositories(ctx, nil)
	if err != nil {
		if errors.Is(err, authz.ErrStalePermissions{}) {
			log15.Debug("searchResolver.determineRepos", "err", err)
			alert := alertForStalePermissions(r.db)
			return searchrepos.Resolved{}, &SearchResultsResolver{db: r.db, alert: alert, start: start}, nil
		}
		e := git.BadCommitError{}
		if errors.As(err, &e) {
			alert := r.alertForInvalidRevision(e.Spec)
			return searchrepos.Resolved{}, &SearchResultsResolver{db: r.db, alert: alert, start: start}, nil
		}
		return searchrepos.Resolved{}, nil, err
	}

	tr.LazyPrintf("searching %d repos, %d missing", len(resolved.RepoRevs), len(resolved.MissingRepoRevs))
	if len(resolved.RepoRevs) == 0 {
		alert := r.alertForNoResolvedRepos(ctx)
		return searchrepos.Resolved{}, &SearchResultsResolver{db: r.db, alert: alert, start: start}, nil
	}
	if resolved.OverLimit {
		alert := r.alertForOverRepoLimit(ctx)
		return searchrepos.Resolved{}, &SearchResultsResolver{db: r.db, alert: alert, start: start}, nil
	}
	return resolved, nil, nil
}

type DiffCommitError struct {
	ResultType string
	Max        int
}

type RepoLimitError DiffCommitError
type TimeLimitError DiffCommitError

func (*RepoLimitError) Error() string {
	return "repo limit error"
}

func (*TimeLimitError) Error() string {
	return "time limit error"
}

func checkDiffCommitSearchLimits(ctx context.Context, args *search.TextParameters, resultType string) error {
	repos, err := getRepos(ctx, args.RepoPromise)
	if err != nil {
		return err
	}

	hasTimeFilter := false
	if _, afterPresent := args.Query.Fields()["after"]; afterPresent {
		hasTimeFilter = true
	}
	if _, beforePresent := args.Query.Fields()["before"]; beforePresent {
		hasTimeFilter = true
	}

	limits := searchrepos.SearchLimits()
	if max := limits.CommitDiffMaxRepos; !hasTimeFilter && len(repos) > max {
		return &RepoLimitError{ResultType: resultType, Max: max}
	}
	if max := limits.CommitDiffWithTimeFilterMaxRepos; hasTimeFilter && len(repos) > max {
		return &TimeLimitError{ResultType: resultType, Max: max}
	}
	return nil
}

func newAggregator(db dbutil.DB, stream Sender, inputs *SearchInputs) *aggregator {
	return &aggregator{
		db:           db,
		parentStream: stream,
		alert: alertObserver{
			db:     db,
			Inputs: inputs,
		},
	}
}

type aggregator struct {
	parentStream Sender
	db           dbutil.DB

	mu      sync.Mutex
	results []SearchResultResolver
	stats   streaming.Stats
	alert   alertObserver
}

// get finalises aggregation over the stream and returns the aggregated
// result. It should only be called once each do* function is finished
// running.
func (a *aggregator) get() ([]SearchResultResolver, streaming.Stats, *searchAlert, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	alert, err := a.alert.Done(&a.stats)
	return a.results, a.stats, alert, err
}

func (a *aggregator) Send(event SearchEvent) {
	if a.parentStream != nil {
		a.parentStream.Send(event)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Do not aggregate results if we are streaming.
	if a.parentStream == nil {
		a.results = append(a.results, event.Results...)
	}

	a.alert.Update(event)
	a.stats.Update(&event.Stats)
}

func (a *aggregator) error(ctx context.Context, err error) {
	a.alert.Error(ctx, err)
}

func (a *aggregator) doRepoSearch(ctx context.Context, args *search.TextParameters, limit int32) (err error) {
	tr, ctx := trace.New(ctx, "doRepoSearch", "")
	defer func() {
		a.error(ctx, err)
		tr.SetError(err)
		tr.Finish()
	}()

	err = searchRepositories(ctx, a.db, args, limit, a)
	return errors.Wrap(err, "repository search failed")
}

func (a *aggregator) doSymbolSearch(ctx context.Context, args *search.TextParameters, limit int) (err error) {
	tr, ctx := trace.New(ctx, "doSymbolSearch", "")
	defer func() {
		a.error(ctx, err)
		tr.SetError(err)
		tr.Finish()
	}()

	err = searchSymbols(ctx, a.db, args, limit, a)
	return errors.Wrap(err, "symbol search failed")
}

func (a *aggregator) doFilePathSearch(ctx context.Context, args *search.TextParameters) (err error) {
	tr, ctx := trace.New(ctx, "doFilePathSearch", "")
	tr.LogFields(trace.Stringer("global_search_mode", args.Mode))
	defer func() {
		a.error(ctx, err)
		tr.SetError(err)
		tr.Finish()
	}()

	isDefaultStructuralSearch := args.PatternInfo.IsStructuralPat && args.PatternInfo.FileMatchLimit == defaultMaxSearchResults

	if !isDefaultStructuralSearch {
		return searchFilesInRepos(ctx, a.db, args, a)
	}

	// For structural search with default limits we retry if we get no results.

	fileResults, stats, err := searchFilesInReposBatch(ctx, a.db, args)

	if len(fileResults) == 0 && err == nil {
		// No results for structural search? Automatically search again and force Zoekt
		// to resolve more potential file matches by setting a higher FileMatchLimit.
		patternCopy := *(args.PatternInfo)
		patternCopy.FileMatchLimit = 1000
		argsCopy := *args
		argsCopy.PatternInfo = &patternCopy
		args = &argsCopy

		fileResults, stats, err = searchFilesInReposBatch(ctx, a.db, args)

		if len(fileResults) == 0 {
			// Still no results? Give up.
			log15.Warn("Structural search gives up after more exhaustive attempt. Results may have been missed.")
			stats.IsLimitHit = false // Ensure we don't display "Show more".
		}
	}

	a.Send(SearchEvent{
		Results: fileMatchResultsToSearchResults(fileResults),
		Stats:   stats,
	})
	return err
}

func (a *aggregator) doDiffSearch(ctx context.Context, tp *search.TextParameters) (err error) {
	tr, ctx := trace.New(ctx, "doDiffSearch", "")
	defer func() {
		a.error(ctx, err)
		tr.SetError(err)
		tr.Finish()
	}()

	if err := checkDiffCommitSearchLimits(ctx, tp, "diff"); err != nil {
		return err
	}

	args, err := resolveCommitParameters(ctx, tp)
	if err != nil {
		log15.Warn("doDiffSearch: error while resolving commit parameters", "error", err)
		return nil
	}

	return searchCommitDiffsInRepos(ctx, a.db, args, a)
}

func (a *aggregator) doCommitSearch(ctx context.Context, tp *search.TextParameters) (err error) {
	tr, ctx := trace.New(ctx, "doCommitSearch", "")
	defer func() {
		a.error(ctx, err)
		tr.SetError(err)
		tr.Finish()
	}()

	if err := checkDiffCommitSearchLimits(ctx, tp, "commit"); err != nil {
		return err
	}

	args, err := resolveCommitParameters(ctx, tp)
	if err != nil {
		log15.Warn("doCommitSearch: error while resolving commit parameters", "error", err)
		return nil
	}

	return searchCommitLogInRepos(ctx, a.db, args, a)
}

func statsDeref(s *streaming.Stats) streaming.Stats {
	if s == nil {
		return streaming.Stats{}
	}
	return *s
}

// isGlobalSearch returns true if the query does not contain repo, repogroup, or
// repohasfile filters. For structural queries, queries with version context,
// and queries with non-global search context, isGlobalSearch always return false.
func (r *searchResolver) isGlobalSearch() bool {
	if r.PatternType == query.SearchTypeStructural {
		return false
	}
	if r.VersionContext != nil && *r.VersionContext != "" {
		return false
	}
	querySearchContextSpec, _ := r.Query.StringValue(query.FieldContext)
	if envvar.SourcegraphDotComMode() && !searchcontexts.IsGlobalSearchContextSpec(querySearchContextSpec) {
		return false
	}
	return len(r.Query.Values(query.FieldRepo)) == 0 && len(r.Query.Values(query.FieldRepoGroup)) == 0 && len(r.Query.Values(query.FieldRepoHasFile)) == 0
}

// doResults is one of the highest level search functions that handles finding results.
//
// If forceOnlyResultType is specified, only results of the given type are returned,
// regardless of what `type:` is specified in the query string.
//
// Partial results AND an error may be returned.
func (r *searchResolver) doResults(ctx context.Context, forceOnlyResultType string) (_ *SearchResultsResolver, err error) {
	tr, ctx := trace.New(ctx, "doResults", r.rawQuery())
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	start := time.Now()

	ctx, cancel, err := r.withTimeout(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	options := &getPatternInfoOptions{}
	limit := r.MaxResults()
	options.fileMatchLimit = int32(limit)
	if r.PatternType == query.SearchTypeStructural {
		options = &getPatternInfoOptions{performStructuralSearch: true}
		forceOnlyResultType = "file"
	}
	if r.PatternType == query.SearchTypeLiteral {
		options = &getPatternInfoOptions{performLiteralSearch: true}
	}
	p, err := r.getPatternInfo(options)
	if err != nil {
		return nil, err
	}

	// Fallback to literal search for searching repos and files if
	// the structural search pattern is empty.
	if r.PatternType == query.SearchTypeStructural && p.Pattern == "" {
		r.PatternType = query.SearchTypeLiteral
		p.IsStructuralPat = false
		forceOnlyResultType = ""
	}

	args := search.TextParameters{
		PatternInfo: p,
		Query:       r.Query,

		// UseFullDeadline if timeout: set or we are streaming.
		UseFullDeadline: r.searchTimeoutFieldSet() || r.stream != nil,

		Zoekt:        r.zoekt,
		SearcherURLs: r.searcherURLs,
		RepoPromise:  &search.Promise{},
	}
	if err := args.PatternInfo.Validate(); err != nil {
		return nil, &badRequestError{err}
	}

	resultTypes := r.determineResultTypes(args, forceOnlyResultType)
	tr.LazyPrintf("resultTypes: %v", resultTypes)
	var (
		requiredWg      sync.WaitGroup
		optionalWg      sync.WaitGroup
		seenResultTypes = make(map[string]struct{})
	)

	waitGroup := func(required bool) *sync.WaitGroup {
		if args.UseFullDeadline {
			// When a custom timeout is specified, all searches are required and get the full timeout.
			return &requiredWg
		}
		if required {
			return &requiredWg
		}
		return &optionalWg
	}

	// For streaming search we want to limit based on all results, not just
	// per backend. This works better than batch based since we have higher
	// defaults.
	stream := r.stream
	if stream != nil {
		var cancelOnLimit context.CancelFunc
		ctx, stream, cancelOnLimit = WithLimit(ctx, stream, limit)
		defer cancelOnLimit()
	}

	agg := newAggregator(r.db, stream, r.SearchInputs)

	// This ensures we properly cleanup in the case of an early return. In
	// particular we want to cancel global searches before returning early.
	hasStartedAllBackends := false
	defer func() {
		if hasStartedAllBackends {
			return
		}
		cancel()
		requiredWg.Wait()
		optionalWg.Wait()
		_, _, _, _ = agg.get()
	}()

	isFileOrPath := func() bool {
		for _, rt := range resultTypes {
			if rt == "file" || rt == "path" {
				return true
			}
		}
		return false
	}

	isIndexedSearch := args.PatternInfo.Index != query.No

	// performance optimization: call zoekt early, resolve repos concurrently, filter
	// search results with resolved repos.
	if r.isGlobalSearch() && isIndexedSearch && isFileOrPath() {
		argsIndexed := args
		argsIndexed.Mode = search.ZoektGlobalSearch
		wg := waitGroup(true)
		wg.Add(1)
		goroutine.Go(func() {
			defer wg.Done()
			_ = agg.doFilePathSearch(ctx, &argsIndexed)
		})
		// On sourcegraph.com and for unscoped queries, determineRepos returns the subset
		// of indexed default searchrepos. No need to call searcher, because
		// len(searcherRepos) will always be 0.
		if envvar.SourcegraphDotComMode() {
			args.Mode = search.NoFilePath
		} else {
			args.Mode = search.SearcherOnly
		}
	}

	resolved, alertResult, err := r.determineRepos(ctx, tr, start)
	if err != nil {
		return nil, err
	}
	if alertResult != nil {
		return alertResult, nil
	}
	if len(resolved.MissingRepoRevs) > 0 {
		agg.error(ctx, &missingRepoRevsError{Missing: resolved.MissingRepoRevs})
	}

	// Send down our first bit of progress.
	{
		repos := make(map[api.RepoID]*types.RepoName, len(resolved.RepoRevs))
		for _, repoRev := range resolved.RepoRevs {
			repos[repoRev.Repo.ID] = repoRev.Repo
		}

		agg.Send(SearchEvent{
			Stats: streaming.Stats{
				Repos:            repos,
				ExcludedForks:    resolved.ExcludedRepos.Forks,
				ExcludedArchived: resolved.ExcludedRepos.Archived,
			},
		})
	}

	// Resolve repo promise so searches waiting on it can proceed. We do this
	// after reporting the above progress to ensure we don't get search
	// results before the above reporting.
	args.RepoPromise.Resolve(resolved.RepoRevs)

	searchedFileContentsOrPaths := false
	for _, resultType := range resultTypes {
		resultType := resultType // shadow so it doesn't change in the goroutine
		if _, seen := seenResultTypes[resultType]; seen {
			continue
		}
		seenResultTypes[resultType] = struct{}{}
		switch resultType {
		case "repo":
			wg := waitGroup(true)
			wg.Add(1)
			goroutine.Go(func() {
				defer wg.Done()
				_ = agg.doRepoSearch(ctx, &args, int32(limit))
			})
		case "symbol":
			wg := waitGroup(len(resultTypes) == 1)
			wg.Add(1)
			goroutine.Go(func() {
				defer wg.Done()
				_ = agg.doSymbolSearch(ctx, &args, limit)
			})
		case "file", "path":
			if searchedFileContentsOrPaths || args.Mode == search.NoFilePath {
				// type:file and type:path use same searchFilesInRepos, so don't call 2x.
				continue
			}
			searchedFileContentsOrPaths = true
			wg := waitGroup(true)
			wg.Add(1)
			goroutine.Go(func() {
				defer wg.Done()
				_ = agg.doFilePathSearch(ctx, &args)
			})
		case "diff":
			wg := waitGroup(len(resultTypes) == 1)
			wg.Add(1)
			goroutine.Go(func() {
				defer wg.Done()
				_ = agg.doDiffSearch(ctx, &args)
			})
		case "commit":
			wg := waitGroup(len(resultTypes) == 1)
			wg.Add(1)
			goroutine.Go(func() {
				defer wg.Done()
				_ = agg.doCommitSearch(ctx, &args)
			})
		}
	}

	hasStartedAllBackends = true

	// Wait for required searches.
	requiredWg.Wait()

	// Give optional searches some minimum budget in case required searches return quickly.
	// Cancel all remaining searches after this minimum budget.
	budget := 100 * time.Millisecond
	elapsed := time.Since(start)
	timer := time.AfterFunc(budget-elapsed, cancel)

	// Wait for remaining optional searches to finish or get cancelled.
	optionalWg.Wait()

	timer.Stop()

	// We have to call get once all waitgroups are done since it relies on
	// collecting from the streams.
	results, common, alert, err := agg.get()

	tr.LazyPrintf("results=%d %s", len(results), &common)

	r.sortResults(results)

	resultsResolver := SearchResultsResolver{
		db:            r.db,
		start:         start,
		Stats:         common,
		SearchResults: results,
		limit:         limit,
		alert:         alert,
	}
	return &resultsResolver, err
}

// isContextError returns true if ctx.Err() is not nil or if err
// is an error caused by context cancelation or timeout.
func isContextError(ctx context.Context, err error) bool {
	return ctx.Err() != nil || err == context.Canceled || err == context.DeadlineExceeded
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

	ResultCount() int32
}

// compareFileLengths sorts file paths such that they appear earlier if they
// match file: patterns in the query exactly.
func compareFileLengths(left, right string, exactFilePatterns map[string]struct{}) bool {
	_, aMatch := exactFilePatterns[path.Base(left)]
	_, bMatch := exactFilePatterns[path.Base(right)]
	if aMatch || bMatch {
		if aMatch && bMatch {
			// Prefer shorter file names (ie root files come first)
			if len(left) != len(right) {
				return len(left) < len(right)
			}
			return left < right
		}
		// Prefer exact match
		return aMatch
	}
	return left < right
}

func compareDates(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left != nil // Place the value that is defined first.
	}
	return (*left).After(*right)
}

// compareSearchResults sorts repository matches, file matches, and commits.
// Repositories and filenames are sorted alphabetically. As a refinement, if any
// filename matches a value in a non-empty set exactFilePatterns, then such
// filenames are listed earlier.
//
// Commits are sorted by date. Commits are not associated with searchrepos, and
// will always list after repository or file match results, if any.
func compareSearchResults(left, right SearchResultResolver, exactFilePatterns map[string]struct{}) bool {
	sortKeys := func(result SearchResultResolver) (string, string, *time.Time) {
		switch r := result.(type) {
		case *RepositoryResolver:
			return r.Name(), "", nil
		case *FileMatchResolver:
			return string(r.Repo.Name), r.Path, nil
		case *CommitSearchResultResolver:
			// Commits are relatively sorted by date, and after repo
			// or path names. We use ~ as the key for repo and
			// paths,lexicographically last in ASCII.
			if r.Commit().commit != nil {
				return "~", "~", &r.Commit().commit.Author.Date
			}
			return "~", "~", &time.Time{}
		}
		// Unreachable.
		panic("unreachable: compareSearchResults expects RepositoryResolver, FileMatchResolver, or CommitSearchResultResolver")
	}

	arepo, afile, adate := sortKeys(left)
	brepo, bfile, bdate := sortKeys(right)

	if arepo == brepo {
		if len(exactFilePatterns) == 0 {
			if afile != bfile {
				return afile < bfile
			}
			return compareDates(adate, bdate)
		}
		return compareFileLengths(afile, bfile, exactFilePatterns)
	}
	return arepo < brepo
}

func selectResults(results []SearchResultResolver, q query.Q) []SearchResultResolver {
	v, _ := q.StringValue(query.FieldSelect)
	if v == "" {
		return results
	}
	sp, _ := filter.SelectPathFromString(v) // Invariant: select already validated

	dedup := NewDeduper()
	for _, result := range results {
		var current SearchResultResolver
		switch v := result.(type) {
		case *FileMatchResolver:
			current = v.Select(sp)
		case *RepositoryResolver:
			current = v.Select(sp)
		case *CommitSearchResultResolver:
			current = v.Select(sp)
		default:
			current = result
		}

		if current == nil {
			continue
		}
		dedup.Add(current)
	}
	return dedup.Results()
}

func (r *searchResolver) sortResults(results []SearchResultResolver) {
	var exactPatterns map[string]struct{}
	if getBoolPtr(r.UserSettings.SearchGlobbing, false) {
		exactPatterns = r.getExactFilePatterns()
	}
	sort.Slice(results, func(i, j int) bool { return compareSearchResults(results[i], results[j], exactPatterns) })
}

// getExactFilePatterns returns the set of file patterns without glob syntax.
func (r *searchResolver) getExactFilePatterns() map[string]struct{} {
	m := map[string]struct{}{}
	query.VisitField(
		r.Query,
		query.FieldFile,
		func(value string, negated bool, annotation query.Annotation) {
			originalValue := r.OriginalQuery[annotation.Range.Start.Column+len(query.FieldFile)+1 : annotation.Range.End.Column]
			if !negated && query.ContainsNoGlobSyntax(originalValue) {
				m[originalValue] = struct{}{}
			}
		})
	return m
}

// orderedFuzzyRegexp interpolate a lazy 'match everything' regexp pattern
// to achieve an ordered fuzzy regexp match.
func orderedFuzzyRegexp(pieces []string) string {
	if len(pieces) == 0 {
		return ""
	}
	if len(pieces) == 1 {
		return pieces[0]
	}
	return "(" + strings.Join(pieces, ").*?(") + ")"
}

// logSlowSearchesThreshold returns the minimum duration configured in site
// settings for logging slow searches.
func logSlowSearchesThreshold() time.Duration {
	ms := conf.Get().ObservabilityLogSlowSearches
	if ms == 0 {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(ms) * time.Millisecond
}

// mapToLog15Ctx translates a map to log15 context fields.
func mapToLog15Ctx(m map[string]interface{}) []interface{} {
	// sort so its stable
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	ctx := make([]interface{}, len(m)*2)
	for i, k := range keys {
		j := i * 2
		ctx[j] = k
		ctx[j+1] = m[k]
	}
	return ctx
}
