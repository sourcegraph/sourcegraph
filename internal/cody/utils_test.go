package cody

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestFreeUserCurrentPeriodDateRange(t *testing.T) {
	db := dbmocks.NewMockDB()

	dotcom.MockSourcegraphDotComMode(t, true)

	users := dbmocks.NewMockUserStore()
	db.UsersFunc.SetDefaultReturn(users)
	now := time.Now()

	tests := []struct {
		name      string
		user      *types.User
		today     time.Time
		createdAt time.Time
		start     time.Time
		end       time.Time
	}{
		{
			name:      "community user created before current day",
			createdAt: time.Date(2024, 9, 5, 0, 0, 0, 0, now.Location()),
			today:     time.Date(2025, 1, 15, 0, 0, 0, 0, now.Location()),
			start:     time.Date(2025, 1, 5, 0, 0, 0, 0, now.Location()),
			end:       time.Date(2025, 2, 4, 23, 59, 59, 59, now.Location()),
		},
		{
			name:      "community user created after current day",
			createdAt: time.Date(2024, 9, 25, 0, 0, 0, 0, now.Location()),
			today:     time.Date(2025, 1, 15, 0, 0, 0, 0, now.Location()),
			start:     time.Date(2024, 12, 25, 0, 0, 0, 0, now.Location()),
			end:       time.Date(2025, 1, 24, 23, 59, 59, 59, now.Location()),
		},
		{
			name:      "community user created on 31st Jan",
			createdAt: time.Date(2025, 1, 31, 0, 0, 0, 0, now.Location()),
			today:     time.Date(2025, 2, 15, 0, 0, 0, 0, now.Location()),
			start:     time.Date(2025, 1, 31, 0, 0, 0, 0, now.Location()),
			end:       time.Date(2025, 2, 28, 23, 59, 59, 59, now.Location()),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user := &types.User{ID: 1}
			user.CreatedAt = test.createdAt

			users.GetByIDFunc.SetDefaultReturn(test.user, nil)

			ctx := actor.WithActor(context.Background(), &actor.Actor{UID: user.ID})
			ctx = withCurrentTimeMock(ctx, test.today)

			startDate, endDate, err := freeUserCurrentPeriodDateRange(ctx, *user, nil)
			assert.NoError(t, err, "freeUserCurrentPeriodDateRange")
			assert.NotNil(t, startDate, "not nil startDate")
			assert.NotNil(t, endDate, "not nil endDate")
			assert.Equal(t, test.start, *startDate, "startDate")
			assert.Equal(t, test.end, *endDate, "endDate")
		})
	}
}
