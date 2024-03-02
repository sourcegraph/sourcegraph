package productsubscription

import (
	"context"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestServiceAccountOrOwnerOrSiteAdmin(t *testing.T) {
	var actorID, anotherID int32 = 1, 2
	for _, tc := range []struct {
		name           string
		featureFlags   map[string]bool
		actorSiteAdmin bool

		ownerUserID            *int32
		serviceAccountCanWrite bool

		wantGrantReason string
		wantErr         autogold.Value
	}{
		{
			name: "reader service account",
			featureFlags: map[string]bool{
				featureFlagProductSubscriptionsReaderServiceAccount: true,
			},
			wantErr:         nil,
			wantGrantReason: "reader_service_account",
		},
		{
			name: "service account",
			featureFlags: map[string]bool{
				featureFlagProductSubscriptionsServiceAccount: true,
			},
			wantErr:         nil,
			wantGrantReason: "writer_service_account",
		},
		{
			name:            "same user",
			ownerUserID:     &actorID,
			wantErr:         nil,
			wantGrantReason: "same_user_or_site_admin",
		},
		{
			name:        "different user",
			ownerUserID: &anotherID,
			wantErr:     autogold.Expect("must be authenticated as the authorized user or site admin"),
		},
		{
			name:            "site admin",
			actorSiteAdmin:  true,
			wantErr:         nil,
			wantGrantReason: "site_admin",
		},
		{
			name:            "site admin can access another user",
			actorSiteAdmin:  true,
			ownerUserID:     &anotherID,
			wantErr:         nil,
			wantGrantReason: "same_user_or_site_admin",
		},
		{
			name:    "not a site admin, not accessing a user-specific resource",
			wantErr: autogold.Expect("must be site admin"),
		},
		{
			name: "service account needs writer flag",
			featureFlags: map[string]bool{
				featureFlagProductSubscriptionsReaderServiceAccount: true,
			},
			serviceAccountCanWrite: true,
			wantErr:                autogold.Expect("must be site admin"),
		},
		{
			name: "service account fulfills writer flag",
			featureFlags: map[string]bool{
				featureFlagProductSubscriptionsServiceAccount: true,
			},
			serviceAccountCanWrite: true,
			wantErr:                nil,
			wantGrantReason:        "writer_service_account",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			t.Parallel()

			db := dbmocks.NewMockDB()
			mockUsers := dbmocks.NewMockUserStore()

			user := &types.User{ID: actorID, SiteAdmin: tc.actorSiteAdmin}
			mockUsers.GetByCurrentAuthUserFunc.SetDefaultReturn(user, nil)
			mockUsers.GetByIDFunc.SetDefaultReturn(user, nil)

			db.UsersFunc.SetDefaultReturn(mockUsers)

			ffStore := dbmocks.NewMockFeatureFlagStore()
			ffStore.GetUserFlagsFunc.SetDefaultReturn(tc.featureFlags, nil)
			db.FeatureFlagsFunc.SetDefaultReturn(ffStore)

			// Test that a feature flag store with potential overrides on the context
			// is NOT used. We don't want to allow ovverriding service account checks.
			ctx := featureflag.WithFlags(context.Background(),
				featureflag.NewMemoryStore(map[string]bool{
					featureFlagProductSubscriptionsReaderServiceAccount: true,
					featureFlagProductSubscriptionsServiceAccount:       true,
				}, nil, nil))

			grantReason, err := serviceAccountOrOwnerOrLicenseManager(
				actor.WithActor(ctx, &actor.Actor{UID: actorID}),
				db,
				tc.ownerUserID,
				tc.serviceAccountCanWrite,
			)
			if tc.wantErr != nil {
				require.Error(t, err)
				tc.wantErr.Equal(t, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantGrantReason, grantReason)
			}
		})
	}
}
