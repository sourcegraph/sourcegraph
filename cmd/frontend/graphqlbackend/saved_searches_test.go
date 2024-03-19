package graphqlbackend

import (
	"context"
	"reflect"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestSavedSearches(t *testing.T) {
	key := int32(1)

	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{SiteAdmin: true, ID: key}, nil)

	ss := dbmocks.NewMockSavedSearchStore()
	ss.ListSavedSearchesByOrgOrUserFunc.SetDefaultHook(func(_ context.Context, userID, orgId *int32, paginationArgs *database.PaginationArgs) ([]*types.SavedSearch, error) {
		return []*types.SavedSearch{{ID: key, Description: "test query", Query: "test type:diff patternType:regexp", UserID: userID, OrgID: nil}}, nil
	})
	ss.CountSavedSearchesByOrgOrUserFunc.SetDefaultHook(func(_ context.Context, userID, orgId *int32) (int, error) {
		return 1, nil
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	args := savedSearchesArgs{
		ConnectionResolverArgs: graphqlutil.ConnectionResolverArgs{First: &key},
		Namespace:              MarshalUserID(key),
	}

	resolver, err := newSchemaResolver(db, gitserver.NewTestClient(t)).SavedSearches(actor.WithActor(context.Background(), actor.FromUser(key)), args)
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := resolver.Nodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	wantNodes := []*savedSearchResolver{{db, types.SavedSearch{
		ID:              key,
		Description:     "test query",
		Query:           "test type:diff patternType:regexp",
		UserID:          &key,
		OrgID:           nil,
		SlackWebhookURL: nil,
	}}}
	if !reflect.DeepEqual(nodes, wantNodes) {
		t.Errorf("got %v+, want %v+", nodes[0], wantNodes[0])
	}
}

func TestSavedSearchesForSameUser(t *testing.T) {
	key := int32(1)

	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{SiteAdmin: false, ID: key}, nil)

	ss := dbmocks.NewMockSavedSearchStore()
	ss.ListSavedSearchesByOrgOrUserFunc.SetDefaultHook(func(_ context.Context, userID, orgId *int32, paginationArgs *database.PaginationArgs) ([]*types.SavedSearch, error) {
		return []*types.SavedSearch{{ID: key, Description: "test query", Query: "test type:diff patternType:regexp", UserID: userID, OrgID: nil}}, nil
	})
	ss.CountSavedSearchesByOrgOrUserFunc.SetDefaultHook(func(_ context.Context, userID, orgId *int32) (int, error) {
		return 1, nil
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	args := savedSearchesArgs{
		ConnectionResolverArgs: graphqlutil.ConnectionResolverArgs{First: &key},
		Namespace:              MarshalUserID(key),
	}

	resolver, err := newSchemaResolver(db, gitserver.NewTestClient(t)).SavedSearches(actor.WithActor(context.Background(), actor.FromUser(key)), args)
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := resolver.Nodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	wantNodes := []*savedSearchResolver{{db, types.SavedSearch{
		ID:              key,
		Description:     "test query",
		Query:           "test type:diff patternType:regexp",
		UserID:          &key,
		OrgID:           nil,
		SlackWebhookURL: nil,
	}}}
	if !reflect.DeepEqual(nodes, wantNodes) {
		t.Errorf("got %v+, want %v+", nodes[0], wantNodes[0])
	}
}

func TestSavedSearchesForDifferentUser(t *testing.T) {
	key := int32(1)
	userID := int32(2)

	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{SiteAdmin: false, ID: userID}, nil)

	ss := dbmocks.NewMockSavedSearchStore()
	ss.ListSavedSearchesByOrgOrUserFunc.SetDefaultHook(func(_ context.Context, userID, orgId *int32, paginationArgs *database.PaginationArgs) ([]*types.SavedSearch, error) {
		return []*types.SavedSearch{{ID: key, Description: "test query", Query: "test type:diff patternType:regexp", UserID: userID, OrgID: nil}}, nil
	})
	ss.CountSavedSearchesByOrgOrUserFunc.SetDefaultHook(func(_ context.Context, userID, orgId *int32) (int, error) {
		return 1, nil
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	args := savedSearchesArgs{
		ConnectionResolverArgs: graphqlutil.ConnectionResolverArgs{First: &key},
		Namespace:              MarshalUserID(key),
	}

	_, err := newSchemaResolver(db, gitserver.NewTestClient(t)).SavedSearches(actor.WithActor(context.Background(), actor.FromUser(userID)), args)
	if err == nil {
		t.Error("got nil, want error to be returned for accessing saved searches of different user by non site admin.")
	}
}

func TestSavedSearchesForDifferentOrg(t *testing.T) {
	key := int32(1)

	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{SiteAdmin: false, ID: key}, nil)
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: false, ID: key}, nil)

	om := dbmocks.NewMockOrgMemberStore()
	om.GetByOrgIDAndUserIDFunc.SetDefaultHook(func(ctx context.Context, oid, uid int32) (*types.OrgMembership, error) {
		return nil, nil
	})

	ss := dbmocks.NewMockSavedSearchStore()
	ss.ListSavedSearchesByOrgOrUserFunc.SetDefaultHook(func(_ context.Context, userID, orgId *int32, paginationArgs *database.PaginationArgs) ([]*types.SavedSearch, error) {
		return []*types.SavedSearch{{ID: key, Description: "test query", Query: "test type:diff patternType:regexp", UserID: nil, OrgID: &key}}, nil
	})
	ss.CountSavedSearchesByOrgOrUserFunc.SetDefaultHook(func(_ context.Context, userID, orgId *int32) (int, error) {
		return 1, nil
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.OrgMembersFunc.SetDefaultReturn(om)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	args := savedSearchesArgs{
		ConnectionResolverArgs: graphqlutil.ConnectionResolverArgs{First: &key},
		Namespace:              MarshalOrgID(key),
	}

	if _, err := newSchemaResolver(db, gitserver.NewTestClient(t)).SavedSearches(actor.WithActor(context.Background(), actor.FromUser(key)), args); err != auth.ErrNotAnOrgMember {
		t.Errorf("got %v+, want %v+", err, auth.ErrNotAnOrgMember)
	}
}

func TestSavedSearchByIDOwner(t *testing.T) {
	ctx := context.Background()

	userID := int32(1)
	ssID := marshalSavedSearchID(1)

	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: false, ID: userID}, nil)

	ss := dbmocks.NewMockSavedSearchStore()
	ss.GetByIDFunc.SetDefaultReturn(
		&api.SavedQuerySpecAndConfig{
			Spec: api.SavedQueryIDSpec{},
			Config: api.ConfigSavedQuery{
				UserID:      &userID,
				Description: "test query",
				Query:       "test type:diff patternType:regexp",
				OrgID:       nil,
			},
		},
		nil,
	)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	ctx = actor.WithActor(ctx, &actor.Actor{
		UID: userID,
	})

	savedSearch, err := newSchemaResolver(db, gitserver.NewTestClient(t)).savedSearchByID(ctx, ssID)
	if err != nil {
		t.Fatal(err)
	}
	want := &savedSearchResolver{
		db: db,
		s: types.SavedSearch{
			ID:          userID,
			Description: "test query",
			Query:       "test type:diff patternType:regexp",
			UserID:      &userID,
			OrgID:       nil,
		},
	}

	if !reflect.DeepEqual(savedSearch, want) {
		t.Errorf("got %v+, want %v+", savedSearch, want)
	}
}

func TestSavedSearchByIDNonOwner(t *testing.T) {
	// Non owners, including site admins cannot view a user's saved searches
	userID := int32(1)
	adminID := int32(2)
	ssID := marshalSavedSearchID(1)

	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true, ID: adminID}, nil)

	ss := dbmocks.NewMockSavedSearchStore()
	ss.GetByIDFunc.SetDefaultReturn(
		&api.SavedQuerySpecAndConfig{
			Spec: api.SavedQueryIDSpec{},
			Config: api.ConfigSavedQuery{
				UserID:      &userID,
				Description: "test query",
				Query:       "test type:diff patternType:regexp",
				OrgID:       nil,
			},
		},
		nil,
	)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	ctx := actor.WithActor(context.Background(), &actor.Actor{
		UID: adminID,
	})

	_, err := newSchemaResolver(db, gitserver.NewTestClient(t)).savedSearchByID(ctx, ssID)
	t.Log(err)
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestCreateSavedSearch(t *testing.T) {
	key := int32(1)

	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true, ID: key}, nil)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: key})

	ss := dbmocks.NewMockSavedSearchStore()
	ss.CreateFunc.SetDefaultHook(func(_ context.Context, newSavedSearch *types.SavedSearch) (*types.SavedSearch, error) {
		return &types.SavedSearch{
			ID:          key,
			Description: newSavedSearch.Description,
			Query:       newSavedSearch.Query,
			Notify:      newSavedSearch.Notify,
			NotifySlack: newSavedSearch.NotifySlack,
			UserID:      newSavedSearch.UserID,
			OrgID:       newSavedSearch.OrgID,
		}, nil
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	userID := MarshalUserID(key)
	savedSearches, err := newSchemaResolver(db, gitserver.NewTestClient(t)).CreateSavedSearch(ctx, &struct {
		Description string
		Query       string
		NotifyOwner bool
		NotifySlack bool
		OrgID       *graphql.ID
		UserID      *graphql.ID
	}{Description: "test query", Query: "test type:diff patternType:regexp", NotifyOwner: true, NotifySlack: false, OrgID: nil, UserID: &userID})
	if err != nil {
		t.Fatal(err)
	}
	want := &savedSearchResolver{db, types.SavedSearch{
		ID:          key,
		Description: "test query",
		Query:       "test type:diff patternType:regexp",
		Notify:      true,
		NotifySlack: false,
		OrgID:       nil,
		UserID:      &key,
	}}

	mockrequire.Called(t, ss.CreateFunc)

	if !reflect.DeepEqual(savedSearches, want) {
		t.Errorf("got %v+, want %v+", savedSearches, want)
	}

	// Ensure create saved search errors when patternType is not provided in the query.
	_, err = newSchemaResolver(db, gitserver.NewTestClient(t)).CreateSavedSearch(ctx, &struct {
		Description string
		Query       string
		NotifyOwner bool
		NotifySlack bool
		OrgID       *graphql.ID
		UserID      *graphql.ID
	}{Description: "test query", Query: "test type:diff", NotifyOwner: true, NotifySlack: false, OrgID: nil, UserID: &userID})
	if err == nil {
		t.Error("Expected error for createSavedSearch when query does not provide a patternType: field.")
	}
}

func TestUpdateSavedSearch(t *testing.T) {
	key := int32(1)
	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true, ID: key}, nil)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: key})

	ss := dbmocks.NewMockSavedSearchStore()
	ss.UpdateFunc.SetDefaultHook(func(ctx context.Context, savedSearch *types.SavedSearch) (*types.SavedSearch, error) {
		return &types.SavedSearch{
			ID:          key,
			Description: savedSearch.Description,
			Query:       savedSearch.Query,
			Notify:      savedSearch.Notify,
			NotifySlack: savedSearch.NotifySlack,
			UserID:      savedSearch.UserID,
			OrgID:       savedSearch.OrgID,
		}, nil
	})
	ss.GetByIDFunc.SetDefaultReturn(&api.SavedQuerySpecAndConfig{
		Config: api.ConfigSavedQuery{
			UserID: &key,
		},
	}, nil)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	userID := MarshalUserID(key)
	savedSearches, err := newSchemaResolver(db, gitserver.NewTestClient(t)).UpdateSavedSearch(ctx, &struct {
		ID          graphql.ID
		Description string
		Query       string
		NotifyOwner bool
		NotifySlack bool
		OrgID       *graphql.ID
		UserID      *graphql.ID
	}{
		ID:          marshalSavedSearchID(key),
		Description: "updated query description",
		Query:       "test type:diff patternType:regexp",
		OrgID:       nil,
		UserID:      &userID,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := &savedSearchResolver{db, types.SavedSearch{
		ID:          key,
		Description: "updated query description",
		Query:       "test type:diff patternType:regexp",
		OrgID:       nil,
		UserID:      &key,
	}}

	mockrequire.Called(t, ss.UpdateFunc)

	if !reflect.DeepEqual(savedSearches, want) {
		t.Errorf("got %v+, want %v+", savedSearches, want)
	}

	// Ensure update saved search errors when patternType is not provided in the query.
	_, err = newSchemaResolver(db, gitserver.NewTestClient(t)).UpdateSavedSearch(ctx, &struct {
		ID          graphql.ID
		Description string
		Query       string
		NotifyOwner bool
		NotifySlack bool
		OrgID       *graphql.ID
		UserID      *graphql.ID
	}{ID: marshalSavedSearchID(key), Description: "updated query description", Query: "test type:diff", NotifyOwner: true, NotifySlack: false, OrgID: nil, UserID: &userID})
	if err == nil {
		t.Error("Expected error for updateSavedSearch when query does not provide a patternType: field.")
	}
}

func TestUpdateSavedSearchPermissions(t *testing.T) {
	user1 := &types.User{ID: 42}
	user2 := &types.User{ID: 43}
	admin := &types.User{ID: 44, SiteAdmin: true}
	org1 := &types.Org{ID: 42}
	org2 := &types.Org{ID: 43}

	cases := []struct {
		execUser *types.User
		ssUserID *int32
		ssOrgID  *int32
		errIs    error
	}{{
		execUser: user1,
		ssUserID: &user1.ID,
		errIs:    nil,
	}, {
		execUser: user1,
		ssUserID: &user2.ID,
		errIs:    &auth.InsufficientAuthorizationError{},
	}, {
		execUser: user1,
		ssOrgID:  &org1.ID,
		errIs:    nil,
	}, {
		execUser: user1,
		ssOrgID:  &org2.ID,
		errIs:    auth.ErrNotAnOrgMember,
	}, {
		execUser: admin,
		ssOrgID:  &user1.ID,
		errIs:    nil,
	}, {
		execUser: admin,
		ssOrgID:  &org1.ID,
		errIs:    nil,
	}}

	for _, tt := range cases {
		t.Run("", func(t *testing.T) {
			ctx := actor.WithActor(context.Background(), actor.FromUser(tt.execUser.ID))
			users := dbmocks.NewMockUserStore()
			users.GetByCurrentAuthUserFunc.SetDefaultHook(func(ctx context.Context) (*types.User, error) {
				switch actor.FromContext(ctx).UID {
				case user1.ID:
					return user1, nil
				case user2.ID:
					return user2, nil
				case admin.ID:
					return admin, nil
				default:
					panic("bad actor")
				}
			})

			savedSearches := dbmocks.NewMockSavedSearchStore()
			savedSearches.UpdateFunc.SetDefaultHook(func(_ context.Context, ss *types.SavedSearch) (*types.SavedSearch, error) {
				return ss, nil
			})
			savedSearches.GetByIDFunc.SetDefaultReturn(&api.SavedQuerySpecAndConfig{
				Config: api.ConfigSavedQuery{
					UserID: tt.ssUserID,
					OrgID:  tt.ssOrgID,
				},
			}, nil)

			orgMembers := dbmocks.NewMockOrgMemberStore()
			orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultHook(func(_ context.Context, orgID int32, userID int32) (*types.OrgMembership, error) {
				if orgID == userID {
					return &types.OrgMembership{}, nil
				}
				return nil, nil
			})

			db := dbmocks.NewMockDB()
			db.UsersFunc.SetDefaultReturn(users)
			db.SavedSearchesFunc.SetDefaultReturn(savedSearches)
			db.OrgMembersFunc.SetDefaultReturn(orgMembers)

			_, err := newSchemaResolver(db, gitserver.NewTestClient(t)).UpdateSavedSearch(ctx, &struct {
				ID          graphql.ID
				Description string
				Query       string
				NotifyOwner bool
				NotifySlack bool
				OrgID       *graphql.ID
				UserID      *graphql.ID
			}{
				ID:    marshalSavedSearchID(1),
				Query: "patterntype:literal",
			})
			if tt.errIs == nil {
				require.NoError(t, err)
			} else {
				require.ErrorAs(t, err, &tt.errIs)
			}
		})
	}
}

func TestDeleteSavedSearch(t *testing.T) {
	key := int32(1)
	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{SiteAdmin: true, ID: key}, nil)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: key})

	ss := dbmocks.NewMockSavedSearchStore()
	ss.GetByIDFunc.SetDefaultReturn(&api.SavedQuerySpecAndConfig{
		Spec: api.SavedQueryIDSpec{
			Subject: api.SettingsSubject{User: &key},
			Key:     "1",
		},
		Config: api.ConfigSavedQuery{
			Key:         "1",
			Description: "test query",
			Query:       "test type:diff",
			UserID:      &key,
			OrgID:       nil,
		},
	}, nil)

	ss.DeleteFunc.SetDefaultReturn(nil)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	firstSavedSearchGraphqlID := graphql.ID("U2F2ZWRTZWFyY2g6NTI=")
	_, err := newSchemaResolver(db, gitserver.NewTestClient(t)).DeleteSavedSearch(ctx, &struct {
		ID graphql.ID
	}{ID: firstSavedSearchGraphqlID})
	if err != nil {
		t.Fatal(err)
	}

	mockrequire.Called(t, ss.DeleteFunc)
}

func TestSavedSearchesConnectionStore(t *testing.T) {
	ctx := context.Background()

	db := database.NewDB(logtest.Scoped(t), dbtest.NewDB(t))

	user, err := db.Users().Create(ctx, database.NewUser{
		Email:           "test@sourcegraph.com",
		Username:        "test",
		EmailIsVerified: true,
	})
	require.NoError(t, err)

	for range 10 {
		_, err := db.SavedSearches().Create(ctx, &types.SavedSearch{
			Description: "Test Search",
			Query:       "r:src-cli",
			UserID:      &user.ID,
		})
		require.NoError(t, err)
	}

	connectionStore := &savedSearchesConnectionStore{
		db:     db,
		userID: &user.ID,
	}

	graphqlutil.TestConnectionResolverStoreSuite(t, connectionStore)
}
