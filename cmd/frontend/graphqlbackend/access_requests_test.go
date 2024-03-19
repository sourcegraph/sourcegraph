package graphqlbackend

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestAccessRequestNode(t *testing.T) {
	mockAccessRequest := &types.AccessRequest{
		ID:             1,
		Email:          "a1@example.com",
		Name:           "a1",
		CreatedAt:      time.Now(),
		AdditionalInfo: "af1",
		Status:         types.AccessRequestStatusPending,
	}
	db := dbmocks.NewMockDB()

	accessRequestStore := dbmocks.NewMockAccessRequestStore()
	db.AccessRequestsFunc.SetDefaultReturn(accessRequestStore)
	accessRequestStore.GetByIDFunc.SetDefaultReturn(mockAccessRequest, nil)

	userStore := dbmocks.NewMockUserStore()
	db.UsersFunc.SetDefaultReturn(userStore)
	userStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

	RunTest(t, &Test{
		Schema:  mustParseGraphQLSchema(t, db),
		Context: ctx,
		Query: `
		query AccessRequestID($id: ID!){
			node(id: $id) {
				__typename
				... on AccessRequest {
					name
				}
			}
		}`,
		ExpectedResult: `{
			"node": {
				"__typename": "AccessRequest",
				"name": "a1"
			}
		}`,
		Variables: map[string]any{
			"id": string(marshalAccessRequestID(mockAccessRequest.ID)),
		},
	})
}

func TestAccessRequestsQuery(t *testing.T) {
	const accessRequestsQuery = `
	query GetAccessRequests($first: Int, $after: String, $before: String, $last: Int) {
		accessRequests(first: $first, after: $after, before: $before, last: $last) {
			nodes {
				id
				name
				email
				status
				createdAt
				additionalInfo
			}
			totalCount
			pageInfo {
				hasNextPage
				hasPreviousPage
				startCursor
				endCursor
			}
		}
	}`

	db := dbmocks.NewMockDB()

	userStore := dbmocks.NewMockUserStore()
	db.UsersFunc.SetDefaultReturn(userStore)

	accessRequestStore := dbmocks.NewMockAccessRequestStore()
	db.AccessRequestsFunc.SetDefaultReturn(accessRequestStore)

	t.Parallel()

	t.Run("non-admin user", func(t *testing.T) {
		userStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: false}, nil)
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
		RunTest(t, &Test{
			Schema:         mustParseGraphQLSchema(t, db),
			Context:        ctx,
			Query:          accessRequestsQuery,
			ExpectedResult: `null`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Path:          []any{"accessRequests"},
					Message:       auth.ErrMustBeSiteAdmin.Error(),
					ResolverError: auth.ErrMustBeSiteAdmin,
				},
			},
			Variables: map[string]any{
				"first": 10,
			},
		})
	})

	t.Run("admin user", func(t *testing.T) {
		createdAtTime, _ := time.Parse(time.RFC3339, "2023-02-24T14:48:30Z")
		mockAccessRequests := []*types.AccessRequest{
			{ID: 1, Email: "a1@example.com", Name: "a1", CreatedAt: createdAtTime, AdditionalInfo: "af1", Status: types.AccessRequestStatusPending},
			{ID: 2, Email: "a2@example.com", Name: "a2", CreatedAt: createdAtTime, Status: types.AccessRequestStatusApproved},
			{ID: 3, Email: "a3@example.com", Name: "a3", CreatedAt: createdAtTime, Status: types.AccessRequestStatusRejected},
		}

		accessRequestStore.ListFunc.SetDefaultReturn(mockAccessRequests, nil)
		accessRequestStore.CountFunc.SetDefaultReturn(len(mockAccessRequests), nil)
		userStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query:   accessRequestsQuery,
			ExpectedResult: `{
				"accessRequests": {
					"nodes": [
						{
							"id": "QWNjZXNzUmVxdWVzdDox",
							"name": "a1",
							"email": "a1@example.com",
							"status": "PENDING",
							"createdAt": "2023-02-24T14:48:30Z",
							"additionalInfo": "af1"
						},
						{
							"id": "QWNjZXNzUmVxdWVzdDoy",
							"name": "a2",
							"email": "a2@example.com",
							"status": "APPROVED",
							"createdAt": "2023-02-24T14:48:30Z",
							"additionalInfo": ""
						},
						{
							"id": "QWNjZXNzUmVxdWVzdDoz",
							"name": "a3",
							"email": "a3@example.com",
							"status": "REJECTED",
							"createdAt": "2023-02-24T14:48:30Z",
							"additionalInfo": ""
						}
					],
					"totalCount": 3,
					"pageInfo": {
						"hasNextPage": false,
						"hasPreviousPage": false,
						"startCursor": "QWNjZXNzUmVxdWVzdDox",
						"endCursor": "QWNjZXNzUmVxdWVzdDoz"
					}
				}
			}`,
			Variables: map[string]any{
				"first": 10,
			},
		})
	})
}

func TestSetAccessRequestStatusMutation(t *testing.T) {
	const setAccessRequestStatusMutation = `
	mutation SetAccessRequestStatus($id: ID!, $status: AccessRequestStatus!) {
		setAccessRequestStatus(id: $id, status: $status) {
			alwaysNil
		}
	}`
	db := dbmocks.NewMockDB()
	db.WithTransactFunc.SetDefaultHook(func(ctx context.Context, f func(database.DB) error) error {
		return f(db)
	})

	userStore := dbmocks.NewMockUserStore()
	db.UsersFunc.SetDefaultReturn(userStore)

	t.Parallel()

	t.Run("non-admin user", func(t *testing.T) {
		accessRequestStore := dbmocks.NewMockAccessRequestStore()
		db.AccessRequestsFunc.SetDefaultReturn(accessRequestStore)

		userStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: false}, nil)
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		RunTest(t, &Test{
			Schema:         mustParseGraphQLSchema(t, db),
			Context:        ctx,
			Query:          setAccessRequestStatusMutation,
			ExpectedResult: `{"setAccessRequestStatus": null }`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Path:          []any{"setAccessRequestStatus"},
					Message:       auth.ErrMustBeSiteAdmin.Error(),
					ResolverError: auth.ErrMustBeSiteAdmin,
				},
			},
			Variables: map[string]any{
				"id":     string(marshalAccessRequestID(1)),
				"status": string(types.AccessRequestStatusApproved),
			},
		})
		assert.Len(t, accessRequestStore.UpdateFunc.History(), 0)
	})

	t.Run("existing access request", func(t *testing.T) {
		accessRequestStore := dbmocks.NewMockAccessRequestStore()
		db.AccessRequestsFunc.SetDefaultReturn(accessRequestStore)

		createdAtTime, _ := time.Parse(time.RFC3339, "2023-02-24T14:48:30Z")
		mockAccessRequest := &types.AccessRequest{ID: 1, Email: "a1@example.com", Name: "a1", CreatedAt: createdAtTime, AdditionalInfo: "af1", Status: types.AccessRequestStatusPending}
		accessRequestStore.GetByIDFunc.SetDefaultReturn(mockAccessRequest, nil)
		accessRequestStore.UpdateFunc.SetDefaultReturn(mockAccessRequest, nil)
		userID := int32(123)
		userStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: userID, SiteAdmin: true}, nil)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		RunTest(t, &Test{
			Schema:         mustParseGraphQLSchema(t, db),
			Context:        ctx,
			Query:          setAccessRequestStatusMutation,
			ExpectedResult: `{"setAccessRequestStatus": { "alwaysNil": null } }`,
			Variables: map[string]any{
				"id":     string(marshalAccessRequestID(1)),
				"status": string(types.AccessRequestStatusApproved),
			},
		})
		assert.Len(t, accessRequestStore.UpdateFunc.History(), 1)
		assert.Equal(t, types.AccessRequest{ID: mockAccessRequest.ID, DecisionByUserID: &userID, Status: types.AccessRequestStatusApproved}, *accessRequestStore.UpdateFunc.History()[0].Arg1)
	})

	t.Run("non-existing access request", func(t *testing.T) {
		accessRequestStore := dbmocks.NewMockAccessRequestStore()
		db.AccessRequestsFunc.SetDefaultReturn(accessRequestStore)

		notFoundErr := &database.ErrAccessRequestNotFound{ID: 1}
		accessRequestStore.GetByIDFunc.SetDefaultReturn(nil, notFoundErr)

		userStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)
		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		RunTest(t, &Test{
			Schema:         mustParseGraphQLSchema(t, db),
			Context:        ctx,
			Query:          setAccessRequestStatusMutation,
			ExpectedResult: `{"setAccessRequestStatus": null }`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Path:          []any{"setAccessRequestStatus"},
					Message:       "access_request with ID 1 not found",
					ResolverError: notFoundErr,
				},
			},
			Variables: map[string]any{
				"id":     string(marshalAccessRequestID(1)),
				"status": string(types.AccessRequestStatusApproved),
			},
		})
		assert.Len(t, accessRequestStore.UpdateFunc.History(), 0)
	})
}

func TestAccessRequestConnectionStore(t *testing.T) {
	ctx := context.Background()

	db := database.NewDB(logtest.Scoped(t), dbtest.NewDB(t))
	for i := range 10 {
		_, err := db.AccessRequests().Create(ctx, &types.AccessRequest{
			Name:   "test" + strconv.Itoa(i),
			Email:  fmt.Sprintf("test%d@sourcegraph.com", i),
			Status: types.AccessRequestStatusPending,
		})
		require.NoError(t, err)
	}

	connectionStore := &accessRequestConnectionStore{
		db: db,
	}

	graphqlutil.TestConnectionResolverStoreSuite(t, connectionStore)
}
