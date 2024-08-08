package subscriptionsservice

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/hexops/autogold/v2"
	"github.com/hexops/valast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/subscriptions"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/utctime"
	"github.com/sourcegraph/sourcegraph/internal/license"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// Protojson output isn't stable by injecting randomized whitespace,
// so we re-marshal it to stabilize the output for golden tests.
// https://github.com/golang/protobuf/issues/1082
func mustMarshalStableProtoJSON(t *testing.T, m protoreflect.ProtoMessage) string {
	t.Helper()

	protoJSON, err := protojson.Marshal(m)
	require.NoError(t, err)

	var gotJSON map[string]any
	require.NoError(t, json.Unmarshal(protoJSON, &gotJSON))
	return mustMarshal(json.MarshalIndent(gotJSON, "", "  "))
}

func mustMarshal(d []byte, err error) string {
	if err != nil {
		return err.Error()
	}
	return string(d)
}

func TestConvertLicenseToProto(t *testing.T) {
	created := utctime.FromTime(newMockTime())
	expired := utctime.FromTime(newMockTime().Add(1 * time.Hour))
	got, err := convertLicenseToProto(&subscriptions.LicenseWithConditions{
		SubscriptionLicense: subscriptions.SubscriptionLicense{
			SubscriptionID: "subscription_id",
			ID:             "license_id",
			CreatedAt:      created,
			ExpireAt:       expired,
			LicenseType:    "ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_KEY",
			// In format subscriptions.DataLicenseKey
			LicenseData: json.RawMessage(fmt.Sprintf(`{
				"SignedKey": "asdf",
				"Info": {"e":%s,"c":%s,"t":["foo"]}
			}`,
				mustMarshal(expired.AsTime().MarshalJSON()),
				mustMarshal(created.AsTime().MarshalJSON()))),
		},
		Conditions: []subscriptions.SubscriptionLicenseCondition{{
			TransitionTime: created,
			Status:         "STATUS_CREATED",
		}},
	})
	require.NoError(t, err)

	autogold.Expect(`{
  "conditions": [
    {
      "lastTransitionTime": "2024-01-01T01:01:00Z",
      "status": "STATUS_CREATED"
    }
  ],
  "id": "esl_license_id",
  "key": {
    "info": {
      "expireTime": "2024-01-01T02:01:00Z",
      "tags": [
        "foo"
      ]
    },
    "infoVersion": 1,
    "licenseKey": "asdf",
    "planDisplayName": "Sourcegraph Enterprise"
  },
  "subscriptionId": "es_subscription_id"
}`).Equal(t, mustMarshalStableProtoJSON(t, got))
}

func TestConvertSubscriptionToProto(t *testing.T) {
	created := newMockTime()
	for _, tc := range []struct {
		name string
		sub  *subscriptions.SubscriptionWithConditions
		want autogold.Value
	}{{
		name: "without salesforce details",
		sub: &subscriptions.SubscriptionWithConditions{
			Subscription: subscriptions.Subscription{
				ID:             "subscription_id",
				InstanceDomain: pointers.Ptr("sourcegraph.com"),
				CreatedAt:      utctime.Time(created),
			},
			Conditions: []subscriptions.SubscriptionCondition{{
				TransitionTime: utctime.Time(created),
				Status:         "STATUS_CREATED",
			}},
		},
		want: autogold.Expect(`{
  "conditions": [
    {
      "lastTransitionTime": "2024-01-01T01:01:00Z",
      "status": "STATUS_CREATED"
    }
  ],
  "id": "es_subscription_id",
  "instanceDomain": "sourcegraph.com"
}`),
	}, {
		name: "with salesforce details",
		sub: &subscriptions.SubscriptionWithConditions{
			Subscription: subscriptions.Subscription{
				ID:                       "subscription_id",
				DisplayName:              pointers.Ptr("s2"),
				CreatedAt:                utctime.Time(created),
				SalesforceSubscriptionID: pointers.Ptr("sf_sub_id"),
			},
			Conditions: []subscriptions.SubscriptionCondition{{
				TransitionTime: utctime.Time(created),
				Status:         "STATUS_CREATED",
			}},
		},
		want: autogold.Expect(`{
  "conditions": [
    {
      "lastTransitionTime": "2024-01-01T01:01:00Z",
      "status": "STATUS_CREATED"
    }
  ],
  "displayName": "s2",
  "id": "es_subscription_id",
  "salesforce": {
    "subscriptionId": "sf_sub_id"
  }
}`),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := convertSubscriptionToProto(tc.sub)

			tc.want.Equal(t, mustMarshalStableProtoJSON(t, got))
		})
	}
}

func TestConvertProtoToIAMTupleObjectType(t *testing.T) {
	// Assert full coverage on API enum values.
	for tid, name := range subscriptionsv1.PermissionType_name {
		typ := subscriptionsv1.PermissionType(tid)
		if typ == subscriptionsv1.PermissionType_PERMISSION_TYPE_UNSPECIFIED {
			continue
		}
		t.Run(name, func(t *testing.T) {
			tupleType := convertProtoToIAMTupleObjectType(typ)
			assert.NotEmpty(t, tupleType)
		})
	}
}

func TestConvertProtoToIAMTupleRelation(t *testing.T) {
	// Assert full coverage on API enum values.
	for pid, name := range subscriptionsv1.PermissionRelation_name {
		action := subscriptionsv1.PermissionRelation(pid)
		if action == subscriptionsv1.PermissionRelation_PERMISSION_RELATION_UNSPECIFIED {
			continue
		}
		t.Run(name, func(t *testing.T) {
			relation := convertProtoToIAMTupleRelation(action)
			assert.NotEmpty(t, relation)
		})
	}
}

func TestConvertProtoRoleToIAMTupleObject(t *testing.T) {
	// Assert full coverage on API enum values.
	for rid, name := range subscriptionsv1.Role_name {
		role := subscriptionsv1.Role(rid)
		if role == subscriptionsv1.Role_ROLE_UNSPECIFIED {
			continue
		}
		t.Run(name, func(t *testing.T) {
			roleObject := convertProtoRoleToIAMTupleObject(role, "foobar")
			assert.NotEmpty(t, roleObject)
		})
	}
}

func TestConvertLicenseKeyToLicenseKeyData(t *testing.T) {
	created := utctime.FromTime(newMockTime())
	for _, tc := range []struct {
		name         string
		sub          *subscriptions.Subscription
		key          *subscriptionsv1.EnterpriseSubscriptionLicenseKey
		requiredTags []string

		wantError autogold.Value
		wantData  autogold.Value
	}{{
		name: "invalid expiry",
		sub:  &subscriptions.Subscription{},
		key: &subscriptionsv1.EnterpriseSubscriptionLicenseKey{
			Info: &subscriptionsv1.EnterpriseSubscriptionLicenseKey_Info{
				UserCount:  123,
				ExpireTime: timestamppb.New(created.AsTime().Add(-time.Hour)),
			},
		},
		wantError: autogold.Expect("invalid_argument: expiry must be in the future"),
	}, {
		name: "missing required tag",
		sub:  &subscriptions.Subscription{},
		key: &subscriptionsv1.EnterpriseSubscriptionLicenseKey{
			Info: &subscriptionsv1.EnterpriseSubscriptionLicenseKey_Info{
				UserCount:  123,
				ExpireTime: timestamppb.New(created.AsTime().Add(time.Hour)),
			},
		},
		requiredTags: []string{"dev"},
		wantError:    autogold.Expect("invalid_argument: key tags [dev] are required"),
	}, {
		name: "has required tag",
		sub:  &subscriptions.Subscription{},
		key: &subscriptionsv1.EnterpriseSubscriptionLicenseKey{
			Info: &subscriptionsv1.EnterpriseSubscriptionLicenseKey_Info{
				UserCount:  123,
				ExpireTime: timestamppb.New(created.AsTime().Add(time.Hour)),
				Tags:       []string{"dev", "plan"},
			},
		},
		requiredTags: []string{"dev"},
		wantData: autogold.Expect(&subscriptions.DataLicenseKey{
			Info: license.Info{
				Tags: []string{
					"dev",
					"plan",
				},
				UserCount: 123,
				CreatedAt: time.Date(2024,
					1,
					1,
					1,
					1,
					0,
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
			SignedKey: "signed-key",
		}),
	}, {
		name: "adds display name as customer tag",
		sub: &subscriptions.Subscription{
			DisplayName: pointers.Ptr(t.Name()),
		},
		key: &subscriptionsv1.EnterpriseSubscriptionLicenseKey{
			Info: &subscriptionsv1.EnterpriseSubscriptionLicenseKey_Info{
				UserCount:  123,
				ExpireTime: timestamppb.New(created.AsTime().Add(time.Hour)),
			},
		},
		wantData: autogold.Expect(&subscriptions.DataLicenseKey{
			Info: license.Info{
				Tags:      []string{"customer:TestConvertLicenseKeyToLicenseKeyData"},
				UserCount: 123,
				CreatedAt: time.Date(2024,
					1,
					1,
					1,
					1,
					0,
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
			SignedKey: "signed-key",
		}),
	}, {
		name: "respects existing customer tag",
		sub: &subscriptions.Subscription{
			DisplayName: pointers.Ptr(t.Name()),
		},
		key: &subscriptionsv1.EnterpriseSubscriptionLicenseKey{
			Info: &subscriptionsv1.EnterpriseSubscriptionLicenseKey_Info{
				UserCount:  123,
				ExpireTime: timestamppb.New(created.AsTime().Add(time.Hour)),
				Tags:       []string{"customer:custom-customer"},
			},
		},
		wantData: autogold.Expect(&subscriptions.DataLicenseKey{
			Info: license.Info{
				Tags:      []string{"customer:custom-customer"},
				UserCount: 123,
				CreatedAt: time.Date(2024,
					1,
					1,
					1,
					1,
					0,
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
			SignedKey: "signed-key",
		}),
	}, {
		name: "adds salesforce metadata",
		sub: &subscriptions.Subscription{
			SalesforceSubscriptionID: pointers.Ptr("sf_sub_id"),
		},
		key: &subscriptionsv1.EnterpriseSubscriptionLicenseKey{
			Info: &subscriptionsv1.EnterpriseSubscriptionLicenseKey_Info{
				UserCount:  123,
				ExpireTime: timestamppb.New(created.AsTime().Add(time.Hour)),
			},
		},
		wantData: autogold.Expect(&subscriptions.DataLicenseKey{
			Info: license.Info{
				UserCount: 123,
				CreatedAt: time.Date(2024,
					1,
					1,
					1,
					1,
					0,
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
				SalesforceSubscriptionID: valast.Ptr("sf_sub_id"),
			},
			SignedKey: "signed-key",
		}),
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := convertLicenseKeyToLicenseKeyData(
				created,
				tc.sub,
				tc.key,
				tc.requiredTags,
				func(i license.Info) (string, error) {
					return "signed-key", nil
				},
			)
			if tc.wantError != nil {
				require.Error(t, err)
				tc.wantError.Equal(t, err.Error())
			} else {
				require.NoError(t, err)
				tc.wantData.Equal(t, got)
			}
		})
	}
}
