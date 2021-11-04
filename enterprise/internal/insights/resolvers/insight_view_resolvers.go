package resolvers

import (
	"context"
	"sync"
	"time"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/internal/database"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"

	"github.com/cockroachdb/errors"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/actor"

	"github.com/segmentio/ksuid"

	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/types"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
)

var _ graphqlbackend.InsightViewResolver = &insightViewResolver{}
var _ graphqlbackend.LineChartInsightViewPresentation = &lineChartInsightViewPresentation{}
var _ graphqlbackend.LineChartDataSeriesPresentationResolver = &lineChartDataSeriesPresentationResolver{}
var _ graphqlbackend.SearchInsightDataSeriesDefinitionResolver = &searchInsightDataSeriesDefinitionResolver{}
var _ graphqlbackend.InsightRepositoryScopeResolver = &insightRepositoryScopeResolver{}
var _ graphqlbackend.InsightIntervalTimeScope = &insightIntervalTimeScopeResolver{}
var _ graphqlbackend.InsightViewFiltersResolver = &insightViewFiltersResolver{}
var _ graphqlbackend.InsightViewPayloadResolver = &insightPayloadResolver{}
var _ graphqlbackend.InsightTimeScope = &insightTimeScopeUnionResolver{}
var _ graphqlbackend.InsightPresentation = &insightPresentationUnionResolver{}
var _ graphqlbackend.InsightDataSeriesDefinition = &insightDataSeriesDefinitionUnionResolver{}
var _ graphqlbackend.InsightViewConnectionResolver = &InsightViewQueryConnectionResolver{}

type insightViewResolver struct {
	view *types.Insight

	baseInsightResolver
}

const insightKind = "insight_view"

func (i *insightViewResolver) ID() graphql.ID {
	return relay.MarshalID(insightKind, i.view.UniqueID)
}

func (i *insightViewResolver) DefaultFilters(ctx context.Context) (graphqlbackend.InsightViewFiltersResolver, error) {
	return &insightViewFiltersResolver{view: i.view}, nil
}

type insightViewFiltersResolver struct {
	view *types.Insight
}

func (i *insightViewFiltersResolver) IncludeRepoRegex(ctx context.Context) (*string, error) {
	return i.view.Filters.IncludeRepoRegex, nil
}

func (i *insightViewFiltersResolver) ExcludeRepoRegex(ctx context.Context) (*string, error) {
	return i.view.Filters.ExcludeRepoRegex, nil
}

func (i *insightViewResolver) AppliedFilters(ctx context.Context) (graphqlbackend.InsightViewFiltersResolver, error) {
	panic("implement me")
}

func (i *insightViewResolver) DataSeries(ctx context.Context) ([]graphqlbackend.InsightSeriesResolver, error) {
	var resolvers []graphqlbackend.InsightSeriesResolver
	for j := range i.view.Series {
		resolvers = append(resolvers, &insightSeriesResolver{
			insightsStore:   i.timeSeriesStore,
			workerBaseStore: i.workerBaseStore,
			series:          i.view.Series[j],
			metadataStore:   i.insightStore,
		})
	}

	return resolvers, nil
}

func (i *insightViewResolver) Presentation(ctx context.Context) (graphqlbackend.InsightPresentation, error) {
	lineChartPresentation := &lineChartInsightViewPresentation{view: i.view}

	return &insightPresentationUnionResolver{resolver: lineChartPresentation}, nil
}

func (i *insightViewResolver) DataSeriesDefinitions(ctx context.Context) ([]graphqlbackend.InsightDataSeriesDefinition, error) {
	var resolvers []graphqlbackend.InsightDataSeriesDefinition
	for j := range i.view.Series {
		resolvers = append(resolvers, &insightDataSeriesDefinitionUnionResolver{resolver: &searchInsightDataSeriesDefinitionResolver{series: &i.view.Series[j]}})
	}
	return resolvers, nil
}

type searchInsightDataSeriesDefinitionResolver struct {
	series *types.InsightViewSeries
}

func (s *searchInsightDataSeriesDefinitionResolver) SeriesId(ctx context.Context) (string, error) {
	return s.series.SeriesID, nil
}

func (s *searchInsightDataSeriesDefinitionResolver) Query(ctx context.Context) (string, error) {
	return s.series.Query, nil
}

func (s *searchInsightDataSeriesDefinitionResolver) RepositoryScope(ctx context.Context) (graphqlbackend.InsightRepositoryScopeResolver, error) {
	return &insightRepositoryScopeResolver{repositories: s.series.Repositories}, nil
}

func (s *searchInsightDataSeriesDefinitionResolver) TimeScope(ctx context.Context) (graphqlbackend.InsightTimeScope, error) {
	intervalResolver := &insightIntervalTimeScopeResolver{
		unit:  s.series.SampleIntervalUnit,
		value: int32(s.series.SampleIntervalValue),
	}

	return &insightTimeScopeUnionResolver{resolver: intervalResolver}, nil
}

type insightIntervalTimeScopeResolver struct {
	unit  string
	value int32
}

func (i *insightIntervalTimeScopeResolver) Unit(ctx context.Context) (string, error) {
	return i.unit, nil
}

func (i *insightIntervalTimeScopeResolver) Value(ctx context.Context) (int32, error) {
	return i.value, nil
}

type insightRepositoryScopeResolver struct {
	repositories []string
}

func (i *insightRepositoryScopeResolver) Repositories(ctx context.Context) ([]string, error) {
	return i.repositories, nil
}

type lineChartInsightViewPresentation struct {
	view *types.Insight
}

func (l *lineChartInsightViewPresentation) Title(ctx context.Context) (string, error) {
	return l.view.Title, nil
}

func (l *lineChartInsightViewPresentation) SeriesPresentation(ctx context.Context) ([]graphqlbackend.LineChartDataSeriesPresentationResolver, error) {
	var resolvers []graphqlbackend.LineChartDataSeriesPresentationResolver

	for i := range l.view.Series {
		resolvers = append(resolvers, &lineChartDataSeriesPresentationResolver{series: &l.view.Series[i]})
	}

	return resolvers, nil
}

type lineChartDataSeriesPresentationResolver struct {
	series *types.InsightViewSeries
}

func (l *lineChartDataSeriesPresentationResolver) SeriesId(ctx context.Context) (string, error) {
	return l.series.SeriesID, nil
}

func (l *lineChartDataSeriesPresentationResolver) Label(ctx context.Context) (string, error) {
	return l.series.Label, nil
}

func (l *lineChartDataSeriesPresentationResolver) Color(ctx context.Context) (string, error) {
	return l.series.LineColor, nil
}

func (r *Resolver) CreateLineChartSearchInsight(ctx context.Context, args *graphqlbackend.CreateLineChartSearchInsightArgs) (_ graphqlbackend.InsightViewPayloadResolver, err error) {
	uid := actor.FromContext(ctx).UID

	tx, err := r.insightStore.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	view, err := tx.CreateView(ctx, types.InsightView{
		Title:    emptyIfNil(args.Input.Options.Title),
		UniqueID: ksuid.New().String(),
		Filters:  types.InsightViewFilters{},
	}, []store.InsightViewGrant{store.UserGrant(int(uid))})
	if err != nil {
		return nil, errors.Wrap(err, "CreateView")
	}

	for _, series := range args.Input.DataSeries {
		err = createAndAttachSeries(ctx, tx, view, series)
		if err != nil {
			return nil, errors.Wrap(err, "createAndAttachSeries")
		}
	}
	return &insightPayloadResolver{baseInsightResolver: r.baseInsightResolver, viewId: view.UniqueID}, nil
}

func (r *Resolver) UpdateLineChartSearchInsight(ctx context.Context, args *graphqlbackend.UpdateLineChartSearchInsightArgs) (_ graphqlbackend.InsightViewPayloadResolver, err error) {
	tx, err := r.insightStore.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	var insightViewId string
	err = relay.UnmarshalSpec(args.Id, &insightViewId)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling the insight view id")
	}

	// TODO: Check permissions #25971

	views, err := tx.GetMapped(ctx, store.InsightQueryArgs{UniqueID: insightViewId, WithoutAuthorization: true})
	if err != nil {
		return nil, errors.Wrap(err, "GetMapped")
	}
	if len(views) == 0 {
		return nil, errors.New("No insight view found with this id")
	}

	view, err := tx.UpdateView(ctx, types.InsightView{
		UniqueID: insightViewId,
		Title:    emptyIfNil(args.Input.PresentationOptions.Title),
		Filters: types.InsightViewFilters{
			IncludeRepoRegex: args.Input.ViewControls.Filters.IncludeRepoRegex,
			ExcludeRepoRegex: args.Input.ViewControls.Filters.ExcludeRepoRegex},
	})
	if err != nil {
		return nil, errors.Wrap(err, "UpdateView")
	}

	for _, existingSeries := range views[0].Series {
		if !seriesFound(existingSeries, args.Input.DataSeries) {
			err = tx.RemoveSeriesFromView(ctx, existingSeries.SeriesID, view.ID)
			if err != nil {
				return nil, errors.Wrap(err, "RemoveSeriesFromView")
			}
		}
	}

	for _, series := range args.Input.DataSeries {
		if series.SeriesId == nil {
			err = createAndAttachSeries(ctx, tx, view, series)
			if err != nil {
				return nil, errors.Wrap(err, "createAndAttachSeries")
			}
		} else {
			// If it's a frontend series, we can just update it.
			existingRepos := getExistingSeriesRepositories(*series.SeriesId, views[0].Series)
			if len(series.RepositoryScope.Repositories) > 0 && len(existingRepos) > 0 {
				err = tx.UpdateFrontendSeries(ctx, store.UpdateFrontendSeriesArgs{
					SeriesID:          *series.SeriesId,
					Query:             series.Query,
					Repositories:      series.RepositoryScope.Repositories,
					StepIntervalUnit:  series.TimeScope.StepInterval.Unit,
					StepIntervalValue: int(series.TimeScope.StepInterval.Value),
				})
				if err != nil {
					return nil, errors.Wrap(err, "UpdateFrontendSeries")
				}
			} else {
				err = tx.RemoveSeriesFromView(ctx, *series.SeriesId, view.ID)
				if err != nil {
					return nil, errors.Wrap(err, "RemoveViewSeries")
				}
				err = createAndAttachSeries(ctx, tx, view, series)
				if err != nil {
					return nil, errors.Wrap(err, "createAndAttachSeries")
				}
			}

			err = tx.UpdateViewSeries(ctx, *series.SeriesId, view.ID, types.InsightViewSeriesMetadata{
				Label:  emptyIfNil(series.Options.Label),
				Stroke: emptyIfNil(series.Options.LineColor),
			})
			if err != nil {
				return nil, errors.Wrap(err, "UpdateViewSeries")
			}
		}
	}
	return &insightPayloadResolver{baseInsightResolver: r.baseInsightResolver, viewId: insightViewId}, nil
}

type insightPayloadResolver struct {
	viewId string
	baseInsightResolver
}

func (c *insightPayloadResolver) View(ctx context.Context) (graphqlbackend.InsightViewResolver, error) {
	mapped, err := c.insightStore.GetMapped(ctx, store.InsightQueryArgs{UniqueID: c.viewId, UserID: []int{int(actor.FromContext(ctx).UID)}})
	if err != nil {
		return nil, err
	}
	if len(mapped) < 1 {
		return nil, err
	}
	return &insightViewResolver{view: &mapped[0], baseInsightResolver: c.baseInsightResolver}, nil
}

func emptyIfNil(in *string) string {
	if in == nil {
		return ""
	}
	return *in
}

// A dummy type to represent the GraphQL union InsightTimeScope
type insightTimeScopeUnionResolver struct {
	resolver interface{}
}

// ToInsightIntervalTimeScope is used by the GraphQL library to resolve type fragments for unions
func (r *insightTimeScopeUnionResolver) ToInsightIntervalTimeScope() (graphqlbackend.InsightIntervalTimeScope, bool) {
	res, ok := r.resolver.(*insightIntervalTimeScopeResolver)
	return res, ok
}

// A dummy type to represent the GraphQL union InsightPresentation
type insightPresentationUnionResolver struct {
	resolver interface{}
}

// ToLineChartInsightViewPresentation is used by the GraphQL library to resolve type fragments for unions
func (r *insightPresentationUnionResolver) ToLineChartInsightViewPresentation() (graphqlbackend.LineChartInsightViewPresentation, bool) {
	res, ok := r.resolver.(*lineChartInsightViewPresentation)
	return res, ok
}

// A dummy type to represent the GraphQL union InsightDataSeriesDefinition
type insightDataSeriesDefinitionUnionResolver struct {
	resolver interface{}
}

// ToSearchInsightDataSeriesDefinition is used by the GraphQL library to resolve type fragments for unions
func (r *insightDataSeriesDefinitionUnionResolver) ToSearchInsightDataSeriesDefinition() (graphqlbackend.SearchInsightDataSeriesDefinitionResolver, bool) {
	res, ok := r.resolver.(*searchInsightDataSeriesDefinitionResolver)
	return res, ok
}

func (r *Resolver) InsightViews(ctx context.Context, args *graphqlbackend.InsightViewQueryArgs) (graphqlbackend.InsightViewConnectionResolver, error) {
	return &InsightViewQueryConnectionResolver{
		baseInsightResolver: r.baseInsightResolver,
		args:                args,
	}, nil
}

type InsightViewQueryConnectionResolver struct {
	baseInsightResolver

	args *graphqlbackend.InsightViewQueryArgs

	// Cache results because they are used by multiple fields
	once  sync.Once
	views []types.Insight
	next  string
	err   error
}

func (d *InsightViewQueryConnectionResolver) Nodes(ctx context.Context) ([]graphqlbackend.InsightViewResolver, error) {
	resolvers := make([]graphqlbackend.InsightViewResolver, 0)

	views, _, err := d.computeViews(ctx)
	if err != nil {
		return nil, err
	}
	for i := range views {
		resolvers = append(resolvers, &insightViewResolver{view: &views[i], baseInsightResolver: d.baseInsightResolver})
	}
	return resolvers, nil
}

func (d *InsightViewQueryConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	_, next, err := d.computeViews(ctx)
	if err != nil {
		return nil, err
	}

	if next != "" {
		return graphqlutil.NextPageCursor(string(relay.MarshalID(insightKind, d.next))), nil
	}
	return graphqlutil.HasNextPage(false), nil
}

func (r *InsightViewQueryConnectionResolver) computeViews(ctx context.Context) ([]types.Insight, string, error) {
	r.once.Do(func() {
		orgStore := database.Orgs(r.postgresDB)

		args := store.InsightQueryArgs{}
		if r.args.After != nil {
			var afterID string
			err := relay.UnmarshalSpec(graphql.ID(*r.args.After), &afterID)
			if err != nil {
				r.err = errors.Wrap(err, "unmarshalID")
				return
			}
			args.After = afterID
		}
		if r.args.First != nil {
			args.Limit = int(*r.args.First)
		}
		var err error
		args.UserID, args.OrgID, err = getUserPermissions(ctx, orgStore)
		if err != nil {
			r.err = errors.Wrap(err, "getUserPermissions")
			return
		}

		if r.args.Id != nil {
			var unique string
			r.err = relay.UnmarshalSpec(*r.args.Id, &unique)
			if r.err != nil {
				return
			}
			log15.Info("unique_id", "id", unique)
			args.UniqueID = unique
		}

		viewSeries, err := r.insightStore.GetAll(ctx, args)
		if err != nil {
			r.err = err
			return
		}

		r.views = r.insightStore.GroupByView(ctx, viewSeries)

		if len(r.views) > 0 {
			r.next = r.views[len(r.views)-1].UniqueID
		}
	})
	return r.views, r.next, r.err
}

func createAndAttachSeries(ctx context.Context, tx *store.InsightStore, view types.InsightView, series graphqlbackend.LineChartSearchInsightDataSeriesInput) error {
	var seriesToAdd types.InsightSeries
	var matchingSeries *types.InsightSeries
	var err error

	// Don't try to match on frontend series
	if len(series.RepositoryScope.Repositories) == 0 {
		matchingSeries, err = tx.FindMatchingSeries(ctx, store.MatchSeriesArgs{
			Query:             series.Query,
			StepIntervalUnit:  series.TimeScope.StepInterval.Unit,
			StepIntervalValue: int(series.TimeScope.StepInterval.Value)})
		if err != nil {
			return errors.Wrap(err, "FindMatchingSeries")
		}
	}

	if matchingSeries == nil {
		seriesToAdd, err = tx.CreateSeries(ctx, types.InsightSeries{
			SeriesID:            ksuid.New().String(),
			Query:               series.Query,
			CreatedAt:           time.Now(),
			Repositories:        series.RepositoryScope.Repositories,
			SampleIntervalUnit:  series.TimeScope.StepInterval.Unit,
			SampleIntervalValue: int(series.TimeScope.StepInterval.Value),
		})
		if err != nil {
			return errors.Wrap(err, "CreateSeries")
		}
	} else {
		seriesToAdd = *matchingSeries
	}

	// BUG: If the user tries to attach the same series (the same query and timescope) to an insight view multiple times,
	// this will fail because it violates the unique key constraint. This will be solved by: #26905
	// Alternately we could detect this and return an error?
	err = tx.AttachSeriesToView(ctx, seriesToAdd, view, types.InsightViewSeriesMetadata{
		Label:  emptyIfNil(series.Options.Label),
		Stroke: emptyIfNil(series.Options.LineColor),
	})
	if err != nil {
		return errors.Wrap(err, "AttachSeriesToView")
	}
	return nil
}

func seriesFound(existingSeries types.InsightViewSeries, inputSeries []graphqlbackend.LineChartSearchInsightDataSeriesInput) bool {
	for i := range inputSeries {
		if inputSeries[i].SeriesId == nil {
			continue
		}
		if existingSeries.SeriesID == *inputSeries[i].SeriesId {
			return true
		}
	}
	return false
}

func getExistingSeriesRepositories(seriesId string, existingSeries []types.InsightViewSeries) []string {
	for i := range existingSeries {
		if existingSeries[i].SeriesID == seriesId {
			return existingSeries[i].Repositories
		}
	}
	return nil
}
