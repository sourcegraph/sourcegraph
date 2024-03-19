package cody

import (
	"context"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/ssc"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type currentTimeCtxKey int

const mockCurrentTimeKey currentTimeCtxKey = iota

func currentTimeFromCtx(ctx context.Context) time.Time {
	t, ok := ctx.Value(mockCurrentTimeKey).(*time.Time)
	if !ok || t == nil {
		return time.Now()
	}
	return *t
}

func withCurrentTimeMock(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, mockCurrentTimeKey, &t)
}

// TODO(sourcegraph#59990): Remove this function and get current period end date
// from Cody Gateway usage cache ttl for free users to show usage reset at timestamp.
// The period end date is only used to show the usage limit reset timestamp in the UI,
// when free users hit their limits.
func freeUserCurrentPeriodDateRange(ctx context.Context, user types.User, subscription *ssc.Subscription) (*time.Time, *time.Time, error) {
	// to allow mocking current time during tests
	currentDate := currentTimeFromCtx(ctx)

	subscriptionStartDate := user.CreatedAt

	// If the subscription is canceled, use the end date of the cancelled period as the start date of the current period.
	if subscription != nil && subscription.Status == ssc.SubscriptionStatusCanceled {
		cancelledPeriodEndDate, err := time.Parse(time.RFC3339, subscription.CurrentPeriodEnd)
		if err != nil {
			return nil, nil, err
		}
		subscriptionStartDate = cancelledPeriodEndDate
	}

	targetDay := subscriptionStartDate.Day()
	startDayOfTheMonth := targetDay
	endDayOfTheMonth := targetDay - 1
	startMonth := currentDate
	endMonth := currentDate

	if currentDate.Day() < targetDay {
		// Set to target day of the previous month
		startMonth = currentDate.AddDate(0, -1, 0)
	} else {
		// Set to target day of the next month
		endMonth = currentDate.AddDate(0, 1, 0)
	}

	daysInStartingMonth := time.Date(startMonth.Year(), startMonth.Month()+1, 0, 0, 0, 0, 0, startMonth.Location()).Day()
	if startDayOfTheMonth > daysInStartingMonth {
		startDayOfTheMonth = daysInStartingMonth
	}

	daysInEndingMonth := time.Date(endMonth.Year(), endMonth.Month()+1, 0, 0, 0, 0, 0, endMonth.Location()).Day()
	if endDayOfTheMonth > daysInEndingMonth {
		endDayOfTheMonth = daysInEndingMonth
	}

	startDate := time.Date(startMonth.Year(), startMonth.Month(), startDayOfTheMonth, 0, 0, 0, 0, startMonth.Location())
	endDate := time.Date(endMonth.Year(), endMonth.Month(), endDayOfTheMonth, 23, 59, 59, 59, endMonth.Location())

	return &startDate, &endDate, nil
}
