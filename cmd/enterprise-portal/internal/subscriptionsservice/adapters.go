package subscriptionsservice

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/subscriptions"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/utctime"
	"github.com/sourcegraph/sourcegraph/internal/license"
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
			return proto, errors.Wrap(err, "unmarshal license data")
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
				LicenseKey: data.SignedKey,
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

	return &subscriptionsv1.EnterpriseSubscription{
		Id:             subscriptionsv1.EnterpriseSubscriptionIDPrefix + subscription.ID,
		Conditions:     conds,
		InstanceDomain: pointers.DerefZero(subscription.InstanceDomain),
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
// actual license key. It only returns valid Connect errors.
func convertLicenseKeyToLicenseKeyData(
	createdAt utctime.Time,
	sub *subscriptions.Subscription,
	key *subscriptionsv1.EnterpriseSubscriptionLicenseKey,
) (*subscriptions.DataLicenseKey, error) {
	expires := key.GetInfo().GetExpireTime().AsTime()
	if expires.Before(time.Now()) {
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

	info := license.Info{
		Tags:      tags,
		UserCount: uint(key.GetInfo().GetUserCount()),
		CreatedAt: createdAt.AsTime(),
		ExpiresAt: expires.UTC(),

		// Inherited from subscription
		SalesforceSubscriptionID: sub.SalesforceSubscriptionID,
		SalesforceOpportunityID:  sub.SalesforceOpportunityID,
	}

	// TODO
	// signedKey, _, err := licensing.GenerateProductLicenseKey(info)
	// if err != nil {
	// 	return nil, connectutil.InternalError(ctx, logger, err, "")
	// }

	return &subscriptions.DataLicenseKey{
		Info:      info,
		SignedKey: "TODO",
	}, nil
}
