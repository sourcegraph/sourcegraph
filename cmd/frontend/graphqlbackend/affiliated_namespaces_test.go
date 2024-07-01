package graphqlbackend

import (
	"context"
	"reflect"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestUserResolver_AffiliatedNamespaces(t *testing.T) {
	userID := int32(1)

	user := &types.User{ID: userID, Username: "test-user"}
	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(user, nil)

	orgs := dbmocks.NewMockOrgStore()
	orgs.GetByUserIDFunc.SetDefaultHook(func(ctx context.Context, uid int32) ([]*types.Org, error) {
		return []*types.Org{
			{ID: 2, Name: "org1"},
			{ID: 3, Name: "org2"},
		}, nil
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.OrgsFunc.SetDefaultReturn(orgs)

	userResolver := &UserResolver{db: db, user: user}

	t.Run("same user", func(t *testing.T) {
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: userID})
		got, err := userResolver.AffiliatedNamespaces(ctx)
		if err != nil {
			t.Fatal(err)
		}

		want := []*NamespaceResolver{
			{Namespace: userResolver},
			{Namespace: &OrgResolver{db: db, org: &types.Org{ID: 2, Name: "org1"}}},
			{Namespace: &OrgResolver{db: db, org: &types.Org{ID: 3, Name: "org2"}}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("different user", func(t *testing.T) {
		otherUserID := int32(2)
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: otherUserID})
		wantErr := auth.ErrMustBeSiteAdminOrSameUser
		if _, err := userResolver.AffiliatedNamespaces(ctx); err != wantErr {
			t.Fatalf("got error %v, want %v", err, wantErr)
		}
	})
}

func TestVisitorResolver_AffiliatedNamespaces(t *testing.T) {
	ctx := context.Background()
	visitorResolver := visitorResolver{}

	namespaces, err := visitorResolver.AffiliatedNamespaces(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(namespaces) != 0 {
		t.Errorf("got %d namespaces, want 0", len(namespaces))
	}
}
