package adminanalytics

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/database"
)

type Search struct {
	Ctx       context.Context
	DateRange string
	Grouping  string
	DB        database.DB
	Cache     bool
}

func (s *Search) Searches() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{"search.results.query"})
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           s.DB,
		dateRange:    s.DateRange,
		grouping:     s.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        "Search:Searches",
		cache:        s.Cache,
	}, nil
}

func (s *Search) ResultClicks() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{"search.result.area.click"})
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           s.DB,
		dateRange:    s.DateRange,
		grouping:     s.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        "Search:ResultClicked",
	}, nil
}

func (s *Search) FileViews() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{"blob.view", "repo.blob.view"})
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           s.DB,
		dateRange:    s.DateRange,
		grouping:     s.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        "Search:FileViews",
		cache:        s.Cache,
	}, nil
}

func (s *Search) FileOpens() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{
		"repo.goToCodeHost.click",
		"blob.openInEditor.click",
	})
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           s.DB,
		dateRange:    s.DateRange,
		grouping:     s.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        "Search:FileOpens",
		cache:        s.Cache,
	}, nil
}

func (s *Search) CodeCopied() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{"code.copy", "repo.blob.code.copy", "notebook.code.copy"})
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           s.DB,
		dateRange:    s.DateRange,
		grouping:     s.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        "Search:CodeCopied",
		cache:        s.Cache,
	}, nil
}

func (s *Search) CacheAll(ctx context.Context) error {
	fetcherBuilders := []func() (*AnalyticsFetcher, error){s.Searches, s.FileViews, s.FileOpens, s.ResultClicks, s.CodeCopied}
	for _, buildFetcher := range fetcherBuilders {
		fetcher, err := buildFetcher()
		if err != nil {
			return err
		}

		if _, err := fetcher.Nodes(ctx); err != nil {
			return err
		}

		if _, err := fetcher.Summary(ctx); err != nil {
			return err
		}
	}
	return nil
}
