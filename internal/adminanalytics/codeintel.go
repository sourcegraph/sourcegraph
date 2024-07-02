package adminanalytics

import (
	"context"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database"
)

type CodeIntel struct {
	Ctx       context.Context
	DateRange string
	Grouping  string
	DB        database.DB
	Cache     bool
}

func (s *CodeIntel) ReferenceClicks() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{"blob.findReferences.executed"})
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           s.DB,
		dateRange:    s.DateRange,
		grouping:     s.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        "CodeIntel:ReferenceClicks",
		cache:        s.Cache,
	}, nil
}

func (s *CodeIntel) DefinitionClicks() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{"blob.goToDefinition.preloaded.executed", "blob.goToDefinition.executed"})
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           s.DB,
		dateRange:    s.DateRange,
		grouping:     s.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        "CodeIntel:DefinitionClicks",
		cache:        s.Cache,
	}, nil
}

func (s *CodeIntel) InAppEvents() (*AnalyticsFetcher, error) {
	sourceCond := sqlf.Sprintf("source = 'server.web'")
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{"blob.goToDefinition.preloaded.executed", "blob.goToDefinition.executed", "blob.findReferences.executed"}, sourceCond)
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           s.DB,
		dateRange:    s.DateRange,
		grouping:     s.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        "CodeIntel:InAppEvents",
		cache:        s.Cache,
	}, nil
}

func (s *CodeIntel) CodeHostEvents() (*AnalyticsFetcher, error) {
	sourceCond := sqlf.Sprintf("source != 'server.web'")
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{"blob.goToDefinition.preloaded.executed", "blob.goToDefinition.executed", "blob.findReferences.executed"}, sourceCond)
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           s.DB,
		dateRange:    s.DateRange,
		grouping:     s.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        "CodeIntel:CodeHostEvents",
		cache:        s.Cache,
	}, nil
}

func (s *CodeIntel) SearchBasedEvents() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{
		"blob.codeintel.searchDefinitions",
		"blob.codeintel.searchDefinitions.xrepo",
		"blob.codeintel.searchReferences",
		"blob.codeintel.searchReferences.xrepo",
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
		group:        "CodeIntel:SearchBasedEvents",
		cache:        s.Cache,
	}, nil
}

func (s *CodeIntel) PreciseEvents() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{
		"blob.codeintel.lsifDefinitions",
		"blob.codeintel.lsifDefinitions.xrepo",
		"blob.codeintel.lsifReferences",
		"blob.codeintel.lsifReferences.xrepo",
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
		group:        "CodeIntel:PreciseEvents",
		cache:        s.Cache,
	}, nil
}

func (s *CodeIntel) CrossRepoEvents() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(s.DateRange, s.Grouping, []string{
		"blob.codeintel.searchDefinitions.xrepo",
		"blob.codeintel.searchReferences.xrepo",
		"blob.codeintel.lsifDefinitions.xrepo",
		"blob.codeintel.lsifReferences.xrepo",
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
		group:        "CodeIntel:CrossRepoEvents",
		cache:        s.Cache,
	}, nil
}

func (s *CodeIntel) CacheAll(ctx context.Context) error {
	fetcherBuilders := []func() (*AnalyticsFetcher, error){
		s.DefinitionClicks,
		s.ReferenceClicks,
		s.InAppEvents,
		s.CodeHostEvents,
		s.SearchBasedEvents,
		s.PreciseEvents,
		s.CrossRepoEvents,
	}

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
