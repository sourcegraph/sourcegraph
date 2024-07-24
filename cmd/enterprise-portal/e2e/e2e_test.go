package qa

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sams "github.com/sourcegraph/sourcegraph-accounts-sdk-go"
	"github.com/sourcegraph/sourcegraph-accounts-sdk-go/scopes"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/grpc/grpcoauth"

	codyaccessv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/codyaccess/v1"
	subscriptionsv1 "github.com/sourcegraph/sourcegraph/lib/enterpriseportal/subscriptions/v1"
)

type Clients struct {
	Subscriptions subscriptionsv1.SubscriptionsServiceClient
	CodyAccess    codyaccessv1.CodyAccessServiceClient
}

func newE2EClients(t *testing.T) *Clients {
	os.Setenv("INSECURE_DEV", "true")

	server, ok := os.LookupEnv("EP_E2E_ENTERPRISEPORTAL_SERVER")
	if !ok || server == "" {
		t.Skip("E2E_GATEWAY_ENDPOINT must be set, skipping")
	}
	addr, err := url.Parse(server)
	require.NoError(t, err)
	if addr.Hostname() != "127.0.0.1" && addr.Hostname() != "localhost" { // CI:LOCALHOST_OK
		// For now, prevent us from running the test against a live instance
		t.Error("EP_E2E_ENTERPRISEPORTAL_SERVER must not be a live Enterprise Portal instance")
		t.FailNow()
	}

	samsServer := os.Getenv("EP_E2E_SAMS_SERVER")
	clientID := os.Getenv("EP_E2E_SAMS_CLIENT_ID")
	clientSecret := os.Getenv("EP_E2E_SAMS_CLIENT_SECRET")
	if samsServer == "" || clientID == "" || clientSecret == "" {
		t.Error("EP_E2E_SAMS_SERVER, EP_E2E_SAMS_CLIENT_ID, and EP_E2E_SAMS_CLIENT_SECRET must be set")
		t.FailNow()
	}

	t.Logf(`== Enterprise Portal E2E testing
- Enterprise Portal: %s
- SAMS server: %s
- Client ID: %s`, server, samsServer, clientID)

	ts := sams.ClientCredentialsTokenSource(
		sams.ConnConfig{ExternalURL: samsServer},
		clientID,
		clientSecret,
		[]scopes.Scope{
			scopes.ToScope(scopes.ServiceEnterprisePortal, "codyaccess", scopes.ActionRead),
			scopes.ToScope(scopes.ServiceEnterprisePortal, "codyaccess", scopes.ActionWrite),
			scopes.ToScope(scopes.ServiceEnterprisePortal, "subscription", scopes.ActionRead),
			scopes.ToScope(scopes.ServiceEnterprisePortal, "subscription", scopes.ActionWrite),
		},
	)
	_, err = ts.Token()
	require.NoError(t, err)
	creds := grpc.WithPerRPCCredentials(grpcoauth.TokenSource{TokenSource: ts})

	client, err := grpc.NewClient("dns:///"+addr.Host,
		defaults.DialOptions(logtest.Scoped(t).Scoped("grpc"), creds)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	return &Clients{
		Subscriptions: subscriptionsv1.NewSubscriptionsServiceClient(client),
		CodyAccess:    codyaccessv1.NewCodyAccessServiceClient(client),
	}
}

func prettyPrint(t *testing.T, m proto.Message) {
	data, err := protojson.MarshalOptions{Multiline: true}.Marshal(m)
	require.NoError(t, err)
	t.Log(string(data))
}

func TestEnterprisePortal(t *testing.T) {
	clients := newE2EClients(t)

	ctx := context.Background()
	if deadline, ok := t.Deadline(); ok {
		var cancel func()
		ctx, cancel = context.WithDeadline(ctx, deadline)
		t.Cleanup(cancel)
	}

	// Run several in parallel, for a more realistic emulation.
	runID := time.Now().UnixMilli()
	for idx := 0; idx < 3; idx += 1 {
		t.Run(fmt.Sprintf("Run %d", idx), func(t *testing.T) {
			t.Parallel()

			runLifecycleTest(t, ctx, clients, fmt.Sprintf("%d-%d", runID, idx))
		})
	}
}

func runLifecycleTest(t *testing.T, ctx context.Context, clients *Clients, runID string) {
	subscriptionName := fmt.Sprintf("E2E test %s", runID)
	subscriptionDomain := fmt.Sprintf("%s.e2etest.org", runID)

	var createdSubscriptionID string
	t.Run("Create subscription", func(t *testing.T) {
		got, err := clients.Subscriptions.CreateEnterpriseSubscription(
			ctx,
			&subscriptionsv1.CreateEnterpriseSubscriptionRequest{
				Subscription: &subscriptionsv1.EnterpriseSubscription{
					DisplayName:    subscriptionName,
					InstanceDomain: subscriptionDomain,
				},
				Message: "E2E test",
			},
		)
		require.NoError(t, err)
		createdSubscriptionID = got.GetSubscription().GetId()
		prettyPrint(t, got)
	})

	t.Run("Update subscription with domain", func(t *testing.T) {
		got, err := clients.Subscriptions.UpdateEnterpriseSubscription(ctx, &subscriptionsv1.UpdateEnterpriseSubscriptionRequest{
			Subscription: &subscriptionsv1.EnterpriseSubscription{
				Id:             createdSubscriptionID,
				InstanceDomain: subscriptionDomain,
			},
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"instance_domain"},
			},
		})
		require.NoError(t, err)
		prettyPrint(t, got)
	})

	t.Run("Get subscription", func(t *testing.T) {
		got, err := clients.Subscriptions.GetEnterpriseSubscription(ctx, &subscriptionsv1.GetEnterpriseSubscriptionRequest{
			Query: &subscriptionsv1.GetEnterpriseSubscriptionRequest_Id{
				Id: createdSubscriptionID,
			},
		})
		require.NoError(t, err)
		prettyPrint(t, got)
	})

	var createdLicenseID string
	t.Run("Create license", func(t *testing.T) {
		got, err := clients.Subscriptions.CreateEnterpriseSubscriptionLicense(ctx, &subscriptionsv1.CreateEnterpriseSubscriptionLicenseRequest{
			License: &subscriptionsv1.EnterpriseSubscriptionLicense{
				SubscriptionId: createdSubscriptionID,
				License: &subscriptionsv1.EnterpriseSubscriptionLicense_Key{
					Key: &subscriptionsv1.EnterpriseSubscriptionLicenseKey{
						Info: &subscriptionsv1.EnterpriseSubscriptionLicenseKey_Info{
							Tags:       []string{"dev", "e2e"},
							UserCount:  123,
							ExpireTime: timestamppb.New(time.Now().Add(time.Hour)),
						},
					},
				},
			},
			Message: "E2E test",
		})
		require.NoError(t, err)
		createdLicenseID = got.GetLicense().GetId()
		prettyPrint(t, got)
	})

	t.Run("Get license", func(t *testing.T) {
		got, err := clients.Subscriptions.ListEnterpriseSubscriptionLicenses(ctx, &subscriptionsv1.ListEnterpriseSubscriptionLicensesRequest{
			Filters: []*subscriptionsv1.ListEnterpriseSubscriptionLicensesFilter{
				{
					Filter: &subscriptionsv1.ListEnterpriseSubscriptionLicensesFilter_SubscriptionId{
						SubscriptionId: createdSubscriptionID,
					},
				},
			},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, got.GetLicenses())
		assert.Equal(t, createdLicenseID, got.GetLicenses()[0].GetId())
		prettyPrint(t, got)
	})

	t.Run("Get Cody Gateway access", func(t *testing.T) {
		got, err := clients.CodyAccess.GetCodyGatewayAccess(ctx, &codyaccessv1.GetCodyGatewayAccessRequest{
			Query: &codyaccessv1.GetCodyGatewayAccessRequest_SubscriptionId{
				SubscriptionId: createdSubscriptionID,
			},
		})
		require.NoError(t, err)
		assert.False(t, got.GetAccess().GetEnabled(),
			"newly created subscription should be disabled")
		prettyPrint(t, got)
	})

	t.Run("Update Cody Gateway access", func(t *testing.T) {
		got, err := clients.CodyAccess.UpdateCodyGatewayAccess(ctx, &codyaccessv1.UpdateCodyGatewayAccessRequest{
			Access: &codyaccessv1.CodyGatewayAccess{
				SubscriptionId: createdSubscriptionID,
				Enabled:        true,
			},
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"enabled"},
			},
		})
		require.NoError(t, err)
		assert.True(t, got.GetAccess().GetEnabled())
		prettyPrint(t, got)
	})

	t.Run("Revoke license", func(t *testing.T) {
		got, err := clients.Subscriptions.RevokeEnterpriseSubscriptionLicense(ctx, &subscriptionsv1.RevokeEnterpriseSubscriptionLicenseRequest{
			LicenseId: createdLicenseID,
		})
		require.NoError(t, err)
		prettyPrint(t, got)

		t.Run("Get Cody Gateway access", func(t *testing.T) {
			got, err := clients.CodyAccess.GetCodyGatewayAccess(ctx, &codyaccessv1.GetCodyGatewayAccessRequest{
				Query: &codyaccessv1.GetCodyGatewayAccessRequest_SubscriptionId{
					SubscriptionId: createdSubscriptionID,
				},
			})
			require.NoError(t, err)
			prettyPrint(t, got)
		})
	})

	t.Run("Archive subscription", func(t *testing.T) {
		got, err := clients.Subscriptions.ArchiveEnterpriseSubscription(ctx, &subscriptionsv1.ArchiveEnterpriseSubscriptionRequest{
			SubscriptionId: createdSubscriptionID,
		})
		require.NoError(t, err)
		prettyPrint(t, got)

		t.Run("Get Cody Gateway access", func(t *testing.T) {
			_, err := clients.CodyAccess.GetCodyGatewayAccess(ctx, &codyaccessv1.GetCodyGatewayAccessRequest{
				Query: &codyaccessv1.GetCodyGatewayAccessRequest_SubscriptionId{
					SubscriptionId: createdSubscriptionID,
				},
			})
			require.Error(t, err)
			t.Logf("Got expected error: %s", err.Error())
		})
	})
}
