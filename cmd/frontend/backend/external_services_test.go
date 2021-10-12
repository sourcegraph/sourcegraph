package backend

import (
	"context"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestCheckExternalServiceAccess(t *testing.T) {
	ctx := testContext()
	nonAuthContext := actor.WithActor(ctx, &actor.Actor{UID: 0})
	db := new(dbtesting.MockDB)

	mockSiteAdmin := func(isSiteAdmin bool) *types.User {
		return &types.User{ID: 1, SiteAdmin: isSiteAdmin}
	}

	tests := []struct {
		name            string
		ctx             context.Context
		mockCurrentUser *types.User
		mockOrgMember   *types.OrgMembership
		namespaceUserId int32
		namespaceOrgId  int32
		expectNil     	bool
		errMessage      string
	}{
		{
			name: "Returns error for non-authenticated actor",
			ctx: nonAuthContext,
			mockCurrentUser: nil,
			mockOrgMember: nil,
			namespaceOrgId: 0,
			namespaceUserId: 1,
			expectNil: false,
			errMessage: "got nil, want ErrNoAccessExternalService",
		},
		{
			name: "Returns error for site-level code host connection if user is not side-admin",
			ctx: ctx,
			mockCurrentUser: mockSiteAdmin(false),
			mockOrgMember: nil,
			namespaceOrgId: 0,
			namespaceUserId: 0,
			expectNil: false,
			errMessage: "got nil, want ErrNoAccessExternalService",
		},
		{
			name: "Returns nil for site-level code host connection if user is side-admin",
			ctx: ctx,
			mockCurrentUser: mockSiteAdmin(true),
			mockOrgMember: nil,
			namespaceOrgId: 0,
			namespaceUserId: 0,
			expectNil: true,
			errMessage: "got ErrNoAccessExternalService, want nil",
		},
		{
			name: "Returns error for personal code host connection and user not matching user ID",
			ctx: ctx,
			mockCurrentUser: mockSiteAdmin(true),
			mockOrgMember: nil,
			namespaceOrgId: 0,
			namespaceUserId: 42,
			expectNil: false,
			errMessage: "got nil, want ErrNoAccessExternalService",
		},
		{
			name: "Returns nil for personal code host connection and user matching user ID",
			ctx: ctx,
			mockCurrentUser: mockSiteAdmin(false),
			mockOrgMember: nil,
			namespaceOrgId: 0,
			namespaceUserId: 1,
			expectNil: true,
			errMessage: "got ErrNoAccessExternalService, want nil",
		},
		{
			name: "Returns error for org code host connection and user not being a member of the org",
			ctx: ctx,
			mockCurrentUser: mockSiteAdmin(true),
			mockOrgMember: nil,
			namespaceOrgId: 42,
			namespaceUserId: 0,
			expectNil: false,
			errMessage: "got nil, want ErrNoAccessExternalService",
		},
		{
			name: "Returns nil for org code host connection and user is a member of the org",
			ctx: ctx,
			mockCurrentUser: mockSiteAdmin(false),
			mockOrgMember: &types.OrgMembership{ID: 1, OrgID: 42, UserID: 1},
			namespaceOrgId: 42,
			namespaceUserId: 0,
			expectNil: true,
			errMessage: "got ErrNoAccessExternalService, want nil",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database.Mocks.Users.GetByCurrentAuthUser = func(ctx context.Context) (*types.User, error) {
				return test.mockCurrentUser, nil
			}
			database.Mocks.OrgMembers.GetByOrgIDAndUserID = func(ctx context.Context, orgID, userID int32) (*types.OrgMembership, error) {
				return test.mockOrgMember, nil
			}

			result := CheckExternalServiceAccess(test.ctx, db, test.namespaceUserId, test.namespaceOrgId)

			if test.expectNil != (result == nil) {
				t.Errorf(test.errMessage)
			}
			defer func() {
				database.Mocks.Users = database.MockUsers{}
				database.Mocks.OrgMembers = database.MockOrgMembers{}
			}()
		})
	}
}
