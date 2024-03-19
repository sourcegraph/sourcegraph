package resolvers

import (
	"context"
	"strings"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestSearchContexts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	userID := int32(1)
	graphqlUserID := graphqlbackend.MarshalUserID(userID)

	query := "ctx"
	tests := []struct {
		name     string
		args     *graphqlbackend.ListSearchContextsArgs
		wantErr  string
		wantOpts database.ListSearchContextsOptions
	}{
		{
			name:     "filtering by namespace",
			args:     &graphqlbackend.ListSearchContextsArgs{Query: &query, Namespaces: []*graphql.ID{&graphqlUserID}},
			wantOpts: database.ListSearchContextsOptions{Name: query, NamespaceUserIDs: []int32{userID}, NamespaceOrgIDs: []int32{}, OrderBy: database.SearchContextsOrderBySpec},
		},
		{
			name:     "filtering by instance",
			args:     &graphqlbackend.ListSearchContextsArgs{Query: &query, Namespaces: []*graphql.ID{nil}},
			wantOpts: database.ListSearchContextsOptions{Name: query, NamespaceUserIDs: []int32{}, NamespaceOrgIDs: []int32{}, NoNamespace: true, OrderBy: database.SearchContextsOrderBySpec},
		},
		{
			name:     "get all",
			args:     &graphqlbackend.ListSearchContextsArgs{Query: &query},
			wantOpts: database.ListSearchContextsOptions{Name: query, NamespaceUserIDs: []int32{}, NamespaceOrgIDs: []int32{}, OrderBy: database.SearchContextsOrderBySpec},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := dbmocks.NewMockSearchContextsStore()
			sc.CountSearchContextsFunc.SetDefaultReturn(0, nil)
			sc.ListSearchContextsFunc.SetDefaultHook(func(ctx context.Context, pageOpts database.ListSearchContextsPageOptions, opts database.ListSearchContextsOptions) ([]*types.SearchContext, error) {
				if diff := cmp.Diff(tt.wantOpts, opts); diff != "" {
					t.Fatalf("Mismatch (-want +got):\n%s", diff)
				}
				return []*types.SearchContext{}, nil
			})

			db := dbmocks.NewMockDB()
			db.SearchContextsFunc.SetDefaultReturn(sc)

			_, err := (&Resolver{db: db}).SearchContexts(ctx, tt.args)
			expectErr := tt.wantErr != ""
			if !expectErr && err != nil {
				t.Fatalf("expected no error, got %s", err)
			}
			if expectErr && err == nil {
				t.Fatalf("wanted error, got none")
			}
			if expectErr && err != nil && !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("wanted error containing %s, got %s", tt.wantErr, err)
			}
			mockrequire.Called(t, sc.CountSearchContextsFunc)
			mockrequire.Called(t, sc.ListSearchContextsFunc)
		})
	}
}

func TestSearchContextsStarDefaultPermissions(t *testing.T) {
	// Note: this test can't do t.Parallel since it mutates global state (MockSourcegraphDotComMode)
	dotcom.MockSourcegraphDotComMode(t, true)

	userID := int32(1)
	graphqlUserID := graphqlbackend.MarshalUserID(userID)
	username := "alice"
	ctx := context.Background()
	ctx = actor.WithActor(ctx, &actor.Actor{UID: userID})

	users := dbmocks.NewMockUserStore()
	users.GetByIDFunc.SetDefaultReturn(&types.User{Username: username}, nil)

	searchContextSpec := "test"
	graphqlSearchContextID := marshalSearchContextID(searchContextSpec)

	sc := dbmocks.NewMockSearchContextsStore()
	sc.GetSearchContextFunc.SetDefaultReturn(&types.SearchContext{ID: 0, Name: searchContextSpec}, nil)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)
	db.SearchContextsFunc.SetDefaultReturn(sc)

	// User not admin, trying to set things for themselves
	_, err := (&Resolver{db: db}).SetDefaultSearchContext(ctx, graphqlbackend.SetDefaultSearchContextArgs{SearchContextID: graphqlSearchContextID, UserID: graphqlUserID})
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	_, err = (&Resolver{db: db}).CreateSearchContextStar(ctx, graphqlbackend.CreateSearchContextStarArgs{SearchContextID: graphqlSearchContextID, UserID: graphqlUserID})
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	_, err = (&Resolver{db: db}).DeleteSearchContextStar(ctx, graphqlbackend.DeleteSearchContextStarArgs{SearchContextID: graphqlSearchContextID, UserID: graphqlUserID})
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	// User not admin, trying to set things for another user
	graphqlUserID2 := graphqlbackend.MarshalUserID(int32(2))
	unauthorizedError := auth.ErrMustBeSiteAdminOrSameUser.Error()

	_, err = (&Resolver{db: db}).SetDefaultSearchContext(ctx, graphqlbackend.SetDefaultSearchContextArgs{SearchContextID: graphqlSearchContextID, UserID: graphqlUserID2})
	if err.Error() != unauthorizedError {
		t.Fatalf("expected error %s, got %s", unauthorizedError, err)
	}
	_, err = (&Resolver{db: db}).CreateSearchContextStar(ctx, graphqlbackend.CreateSearchContextStarArgs{SearchContextID: graphqlSearchContextID, UserID: graphqlUserID2})
	if err.Error() != unauthorizedError {
		t.Fatalf("expected error %s, got %s", unauthorizedError, err)
	}
	_, err = (&Resolver{db: db}).DeleteSearchContextStar(ctx, graphqlbackend.DeleteSearchContextStarArgs{SearchContextID: graphqlSearchContextID, UserID: graphqlUserID2})
	if err.Error() != unauthorizedError {
		t.Fatalf("expected error %s, got %s", unauthorizedError, err)
	}

	// User is admin, trying to set things for another user
	users.GetByIDFunc.SetDefaultReturn(&types.User{ID: userID, Username: username, SiteAdmin: true}, nil)
	// Create a new context with actor so that the user cached on actor is not reused
	ctx = actor.WithActor(ctx, &actor.Actor{UID: userID})

	_, err = (&Resolver{db: db}).SetDefaultSearchContext(ctx, graphqlbackend.SetDefaultSearchContextArgs{SearchContextID: graphqlSearchContextID, UserID: graphqlUserID2})
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	_, err = (&Resolver{db: db}).CreateSearchContextStar(ctx, graphqlbackend.CreateSearchContextStarArgs{SearchContextID: graphqlSearchContextID, UserID: graphqlUserID2})
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	_, err = (&Resolver{db: db}).DeleteSearchContextStar(ctx, graphqlbackend.DeleteSearchContextStarArgs{SearchContextID: graphqlSearchContextID, UserID: graphqlUserID2})
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
}
