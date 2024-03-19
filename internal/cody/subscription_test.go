package cody

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/ssc"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func toTimePtr(t time.Time) *time.Time {
	return &t
}

func TestGetSubscriptionForUser(t *testing.T) {
	SAMSAccountIDWithSubscription := "having-subscription"
	SAMSAccountIDWithoutSubscription := "no-subscription"

	tests := []struct {
		name                 string
		user                 types.User
		today                time.Time
		mockSSC              []MockSSCValue
		expectedSubscription UserSubscription
	}{
		{
			name: "user without SAMS account & SSC subscription",
			user: types.User{
				ID:        1,
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			today:   time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
			mockSSC: []MockSSCValue{},
			expectedSubscription: UserSubscription{
				Status:               ssc.SubscriptionStatusActive,
				Plan:                 UserSubscriptionPlanFree,
				ApplyProRateLimits:   false,
				CurrentPeriodStartAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				CurrentPeriodEndAt:   time.Date(2024, 1, 31, 23, 59, 59, 59, time.UTC),
			},
		},
		{
			name: "user with SAMS account without SSC subscription",
			user: types.User{
				ID:        1,
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			today:   time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
			mockSSC: []MockSSCValue{{SAMSAccountID: SAMSAccountIDWithoutSubscription}},
			expectedSubscription: UserSubscription{
				Status:               ssc.SubscriptionStatusActive,
				Plan:                 UserSubscriptionPlanFree,
				ApplyProRateLimits:   false,
				CurrentPeriodStartAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				CurrentPeriodEndAt:   time.Date(2024, 1, 31, 23, 59, 59, 59, time.UTC),
			},
		},
		{
			name: "user with SAMS account & SSC subscription",
			user: types.User{
				ID:        1,
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			today: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
			mockSSC: []MockSSCValue{{
				Subscription: &ssc.Subscription{
					Status:             ssc.SubscriptionStatusActive,
					BillingInterval:    ssc.BillingIntervalMonthly,
					CancelAtPeriodEnd:  false,
					CurrentPeriodStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
					CurrentPeriodEnd:   time.Date(2024, 1, 31, 23, 59, 59, 59, time.UTC).Format(time.RFC3339Nano),
				},
				SAMSAccountID: SAMSAccountIDWithSubscription,
			}},
			expectedSubscription: UserSubscription{
				Status:               ssc.SubscriptionStatusActive,
				Plan:                 UserSubscriptionPlanPro,
				ApplyProRateLimits:   true,
				CurrentPeriodStartAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				CurrentPeriodEndAt:   time.Date(2024, 1, 31, 23, 59, 59, 59, time.UTC),
			},
		},
		{
			// to cover the bug where 1 dotcom user can have mutliple SAMS identities linked
			name: "user with multiple SAMS account but 1 SSC subscription",
			user: types.User{
				ID:        1,
				CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			today: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
			mockSSC: []MockSSCValue{{
				SAMSAccountID: SAMSAccountIDWithoutSubscription,
			}, {
				Subscription: &ssc.Subscription{
					Status:             ssc.SubscriptionStatusActive,
					BillingInterval:    ssc.BillingIntervalMonthly,
					CancelAtPeriodEnd:  false,
					CurrentPeriodStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
					CurrentPeriodEnd:   time.Date(2024, 1, 31, 23, 59, 59, 59, time.UTC).Format(time.RFC3339Nano),
				},
				SAMSAccountID: SAMSAccountIDWithSubscription,
			}},
			expectedSubscription: UserSubscription{
				Status:               ssc.SubscriptionStatusActive,
				Plan:                 UserSubscriptionPlanPro,
				ApplyProRateLimits:   true,
				CurrentPeriodStartAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				CurrentPeriodEndAt:   time.Date(2024, 1, 31, 23, 59, 59, 59, time.UTC),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := actor.WithActor(context.Background(), actor.FromUser(test.user.ID))
			ctx = withCurrentTimeMock(ctx, test.today)

			// Mock out the lookup for the user's external identities, where we fetch their SAMS account ID.
			db := dbmocks.NewMockDB()
			userExternalAccount := dbmocks.NewMockUserExternalAccountsStore()
			userExternalAccount.ListFunc.SetDefaultHook(func(ctx context.Context, opts database.ExternalAccountsListOptions) ([]*extsvc.Account, error) {
				assert.Equal(t, test.user.ID, opts.UserID)
				var accounts []*extsvc.Account

				for _, MockSSCValue := range test.mockSSC {
					accounts = append(accounts, &extsvc.Account{AccountSpec: extsvc.AccountSpec{
						AccountID:   MockSSCValue.SAMSAccountID,
						ServiceType: "openidconnect",
						ServiceID:   fmt.Sprintf("https://%s/", ssc.GetSAMSHostName()),
					}})
				}

				return accounts, nil
			})
			db.UserExternalAccountsFunc.SetDefaultReturn(userExternalAccount)

			expectToBeCalled := len(test.mockSSC) != 0

			ctx = WithMockSSCClient(ctx, MockSSCClient{
				MockSSCValue:   test.mockSSC,
				ShouldBeCalled: expectToBeCalled,
			})

			actualSubscription, err := SubscriptionForUser(ctx, db, test.user)
			assert.NoError(t, err)
			assert.NotNil(t, actualSubscription)
			assert.Equal(t, test.expectedSubscription, *actualSubscription)
		})
	}
}
