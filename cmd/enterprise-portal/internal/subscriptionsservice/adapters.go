package subscriptionsservice

import (
	"encoding/json"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/subscriptions"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/utctime"
	"github.com/sourcegraph/sourcegraph/internal/license"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/iam"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func convertLicenseToProto(license *subscriptions.LicenseWithConditions) (*subscriptionsv1.EnterpriseSubscriptionLicense, error) {
	conds := make([]*subscriptionsv1.EnterpriseSubscriptionLicenseCondition, len(license.Conditions))
	for i, c := range license.Conditions {
		conds[i] = &subscriptionsv1.EnterpriseSubscriptionLicenseCondition{
			LastTransitionTime: timestamppb.New(c.TransitionTime.AsTime()),
			Status: subscriptionsv1.EnterpriseSubscriptionLicenseCondition_Status(
				subscriptionsv1.EnterpriseSubscriptionLicenseCondition_Status_value[c.Status],
			),
			Message: pointers.DerefZero(c.Message),
		}
	}

	proto := &subscriptionsv1.EnterpriseSubscriptionLicense{
		Id:             subscriptionsv1.EnterpriseSubscriptionLicenseIDPrefix + license.ID,
		SubscriptionId: subscriptionsv1.EnterpriseSubscriptionIDPrefix + license.SubscriptionID,
		Conditions:     conds,
	}

	switch t := license.LicenseType; t {
	case subscriptionsv1.EnterpriseSubscriptionLicenseType_ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_KEY.String():
		var data subscriptions.DataLicenseKey
		if err := json.Unmarshal(license.LicenseData, &data); err != nil {
			return proto, errors.Wrapf(err, "unmarshal license data: %q", string(license.LicenseData))
		}
		proto.License = &subscriptionsv1.EnterpriseSubscriptionLicense_Key{
			Key: &subscriptionsv1.EnterpriseSubscriptionLicenseKey{
				InfoVersion: uint32(data.Info.Version()),
				Info: &subscriptionsv1.EnterpriseSubscriptionLicenseKey_Info{
					Tags:                     data.Info.Tags,
					UserCount:                uint64(data.Info.UserCount),
					ExpireTime:               timestamppb.New(data.Info.ExpiresAt),
					SalesforceSubscriptionId: pointers.DerefZero(data.Info.SalesforceSubscriptionID),
					SalesforceOpportunityId:  pointers.DerefZero(data.Info.SalesforceOpportunityID),
				},
				LicenseKey:      data.SignedKey,
				PlanDisplayName: licensing.ProductNameWithBrand(data.Info.Tags),
			},
		}

	default:
		return proto, errors.Newf("unknown license type %q", t)
	}

	return proto, nil
}

func convertSubscriptionToProto(subscription *subscriptions.SubscriptionWithConditions) *subscriptionsv1.EnterpriseSubscription {
	conds := make([]*subscriptionsv1.EnterpriseSubscriptionCondition, len(subscription.Conditions))
	for i, c := range subscription.Conditions {
		conds[i] = &subscriptionsv1.EnterpriseSubscriptionCondition{
			LastTransitionTime: timestamppb.New(c.TransitionTime.AsTime()),
			Status: subscriptionsv1.EnterpriseSubscriptionCondition_Status(
				subscriptionsv1.EnterpriseSubscriptionCondition_Status_value[c.Status],
			),
			Message: pointers.DerefZero(c.Message),
		}
	}

	var sf *subscriptionsv1.EnterpriseSubscriptionSalesforceMetadata
	if subscription.SalesforceSubscriptionID != nil {
		sf = &subscriptionsv1.EnterpriseSubscriptionSalesforceMetadata{
			SubscriptionId: pointers.DerefZero(subscription.SalesforceSubscriptionID),
		}
	}

	var instanceType subscriptionsv1.EnterpriseSubscriptionInstanceType
	if subscription.InstanceType != nil {
		instanceType = subscriptionsv1.EnterpriseSubscriptionInstanceType(
			subscriptionsv1.EnterpriseSubscriptionInstanceType_value[*subscription.InstanceType],
		)
	}

	return &subscriptionsv1.EnterpriseSubscription{
		Id:             subscriptionsv1.EnterpriseSubscriptionIDPrefix + subscription.ID,
		Conditions:     conds,
		InstanceDomain: pointers.DerefZero(subscription.InstanceDomain),
		InstanceType:   instanceType,
		DisplayName:    pointers.DerefZero(subscription.DisplayName),
		Salesforce:     sf,
	}
}

func convertProtoToIAMTupleObjectType(typ subscriptionsv1.PermissionType) iam.TupleType {
	switch typ {
	case subscriptionsv1.PermissionType_PERMISSION_TYPE_SUBSCRIPTION_CODY_ANALYTICS:
		return iam.TupleTypeSubscriptionCodyAnalytics
	default:
		return ""
	}
}

func convertProtoToIAMTupleRelation(action subscriptionsv1.PermissionRelation) iam.TupleRelation {
	switch action {
	case subscriptionsv1.PermissionRelation_PERMISSION_RELATION_VIEW:
		return iam.TupleRelationView
	default:
		return ""
	}
}

func convertProtoRoleToIAMTupleObject(role subscriptionsv1.Role, subscriptionID string) iam.TupleObject {
	switch role {
	case subscriptionsv1.Role_ROLE_SUBSCRIPTION_CUSTOMER_ADMIN:
		return iam.ToTupleObject(iam.TupleTypeCustomerAdmin,
			strings.TrimPrefix(subscriptionID, subscriptionsv1.EnterpriseSubscriptionIDPrefix))
	default:
		return ""
	}
}

// convertLicenseKeyToLicenseKeyData converts a create-license request into an
// actual license key for creating a database entry.
//
// It may return Connect errors - all other errors should be considered internal
// errors.
func convertLicenseKeyToLicenseKeyData(
	createdAt utctime.Time,
	sub *subscriptions.Subscription,
	key *subscriptionsv1.EnterpriseSubscriptionLicenseKey,
	// StoreV1.GetRequiredEnterpriseSubscriptionLicenseKeyTags
	requiredTags []string,
	// StoreV1.SignEnterpriseSubscriptionLicenseKey
	signKeyFn func(license.Info) (string, error),
) (*subscriptions.DataLicenseKey, error) {
	if key.GetInfo().GetUserCount() == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_count is invalid"))
	}
	expires := key.GetInfo().GetExpireTime().AsTime()
	if expires.Before(createdAt.AsTime()) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("expiry must be in the future"))
	}
	tags := key.GetInfo().GetTags()
	providedTagPrefixes := map[string]struct{}{}
	for _, t := range tags {
		providedTagPrefixes[strings.SplitN(t, ":", 2)[0]] = struct{}{}
	}
	if _, exists := providedTagPrefixes["customer"]; !exists && sub.DisplayName != nil {
		tags = append(tags, fmt.Sprintf("customer:%s", *sub.DisplayName))
	}
	for _, r := range requiredTags {
		if _, ok := providedTagPrefixes[r]; !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				errors.Newf("key tags [%s] are required", strings.Join(requiredTags, ", ")))
		}
	}

	info := license.Info{
		Tags:      tags,
		UserCount: uint(key.GetInfo().GetUserCount()),
		CreatedAt: createdAt.AsTime(),
		// Cast expiry to utctime and back for uniform representation
		ExpiresAt: utctime.FromTime(expires).AsTime(),
		// Provided at creation
		SalesforceOpportunityID: pointers.NilIfZero(key.GetInfo().GetSalesforceOpportunityId()),
		// Inherited from subscription
		SalesforceSubscriptionID: sub.SalesforceSubscriptionID,
	}
	signedKey, err := signKeyFn(info)
	if err != nil {
		// See StoreV1.SignEnterpriseSubscriptionLicenseKey
		if errors.Is(err, errStoreUnimplemented) {
			return nil, connect.NewError(connect.CodeUnimplemented,
				errors.Wrap(err, "key signing not available"))
		}
		return nil, errors.Wrap(err, "sign key")
	}

	return &subscriptions.DataLicenseKey{
		Info:      info,
		SignedKey: signedKey,
	}, nil
}
