package usagestats

import (
	"context"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// GetAggregatedCodyStats queries the database for Cody usage and returns
// the aggregates statistics in the format of our BigQuery schema.
func GetAggregatedCodyStats(ctx context.Context, db database.DB) (*types.CodyUsageStatistics, error) {
	events, err := db.EventLogs().AggregatedCodyUsage(ctx, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	stats := &types.CodyUsageStatistics{
		Daily:   newCodyEventPeriodLimited(),
		Weekly:  newCodyEventPeriodLimited(),
		Monthly: newCodyEventPeriod(),
	}

	if events == nil {
		// If there are no events, return the empty stats.
		return stats, nil
	}

	stats.Daily.StartTime = events.Day
	stats.Daily.TotalCodyUsers.EventsCount = &events.TotalDay
	stats.Daily.TotalCodyUsers.UserCount = &events.UniquesDay
	stats.Daily.TotalProductUsers.UserCount = &events.ProductUsersDay

	stats.Weekly.StartTime = events.Week
	stats.Weekly.TotalCodyUsers.EventsCount = &events.TotalWeek
	stats.Weekly.TotalCodyUsers.UserCount = &events.UniquesWeek
	stats.Weekly.TotalProductUsers.UserCount = &events.ProductUsersWeek

	stats.Monthly.StartTime = events.Month
	stats.Monthly.TotalCodyUsers.EventsCount = &events.TotalMonth
	stats.Monthly.TotalCodyUsers.UserCount = &events.UniquesMonth
	stats.Monthly.TotalProductUsers.UserCount = &events.ProductUsersMonth
	stats.Monthly.TotalVSCodeProductUsers.UserCount = &events.VSCodeProductUsersMonth
	stats.Monthly.TotalJetBrainsProductUsers.UserCount = &events.JetBrainsProductUsersMonth
	stats.Monthly.TotalNeovimProductUsers.UserCount = &events.NeovimProductUsersMonth
	stats.Monthly.TotalEmacsProductUsers.UserCount = &events.EmacsProductUsersMonth
	stats.Monthly.TotalWebProductUsers.UserCount = &events.WebProductUsersMonth

	return stats, nil
}

func newCodyEventPeriod() *types.CodyUsagePeriod {
	return &types.CodyUsagePeriod{
		StartTime:                  time.Now().UTC(),
		TotalCodyUsers:             newCodyCountStatistics(),
		TotalProductUsers:          newCodyCountStatistics(),
		TotalVSCodeProductUsers:    newCodyCountStatistics(),
		TotalJetBrainsProductUsers: newCodyCountStatistics(),
		TotalNeovimProductUsers:    newCodyCountStatistics(),
		TotalEmacsProductUsers:     newCodyCountStatistics(),
		TotalWebProductUsers:       newCodyCountStatistics(),
	}
}

func newCodyEventPeriodLimited() *types.CodyUsagePeriodLimited {
	return &types.CodyUsagePeriodLimited{
		StartTime:         time.Now().UTC(),
		TotalCodyUsers:    newCodyCountStatistics(),
		TotalProductUsers: newCodyCountStatistics(),
	}
}

func newCodyCountStatistics() *types.CodyCountStatistics {
	return &types.CodyCountStatistics{}
}
