package graphqlbackend

import (
	"context"
	"fmt"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestNamespace(t *testing.T) {
	t.Run("user", func(t *testing.T) {
		const wantUserID = 3
		users := dbmocks.NewMockUserStore()
		users.GetByIDFunc.SetDefaultHook(func(_ context.Context, id int32) (*types.User, error) {
			if id != wantUserID {
				t.Errorf("got %d, want %d", id, wantUserID)
			}
			return &types.User{ID: wantUserID, Username: "alice"}, nil
		})

		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
				{
					namespace(id: "VXNlcjoz") {
						__typename
						... on User { username }
					}
				}
			`,
				ExpectedResult: `
				{
					"namespace": {
						"__typename": "User",
						"username": "alice"
					}
				}
			`,
			},
		})
	})

	t.Run("organization", func(t *testing.T) {
		const wantOrgID = 3
		orgs := dbmocks.NewMockOrgStore()
		orgs.GetByIDFunc.SetDefaultHook(func(_ context.Context, id int32) (*types.Org, error) {
			if id != wantOrgID {
				t.Errorf("got %d, want %d", id, wantOrgID)
			}
			return &types.Org{ID: wantOrgID, Name: "acme"}, nil
		})

		db := dbmocks.NewMockDB()
		db.OrgsFunc.SetDefaultReturn(orgs)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
				{
					namespace(id: "T3JnOjM=") {
						__typename
						... on Org { name }
					}
				}
			`,
				ExpectedResult: `
				{
					"namespace": {
						"__typename": "Org",
						"name": "acme"
					}
				}
			`,
			},
		})
	})

	t.Run("invalid", func(t *testing.T) {
		invalidID := "aW52YWxpZDoz"
		wantErr := InvalidNamespaceIDErr{id: graphql.ID(invalidID)}

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, dbmocks.NewMockDB()),
				Query: fmt.Sprintf(`
				{
					namespace(id: %q) {
						__typename
					}
				}
			`, invalidID),
				ExpectedResult: `
				{
					"namespace": null
				}
			`,
				ExpectedErrors: []*gqlerrors.QueryError{
					{
						Path:          []any{"namespace"},
						Message:       wantErr.Error(),
						ResolverError: wantErr,
					},
				},
			},
		})
	})
}

func TestNamespaceByName(t *testing.T) {
	t.Run("user", func(t *testing.T) {
		const (
			wantName   = "alice"
			wantUserID = 123
		)

		ns := dbmocks.NewMockNamespaceStore()
		ns.GetByNameFunc.SetDefaultHook(func(ctx context.Context, name string) (*database.Namespace, error) {
			if name != wantName {
				t.Errorf("got %q, want %q", name, wantName)
			}
			return &database.Namespace{Name: "alice", User: wantUserID}, nil
		})

		users := dbmocks.NewMockUserStore()
		users.GetByIDFunc.SetDefaultHook(func(_ context.Context, id int32) (*types.User, error) {
			if id != wantUserID {
				t.Errorf("got %d, want %d", id, wantUserID)
			}
			return &types.User{ID: wantUserID, Username: wantName}, nil
		})

		db := dbmocks.NewMockDB()
		db.NamespacesFunc.SetDefaultReturn(ns)
		db.UsersFunc.SetDefaultReturn(users)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
				{
					namespaceByName(name: "alice") {
						__typename
						... on User { username }
					}
				}
			`,
				ExpectedResult: `
				{
					"namespaceByName": {
						"__typename": "User",
						"username": "alice"
					}
				}
			`,
			},
		})
		mockrequire.Called(t, ns.GetByNameFunc)
		mockrequire.Called(t, users.GetByIDFunc)
	})

	t.Run("organization", func(t *testing.T) {
		const (
			wantName  = "acme"
			wantOrgID = 3
		)

		ns := dbmocks.NewMockNamespaceStore()
		ns.GetByNameFunc.SetDefaultHook(func(ctx context.Context, name string) (*database.Namespace, error) {
			if name != wantName {
				t.Errorf("got %q, want %q", name, wantName)
			}
			return &database.Namespace{Name: "alice", Organization: wantOrgID}, nil
		})

		orgs := dbmocks.NewMockOrgStore()
		orgs.GetByIDFunc.SetDefaultHook(func(_ context.Context, id int32) (*types.Org, error) {
			if id != wantOrgID {
				t.Errorf("got %d, want %d", id, wantOrgID)
			}
			return &types.Org{ID: wantOrgID, Name: "acme"}, nil
		})

		db := dbmocks.NewMockDB()
		db.NamespacesFunc.SetDefaultReturn(ns)
		db.OrgsFunc.SetDefaultReturn(orgs)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
				{
					namespaceByName(name: "acme") {
						__typename
						... on Org { name }
					}
				}
			`,
				ExpectedResult: `
				{
					"namespaceByName": {
						"__typename": "Org",
						"name": "acme"
					}
				}
			`,
			},
		})

		mockrequire.Called(t, ns.GetByNameFunc)
		mockrequire.Called(t, orgs.GetByIDFunc)
	})

	t.Run("invalid", func(t *testing.T) {
		ns := dbmocks.NewMockNamespaceStore()
		ns.GetByNameFunc.SetDefaultReturn(nil, database.ErrNamespaceNotFound)
		db := dbmocks.NewMockDB()
		db.NamespacesFunc.SetDefaultReturn(ns)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
				{
					namespaceByName(name: "doesntexist") {
						__typename
					}
				}
			`,
				ExpectedResult: `
				{
					"namespaceByName": null
				}
			`,
			},
		})
	})
}
