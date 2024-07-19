package subscriptionsservice

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"testing"

	"connectrpc.com/connect"
	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/hexops/autogold/v2"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	sams "github.com/sourcegraph/sourcegraph-accounts-sdk-go"
	"github.com/sourcegraph/sourcegraph-accounts-sdk-go/scopes"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/subscriptions"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/samsm2m"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/iam"
)

type testHandlerV1 struct {
	*handlerV1
	mockStore *MockStoreV1
}

func newTestHandlerV1() *testHandlerV1 {
	mockStore := NewMockStoreV1()
	mockStore.IntrospectSAMSTokenFunc.SetDefaultReturn(
		&sams.IntrospectTokenResponse{
			Active: true,
			Scopes: scopes.Scopes{
				samsm2m.EnterprisePortalScope("subscription", scopes.ActionRead),
				samsm2m.EnterprisePortalScope("subscription", scopes.ActionWrite),
				samsm2m.EnterprisePortalScope("permission.subscription", scopes.ActionWrite),
			},
		},
		nil,
	)
	return &testHandlerV1{
		handlerV1: &handlerV1{
			logger: logtest.NoOp(nil),
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

			h := newTestHandlerV1()
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

func TestHandlerV1_UpdateEnterpriseSubscription(t *testing.T) {
	ctx := context.Background()
	const mockSubscriptionID = "es_80ca12e2-54b4-448c-a61a-390b1a9c1224"

	for _, tc := range []struct {
		name           string
		update         *subscriptionsv1.UpdateEnterpriseSubscriptionRequest
		wantUpdateOpts autogold.Value
	}{
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

			h := newTestHandlerV1()
			h.mockStore.ListEnterpriseSubscriptionsFunc.SetDefaultReturn(
				[]*subscriptions.SubscriptionWithConditions{
					{Subscription: subscriptions.Subscription{
						ID: "80ca12e2-54b4-448c-a61a-390b1a9c1224",
					}},
				}, nil)
			h.mockStore.UpsertEnterpriseSubscriptionFunc.SetDefaultHook(func(_ context.Context, _ string, opts subscriptions.UpsertSubscriptionOptions, conds ...subscriptions.CreateSubscriptionConditionOptions) (*subscriptions.SubscriptionWithConditions, error) {
				tc.wantUpdateOpts.Equal(t, opts)
				assert.Len(t, conds, 0) // no conditions for standard updates
				return &subscriptions.SubscriptionWithConditions{}, nil
			})
			_, err := h.UpdateEnterpriseSubscription(ctx, req)
			require.NoError(t, err)
			if tc.wantUpdateOpts != nil {
				mockrequire.CalledOnce(t, h.mockStore.UpsertEnterpriseSubscriptionFunc)
			} else {
				mockrequire.NotCalled(t, h.mockStore.UpsertEnterpriseSubscriptionFunc)
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

			h := newTestHandlerV1()
			h.mockStore.ListEnterpriseSubscriptionsFunc.SetDefaultHook(
				func(_ context.Context, opts subscriptions.ListEnterpriseSubscriptionsOptions) ([]*subscriptions.SubscriptionWithConditions, error) {
					if slices.Contains(opts.IDs, subscriptionID) ||
						slices.Contains(opts.InstanceDomains, instanceDomain) {
						return []*subscriptions.SubscriptionWithConditions{
							{Subscription: subscriptions.Subscription{ID: subscriptionID}},
						}, nil
					}
					return nil, nil
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
