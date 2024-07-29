package subscriptions_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/databasetest"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/internal/tables"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/subscriptions"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/utctime"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestSubscriptionsStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := databasetest.NewTestDB(t, "enterprise-portal", t.Name(), tables.All()...)

	for _, tc := range []struct {
		name string
		test func(t *testing.T, ctx context.Context, s *subscriptions.Store)
	}{
		{"List", SubscriptionsStoreList},
		{"Upsert", SubscriptionsStoreUpsert},
		{"Get", SubscriptionsStoreGet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()
			databasetest.ClearTablesAfterTest(t, db, tables.All()...)

			tc.test(t, ctx, subscriptions.NewStore(db))
		})
		if t.Failed() {
			break
		}
	}
}

func assertHasMatch[T any](t *testing.T, values []T, match func(v T) bool) {
	for _, v := range values {
		if match(v) {
			return
		}
	}
	t.Errorf("values %+v does not contain match", values)
}

func SubscriptionsStoreList(t *testing.T, ctx context.Context, s *subscriptions.Store) {
	// Create test records.
	s1, err := s.Upsert(
		ctx,
		uuid.New().String(),
		subscriptions.UpsertSubscriptionOptions{
			DisplayName:              database.NewNullString("Subscription 1"),
			InstanceDomain:           database.NewNullString("s1.sourcegraph.com"),
			SalesforceSubscriptionID: database.NewNullString("sf_sub_id"),
		},
	)
	require.NoError(t, err)
	s2, err := s.Upsert(
		ctx,
		uuid.New().String(),
		subscriptions.UpsertSubscriptionOptions{
			DisplayName:    database.NewNullString("Subscription 2"),
			InstanceDomain: database.NewNullString("s2.sourcegraph.com"),
		},
	)
	require.NoError(t, err)
	s3, err := s.Upsert(
		ctx,
		uuid.New().String(),
		subscriptions.UpsertSubscriptionOptions{
			DisplayName:    database.NewNullString("Subscription 3"),
			InstanceDomain: database.NewNullString("s3.sourcegraph.com"),
		},
	)
	require.NoError(t, err)

	t.Run("list by IDs", func(t *testing.T) {
		t.Parallel()

		ss, err := s.List(ctx, subscriptions.ListEnterpriseSubscriptionsOptions{IDs: []string{s1.ID, s2.ID}})
		require.NoError(t, err)
		require.Len(t, ss, 2)
		assertHasMatch(t, ss, func(v *subscriptions.SubscriptionWithConditions) bool {
			return v.ID == s1.ID
		})
		assertHasMatch(t, ss, func(v *subscriptions.SubscriptionWithConditions) bool {
			return v.ID == s2.ID
		})

		t.Run("no match", func(t *testing.T) {
			ss, err = s.List(ctx, subscriptions.ListEnterpriseSubscriptionsOptions{
				IDs: []string{uuid.Must(uuid.NewV7()).String()},
			})
			require.NoError(t, err)
			require.Len(t, ss, 0)
		})
	})

	t.Run("list by instance domains", func(t *testing.T) {
		t.Parallel()

		ss, err := s.List(ctx, subscriptions.ListEnterpriseSubscriptionsOptions{
			InstanceDomains: []string{*s1.InstanceDomain, *s2.InstanceDomain}},
		)
		require.NoError(t, err)
		require.Len(t, ss, 2)
		assertHasMatch(t, ss, func(v *subscriptions.SubscriptionWithConditions) bool {
			return v.ID == s1.ID
		})
		assertHasMatch(t, ss, func(v *subscriptions.SubscriptionWithConditions) bool {
			return v.ID == s2.ID
		})

		t.Run("no match", func(t *testing.T) {
			ss, err = s.List(ctx, subscriptions.ListEnterpriseSubscriptionsOptions{
				InstanceDomains: []string{"1234"},
			})
			require.NoError(t, err)
			require.Len(t, ss, 0)
		})
	})

	t.Run("list by display name", func(t *testing.T) {
		t.Parallel()

		ss, err := s.List(
			ctx,
			subscriptions.ListEnterpriseSubscriptionsOptions{
				DisplayNameSubstring: "Subscription",
			},
		)
		require.NoError(t, err)
		assert.Len(t, ss, 3) // all 3 are returned

		t.Run("single match", func(t *testing.T) {
			t.Parallel()

			ss, err := s.List(
				ctx,
				subscriptions.ListEnterpriseSubscriptionsOptions{
					DisplayNameSubstring: "tion 3",
				},
			)
			require.NoError(t, err)
			assert.Len(t, ss, 1)
			assert.Equal(t, s3.ID, ss[0].ID)
		})

		t.Run("exact match", func(t *testing.T) {
			t.Parallel()

			ss, err := s.List(
				ctx,
				subscriptions.ListEnterpriseSubscriptionsOptions{
					DisplayNameSubstring: "Subscription 2",
				},
			)
			require.NoError(t, err)
			assert.Len(t, ss, 1)
			assert.Equal(t, s2.ID, ss[0].ID)
		})

		t.Run("case-insensitive match", func(t *testing.T) {
			t.Parallel()

			ss, err := s.List(
				ctx,
				subscriptions.ListEnterpriseSubscriptionsOptions{
					DisplayNameSubstring: "subscription 2",
				},
			)
			require.NoError(t, err)
			assert.Len(t, ss, 1)
			assert.Equal(t, s2.ID, ss[0].ID)
		})
	})

	t.Run("list by Salesforce subscription ID", func(t *testing.T) {
		t.Parallel()

		ss, err := s.List(
			ctx,
			subscriptions.ListEnterpriseSubscriptionsOptions{
				SalesforceSubscriptionIDs: []string{"sf_sub_id"},
			},
		)
		require.NoError(t, err)
		assert.Len(t, ss, 1)
		assert.Equal(t, s1.ID, ss[0].ID)
	})

	t.Run("list with page size", func(t *testing.T) {
		t.Parallel()

		ss, err := s.List(
			ctx,
			subscriptions.ListEnterpriseSubscriptionsOptions{
				IDs:      []string{s1.ID, s2.ID}, // Two matching but only of them will be returned.
				PageSize: 1,
			},
		)
		require.NoError(t, err)
		assert.Len(t, ss, 1)
	})
}

func SubscriptionsStoreUpsert(t *testing.T, ctx context.Context, s *subscriptions.Store) {
	// Create initial test record. The currentSubscription should be reassigned
	// throughout various test cases to represent the current state of the test
	// record, as the subtests are run in sequence.
	created := utctime.Now()
	currentSubscription, err := s.Upsert(
		ctx,
		uuid.New().String(),
		subscriptions.UpsertSubscriptionOptions{
			InstanceDomain: database.NewNullString("s1.sourcegraph.com"),
			CreatedAt:      created,
		},
		// Represent the creation of this subscription
		subscriptions.CreateSubscriptionConditionOptions{
			Status:         subscriptionsv1.EnterpriseSubscriptionCondition_STATUS_CREATED,
			Message:        t.Name(),
			TransitionTime: created,
		},
	)
	require.NoError(t, err)

	got, err := s.Get(ctx, currentSubscription.ID)
	require.NoError(t, err)
	assert.Equal(t, currentSubscription.ID, got.ID)
	assert.Equal(t, *currentSubscription.InstanceDomain, *got.InstanceDomain)
	assert.Empty(t, got.DisplayName)
	assert.NotZero(t, got.CreatedAt)
	assert.NotZero(t, got.UpdatedAt)
	assert.Nil(t, got.ArchivedAt) // not archived yet

	// The condition we requested should exist
	require.Len(t, got.Conditions, 1)
	assert.Equal(t, "STATUS_CREATED", got.Conditions[0].Status)
	assert.Equal(t, t.Name(), *got.Conditions[0].Message)
	assert.Equal(t, got.CreatedAt, got.Conditions[0].TransitionTime)

	t.Run("noop", func(t *testing.T) {
		t.Cleanup(func() { currentSubscription = got })

		got, err = s.Upsert(ctx, currentSubscription.ID, subscriptions.UpsertSubscriptionOptions{})
		require.NoError(t, err)
		assert.Equal(t,
			pointers.DerefZero(currentSubscription.InstanceDomain),
			pointers.DerefZero(got.InstanceDomain))
	})

	t.Run("update only domain", func(t *testing.T) {
		t.Cleanup(func() { currentSubscription = got })

		got, err = s.Upsert(ctx, currentSubscription.ID, subscriptions.UpsertSubscriptionOptions{
			InstanceDomain: database.NewNullString("s1-new.sourcegraph.com"),
		})
		require.NoError(t, err)
		assert.Equal(t, "s1-new.sourcegraph.com", pointers.DerefZero(got.InstanceDomain))
		assert.Equal(t, currentSubscription.DisplayName, got.DisplayName)
	})

	t.Run("update only display name", func(t *testing.T) {
		t.Cleanup(func() { currentSubscription = got })

		got, err = s.Upsert(ctx, currentSubscription.ID, subscriptions.UpsertSubscriptionOptions{
			DisplayName: database.NewNullString("My New Display Name"),
		})
		require.NoError(t, err)
		assert.Equal(t, *currentSubscription.InstanceDomain, *got.InstanceDomain)
		assert.Equal(t, "My New Display Name", pointers.DerefZero(got.DisplayName))
	})

	t.Run("update only created at", func(t *testing.T) {
		t.Cleanup(func() { currentSubscription = got })

		yesterday := utctime.FromTime(time.Now().Add(-24 * time.Hour))
		got, err = s.Upsert(ctx, currentSubscription.ID, subscriptions.UpsertSubscriptionOptions{
			CreatedAt: yesterday,
		})
		require.NoError(t, err)
		assert.Equal(t,
			pointers.DerefZero(currentSubscription.InstanceDomain),
			pointers.DerefZero(got.InstanceDomain))
		assert.Equal(t, currentSubscription.DisplayName, got.DisplayName)
		assert.Equal(t, yesterday.AsTime(), got.CreatedAt.AsTime())
	})

	t.Run("update only archived at", func(t *testing.T) {
		t.Cleanup(func() { currentSubscription = got })

		yesterday := utctime.FromTime(time.Now().Add(-24 * time.Hour))
		got, err = s.Upsert(ctx, currentSubscription.ID, subscriptions.UpsertSubscriptionOptions{
			ArchivedAt: pointers.Ptr(yesterday),
		})
		require.NoError(t, err)
		assert.Equal(t, *currentSubscription.InstanceDomain, *got.InstanceDomain)
		assert.Equal(t, *currentSubscription.DisplayName, *got.DisplayName)
		assert.Equal(t, currentSubscription.CreatedAt, got.CreatedAt)
		assert.Equal(t, yesterday.AsTime(), got.ArchivedAt.AsTime())
	})

	t.Run("force update to zero values", func(t *testing.T) {
		t.Cleanup(func() { currentSubscription = got })

		got, err = s.Upsert(ctx, currentSubscription.ID, subscriptions.UpsertSubscriptionOptions{
			ForceUpdate: true,
		})
		require.NoError(t, err)
		assert.Empty(t, got.InstanceDomain)
		assert.Empty(t, got.DisplayName)

		// Some fields cannot be updated in a force-update.
		assert.Equal(t, currentSubscription.ArchivedAt, got.ArchivedAt)
		assert.Equal(t, currentSubscription.ID, got.ID)
		assert.Equal(t, currentSubscription.CreatedAt, got.CreatedAt)
	})
}

func SubscriptionsStoreGet(t *testing.T, ctx context.Context, s *subscriptions.Store) {
	// Create initial test record.
	s1, err := s.Upsert(
		ctx,
		uuid.New().String(),
		subscriptions.UpsertSubscriptionOptions{
			InstanceDomain: database.NewNullString("s1.sourcegraph.com"),
		},
	)
	require.NoError(t, err)

	t.Run("not found", func(t *testing.T) {
		_, err := s.Get(ctx, uuid.New().String())
		assert.ErrorIs(t, err, subscriptions.ErrSubscriptionNotFound)
	})

	t.Run("found", func(t *testing.T) {
		got, err := s.Get(ctx, s1.ID)
		require.NoError(t, err)
		assert.Equal(t, s1.ID, got.ID)
		assert.Equal(t, s1.InstanceDomain, got.InstanceDomain)
	})
}
