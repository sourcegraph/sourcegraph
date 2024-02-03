package adminanalytics

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

type Custom struct {
	Ctx       context.Context
	DateRange string
	Grouping  string
	Events    []string
	DB        database.DB
	Cache     bool
}

// Custom:Users

func (c *Custom) Users() (*AnalyticsFetcher, error) {
	nodesQuery, summaryQuery, err := makeEventLogsQueries(c.DateRange, c.Grouping, c.Events)
	if err != nil {
		return nil, err
	}

	return &AnalyticsFetcher{
		db:           c.DB,
		dateRange:    c.DateRange,
		grouping:     c.Grouping,
		nodesQuery:   nodesQuery,
		summaryQuery: summaryQuery,
		group:        fmt.Sprintf("Custom:Users:[%s]", strings.Join(c.Events, "::")),
		cache:        c.Cache,
	}, nil
}

// Custom:UserActivity

func (c *Custom) UserActivity(args *struct {
	graphqlutil.ConnectionArgs
	After *string
}) (*analyticsUserActivePeriodsConnection, error) {
	opt := analyticsUserActivePeriodsListOptions{}

	args.ConnectionArgs.Set(&opt.LimitOffset)
	if args.After != nil && opt.LimitOffset != nil {
		cursor, err := strconv.ParseInt(*args.After, 10, 32)
		if err != nil {
			return nil, err
		}
		opt.LimitOffset.Offset = int(cursor)
	}

	return &analyticsUserActivePeriodsConnection{
		db:        c.DB,
		opt:       opt,
		dateRange: c.DateRange,
		events:    c.Events,
		grouping:  c.Grouping,
	}, nil
}
