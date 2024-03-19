package resolvers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/insights/background/queryrunner"
	"github.com/sourcegraph/sourcegraph/internal/insights/query/querybuilder"
	"github.com/sourcegraph/sourcegraph/internal/insights/scheduler"
	"github.com/sourcegraph/sourcegraph/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/insights/types"
	searchquery "github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	_ graphqlbackend.InsightSeriesResolver     = &precalculatedInsightSeriesResolver{}
	_ graphqlbackend.InsightsDataPointResolver = insightsDataPointResolver{}
)

type insightsDataPointResolver struct {
	p        store.SeriesPoint
	diffInfo *querybuilder.PointDiffQueryOpts
}

func (i insightsDataPointResolver) DateTime() gqlutil.DateTime {
	return gqlutil.DateTime{Time: i.p.Time}
}

func (i insightsDataPointResolver) Value() float64 { return i.p.Value }

func (i insightsDataPointResolver) DiffQuery() (*string, error) {
	if i.diffInfo == nil {
		return nil, nil
	}
	query, err := querybuilder.PointDiffQuery(*i.diffInfo)
	if err != nil {
		// we don't want to error the whole process if diff query building errored.
		return nil, nil
	}
	q := query.String()
	return &q, nil
}

type statusInfo struct {
	totalPoints, pendingJobs, completedJobs, failedJobs int32
	backfillQueuedAt                                    *time.Time
	isLoading                                           bool
}

type (
	GetSeriesQueueStatusFunc    func(ctx context.Context, seriesID string) (*queryrunner.JobsStatus, error)
	GetSeriesBackfillsFunc      func(ctx context.Context, seriesID int) ([]scheduler.SeriesBackfill, error)
	GetIncompleteDatapointsFunc func(ctx context.Context, seriesID int) ([]store.IncompleteDatapoint, error)
	insightStatusResolver       struct {
		getQueueStatus          GetSeriesQueueStatusFunc
		getSeriesBackfills      GetSeriesBackfillsFunc
		getIncompleteDatapoints GetIncompleteDatapointsFunc
		statusOnce              sync.Once
		series                  types.InsightViewSeries

		status    statusInfo
		statusErr error
	}
)

func (i *insightStatusResolver) TotalPoints(ctx context.Context) (int32, error) {
	status, err := i.calculateStatus(ctx)
	return status.totalPoints, err
}

func (i *insightStatusResolver) PendingJobs(ctx context.Context) (int32, error) {
	status, err := i.calculateStatus(ctx)
	return status.pendingJobs, err
}

func (i *insightStatusResolver) CompletedJobs(ctx context.Context) (int32, error) {
	status, err := i.calculateStatus(ctx)
	return status.completedJobs, err
}

func (i *insightStatusResolver) FailedJobs(ctx context.Context) (int32, error) {
	status, err := i.calculateStatus(ctx)
	return status.failedJobs, err
}

func (i *insightStatusResolver) BackfillQueuedAt(ctx context.Context) *gqlutil.DateTime {
	return gqlutil.DateTimeOrNil(i.series.BackfillQueuedAt)
}

func (i *insightStatusResolver) IsLoadingData(ctx context.Context) (*bool, error) {
	status, err := i.calculateStatus(ctx)
	if err != nil {
		return nil, err
	}
	return &status.isLoading, nil
}

func (i *insightStatusResolver) calculateStatus(ctx context.Context) (statusInfo, error) {
	i.statusOnce.Do(func() {
		status, statusErr := i.getQueueStatus(ctx, i.series.SeriesID)
		if statusErr != nil {
			i.statusErr = errors.Wrap(statusErr, "QueryJobsStatus")
			return
		}
		i.status.backfillQueuedAt = i.series.BackfillQueuedAt
		i.status.completedJobs = int32(status.Completed)
		i.status.failedJobs = int32(status.Failed)
		i.status.pendingJobs = int32(status.Queued + status.Processing + status.Errored)

		seriesBackfills, backillErr := i.getSeriesBackfills(ctx, i.series.InsightSeriesID)
		if backillErr != nil {
			i.statusErr = errors.Wrap(backillErr, "LoadSeriesBackfills")
			return
		}
		backfillInProgress := false
		for n := range seriesBackfills {
			if seriesBackfills[n].SeriesId == i.series.InsightSeriesID && !seriesBackfills[n].IsTerminalState() {
				backfillInProgress = true
				break
			}
		}
		i.status.isLoading = i.status.backfillQueuedAt == nil || i.status.pendingJobs > 0 || backfillInProgress
	})
	return i.status, i.statusErr
}

func NewStatusResolver(r *baseInsightResolver, viewSeries types.InsightViewSeries) *insightStatusResolver {
	getStatus := func(ctx context.Context, series string) (*queryrunner.JobsStatus, error) {
		return queryrunner.QueryJobsStatus(ctx, r.workerBaseStore, series)
	}
	getBackfills := func(ctx context.Context, seriesID int) ([]scheduler.SeriesBackfill, error) {
		backfillStore := scheduler.NewBackfillStore(r.insightsDB)
		return backfillStore.LoadSeriesBackfills(ctx, seriesID)
	}
	getIncompletes := func(ctx context.Context, seriesID int) ([]store.IncompleteDatapoint, error) {
		return r.timeSeriesStore.LoadAggregatedIncompleteDatapoints(ctx, seriesID)
	}
	return newStatusResolver(getStatus, getBackfills, getIncompletes, viewSeries)
}

func newStatusResolver(getQueueStatus GetSeriesQueueStatusFunc, getSeriesBackfills GetSeriesBackfillsFunc, getIncompleteDatapoints GetIncompleteDatapointsFunc, series types.InsightViewSeries) *insightStatusResolver {
	return &insightStatusResolver{
		getQueueStatus:          getQueueStatus,
		getSeriesBackfills:      getSeriesBackfills,
		series:                  series,
		getIncompleteDatapoints: getIncompleteDatapoints,
	}
}

type precalculatedInsightSeriesResolver struct {
	insightsStore   store.Interface
	workerBaseStore *basestore.Store
	series          types.InsightViewSeries
	metadataStore   store.InsightMetadataStore
	statusResolver  graphqlbackend.InsightStatusResolver

	seriesId string
	points   []store.SeriesPoint
	label    string
	filters  types.InsightViewFilters
}

func (p *precalculatedInsightSeriesResolver) SeriesId() string {
	return p.seriesId
}

func (p *precalculatedInsightSeriesResolver) Label() string {
	return p.label
}

func (p *precalculatedInsightSeriesResolver) Points(ctx context.Context, _ *graphqlbackend.InsightsPointsArgs) ([]graphqlbackend.InsightsDataPointResolver, error) {
	resolvers := make([]graphqlbackend.InsightsDataPointResolver, 0, len(p.points))
	db := database.NewDBWith(log.Scoped("Points"), p.workerBaseStore)
	scHandler := store.NewSearchContextHandler(db)
	modifiedPoints := removeClosePoints(p.points, p.series)
	filterRepoIncludes := []string{}
	filterRepoExcludes := []string{}

	if !isNilOrEmpty(p.filters.IncludeRepoRegex) {
		filterRepoIncludes = append(filterRepoIncludes, *p.filters.IncludeRepoRegex)
	}
	if !isNilOrEmpty(p.filters.ExcludeRepoRegex) {
		filterRepoExcludes = append(filterRepoExcludes, *p.filters.ExcludeRepoRegex)
	}

	// ignoring error to ensure points return - if a search context error would occure it would have likely already happened.
	includeRepos, excludeRepos, _ := scHandler.UnwrapSearchContexts(ctx, p.filters.SearchContexts)
	filterRepoIncludes = append(filterRepoIncludes, includeRepos...)
	filterRepoExcludes = append(filterRepoExcludes, excludeRepos...)

	// Replacing capture group values if present
	// Ignoring errors so it falls back to the entered query
	query := p.series.Query
	if p.series.GeneratedFromCaptureGroups && len(modifiedPoints) > 0 {
		replacer, _ := querybuilder.NewPatternReplacer(querybuilder.BasicQuery(query), searchquery.SearchTypeRegex)
		if replacer != nil {
			replaced, err := replacer.Replace(*modifiedPoints[0].Capture)
			if err == nil {
				query = replaced.String()
			}
		}
	}

	for i := range len(modifiedPoints) {
		var after *time.Time
		if i > 0 {
			after = &modifiedPoints[i-1].Time
		}

		pointResolver := insightsDataPointResolver{
			p: modifiedPoints[i],
			diffInfo: &querybuilder.PointDiffQueryOpts{
				After:              after,
				Before:             modifiedPoints[i].Time,
				FilterRepoIncludes: filterRepoIncludes,
				FilterRepoExcludes: filterRepoExcludes,
				RepoList:           p.series.Repositories,
				RepoSearch:         p.series.RepositoryCriteria,
				SearchQuery:        querybuilder.BasicQuery(query),
			},
		}
		resolvers = append(resolvers, pointResolver)
	}

	return resolvers, nil
}

// This will make sure that no two snapshots are too close together. We'll use 20% of the time interval to
// remove these "close" points.
func removeClosePoints(points []store.SeriesPoint, series types.InsightViewSeries) []store.SeriesPoint {
	buffer := intervalToMinutes(types.IntervalUnit(series.SampleIntervalUnit), series.SampleIntervalValue) / 5
	modifiedPoints := []store.SeriesPoint{}
	for i := 0; i < len(points)-1; i++ {
		modifiedPoints = append(modifiedPoints, points[i])
		if points[i+1].Time.Sub(points[i].Time).Minutes() < buffer {
			i++
		}
	}
	// Always add the very last snapshot point if it exists
	if len(points) > 0 {
		return append(modifiedPoints, points[len(points)-1])
	}
	return modifiedPoints
}

// This only needs to be approximate to calculate a comfortable buffer in which to remove points
func intervalToMinutes(unit types.IntervalUnit, value int) float64 {
	switch unit {
	case types.Day:
		return time.Hour.Minutes() * 24 * float64(value)
	case types.Week:
		return time.Hour.Minutes() * 24 * 7 * float64(value)
	case types.Month:
		return time.Hour.Minutes() * 24 * 30 * float64(value)
	case types.Year:
		return time.Hour.Minutes() * 24 * 365 * float64(value)
	default:
		// By default return the smallest interval (an hour)
		return time.Hour.Minutes() * float64(value)
	}
}

func (p *precalculatedInsightSeriesResolver) Status(ctx context.Context) (graphqlbackend.InsightStatusResolver, error) {
	return p.statusResolver, nil
}

type insightSeriesResolverGenerator interface {
	Generate(ctx context.Context, series types.InsightViewSeries, baseResolver baseInsightResolver, filters types.InsightViewFilters, options types.SeriesDisplayOptions) ([]graphqlbackend.InsightSeriesResolver, error)
	handles(series types.InsightViewSeries) bool
	SetNext(nextGenerator insightSeriesResolverGenerator)
}

type (
	handleSeriesFunc  func(series types.InsightViewSeries) bool
	resolverGenerator func(ctx context.Context, series types.InsightViewSeries, baseResolver baseInsightResolver, filters types.InsightViewFilters, options types.SeriesDisplayOptions) ([]graphqlbackend.InsightSeriesResolver, error)
)

type seriesResolverGenerator struct {
	next             insightSeriesResolverGenerator
	handlesSeries    handleSeriesFunc
	generateResolver resolverGenerator
}

func (j *seriesResolverGenerator) handles(series types.InsightViewSeries) bool {
	if j.handlesSeries == nil {
		return false
	}
	return j.handlesSeries(series)
}

func (j *seriesResolverGenerator) SetNext(nextGenerator insightSeriesResolverGenerator) {
	j.next = nextGenerator
}

func (j *seriesResolverGenerator) Generate(ctx context.Context, series types.InsightViewSeries, baseResolver baseInsightResolver, filters types.InsightViewFilters, options types.SeriesDisplayOptions) ([]graphqlbackend.InsightSeriesResolver, error) {
	if j.handles(series) {
		return j.generateResolver(ctx, series, baseResolver, filters, options)
	}
	if j.next != nil {
		return j.next.Generate(ctx, series, baseResolver, filters, options)
	} else {
		return nil, errors.Newf("no resolvers for insights series with ID %s", series.SeriesID)
	}
}

func newSeriesResolverGenerator(handles handleSeriesFunc, generate resolverGenerator) insightSeriesResolverGenerator {
	return &seriesResolverGenerator{
		handlesSeries:    handles,
		generateResolver: generate,
	}
}

func getRecordedSeriesPointOpts(ctx context.Context, db database.DB, timeseriesStore *store.Store, definition types.InsightViewSeries, filters types.InsightViewFilters, options types.SeriesDisplayOptions) (*store.SeriesPointsOpts, error) {
	opts := &store.SeriesPointsOpts{}
	// Query data points only for the series we are representing.
	seriesID := definition.SeriesID
	opts.SeriesID = &seriesID
	opts.ID = &definition.InsightSeriesID
	opts.SupportsAugmentation = definition.SupportsAugmentation

	// by this point the numSamples option should be set correctly but we're reusing the same struct across functions
	// so set max again.
	numSamples := 90
	if options.NumSamples != nil && *options.NumSamples < 90 && *options.NumSamples > 0 {
		numSamples = int(*options.NumSamples)
	}
	oldest, err := timeseriesStore.GetOffsetNRecordingTime(ctx, definition.InsightSeriesID, numSamples, false)
	if err != nil {
		return nil, errors.Wrap(err, "GetOffsetNRecordingTime")
	}
	if !oldest.IsZero() {
		opts.After = &oldest
	}

	includeRepo := func(regex ...string) {
		opts.IncludeRepoRegex = append(opts.IncludeRepoRegex, regex...)
	}
	excludeRepo := func(regex ...string) {
		opts.ExcludeRepoRegex = append(opts.ExcludeRepoRegex, regex...)
	}

	if filters.IncludeRepoRegex != nil {
		includeRepo(*filters.IncludeRepoRegex)
	}
	if filters.ExcludeRepoRegex != nil {
		excludeRepo(*filters.ExcludeRepoRegex)
	}

	scHandler := store.NewSearchContextHandler(db)
	inc, exc, err := scHandler.UnwrapSearchContexts(ctx, filters.SearchContexts)
	if err != nil {
		return nil, errors.Wrap(err, "unwrapSearchContexts")
	}
	includeRepo(inc...)
	excludeRepo(exc...)
	return opts, nil
}

var loadingStrategyRED = metrics.NewREDMetrics(prometheus.DefaultRegisterer, "src_insights_loading_strategy", metrics.WithLabels("in_mem", "capture"))

func fetchSeries(ctx context.Context, definition types.InsightViewSeries, filters types.InsightViewFilters, options types.SeriesDisplayOptions, r *baseInsightResolver) (points []store.SeriesPoint, err error) {
	opts, err := getRecordedSeriesPointOpts(ctx, database.NewDBWith(log.Scoped("recordedSeries"), r.postgresDB), r.timeSeriesStore, definition, filters, options)
	if err != nil {
		return nil, errors.Wrap(err, "getRecordedSeriesPointOpts")
	}

	getAltFlag := func() bool {
		ex := conf.Get().ExperimentalFeatures
		if ex == nil {
			return false
		}
		return ex.InsightsAlternateLoadingStrategy
	}
	alternativeLoadingStrategy := getAltFlag()

	var start, end time.Time
	start = time.Now()
	if !alternativeLoadingStrategy {
		points, err = r.timeSeriesStore.SeriesPoints(ctx, *opts)
		if err != nil {
			return nil, err
		}
	} else {
		points, err = r.timeSeriesStore.LoadSeriesInMem(ctx, *opts)
		if err != nil {
			return nil, err
		}
		sort.Slice(points, func(i, j int) bool {
			return points[i].Time.Before(points[j].Time)
		})
	}
	end = time.Now()
	loadingStrategyRED.Observe(end.Sub(start).Seconds(), 1, &err, strconv.FormatBool(alternativeLoadingStrategy), strconv.FormatBool(definition.GeneratedFromCaptureGroups))

	return points, err
}

func recordedSeries(ctx context.Context, definition types.InsightViewSeries, r baseInsightResolver, filters types.InsightViewFilters, options types.SeriesDisplayOptions) (_ []graphqlbackend.InsightSeriesResolver, err error) {
	points, err := fetchSeries(ctx, definition, filters, options, &r)
	if err != nil {
		return nil, err
	}

	statusResolver := NewStatusResolver(&r, definition)

	var resolvers []graphqlbackend.InsightSeriesResolver

	resolvers = append(resolvers, &precalculatedInsightSeriesResolver{
		insightsStore:   r.timeSeriesStore,
		workerBaseStore: r.workerBaseStore,
		series:          definition,
		metadataStore:   r.insightStore,
		points:          points,
		label:           definition.Label,
		filters:         filters,
		seriesId:        definition.SeriesID,
		statusResolver:  statusResolver,
	})
	return resolvers, nil
}

func expandCaptureGroupSeriesRecorded(ctx context.Context, definition types.InsightViewSeries, r baseInsightResolver, filters types.InsightViewFilters, options types.SeriesDisplayOptions) ([]graphqlbackend.InsightSeriesResolver, error) {
	allPoints, err := fetchSeries(ctx, definition, filters, options, &r)
	if err != nil {
		return nil, err
	}
	groupedByCapture := make(map[string][]store.SeriesPoint)

	for i := range allPoints {
		point := allPoints[i]
		if point.Capture == nil {
			// skip nil values, this shouldn't be a real possibility
			continue
		}
		groupedByCapture[*point.Capture] = append(groupedByCapture[*point.Capture], point)
	}

	statusResolver := NewStatusResolver(&r, definition)

	var resolvers []graphqlbackend.InsightSeriesResolver
	for capturedValue, points := range groupedByCapture {
		sort.Slice(points, func(i, j int) bool {
			return points[i].Time.Before(points[j].Time)
		})
		resolvers = append(resolvers, &precalculatedInsightSeriesResolver{
			insightsStore:   r.timeSeriesStore,
			workerBaseStore: r.workerBaseStore,
			series:          definition,
			metadataStore:   r.insightStore,
			points:          points,
			label:           capturedValue,
			filters:         filters,
			seriesId:        fmt.Sprintf("%s-%s", definition.SeriesID, capturedValue),
			statusResolver:  statusResolver,
		})
	}
	if len(resolvers) == 0 {
		// We are manually populating a mostly empty resolver here - this slightly hacky solution is to unify the
		// expectations of the webapp when querying for series state. For a standard search series there is
		// always a resolver since each series maps one to one with it's definition.
		// With a capture groups series we derive each unique series dynamically - which means it's possible to have a
		// series definition with zero resulting series. This most commonly occurs when the insight is just created,
		// before any data has been generated yet. Without this,
		// our capture groups insights don't share the loading state behavior.
		resolvers = append(resolvers, &precalculatedInsightSeriesResolver{
			insightsStore:   r.timeSeriesStore,
			workerBaseStore: r.workerBaseStore,
			series:          definition,
			metadataStore:   r.insightStore,
			statusResolver:  statusResolver,
			seriesId:        definition.SeriesID,
			points:          nil,
			label:           definition.Label,
			filters:         filters,
		})
	}
	return resolvers, nil
}

var (
	_ graphqlbackend.TimeoutDatapointAlert           = &timeoutDatapointAlertResolver{}
	_ graphqlbackend.GenericIncompleteDatapointAlert = &genericIncompleteDatapointAlertResolver{}
	_ graphqlbackend.IncompleteDatapointAlert        = &IncompleteDataPointAlertResolver{}
)

type IncompleteDataPointAlertResolver struct {
	point store.IncompleteDatapoint
}

func (i *IncompleteDataPointAlertResolver) ToTimeoutDatapointAlert() (graphqlbackend.TimeoutDatapointAlert, bool) {
	if i.point.Reason == store.ReasonTimeout {
		return &timeoutDatapointAlertResolver{point: i.point}, true
	}
	return nil, false
}

func (i *IncompleteDataPointAlertResolver) ToGenericIncompleteDatapointAlert() (graphqlbackend.GenericIncompleteDatapointAlert, bool) {
	switch i.point.Reason {
	case store.ReasonTimeout:
		return nil, false
	}
	return &genericIncompleteDatapointAlertResolver{point: i.point}, true
}

func (i *IncompleteDataPointAlertResolver) Time() gqlutil.DateTime {
	return gqlutil.DateTime{Time: i.point.Time}
}

type timeoutDatapointAlertResolver struct {
	point store.IncompleteDatapoint
}

func (t *timeoutDatapointAlertResolver) Time() gqlutil.DateTime {
	return gqlutil.DateTime{Time: t.point.Time}
}

type genericIncompleteDatapointAlertResolver struct {
	point store.IncompleteDatapoint
}

func (g *genericIncompleteDatapointAlertResolver) Time() gqlutil.DateTime {
	return gqlutil.DateTime{Time: g.point.Time}
}

func (g *genericIncompleteDatapointAlertResolver) Reason() string {
	switch g.point.Reason {
	default:
		return "There was an issue during data processing that caused this point to be incomplete."
	}
}

func (i *insightStatusResolver) IncompleteDatapoints(ctx context.Context) (resolvers []graphqlbackend.IncompleteDatapointAlert, err error) {
	incomplete, err := i.getIncompleteDatapoints(ctx, i.series.InsightSeriesID)
	for _, reason := range incomplete {
		resolvers = append(resolvers, &IncompleteDataPointAlertResolver{point: reason})
	}

	return resolvers, err
}

func isNilOrEmpty(s *string) bool {
	if s == nil {
		return true
	}
	return *s == ""
}
