package resolvers

import (
	"context"
	"reflect"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestSavedSearches(t *testing.T) {
	userID := int32(1)
	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{SiteAdmin: true, ID: userID}, nil)

	ss := dbmocks.NewMockSavedSearchStore()
	ss.ListFunc.SetDefaultHook(func(_ context.Context, args database.SavedSearchListArgs, paginationArgs *database.PaginationArgs) ([]*types.SavedSearch, error) {
		return []*types.SavedSearch{{ID: userID, Description: "test query", Query: "test type:diff patternType:regexp", Owner: *args.Owner}}, nil
	})
	ss.CountFunc.SetDefaultHook(func(_ context.Context, args database.SavedSearchListArgs) (int, error) {
		return 1, nil
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	ownerID := graphqlbackend.MarshalUserID(userID)
	args := graphqlbackend.SavedSearchesArgs{Owner: &ownerID, ConnectionResolverArgs: dummyConnectionResolverArgs}

	resolver, err := newTestResolver(t, db).SavedSearches(actor.WithActor(context.Background(), actor.FromUser(userID)), args)
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := resolver.Nodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	wantNodes := []graphqlbackend.SavedSearchResolver{
		&savedSearchResolver{db, types.SavedSearch{
			ID:          userID,
			Description: "test query",
			Query:       "test type:diff patternType:regexp",
			Owner:       types.NamespaceUser(userID),
		}},
	}
	if !reflect.DeepEqual(nodes, wantNodes) {
		t.Errorf("got %v+, want %v+", nodes[0], wantNodes[0])
	}
}

func TestSavedSearchesForSameUser(t *testing.T) {
	userID := int32(1)
	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{ID: userID}, nil)

	ss := dbmocks.NewMockSavedSearchStore()
	ss.ListFunc.SetDefaultHook(func(_ context.Context, args database.SavedSearchListArgs, paginationArgs *database.PaginationArgs) ([]*types.SavedSearch, error) {
		return []*types.SavedSearch{{ID: 1, Description: "test query", Query: "test type:diff patternType:regexp", Owner: *args.Owner}}, nil
	})
	ss.CountFunc.SetDefaultHook(func(_ context.Context, args database.SavedSearchListArgs) (int, error) {
		return 1, nil
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	ownerID := graphqlbackend.MarshalUserID(userID)
	args := graphqlbackend.SavedSearchesArgs{Owner: &ownerID, ConnectionResolverArgs: dummyConnectionResolverArgs}

	resolver, err := newTestResolver(t, db).SavedSearches(actor.WithActor(context.Background(), actor.FromUser(userID)), args)
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := resolver.Nodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	wantNodes := []graphqlbackend.SavedSearchResolver{
		&savedSearchResolver{db, types.SavedSearch{
			ID:          1,
			Description: "test query",
			Query:       "test type:diff patternType:regexp",
			Owner:       types.NamespaceUser(userID),
		}},
	}
	if !reflect.DeepEqual(nodes, wantNodes) {
		t.Errorf("got %v+, want %v+", nodes[0], wantNodes[0])
	}
}

func TestSavedSearchesForDifferentUser(t *testing.T) {
	userID := int32(2)
	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{ID: userID}, nil)

	ss := dbmocks.NewMockSavedSearchStore()
	ss.ListFunc.SetDefaultHook(func(_ context.Context, args database.SavedSearchListArgs, paginationArgs *database.PaginationArgs) ([]*types.SavedSearch, error) {
		panic("should fail auth check and never be called")
	})
	ss.CountFunc.SetDefaultHook(func(_ context.Context, args database.SavedSearchListArgs) (int, error) {
		panic("should fail auth check and never be called")
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	ownerID := graphqlbackend.MarshalUserID(3)
	args := graphqlbackend.SavedSearchesArgs{Owner: &ownerID, ConnectionResolverArgs: dummyConnectionResolverArgs}

	_, err := newTestResolver(t, db).SavedSearches(actor.WithActor(context.Background(), actor.FromUser(userID)), args)
	if err == nil {
		t.Error("got nil, want error to be returned for accessing saved searches of different user by non-site admin")
	}
}

func TestSavedSearchesForDifferentOrg(t *testing.T) {
	userID := int32(1)
	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{ID: userID}, nil)
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: userID}, nil)

	orgID := int32(2)
	om := dbmocks.NewMockOrgMemberStore()
	om.GetByOrgIDAndUserIDFunc.SetDefaultHook(func(ctx context.Context, oid, uid int32) (*types.OrgMembership, error) {
		return nil, nil
	})

	ss := dbmocks.NewMockSavedSearchStore()
	ss.ListFunc.SetDefaultHook(func(_ context.Context, args database.SavedSearchListArgs, paginationArgs *database.PaginationArgs) ([]*types.SavedSearch, error) {
		return []*types.SavedSearch{{ID: 1, Description: "test query", Query: "test type:diff patternType:regexp", Owner: types.NamespaceOrg(orgID)}}, nil
	})
	ss.CountFunc.SetDefaultHook(func(_ context.Context, args database.SavedSearchListArgs) (int, error) {
		return 1, nil
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.OrgMembersFunc.SetDefaultReturn(om)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	ownerID := graphqlbackend.MarshalOrgID(orgID)
	args := graphqlbackend.SavedSearchesArgs{Owner: &ownerID, ConnectionResolverArgs: dummyConnectionResolverArgs}

	if _, err := newTestResolver(t, db).SavedSearches(actor.WithActor(context.Background(), actor.FromUser(userID)), args); err != auth.ErrNotAnOrgMember {
		t.Errorf("got %v+, want %v+", err, auth.ErrNotAnOrgMember)
	}
}

func TestSavedSearchByIDOwner(t *testing.T) {
	ctx := context.Background()

	userID := int32(1)
	fixtureID := int32(1)

	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: userID}, nil)

	ss := dbmocks.NewMockSavedSearchStore()
	ss.GetByIDFunc.SetDefaultReturn(
		&types.SavedSearch{
			ID:          fixtureID,
			Description: "test query",
			Query:       "test type:diff patternType:regexp",
			Owner:       types.NamespaceUser(userID),
		},
		nil,
	)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	ctx = actor.WithActor(ctx, &actor.Actor{UID: userID})

	savedSearch, err := newTestResolver(t, db).SavedSearchByID(ctx, marshalSavedSearchID(fixtureID))
	if err != nil {
		t.Fatal(err)
	}
	want := &savedSearchResolver{
		db: db,
		s: types.SavedSearch{
			ID:          fixtureID,
			Description: "test query",
			Query:       "test type:diff patternType:regexp",
			Owner:       types.NamespaceUser(userID),
		},
	}

	if !reflect.DeepEqual(savedSearch, want) {
		t.Errorf("got %v+, want %v+", savedSearch, want)
	}
}

func TestSavedSearchByIDNonOwner(t *testing.T) {
	// Non-owners cannot view a user's saved searches.
	userID := int32(1)
	otherUserID := int32(2)
	fixtureID := marshalSavedSearchID(1)

	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true, ID: otherUserID}, nil)

	ss := dbmocks.NewMockSavedSearchStore()
	ss.GetByIDFunc.SetDefaultReturn(
		&types.SavedSearch{
			Description: "test query",
			Query:       "test type:diff patternType:regexp",
			Owner:       types.NamespaceUser(userID),
		},
		nil,
	)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: otherUserID})

	_, err := newTestResolver(t, db).SavedSearchByID(ctx, fixtureID)
	t.Log(err)
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestCreateSavedSearch(t *testing.T) {
	userID := int32(1)
	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true, ID: userID}, nil)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: userID})

	ss := dbmocks.NewMockSavedSearchStore()
	ss.CreateFunc.SetDefaultHook(func(_ context.Context, newSavedSearch *types.SavedSearch) (*types.SavedSearch, error) {
		return &types.SavedSearch{
			ID:          1,
			Description: newSavedSearch.Description,
			Query:       newSavedSearch.Query,
			Owner:       newSavedSearch.Owner,
		}, nil
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	savedSearches, err := newTestResolver(t, db).CreateSavedSearch(ctx, &graphqlbackend.CreateSavedSearchArgs{
		Input: graphqlbackend.SavedSearchInput{
			Description: "test query",
			Query:       "test type:diff patternType:regexp",
			Owner:       graphqlbackend.MarshalUserID(userID),
		}})
	if err != nil {
		t.Fatal(err)
	}
	want := &savedSearchResolver{db, types.SavedSearch{
		ID:          1,
		Description: "test query",
		Query:       "test type:diff patternType:regexp",
		Owner:       types.NamespaceUser(userID),
	}}

	mockrequire.Called(t, ss.CreateFunc)

	if !reflect.DeepEqual(savedSearches, want) {
		t.Errorf("got %v+, want %v+", savedSearches, want)
	}

	// Ensure create saved search errors when patternType is not provided in the query.
	_, err = newTestResolver(t, db).CreateSavedSearch(ctx, &graphqlbackend.CreateSavedSearchArgs{
		Input: graphqlbackend.SavedSearchInput{
			Description: "test query",
			Query:       "test type:diff",
			Owner:       graphqlbackend.MarshalUserID(userID),
		}})
	if err == nil {
		t.Error("Expected error for createSavedSearch when query does not provide a patternType: field.")
	}
}

func TestUpdateSavedSearch(t *testing.T) {
	fixtureID := int32(1)
	userID := int32(1)
	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true, ID: userID}, nil)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: userID})

	ss := dbmocks.NewMockSavedSearchStore()
	ss.UpdateFunc.SetDefaultHook(func(ctx context.Context, savedSearch *types.SavedSearch) (*types.SavedSearch, error) {
		return &types.SavedSearch{
			ID:          fixtureID,
			Description: savedSearch.Description,
			Query:       savedSearch.Query,
			Owner:       savedSearch.Owner,
		}, nil
	})
	ss.GetByIDFunc.SetDefaultReturn(&types.SavedSearch{ID: fixtureID, Owner: types.NamespaceUser(userID)}, nil)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	savedSearches, err := newTestResolver(t, db).UpdateSavedSearch(ctx, &graphqlbackend.UpdateSavedSearchArgs{
		ID: marshalSavedSearchID(fixtureID),
		Input: graphqlbackend.SavedSearchUpdateInput{
			Description: "updated query description",
			Query:       "test type:diff patternType:regexp",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := &savedSearchResolver{db, types.SavedSearch{
		ID:          fixtureID,
		Description: "updated query description",
		Query:       "test type:diff patternType:regexp",
		Owner:       types.NamespaceUser(userID),
	}}

	mockrequire.Called(t, ss.UpdateFunc)

	if !reflect.DeepEqual(savedSearches, want) {
		t.Errorf("got %v+, want %v+", savedSearches, want)
	}

	// Ensure update saved search errors when patternType is not provided in the query.
	_, err = newTestResolver(t, db).UpdateSavedSearch(ctx, &graphqlbackend.UpdateSavedSearchArgs{
		ID:    marshalSavedSearchID(fixtureID),
		Input: graphqlbackend.SavedSearchUpdateInput{Description: "updated query description", Query: "test type:diff"}},
	)
	if err == nil {
		t.Error("Expected error for updateSavedSearch when query does not provide a patternType: field.")
	}
}

func TestTransferSavedSearchOwnership(t *testing.T) {
	fixtureID := int32(1)
	userID := int32(1)
	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: userID}, nil)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: userID})

	orgID := int32(2)
	om := dbmocks.NewMockOrgMemberStore()
	om.GetByOrgIDAndUserIDFunc.SetDefaultHook(func(ctx context.Context, oid, uid int32) (*types.OrgMembership, error) {
		return &types.OrgMembership{OrgID: oid, UserID: uid}, nil
	})

	ss := dbmocks.NewMockSavedSearchStore()
	ss.UpdateOwnerFunc.SetDefaultHook(func(ctx context.Context, id int32, newOwner types.Namespace) (*types.SavedSearch, error) {
		return &types.SavedSearch{
			ID:    id,
			Owner: newOwner,
		}, nil
	})
	ss.GetByIDFunc.SetDefaultReturn(&types.SavedSearch{ID: fixtureID, Owner: types.NamespaceUser(userID)}, nil)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.OrgMembersFunc.SetDefaultReturn(om)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	result, err := newTestResolver(t, db).TransferSavedSearchOwnership(ctx, &graphqlbackend.TransferSavedSearchOwnershipArgs{
		ID:       marshalSavedSearchID(fixtureID),
		NewOwner: graphqlbackend.MarshalOrgID(orgID),
	})
	if err != nil {
		t.Fatal(err)
	}

	mockrequire.Called(t, ss.UpdateOwnerFunc)
	mockrequire.Called(t, om.GetByOrgIDAndUserIDFunc)
	want := &savedSearchResolver{db, types.SavedSearch{
		ID:    fixtureID,
		Owner: types.NamespaceOrg(orgID),
	}}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("got %v+, want %v+", result, want)
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
			savedSearches.GetByIDFunc.SetDefaultReturn(&types.SavedSearch{
				Owner: types.Namespace{
					User: tt.ssUserID,
					Org:  tt.ssOrgID,
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

			_, err := newTestResolver(t, db).UpdateSavedSearch(ctx, &graphqlbackend.UpdateSavedSearchArgs{
				ID: marshalSavedSearchID(1),
				Input: graphqlbackend.SavedSearchUpdateInput{
					Query: "patterntype:literal",
				},
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
	userID := int32(1)
	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{SiteAdmin: true, ID: userID}, nil)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: userID})

	ss := dbmocks.NewMockSavedSearchStore()
	ss.GetByIDFunc.SetDefaultReturn(&types.SavedSearch{
		ID:          1,
		Description: "test query",
		Query:       "test type:diff",
		Owner:       types.NamespaceUser(userID),
	}, nil)

	ss.DeleteFunc.SetDefaultReturn(nil)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SavedSearchesFunc.SetDefaultReturn(ss)

	firstSavedSearchGraphqlID := graphql.ID("U2F2ZWRTZWFyY2g6NTI=")
	_, err := newTestResolver(t, db).DeleteSavedSearch(ctx, &graphqlbackend.DeleteSavedSearchArgs{ID: firstSavedSearchGraphqlID})
	if err != nil {
		t.Fatal(err)
	}

	mockrequire.Called(t, ss.DeleteFunc)
}

func TestSavedSearchesConnectionStore(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

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
			Owner:       types.NamespaceUser(user.ID),
		})
		require.NoError(t, err)
	}

	owner := types.NamespaceUser(user.ID)
	connectionStore := &savedSearchesConnectionStore{
		db:       db,
		listArgs: database.SavedSearchListArgs{Owner: &owner},
	}

	graphqlutil.TestConnectionResolverStoreSuite(t, connectionStore)
}

var dummyConnectionResolverArgs = graphqlutil.ConnectionResolverArgs{First: pointers.Ptr[int32](1)}

func newTestResolver(t *testing.T, db database.DB) *Resolver {
	t.Helper()
	return &Resolver{db: db}
}
