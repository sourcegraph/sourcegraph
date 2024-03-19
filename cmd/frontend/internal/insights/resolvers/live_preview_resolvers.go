package resolvers

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/insights/query"
	"github.com/sourcegraph/sourcegraph/internal/insights/query/querybuilder"
	"github.com/sourcegraph/sourcegraph/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/insights/timeseries"
	"github.com/sourcegraph/sourcegraph/internal/insights/types"
	searchquery "github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const maxPreviewRepos = 20

func (r *Resolver) SearchInsightLivePreview(ctx context.Context, args graphqlbackend.SearchInsightLivePreviewArgs) ([]graphqlbackend.SearchInsightLivePreviewSeriesResolver, error) {
	if !args.Input.GeneratedFromCaptureGroups {
		return nil, errors.New("live preview is currently only supported for generated series from capture groups")
	}
	previewArgs := graphqlbackend.SearchInsightPreviewArgs{
		Input: graphqlbackend.SearchInsightPreviewInput{
			RepositoryScope: args.Input.RepositoryScope,
			TimeScope:       args.Input.TimeScope,
			Series: []graphqlbackend.SearchSeriesPreviewInput{
				{
					Query:                      args.Input.Query,
					Label:                      args.Input.Label,
					GeneratedFromCaptureGroups: args.Input.GeneratedFromCaptureGroups,
					GroupBy:                    args.Input.GroupBy,
				},
			},
		},
	}
	return r.SearchInsightPreview(ctx, previewArgs)
}

func (r *Resolver) SearchInsightPreview(ctx context.Context, args graphqlbackend.SearchInsightPreviewArgs) ([]graphqlbackend.SearchInsightLivePreviewSeriesResolver, error) {

	err := isValidPreviewArgs(args)
	if err != nil {
		return nil, err
	}

	var resolvers []graphqlbackend.SearchInsightLivePreviewSeriesResolver

	// get a consistent time to use across all preview series
	previewTime := time.Now().UTC()
	clock := func() time.Time {
		return previewTime
	}
	interval := timeseries.TimeInterval{
		Unit:  types.IntervalUnit(args.Input.TimeScope.StepInterval.Unit),
		Value: int(args.Input.TimeScope.StepInterval.Value),
	}

	repos, err := getPreviewRepos(ctx, args.Input.RepositoryScope, r.logger)
	if err != nil {
		return nil, err
	}
	if len(repos) > maxPreviewRepos {
		return nil, &livePreviewError{Code: repoLimitExceededErrorCode, Message: fmt.Sprintf("live preview is limited to %d repositories", maxPreviewRepos)}
	}
	foundData := false
	for _, seriesArgs := range args.Input.Series {

		var series []query.GeneratedTimeSeries
		var err error
		if seriesArgs.GeneratedFromCaptureGroups {
			if seriesArgs.GroupBy != nil {
				executor := query.NewComputeExecutor(r.postgresDB, clock)
				series, err = executor.Execute(ctx, seriesArgs.Query, *seriesArgs.GroupBy, repos)
				if err != nil {
					return nil, err
				}
			} else {
				executor := query.NewCaptureGroupExecutor(r.postgresDB, clock)
				series, err = executor.Execute(ctx, seriesArgs.Query, repos, interval)
				if err != nil {
					return nil, err
				}
			}
		} else {
			executor := query.NewStreamingExecutor(r.postgresDB, clock)
			series, err = executor.Execute(ctx, seriesArgs.Query, seriesArgs.Label, seriesArgs.Label, repos, interval)
			if err != nil {
				return nil, err
			}
		}
		for i := range series {
			foundData = foundData || len(series[i].Points) > 0
			// Replacing capture group values if present
			// Ignoring errors so it falls back to the entered query
			seriesQuery := seriesArgs.Query
			if seriesArgs.GeneratedFromCaptureGroups && len(series[i].Points) > 0 {
				replacer, _ := querybuilder.NewPatternReplacer(querybuilder.BasicQuery(seriesQuery), searchquery.SearchTypeRegex)
				if replacer != nil {
					replaced, err := replacer.Replace(series[i].Label)
					if err == nil {
						seriesQuery = replaced.String()
					}
				}
			}
			resolvers = append(resolvers, &searchInsightLivePreviewSeriesResolver{
				series:      &series[i],
				repoList:    args.Input.RepositoryScope.Repositories,
				repoSearch:  args.Input.RepositoryScope.RepositoryCriteria,
				searchQuery: seriesQuery,
			})
		}
	}

	if !foundData {
		return nil, &livePreviewError{Code: noDataErrorCode, Message: fmt.Sprintf("Data for %s not found", pluralize("this repository", "these repositories", len(repos)))}
	}

	return resolvers, nil
}

func pluralize(singular, plural string, n int) string {
	if n == 1 {
		return singular
	}
	return plural
}

type searchInsightLivePreviewSeriesResolver struct {
	series      *query.GeneratedTimeSeries
	repoList    []string
	repoSearch  *string
	searchQuery string
}

func (s *searchInsightLivePreviewSeriesResolver) Points(ctx context.Context) ([]graphqlbackend.InsightsDataPointResolver, error) {
	var resolvers []graphqlbackend.InsightsDataPointResolver
	for i := range len(s.series.Points) {
		point := store.SeriesPoint{
			SeriesID: s.series.SeriesId,
			Time:     s.series.Points[i].Time,
			Value:    float64(s.series.Points[i].Count),
		}
		var after *time.Time
		if i > 0 {
			after = &s.series.Points[i-1].Time
		}
		pointResolver := &insightsDataPointResolver{
			p: point,
			diffInfo: &querybuilder.PointDiffQueryOpts{
				After:       after,
				Before:      point.Time,
				RepoList:    s.repoList,
				RepoSearch:  s.repoSearch,
				SearchQuery: querybuilder.BasicQuery(s.searchQuery),
			}}
		resolvers = append(resolvers, pointResolver)
	}

	return resolvers, nil
}

func (s *searchInsightLivePreviewSeriesResolver) Label(ctx context.Context) (string, error) {
	return s.series.Label, nil
}

func getPreviewRepos(ctx context.Context, repoScope graphqlbackend.RepositoryScopeInput, logger log.Logger) ([]string, error) {
	var repos []string
	if repoScope.RepositoryCriteria != nil {
		repoQueryExecutor := query.NewStreamingRepoQueryExecutor(logger.Scoped("live_preview_resolver"))
		repoQuery, err := querybuilder.RepositoryScopeQuery(*repoScope.RepositoryCriteria)
		if err != nil {
			return nil, err
		}
		// Since preview is not allowed over "max_preview_repos" limit result set to avoid processing more results than neccessary
		limitedRepoQuery, err := repoQuery.WithCount(fmt.Sprintf("%d", maxPreviewRepos+1))
		if err != nil {
			return nil, err
		}
		repoList, err := repoQueryExecutor.ExecuteRepoList(ctx, string(limitedRepoQuery))
		if err != nil {
			return nil, err
		}
		for i := range len(repoList) {
			repos = append(repos, string(repoList[i].Name))
		}
	} else {
		repos = repoScope.Repositories
	}
	return repos, nil
}

func isValidPreviewArgs(args graphqlbackend.SearchInsightPreviewArgs) error {
	if args.Input.TimeScope.StepInterval == nil {
		return &livePreviewError{Code: invalidArgsErrorCode, Message: "live preview currently only supports a time interval time scope"}
	}
	hasRepoCriteria := args.Input.RepositoryScope.RepositoryCriteria != nil
	// Error if both are provided
	if hasRepoCriteria && len(args.Input.RepositoryScope.Repositories) > 0 {
		return &livePreviewError{Code: invalidArgsErrorCode, Message: "can not specify both a repository list and a repository search"}
	}

	if hasRepoCriteria {
		for i := range len(args.Input.Series) {
			if args.Input.Series[i].GroupBy != nil {
				return &livePreviewError{Code: invalidArgsErrorCode, Message: "group by insights do not support selecting repositories using a search"}
			}
		}
	}

	return nil
}

const repoLimitExceededErrorCode = "RepoLimitExceeded"
const noDataErrorCode = "NoData"
const invalidArgsErrorCode = "InvalidArgs"

type livePreviewError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e livePreviewError) Error() string {
	return e.Message
}

func (e livePreviewError) Extensions() map[string]interface{} {
	return map[string]interface{}{
		"code": e.Code,
	}
}
