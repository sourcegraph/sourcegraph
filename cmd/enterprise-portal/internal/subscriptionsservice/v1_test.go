package subscriptionsservice

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/google/uuid"
	"github.com/hexops/autogold/v2"
	"github.com/hexops/valast"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	sams "github.com/sourcegraph/sourcegraph-accounts-sdk-go"
	"github.com/sourcegraph/sourcegraph-accounts-sdk-go/scopes"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/subscriptions"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/utctime"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/samsm2m"
	"github.com/sourcegraph/sourcegraph/internal/license"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/iam"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type testHandlerV1 struct {
	*handlerV1
	mockStore *MockStoreV1
}

func newPredictableGenerator(ns string) func() (string, error) {
	var seq atomic.Int32
	return func() (string, error) {
		return fmt.Sprintf("%s-%d", ns, seq.Add(1)), nil
	}
}

func newMockTime() time.Time {
	return time.Date(2024, 1, 1, 1, 1, 0, 0, time.UTC)
}

func newTestHandlerV1(t *testing.T, tokenScopes ...scopes.Scope) *testHandlerV1 {
	mockStore := NewMockStoreV1()
	mockStore.IntrospectSAMSTokenFunc.SetDefaultReturn(
		&sams.IntrospectTokenResponse{
			Active: true,
			Scopes: tokenScopes,
		},
		nil,
	)

	mockStore.GenerateSubscriptionIDFunc.SetDefaultHook(
		newPredictableGenerator("uuid"))

	keySigner := newPredictableGenerator("signedkey")
	mockStore.SignEnterpriseSubscriptionLicenseKeyFunc.SetDefaultHook(
		func(i license.Info) (string, error) { return keySigner() })

	// Stable time generator that increments by 1 second on each call
	var timeSeq atomic.Int32
	mockStore.NowFunc.SetDefaultHook(func() utctime.Time {
		return utctime.FromTime(newMockTime().
			Add(time.Duration(timeSeq.Add(1)) * time.Second))
	})

	return &testHandlerV1{
		handlerV1: &handlerV1{
			logger: logtest.Scoped(t),
			store:  mockStore,
		},
		mockStore: mockStore,
	}
}

func TestHandlerV1_ListEnterpriseSubscriptions(t *testing.T) {
	ctx := context.Background()
	const mockSubscriptionID = "80ca12e2-54b4-448c-a61a-390b1a9c1224"

	for _, tc := range []struct {
		name           string
		list           *subscriptionsv1.ListEnterpriseSubscriptionsRequest
		iamObjectsHook func(opts iam.ListObjectsOptions) ([]string, error)
		wantError      autogold.Value
		wantListOpts   autogold.Value
	}{
		{
			name: "more than one permission filter",
			list: &subscriptionsv1.ListEnterpriseSubscriptionsRequest{
				Filters: []*subscriptionsv1.ListEnterpriseSubscriptionsFilter{
					{Filter: &subscriptionsv1.ListEnterpriseSubscriptionsFilter_Permission{
						Permission: &subscriptionsv1.Permission{
							Type:          subscriptionsv1.PermissionType_PERMISSION_TYPE_SUBSCRIPTION_CODY_ANALYTICS,
							Relation:      subscriptionsv1.PermissionRelation_PERMISSION_RELATION_VIEW,
							SamsAccountId: "018d21f2-04a6-7aaf-9f6f-6fc58c4187b9",
						},
					}},
					{Filter: &subscriptionsv1.ListEnterpriseSubscriptionsFilter_Permission{
						Permission: &subscriptionsv1.Permission{
							Type:          subscriptionsv1.PermissionType_PERMISSION_TYPE_SUBSCRIPTION_CODY_ANALYTICS,
							Relation:      subscriptionsv1.PermissionRelation_PERMISSION_RELATION_VIEW,
							SamsAccountId: "018d21f2-0b8d-756a-84f0-13b942a2bae5",
						},
					}},
				},
			},
			wantError: autogold.Expect(`invalid_argument: invalid filter: "permission" can only be provided once`),
		},
		{
			name: "only subscription ID filter",
			list: &subscriptionsv1.ListEnterpriseSubscriptionsRequest{
				Filters: []*subscriptionsv1.ListEnterpriseSubscriptionsFilter{
					{Filter: &subscriptionsv1.ListEnterpriseSubscriptionsFilter_SubscriptionId{
						SubscriptionId: mockSubscriptionID,
					}},
				},
			},
			wantListOpts: autogold.Expect(subscriptions.ListEnterpriseSubscriptionsOptions{IDs: []string{
				"80ca12e2-54b4-448c-a61a-390b1a9c1224",
			}}),
		},
		{
			name: "only permission filter",
			iamObjectsHook: func(_ iam.ListObjectsOptions) ([]string, error) {
				return []string{"subscription_cody_analytics:" + mockSubscriptionID}, nil
			},
			list: &subscriptionsv1.ListEnterpriseSubscriptionsRequest{
				Filters: []*subscriptionsv1.ListEnterpriseSubscriptionsFilter{
					{Filter: &subscriptionsv1.ListEnterpriseSubscriptionsFilter_Permission{
						Permission: &subscriptionsv1.Permission{
							Type:          subscriptionsv1.PermissionType_PERMISSION_TYPE_SUBSCRIPTION_CODY_ANALYTICS,
							Relation:      subscriptionsv1.PermissionRelation_PERMISSION_RELATION_VIEW,
							SamsAccountId: "018d21f2-04a6-7aaf-9f6f-6fc58c4187b9",
						},
					}},
				},
			},
			wantListOpts: autogold.Expect(subscriptions.ListEnterpriseSubscriptionsOptions{IDs: []string{
				"80ca12e2-54b4-448c-a61a-390b1a9c1224",
			}}),
		},
		{
			name: "both subscription ID and permission filter that DO NOT intersect",
			iamObjectsHook: func(_ iam.ListObjectsOptions) ([]string, error) {
				return []string{
					"subscription_cody_analytics:a-different-subscription-id",
				}, nil
			},
			list: &subscriptionsv1.ListEnterpriseSubscriptionsRequest{
				Filters: []*subscriptionsv1.ListEnterpriseSubscriptionsFilter{
					{Filter: &subscriptionsv1.ListEnterpriseSubscriptionsFilter_Permission{
						Permission: &subscriptionsv1.Permission{
							Type:          subscriptionsv1.PermissionType_PERMISSION_TYPE_SUBSCRIPTION_CODY_ANALYTICS,
							Relation:      subscriptionsv1.PermissionRelation_PERMISSION_RELATION_VIEW,
							SamsAccountId: "018d21f2-04a6-7aaf-9f6f-6fc58c4187b9",
						},
					}},
					{Filter: &subscriptionsv1.ListEnterpriseSubscriptionsFilter_SubscriptionId{
						SubscriptionId: mockSubscriptionID,
					}},
				},
			},
			// No error or list opts - the requested subscription ID and permissions
			// do not intersect, so there is no result and we fast-return.
		},
		{
			name: "both subscription ID and permission filter",
			iamObjectsHook: func(_ iam.ListObjectsOptions) ([]string, error) {
				return []string{"subscription_cody_analytics:" + mockSubscriptionID}, nil
			},
			list: &subscriptionsv1.ListEnterpriseSubscriptionsRequest{
				Filters: []*subscriptionsv1.ListEnterpriseSubscriptionsFilter{
					{Filter: &subscriptionsv1.ListEnterpriseSubscriptionsFilter_Permission{
						Permission: &subscriptionsv1.Permission{
							Type:          subscriptionsv1.PermissionType_PERMISSION_TYPE_SUBSCRIPTION_CODY_ANALYTICS,
							Relation:      subscriptionsv1.PermissionRelation_PERMISSION_RELATION_VIEW,
							SamsAccountId: "018d21f2-04a6-7aaf-9f6f-6fc58c4187b9",
						},
					}},
					{Filter: &subscriptionsv1.ListEnterpriseSubscriptionsFilter_SubscriptionId{
						SubscriptionId: mockSubscriptionID,
					}},
				},
			},
			wantListOpts: autogold.Expect(subscriptions.ListEnterpriseSubscriptionsOptions{IDs: []string{
				"80ca12e2-54b4-448c-a61a-390b1a9c1224",
			}}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := connect.NewRequest(tc.list)
			req.Header().Add("Authorization", "Bearer foolmeifyoucan")

			h := newTestHandlerV1(t,
				samsm2m.EnterprisePortalScope(
					scopes.PermissionEnterprisePortalSubscription,
					scopes.ActionRead,
				),
			)
			h.mockStore.IAMListObjectsFunc.SetDefaultHook(func(_ context.Context, opts iam.ListObjectsOptions) ([]string, error) {
				return tc.iamObjectsHook(opts)
			})
			h.mockStore.ListEnterpriseSubscriptionsFunc.SetDefaultHook(func(_ context.Context, opts subscriptions.ListEnterpriseSubscriptionsOptions) ([]*subscriptions.SubscriptionWithConditions, error) {
				tc.wantListOpts.Equal(t, opts)
				return []*subscriptions.SubscriptionWithConditions{}, nil
			})

			if _, err := h.ListEnterpriseSubscriptions(ctx, req); tc.wantError != nil {
				tc.wantError.Equal(t, fmt.Sprintf("%v", err.Error()))
			} else {
				assert.NoError(t, err)
			}

			if tc.wantListOpts != nil {
				mockrequire.CalledOnce(t, h.mockStore.ListEnterpriseSubscriptionsFunc)
			} else {
				mockrequire.NotCalled(t, h.mockStore.ListEnterpriseSubscriptionsFunc)
			}
			if tc.iamObjectsHook != nil {
				mockrequire.CalledOnce(t, h.mockStore.IAMListObjectsFunc)
			} else {
				mockrequire.NotCalled(t, h.mockStore.IAMListObjectsFunc)
			}
		})
	}
}

func TestHandlerV1_CreateEnterpriseSubscription(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name           string
		tokenScopes    scopes.Scopes
		create         *subscriptionsv1.CreateEnterpriseSubscriptionRequest
		wantError      autogold.Value
		wantUpsertOpts autogold.Value
	}{
		{
			name: "no parameters",
			create: &subscriptionsv1.CreateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{},
			},
			wantError: autogold.Expect("invalid_argument: display_name is required"),
		},
		{
			name: "custom subscription ID",
			create: &subscriptionsv1.CreateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					Id:          "not-allowed",
					DisplayName: t.Name(),
				},
			},
			wantError: autogold.Expect("invalid_argument: subscription_id can not be set"),
		},
		{
			name: "insufficient scopes",
			tokenScopes: scopes.Scopes{
				samsm2m.EnterprisePortalScope(
					scopes.PermissionEnterprisePortalSubscription,
					scopes.ActionRead,
				),
			},
			create: &subscriptionsv1.CreateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					Id:          "not-allowed",
					DisplayName: t.Name(),
				},
			},
			wantError: autogold.Expect("permission_denied: insufficient scope"),
		},
		{
			name: "with required params only",
			create: &subscriptionsv1.CreateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					DisplayName: t.Name(),
				},
			},
			wantUpsertOpts: autogold.Expect(subscriptions.UpsertSubscriptionOptions{
				InstanceDomain: &sql.NullString{},
				DisplayName: &sql.NullString{
					String: "TestHandlerV1_CreateEnterpriseSubscription",
					Valid:  true,
				},
				CreatedAt: utctime.Date(2024,
					1,
					1,
					1,
					1,
					1,
					0),
				SalesforceSubscriptionID: &sql.NullString{},
				SalesforceOpportunityID:  &sql.NullString{},
			}),
		},
		{
			name: "with message and optional fields",
			create: &subscriptionsv1.CreateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					DisplayName: t.Name(),
					Salesforce: &subscriptionsv1.EnterpriseSubscription_SalesforceMetadata{
						SubscriptionId: "sf_sub",
						OpportunityId:  "sf_opp",
					},
				},
				Message: "hello world",
			},
			wantUpsertOpts: autogold.Expect(subscriptions.UpsertSubscriptionOptions{
				InstanceDomain: &sql.NullString{},
				DisplayName: &sql.NullString{
					String: "TestHandlerV1_CreateEnterpriseSubscription",
					Valid:  true,
				},
				CreatedAt: utctime.Date(2024,
					1,
					1,
					1,
					1,
					1,
					0),
				SalesforceSubscriptionID: &sql.NullString{
					String: "sf_sub",
					Valid:  true,
				},
				SalesforceOpportunityID: &sql.NullString{
					String: "sf_opp",
					Valid:  true,
				},
			}),
		},
	} {
		req := connect.NewRequest(tc.create)
		req.Header().Add("Authorization", "Bearer foolmeifyoucan")

		if tc.tokenScopes == nil {
			tc.tokenScopes = scopes.Scopes{
				samsm2m.EnterprisePortalScope(
					scopes.PermissionEnterprisePortalSubscription,
					scopes.ActionWrite,
				),
			}
		}
		h := newTestHandlerV1(t, tc.tokenScopes...)
		h.mockStore.GetEnterpriseSubscriptionFunc.SetDefaultHook(func(_ context.Context, id string) (*subscriptions.SubscriptionWithConditions, error) {
			return nil, subscriptions.ErrSubscriptionNotFound
		})
		h.mockStore.UpsertEnterpriseSubscriptionFunc.SetDefaultHook(func(_ context.Context, _ string, opts subscriptions.UpsertSubscriptionOptions, conds ...subscriptions.CreateSubscriptionConditionOptions) (*subscriptions.SubscriptionWithConditions, error) {
			require.Len(t, conds, 1) // create must have condition

			// Condition must match upsert
			assert.Equal(t, tc.create.GetMessage(), conds[0].Message)
			assert.Equal(t, opts.CreatedAt, conds[0].TransitionTime)
			assert.Equal(t, subscriptionsv1.EnterpriseSubscriptionCondition_STATUS_CREATED,
				conds[0].Status)

			tc.wantUpsertOpts.Equal(t, opts)

			return &subscriptions.SubscriptionWithConditions{}, nil
		})
		_, err := h.CreateEnterpriseSubscription(ctx, req)
		if tc.wantError != nil {
			require.Error(t, err)
			tc.wantError.Equal(t, err.Error())
		} else {
			require.NoError(t, err)
		}
		if tc.wantUpsertOpts != nil {
			mockrequire.CalledOnce(t, h.mockStore.UpsertEnterpriseSubscriptionFunc)
			mockrequire.CalledOnce(t, h.mockStore.GetEnterpriseSubscriptionFunc)
		} else {
			mockrequire.NotCalled(t, h.mockStore.UpsertEnterpriseSubscriptionFunc)
		}
	}
}

func TestHandlerV1_UpdateEnterpriseSubscription(t *testing.T) {
	ctx := context.Background()
	const mockSubscriptionID = "es_80ca12e2-54b4-448c-a61a-390b1a9c1224"

	for _, tc := range []struct {
		name           string
		tokenScopes    scopes.Scopes
		update         *subscriptionsv1.UpdateEnterpriseSubscriptionRequest
		wantUpdateOpts autogold.Value
		wantError      autogold.Value
	}{
		{
			name: "insufficient scopes",
			tokenScopes: scopes.Scopes{
				samsm2m.EnterprisePortalScope(
					scopes.PermissionEnterprisePortalSubscription,
					scopes.ActionRead,
				),
			},
			update: &subscriptionsv1.UpdateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					Id: mockSubscriptionID,
				},
				UpdateMask: nil,
			},
			wantError: autogold.Expect("permission_denied: insufficient scope"),
		},
		{
			name: "subscription ID is required",
			update: &subscriptionsv1.UpdateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					Id: "",
				},
				UpdateMask: nil,
			},
			wantError: autogold.Expect("invalid_argument: subscription.id is required"),
		},
		{
			name: "subscription does not exist",
			update: &subscriptionsv1.UpdateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					Id: uuid.NewString(),
				},
				UpdateMask: nil,
			},
			wantError: autogold.Expect("not_found: subscription not found"),
		},
		{
			name: "no update mask",
			update: &subscriptionsv1.UpdateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					Id:             mockSubscriptionID,
					InstanceDomain: "s1.sourcegraph.com",
					DisplayName:    "My Test Subscription",
				},
				UpdateMask: nil,
			},
			// All non-zero values should be included in update
			wantUpdateOpts: autogold.Expect(subscriptions.UpsertSubscriptionOptions{
				InstanceDomain: &sql.NullString{
					String: "s1.sourcegraph.com",
					Valid:  true,
				},
				DisplayName: &sql.NullString{
					String: "My Test Subscription",
					Valid:  true,
				},
			}),
		},
		{
			name: "unknown field mask",
			update: &subscriptionsv1.UpdateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					Id:             mockSubscriptionID,
					InstanceDomain: "s1.sourcegraph.com",
					DisplayName:    "My Test Subscription",
				},
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"asdfasdf"}},
			},
			wantError: autogold.Expect("invalid_argument: unknown field path: asdfasdf"),
		},
		{
			name: "specified field mask",
			update: &subscriptionsv1.UpdateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					Id:             mockSubscriptionID,
					InstanceDomain: "s1.sourcegraph.com",
					// Should not be included, as only instance_domain is in
					// the field mask.
					DisplayName: "My Test Subscription",
				},
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"instance_domain"}},
			},
			wantUpdateOpts: autogold.Expect(subscriptions.UpsertSubscriptionOptions{InstanceDomain: &sql.NullString{
				String: "s1.sourcegraph.com",
				Valid:  true,
			}}),
		},
		{
			name: "* update_mask",
			update: &subscriptionsv1.UpdateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					Id: mockSubscriptionID,
					// All update-able values are zero
				},
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"*"}},
			},
			// All update-able values should be set to their defaults explicitly
			wantUpdateOpts: autogold.Expect(subscriptions.UpsertSubscriptionOptions{
				InstanceDomain:           &sql.NullString{},
				DisplayName:              &sql.NullString{},
				SalesforceSubscriptionID: &sql.NullString{},
				SalesforceOpportunityID:  &sql.NullString{},
				ForceUpdate:              true,
			}),
		},
		{
			name: "no-op update",
			update: &subscriptionsv1.UpdateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					Id: mockSubscriptionID,
					// All update-able values are zero
				},
				UpdateMask: nil,
			},
			// Update should be empty
			wantUpdateOpts: autogold.Expect(subscriptions.UpsertSubscriptionOptions{}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := connect.NewRequest(tc.update)
			req.Header().Add("Authorization", "Bearer foolmeifyoucan")

			if tc.tokenScopes == nil {
				tc.tokenScopes = scopes.Scopes{
					samsm2m.EnterprisePortalScope(
						scopes.PermissionEnterprisePortalSubscription,
						scopes.ActionWrite,
					),
				}
			}
			h := newTestHandlerV1(t, tc.tokenScopes...)
			h.mockStore.GetEnterpriseSubscriptionFunc.SetDefaultHook(func(ctx context.Context, id string) (*subscriptions.SubscriptionWithConditions, error) {
				if id == mockSubscriptionID {
					return &subscriptions.SubscriptionWithConditions{
						Subscription: subscriptions.Subscription{
							ID: id,
						},
					}, nil
				}
				return nil, subscriptions.ErrSubscriptionNotFound
			})
			h.mockStore.UpsertEnterpriseSubscriptionFunc.SetDefaultHook(func(_ context.Context, _ string, opts subscriptions.UpsertSubscriptionOptions, conds ...subscriptions.CreateSubscriptionConditionOptions) (*subscriptions.SubscriptionWithConditions, error) {
				tc.wantUpdateOpts.Equal(t, opts)
				assert.Len(t, conds, 0) // no conditions for standard updates
				return &subscriptions.SubscriptionWithConditions{}, nil
			})
			_, err := h.UpdateEnterpriseSubscription(ctx, req)
			if tc.wantError != nil {
				require.Error(t, err)
				tc.wantError.Equal(t, err.Error())
			} else {
				require.NoError(t, err)
			}
			if tc.wantUpdateOpts != nil {
				mockrequire.CalledOnce(t, h.mockStore.UpsertEnterpriseSubscriptionFunc)
			} else {
				mockrequire.NotCalled(t, h.mockStore.UpsertEnterpriseSubscriptionFunc)
			}
		})
	}
}

func TestHandlerV1_ArchiveEnterpriseSubscription(t *testing.T) {
	ctx := context.Background()
	const mockSubscriptionID = "es_80ca12e2-54b4-448c-a61a-390b1a9c1224"

	for _, tc := range []struct {
		name           string
		tokenScopes    scopes.Scopes
		archive        *subscriptionsv1.ArchiveEnterpriseSubscriptionRequest
		wantUpsertOpts autogold.Value
		wantError      autogold.Value
	}{
		{
			name: "insufficient scopes",
			tokenScopes: scopes.Scopes{
				samsm2m.EnterprisePortalScope(
					scopes.PermissionEnterprisePortalSubscription,
					scopes.ActionRead,
				),
			},
			archive:   &subscriptionsv1.ArchiveEnterpriseSubscriptionRequest{},
			wantError: autogold.Expect("permission_denied: insufficient scope"),
		},
		{
			name:      "subscription ID is required",
			archive:   &subscriptionsv1.ArchiveEnterpriseSubscriptionRequest{},
			wantError: autogold.Expect("invalid_argument: subscription_id is required"),
		},
		{
			name: "subscription does not exist",
			archive: &subscriptionsv1.ArchiveEnterpriseSubscriptionRequest{
				SubscriptionId: uuid.NewString(),
			},
			wantError: autogold.Expect("not_found: subscription not found"),
		},
		{
			name: "ok with reason",
			archive: &subscriptionsv1.ArchiveEnterpriseSubscriptionRequest{
				SubscriptionId: mockSubscriptionID,
				Reason:         t.Name(),
			},
			wantUpsertOpts: autogold.Expect(subscriptions.UpsertSubscriptionOptions{ArchivedAt: valast.Ptr(utctime.Date(2024,
				1,
				1,
				1,
				1,
				1,
				0))}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := connect.NewRequest(tc.archive)
			req.Header().Add("Authorization", "Bearer foolmeifyoucan")

			if tc.tokenScopes == nil {
				tc.tokenScopes = scopes.Scopes{
					samsm2m.EnterprisePortalScope(
						scopes.PermissionEnterprisePortalSubscription,
						scopes.ActionWrite,
					),
				}
			}
			h := newTestHandlerV1(t, tc.tokenScopes...)
			h.mockStore.GetEnterpriseSubscriptionFunc.SetDefaultHook(func(ctx context.Context, id string) (*subscriptions.SubscriptionWithConditions, error) {
				if id == mockSubscriptionID {
					return &subscriptions.SubscriptionWithConditions{
						Subscription: subscriptions.Subscription{
							ID: id,
						},
					}, nil
				}
				return nil, subscriptions.ErrSubscriptionNotFound
			})
			h.mockStore.UpsertEnterpriseSubscriptionFunc.SetDefaultHook(func(_ context.Context, _ string, opts subscriptions.UpsertSubscriptionOptions, conds ...subscriptions.CreateSubscriptionConditionOptions) (*subscriptions.SubscriptionWithConditions, error) {
				require.Len(t, conds, 1) // create must have condition

				// Condition must match upsert
				assert.Equal(t, tc.archive.GetReason(), conds[0].Message)
				require.NotNil(t, opts.ArchivedAt)
				assert.Equal(t, *opts.ArchivedAt, conds[0].TransitionTime)
				assert.Equal(t, subscriptionsv1.EnterpriseSubscriptionCondition_STATUS_ARCHIVED,
					conds[0].Status)

				tc.wantUpsertOpts.Equal(t, opts, autogold.ExportedOnly())

				return &subscriptions.SubscriptionWithConditions{}, nil
			})
			_, err := h.ArchiveEnterpriseSubscription(ctx, req)
			if tc.wantError != nil {
				require.Error(t, err)
				tc.wantError.Equal(t, err.Error())
			} else {
				require.NoError(t, err)
			}
			if tc.wantUpsertOpts != nil {
				mockrequire.CalledOnce(t, h.mockStore.UpsertEnterpriseSubscriptionFunc)
			} else {
				mockrequire.NotCalled(t, h.mockStore.UpsertEnterpriseSubscriptionFunc)
			}
		})
	}
}

func TestHandlerV1_CreateEnterpriseSubscriptionLicense(t *testing.T) {
	ctx := context.Background()
	const mockSubscriptionID = "es_80ca12e2-54b4-448c-a61a-390b1a9c1224"
	const archivedSubscriptionID = "es_b7df32dd-509c-4114-a6bb-4e6d09090fba"
	requiredTags := []string{"test", "dev"}

	for _, tc := range []struct {
		name        string
		tokenScopes scopes.Scopes
		create      *subscriptionsv1.CreateEnterpriseSubscriptionLicenseRequest
		wantKeyOpts autogold.Value
		wantError   autogold.Value
	}{
		{
			name: "insufficient scopes",
			tokenScopes: scopes.Scopes{
				samsm2m.EnterprisePortalScope(
					scopes.PermissionEnterprisePortalSubscription,
					scopes.ActionRead,
				),
			},
			create:    &subscriptionsv1.CreateEnterpriseSubscriptionLicenseRequest{},
			wantError: autogold.Expect("permission_denied: insufficient scope"),
		},
		{
			name:      "subscription ID is required",
			create:    &subscriptionsv1.CreateEnterpriseSubscriptionLicenseRequest{},
			wantError: autogold.Expect("invalid_argument: license.subscription_id is required"),
		},
		{
			name: "subscription does not exist",
			create: &subscriptionsv1.CreateEnterpriseSubscriptionLicenseRequest{
				License: &subscriptionsv1.EnterpriseSubscriptionLicense{
					SubscriptionId: uuid.NewString(),
				},
			},
			wantError: autogold.Expect("not_found: subscription not found"),
		},
		{
			name: "license data required",
			create: &subscriptionsv1.CreateEnterpriseSubscriptionLicenseRequest{
				License: &subscriptionsv1.EnterpriseSubscriptionLicense{
					SubscriptionId: mockSubscriptionID,
				},
			},
			wantError: autogold.Expect("invalid_argument: unsupported licnese type <nil>"),
		},
		{
			name: "license key: required tags not provided",
			create: &subscriptionsv1.CreateEnterpriseSubscriptionLicenseRequest{
				License: &subscriptionsv1.EnterpriseSubscriptionLicense{
					SubscriptionId: mockSubscriptionID,
					License:        &subscriptionsv1.EnterpriseSubscriptionLicense_Key{},
				},
			},
			wantError: autogold.Expect("invalid_argument: user_count is invalid"),
		},
		{
			name: "license key: expiration is required",
			create: &subscriptionsv1.CreateEnterpriseSubscriptionLicenseRequest{
				License: &subscriptionsv1.EnterpriseSubscriptionLicense{
					SubscriptionId: mockSubscriptionID,
					License: &subscriptionsv1.EnterpriseSubscriptionLicense_Key{
						Key: &subscriptionsv1.EnterpriseSubscriptionLicenseKey{
							Info: &subscriptionsv1.EnterpriseSubscriptionLicenseKey_Info{
								Tags:      requiredTags,
								UserCount: 100,
							},
						},
					},
				},
			},
			wantError: autogold.Expect("invalid_argument: expiry must be in the future"),
		},
		{
			name: "license key: ok with reason",
			create: &subscriptionsv1.CreateEnterpriseSubscriptionLicenseRequest{
				License: &subscriptionsv1.EnterpriseSubscriptionLicense{
					SubscriptionId: mockSubscriptionID,
					License: &subscriptionsv1.EnterpriseSubscriptionLicense_Key{
						Key: &subscriptionsv1.EnterpriseSubscriptionLicenseKey{
							Info: &subscriptionsv1.EnterpriseSubscriptionLicenseKey_Info{
								Tags:      requiredTags,
								UserCount: 100,
								// time in the future relative to newMockTime
								ExpireTime: timestamppb.New(newMockTime().Add(time.Hour)),
							},
						},
					},
				},
				Message: t.Name(),
			},
			wantKeyOpts: autogold.Expect(&subscriptions.DataLicenseKey{
				Info: license.Info{
					Tags: []string{
						"test",
						"dev",
					},
					UserCount: 100,
					CreatedAt: time.Date(2024,
						1,
						1,
						1,
						1,
						1,
						0,
						time.UTC),
					ExpiresAt: time.Date(2024,
						1,
						1,
						2,
						1,
						0,
						0,
						time.UTC),
				},
				SignedKey: "signedkey-1",
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := connect.NewRequest(tc.create)
			req.Header().Add("Authorization", "Bearer foolmeifyoucan")

			if tc.tokenScopes == nil {
				tc.tokenScopes = scopes.Scopes{
					samsm2m.EnterprisePortalScope(
						scopes.PermissionEnterprisePortalSubscription,
						scopes.ActionWrite,
					),
				}
			}
			h := newTestHandlerV1(t, tc.tokenScopes...)
			h.mockStore.GetEnterpriseSubscriptionFunc.SetDefaultHook(func(ctx context.Context, id string) (*subscriptions.SubscriptionWithConditions, error) {
				if id == mockSubscriptionID {
					return &subscriptions.SubscriptionWithConditions{
						Subscription: subscriptions.Subscription{
							ID: id,
						},
					}, nil
				}
				if id == archivedSubscriptionID {
					return &subscriptions.SubscriptionWithConditions{
						Subscription: subscriptions.Subscription{
							ID: id,
							ArchivedAt: pointers.Ptr(utctime.FromTime(
								// Archived in the past relative to newMockTime
								newMockTime().Add(-1 * time.Hour)),
							),
						},
					}, nil
				}
				return nil, subscriptions.ErrSubscriptionNotFound
			})
			h.mockStore.GetRequiredEnterpriseSubscriptionLicenseKeyTagsFunc.SetDefaultReturn(
				requiredTags,
			)
			h.mockStore.CreateEnterpriseSubscriptionLicenseKeyFunc.SetDefaultHook(func(_ context.Context, subscription string, key *subscriptions.DataLicenseKey, opts subscriptions.CreateLicenseOpts) (*subscriptions.LicenseWithConditions, error) {
				assert.Empty(t, opts.ImportLicenseID)
				// Condition must match upsert
				assert.Equal(t, tc.create.GetMessage(), opts.Message)
				require.NotNil(t, opts.Time)
				assert.Equal(t, key.Info.CreatedAt, opts.Time.AsTime())
				require.NotZero(t, opts.ExpireTime)
				assert.Equal(t, key.Info.ExpiresAt, opts.ExpireTime.AsTime())

				tc.wantKeyOpts.Equal(t, key)

				return &subscriptions.LicenseWithConditions{
					SubscriptionLicense: subscriptions.SubscriptionLicense{
						LicenseType: "ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_KEY",
						LicenseData: json.RawMessage("{}"),
					},
				}, nil
			})
			_, err := h.CreateEnterpriseSubscriptionLicense(ctx, req)
			if tc.wantError != nil {
				require.Error(t, err)
				tc.wantError.Equal(t, err.Error())
			} else {
				require.NoError(t, err)
			}
			if tc.wantKeyOpts != nil {
				mockrequire.CalledOnce(t, h.mockStore.CreateEnterpriseSubscriptionLicenseKeyFunc)
			} else {
				mockrequire.NotCalled(t, h.mockStore.CreateEnterpriseSubscriptionLicenseKeyFunc)
			}
		})
	}
}

func TestHandlerV1_RevokeEnterpriseSubscriptionLicense(t *testing.T) {
	ctx := context.Background()
	const mockLicenseID = "es_80ca12e2-54b4-448c-a61a-390b1a9c1224"

	for _, tc := range []struct {
		name           string
		tokenScopes    scopes.Scopes
		revoke         *subscriptionsv1.RevokeEnterpriseSubscriptionLicenseRequest
		wantRevokeOpts autogold.Value
		wantError      autogold.Value
	}{
		{
			name: "insufficient scopes",
			tokenScopes: scopes.Scopes{
				samsm2m.EnterprisePortalScope(
					scopes.PermissionEnterprisePortalSubscription,
					scopes.ActionRead,
				),
			},
			revoke:    &subscriptionsv1.RevokeEnterpriseSubscriptionLicenseRequest{},
			wantError: autogold.Expect("permission_denied: insufficient scope"),
		},
		{
			name:      "license ID is required",
			revoke:    &subscriptionsv1.RevokeEnterpriseSubscriptionLicenseRequest{},
			wantError: autogold.Expect("invalid_argument: license_id is required"),
		},
		{
			name: "license does not exist",
			revoke: &subscriptionsv1.RevokeEnterpriseSubscriptionLicenseRequest{
				LicenseId: uuid.NewString(),
			},
			wantError: autogold.Expect("not_found: subscription license not found"),
		},
		{
			name: "revoke ok with reason",
			revoke: &subscriptionsv1.RevokeEnterpriseSubscriptionLicenseRequest{
				LicenseId: mockLicenseID,
				Reason:    t.Name(),
			},
			wantRevokeOpts: autogold.Expect(subscriptions.RevokeLicenseOpts{
				Message: "TestHandlerV1_RevokeEnterpriseSubscriptionLicense",
				Time: valast.Ptr(utctime.Date(2024,
					1,
					1,
					1,
					1,
					1,
					0)),
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := connect.NewRequest(tc.revoke)
			req.Header().Add("Authorization", "Bearer foolmeifyoucan")

			if tc.tokenScopes == nil {
				tc.tokenScopes = scopes.Scopes{
					samsm2m.EnterprisePortalScope(
						scopes.PermissionEnterprisePortalSubscription,
						scopes.ActionWrite,
					),
				}
			}
			h := newTestHandlerV1(t, tc.tokenScopes...)
			h.mockStore.RevokeEnterpriseSubscriptionLicenseFunc.SetDefaultHook(func(ctx context.Context, licenseID string, opts subscriptions.RevokeLicenseOpts) (*subscriptions.LicenseWithConditions, error) {
				if licenseID != mockLicenseID {
					return nil, subscriptions.ErrSubscriptionLicenseNotFound
				}

				// Condition must match upsert
				assert.Equal(t, tc.revoke.GetReason(), opts.Message)

				tc.wantRevokeOpts.Equal(t, opts)

				return &subscriptions.LicenseWithConditions{
					SubscriptionLicense: subscriptions.SubscriptionLicense{
						LicenseType: "ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_KEY",
						LicenseData: json.RawMessage("{}"),
					},
				}, nil
			})
			_, err := h.RevokeEnterpriseSubscriptionLicense(ctx, req)
			if tc.wantError != nil {
				require.Error(t, err)
				tc.wantError.Equal(t, err.Error())
			} else {
				require.NoError(t, err)
			}
			if tc.wantRevokeOpts != nil {
				mockrequire.CalledOnce(t, h.mockStore.RevokeEnterpriseSubscriptionLicenseFunc)
			}
		})
	}
}

func TestHandlerV1_UpdateEnterpriseSubscriptionMembership(t *testing.T) {
	const (
		subscriptionID = "80ca12e2-54b4-448c-a61a-390b1a9c1224"
		instanceDomain = "s1.sourcegraph.com"
	)

	type assertIAMWrite struct {
		wantWrites  autogold.Value
		wantDeletes autogold.Value
	}

	for _, tc := range []struct {
		name string
		req  *subscriptionsv1.UpdateEnterpriseSubscriptionMembershipRequest

		iamCheckFunc func(opts iam.CheckOptions) (bool, error)

		iamWrite *assertIAMWrite
	}{
		{
			name: "via subscription ID",
			req: &subscriptionsv1.UpdateEnterpriseSubscriptionMembershipRequest{
				Membership: &subscriptionsv1.EnterpriseSubscriptionMembership{
					SubscriptionId:      subscriptionID,
					MemberSamsAccountId: "018d21f2-04a6-7aaf-9f6f-6fc58c4187b9",
					MemberRoles:         []subscriptionsv1.Role{subscriptionsv1.Role_ROLE_SUBSCRIPTION_CUSTOMER_ADMIN},
				},
			},
			iamWrite: &assertIAMWrite{
				wantWrites: autogold.Expect([]iam.TupleKey{
					{
						Object:        iam.TupleObject("subscription_cody_analytics:80ca12e2-54b4-448c-a61a-390b1a9c1224"),
						TupleRelation: iam.TupleRelation("view"),
						Subject:       iam.TupleSubject("customer_admin:80ca12e2-54b4-448c-a61a-390b1a9c1224#member"),
					},
					{
						Object:        iam.TupleObject("customer_admin:80ca12e2-54b4-448c-a61a-390b1a9c1224"),
						TupleRelation: iam.TupleRelation("member"),
						Subject:       iam.TupleSubject("user:018d21f2-04a6-7aaf-9f6f-6fc58c4187b9"),
					},
				}),
				wantDeletes: autogold.Expect([]iam.TupleKey{}),
			},
		},
		{
			name: "via instance domain",
			req: &subscriptionsv1.UpdateEnterpriseSubscriptionMembershipRequest{
				Membership: &subscriptionsv1.EnterpriseSubscriptionMembership{
					InstanceDomain:      instanceDomain,
					MemberSamsAccountId: "018d21f2-04a6-7aaf-9f6f-6fc58c4187b9",
					MemberRoles:         []subscriptionsv1.Role{subscriptionsv1.Role_ROLE_SUBSCRIPTION_CUSTOMER_ADMIN},
				},
			},
			iamWrite: &assertIAMWrite{
				wantWrites: autogold.Expect([]iam.TupleKey{
					{
						Object:        iam.TupleObject("subscription_cody_analytics:80ca12e2-54b4-448c-a61a-390b1a9c1224"),
						TupleRelation: iam.TupleRelation("view"),
						Subject:       iam.TupleSubject("customer_admin:80ca12e2-54b4-448c-a61a-390b1a9c1224#member"),
					},
					{
						Object:        iam.TupleObject("customer_admin:80ca12e2-54b4-448c-a61a-390b1a9c1224"),
						TupleRelation: iam.TupleRelation("member"),
						Subject:       iam.TupleSubject("user:018d21f2-04a6-7aaf-9f6f-6fc58c4187b9"),
					},
				}),
				wantDeletes: autogold.Expect([]iam.TupleKey{}),
			},
		},
		{
			name: "via subscription ID, customer_admin -> cody analytics already exists",
			req: &subscriptionsv1.UpdateEnterpriseSubscriptionMembershipRequest{
				Membership: &subscriptionsv1.EnterpriseSubscriptionMembership{
					SubscriptionId:      subscriptionID,
					MemberSamsAccountId: "018d21f2-04a6-7aaf-9f6f-6fc58c4187b9",
					MemberRoles:         []subscriptionsv1.Role{subscriptionsv1.Role_ROLE_SUBSCRIPTION_CUSTOMER_ADMIN},
				},
			},
			iamCheckFunc: func(opts iam.CheckOptions) (bool, error) {
				if opts.TupleKey.Object == iam.ToTupleObject(iam.TupleTypeSubscriptionCodyAnalytics, subscriptionID) {
					return true, nil // already exists
				}
				return false, nil
			},
			iamWrite: &assertIAMWrite{
				wantWrites: autogold.Expect([]iam.TupleKey{
					{
						Object:        iam.TupleObject("customer_admin:80ca12e2-54b4-448c-a61a-390b1a9c1224"),
						TupleRelation: iam.TupleRelation("member"),
						Subject:       iam.TupleSubject("user:018d21f2-04a6-7aaf-9f6f-6fc58c4187b9"),
					},
				}),
				wantDeletes: autogold.Expect([]iam.TupleKey{}),
			},
		},
		{
			name: "deletes preexisting roles",
			req: &subscriptionsv1.UpdateEnterpriseSubscriptionMembershipRequest{
				Membership: &subscriptionsv1.EnterpriseSubscriptionMembership{
					InstanceDomain:      "s1.sourcegraph.com",
					MemberSamsAccountId: "018d21f2-04a6-7aaf-9f6f-6fc58c4187b9",
					MemberRoles:         []subscriptionsv1.Role{},
				},
			},
			iamCheckFunc: func(_ iam.CheckOptions) (bool, error) {
				return true, nil // all tuples exist
			},
			iamWrite: &assertIAMWrite{
				wantWrites: autogold.Expect([]iam.TupleKey{}),
				wantDeletes: autogold.Expect([]iam.TupleKey{{
					Object:        iam.TupleObject("customer_admin:80ca12e2-54b4-448c-a61a-390b1a9c1224"),
					TupleRelation: iam.TupleRelation("member"),
					Subject:       iam.TupleSubject("user:018d21f2-04a6-7aaf-9f6f-6fc58c4187b9"),
				}}),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			req := connect.NewRequest(tc.req)
			req.Header().Add("Authorization", "Bearer foolmeifyoucan")

			h := newTestHandlerV1(t,
				samsm2m.EnterprisePortalScope(
					scopes.PermissionEnterprisePortalSubscriptionPermission,
					scopes.ActionWrite,
				),
			)
			h.mockStore.ListEnterpriseSubscriptionsFunc.SetDefaultHook(
				func(_ context.Context, opts subscriptions.ListEnterpriseSubscriptionsOptions) ([]*subscriptions.SubscriptionWithConditions, error) {
					// List should only be called when updating via instance domain
					if slices.Contains(opts.InstanceDomains, instanceDomain) {
						return []*subscriptions.SubscriptionWithConditions{
							{Subscription: subscriptions.Subscription{ID: subscriptionID}},
						}, nil
					}
					return nil, nil
				},
			)
			h.mockStore.GetEnterpriseSubscriptionFunc.SetDefaultHook(
				func(ctx context.Context, id string) (*subscriptions.SubscriptionWithConditions, error) {
					if id == subscriptionID {
						return &subscriptions.SubscriptionWithConditions{
							Subscription: subscriptions.Subscription{
								ID: subscriptionID,
							},
						}, nil
					}
					return nil, subscriptions.ErrSubscriptionNotFound
				},
			)
			h.mockStore.IAMCheckFunc.SetDefaultHook(func(_ context.Context, opts iam.CheckOptions) (bool, error) {
				if tc.iamCheckFunc != nil {
					return tc.iamCheckFunc(opts)
				}
				return false, nil
			})

			_, err := h.UpdateEnterpriseSubscriptionMembership(ctx, req)
			require.NoError(t, err)

			if tc.iamWrite == nil {
				mockrequire.NotCalled(t, h.mockStore.IAMWriteFunc)
			} else {
				mockrequire.Called(t, h.mockStore.IAMWriteFunc)
				assert.Len(t, h.mockStore.IAMWriteFunc.History(), 1,
					"IAMWrite should only be called once")
				w := h.mockStore.IAMWriteFunc.History()[0].Arg1
				tc.iamWrite.wantWrites.Equal(t, w.Writes)
				tc.iamWrite.wantDeletes.Equal(t, w.Deletes)
			}

			if tc.iamCheckFunc != nil { // if mock is provided, presumably it should be called
				mockrequire.Called(t, h.mockStore.IAMCheckFunc)
			}
		})
	}
}
