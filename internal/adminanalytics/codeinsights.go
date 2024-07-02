package adminanalytics

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/database"
)

type CodeInsights struct {
	Ctx       context.Context
	DateRange string
	Grouping  string
	DB        database.DB
	Cache     bool
}

// Insights:Hovers

func (c *CodeInsights) InsightHovers() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(
		c.DateRange,
		c.Grouping,
		[]string{"insight.hover"},
	)
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           c.DB,
		dateRange:    c.DateRange,
		grouping:     c.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        "Insights:InsightHovers",
		cache:        c.Cache,
	}, nil
}

// Insights:DataPointClicks

func (c *CodeInsights) InsightDataPointClicks() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(
		c.DateRange,
		c.Grouping,
		[]string{"insight.dataPoint.click"},
	)
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           c.DB,
		dateRange:    c.DateRange,
		grouping:     c.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        "Insights:InsightDataPointClicks",
		cache:        c.Cache,
	}, nil
}

// Insights caching job entrypoint

func (c *CodeInsights) CacheAll(ctx context.Context) error {
	fetcherBuilders := []func() (*AnalyticsFetcher, error){c.InsightHovers, c.InsightDataPointClicks}
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
