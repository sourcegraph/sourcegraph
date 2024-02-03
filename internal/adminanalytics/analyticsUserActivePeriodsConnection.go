package adminanalytics

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

type analyticsUserActivePeriodsListOptions struct {
	*database.LimitOffset
}

type analyticsUserActivePeriodsConnection struct {
	db        database.DB
	opt       analyticsUserActivePeriodsListOptions
	dateRange string
	events    []string
	grouping  string

	// cache results because they are used by multiple fields
	once       sync.Once
	users      []*analyticsUserActivityNode
	totalCount int
	err        error
}

type analyticsUserActivePeriodNode struct {
	date  time.Time
	count float64
}

type analyticsUserActivityNode struct {
	userID          int32
	username        *string
	displayName     *string
	periods         []*analyticsUserActivePeriodNode
	totalEventCount int
}

type dbAnalyticsUserActivity struct {
	UserID          int32
	Username        *string
	DisplayName     *string
	EventsOverTime  []byte
	TotalEventCount int32
}

func (n *analyticsUserActivePeriodNode) Date() string { return n.date.Format(time.RFC3339) }

func (n *analyticsUserActivePeriodNode) Count() float64 { return n.count }

func (n *analyticsUserActivityNode) UserID() int32 { return n.userID }

func (n *analyticsUserActivityNode) UserName() *string { return n.username }

func (n *analyticsUserActivityNode) DisplayName() *string { return n.displayName }

func (n *analyticsUserActivityNode) Periods() []*analyticsUserActivePeriodNode { return n.periods }

func (n *analyticsUserActivityNode) TotalEventCount() int32 { return int32(n.totalEventCount) }

func (r *analyticsUserActivePeriodsConnection) Nodes(ctx context.Context) ([]*analyticsUserActivityNode, error) {
	nodes, _, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *analyticsUserActivePeriodsConnection) TotalCount(ctx context.Context) (int32, error) {
	_, count, err := r.compute(ctx)
	return int32(count), err
}

func (r *analyticsUserActivePeriodsConnection) compute(ctx context.Context) ([]*analyticsUserActivityNode, int, error) {
	r.once.Do(func() {
		nodesQuery, countQuery, err := makeUserActivePeriodsQueries(r.dateRange, r.grouping, r.events, r.opt)
		if err != nil {
			r.err = err
			return
		}

		rows, err := r.db.QueryContext(ctx, nodesQuery.Query(sqlf.PostgresBindVar), nodesQuery.Args()...)
		if err != nil {
			r.err = err
			return
		}
		defer rows.Close()

		nodes := []*analyticsUserActivityNode{}

		// prepare date range constants
		now := time.Now()
		to := now
		daysOffset := 1
		from, err := getFromDate(r.dateRange, now)
		if err != nil {
			r.err = err
			return
		}
		if r.grouping == Weekly {
			to = now.AddDate(0, 0, -int(now.Weekday())+1)     // monday of current week
			from = from.AddDate(0, 0, -int(from.Weekday())+1) // monday of original week
			daysOffset = 7
		}

		for rows.Next() {
			// first, scan the data into a temp flat var
			tmpNode := &dbAnalyticsUserActivity{}
			if err := rows.Scan(&tmpNode.UserID, &tmpNode.Username, &tmpNode.DisplayName, &tmpNode.EventsOverTime, &tmpNode.TotalEventCount); err != nil {
				r.err = err
				return
			}

			eventsOverTime := make(map[string]float64, 0)
			if err := json.Unmarshal(tmpNode.EventsOverTime, &eventsOverTime); err != nil {
				r.err = err
				return
			}

			node := &analyticsUserActivityNode{
				userID:          tmpNode.UserID,
				username:        tmpNode.Username,
				displayName:     tmpNode.DisplayName,
				totalEventCount: int(tmpNode.TotalEventCount),
				periods:         make([]*analyticsUserActivePeriodNode, 0),
			}

			// generate a periods array based on the date range requested.
			for date := from; date.Before(to) || date.Equal(to); date = date.AddDate(0, 0, daysOffset) {
				period := &analyticsUserActivePeriodNode{
					date:  bod(date),
					count: 0,
				}

				node.periods = append(node.periods, period)
			}

			// loop through node.periods and update the count.
			for _, period := range node.periods {
				for date, count := range eventsOverTime {
					if period.date.Format("2006-01-02T15:04:05") == date {
						period.count = count
						// Break out to next period
						break
					}
				}
			}

			nodes = append(nodes, node)
		}

		r.users = nodes
		r.err = r.db.QueryRowContext(ctx, countQuery.Query(sqlf.PostgresBindVar), countQuery.Args()...).Scan(&r.totalCount)
	})

	return r.users, r.totalCount, r.err
}

func (r *analyticsUserActivePeriodsConnection) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	users, totalCount, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	// We would have had all results when no limit set
	if r.opt.LimitOffset == nil {
		return graphqlutil.HasNextPage(false), nil
	}

	after := r.opt.LimitOffset.Offset + len(users)

	// We got less results than limit, means we've had all results
	if after < r.opt.Limit {
		return graphqlutil.HasNextPage(false), nil
	}

	if totalCount > after {
		return graphqlutil.NextPageCursor(strconv.Itoa(after)), nil
	}
	return graphqlutil.HasNextPage(false), nil
}
