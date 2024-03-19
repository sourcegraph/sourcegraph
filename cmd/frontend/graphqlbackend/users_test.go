package graphqlbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestUsers(t *testing.T) {
	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
	users.ListFunc.SetDefaultReturn([]*types.User{{Username: "user1"}, {Username: "user2"}}, nil)
	users.CountFunc.SetDefaultReturn(2, nil)
	users.GetByIDFunc.SetDefaultHook(func(ctx context.Context, id int32) (*types.User, error) {
		if id == 1 {
			return &types.User{ID: 1, SiteAdmin: true}, nil
		}
		return nil, database.NewUserNotFoundError(id)
	})

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)

	RunTests(t, []*Test{
		{
			Context: actor.WithActor(context.Background(), actor.FromMockUser(1)),
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				{
					users {
						nodes {
							username
							siteAdmin
						}
						totalCount
					}
				}
			`,
			ExpectedResult: `
				{
					"users": {
						"nodes": [
							{
								"username": "user1",
								"siteAdmin": false
							},
							{
								"username": "user2",
								"siteAdmin": false
							}
						],
						"totalCount": 2
					}
				}
			`,
		},
	})
}

func TestUsers_Pagination(t *testing.T) {
	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
	users.ListFunc.SetDefaultHook(func(ctx context.Context, opt *database.UsersListOptions) ([]*types.User, error) {
		if opt.LimitOffset.Offset == 2 {
			return []*types.User{
				{Username: "user3"},
				{Username: "user4"},
			}, nil
		}
		return []*types.User{
			{Username: "user1"},
			{Username: "user2"},
		}, nil
	})
	users.CountFunc.SetDefaultReturn(4, nil)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)

	RunTests(t, []*Test{
		{
			Schema: mustParseGraphQLSchema(t, db),
			Query: `
				{
					users(first: 2) {
						nodes { username }
						totalCount
						pageInfo { hasNextPage, endCursor }
					}
				}
			`,
			ExpectedResult: `
				{
					"users": {
						"nodes": [
							{
								"username": "user1"
							},
							{
								"username": "user2"
							}
						],
						"totalCount": 4,
						"pageInfo": {
							"hasNextPage": true,
							"endCursor": "2"
						 }
					}
				}
			`,
		},
		{
			Schema: mustParseGraphQLSchema(t, db),
			Query: `
				{
					users(first: 2, after: "2") {
						nodes { username }
						totalCount
						pageInfo { hasNextPage, endCursor }
					}
				}
			`,
			ExpectedResult: `
				{
					"users": {
						"nodes": [
							{
								"username": "user3"
							},
							{
								"username": "user4"
							}
						],
						"totalCount": 4,
						"pageInfo": {
							"hasNextPage": false,
							"endCursor": null
						 }
					}
				}
			`,
		},
	})
}

func TestUsers_Pagination_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	schema := mustParseGraphQLSchema(t, db)

	org, err := db.Orgs().Create(ctx, "acme", nil)
	if err != nil {
		t.Fatal(err)
		return
	}

	newUsers := []struct{ username string }{
		{username: "user1"},
		{username: "user2"},
		{username: "user3"},
		{username: "user4"},
	}
	users := make([]*types.User, len(newUsers))
	for i, newUser := range newUsers {
		user, err := db.Users().Create(ctx, database.NewUser{Username: newUser.username})
		if err != nil {
			t.Fatal(err)
			return
		}
		users[i] = user
		_, err = db.OrgMembers().Create(ctx, org.ID, user.ID)
		if err != nil {
			t.Fatal(err)
			return
		}
	}

	admin := users[0]
	nonadmin := users[1]

	tests := []usersQueryTest{
		// no args
		{
			ctx:            actor.WithActor(ctx, actor.FromUser(admin.ID)),
			wantUsers:      []string{"user1", "user2", "user3", "user4"},
			wantTotalCount: 4,
		},
		// first: 1
		{
			ctx:            actor.WithActor(ctx, actor.FromUser(admin.ID)),
			args:           "first: 1",
			wantUsers:      []string{"user1"},
			wantTotalCount: 4,
		},
		// first: 2
		{
			ctx:            actor.WithActor(ctx, actor.FromUser(admin.ID)),
			args:           "first: 2",
			wantUsers:      []string{"user1", "user2"},
			wantTotalCount: 4,
		},
		// first: 2, after: 2
		{
			ctx:            actor.WithActor(ctx, actor.FromUser(admin.ID)),
			args:           "first: 2, after: \"2\"",
			wantUsers:      []string{"user3", "user4"},
			wantTotalCount: 4,
		},
		// first: 1, after: 2
		{
			ctx:            actor.WithActor(ctx, actor.FromUser(admin.ID)),
			args:           "first: 1, after: \"2\"",
			wantUsers:      []string{"user3"},
			wantTotalCount: 4,
		},
		// no admin on dotcom
		{
			ctx:       actor.WithActor(ctx, actor.FromUser(nonadmin.ID)),
			wantError: "must be site admin",
			dotcom:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.args, func(t *testing.T) {
			runUsersQuery(t, schema, tt)
		})
	}
}

type usersQueryTest struct {
	args string
	ctx  context.Context

	wantError string

	wantUsers []string

	wantNoTotalCount bool
	wantTotalCount   int
	dotcom           bool
}

func runUsersQuery(t *testing.T, schema *graphql.Schema, want usersQueryTest) {
	t.Helper()
	dotcom.MockSourcegraphDotComMode(t, want.dotcom)

	type node struct {
		Username string `json:"username"`
	}

	type pageInfo struct {
		HasNextPage bool `json:"hasNextPage"`
	}

	type users struct {
		Nodes      []node `json:"nodes"`
		TotalCount *int   `json:"totalCount"`
	}

	type expected struct {
		Users users `json:"users"`
	}

	nodes := make([]node, 0, len(want.wantUsers))
	for _, username := range want.wantUsers {
		nodes = append(nodes, node{Username: username})
	}

	ex := expected{
		Users: users{
			Nodes:      nodes,
			TotalCount: &want.wantTotalCount,
		},
	}

	if want.wantNoTotalCount {
		ex.Users.TotalCount = nil
	}

	marshaled, err := json.Marshal(ex)
	if err != nil {
		t.Fatalf("failed to marshal expected repositories query result: %s", err)
	}

	var query string
	if want.args != "" {
		query = fmt.Sprintf(`{ users(%s) { nodes { username } totalCount } } `, want.args)
	} else {
		query = `{ users { nodes { username } totalCount } }`
	}

	if want.wantError != "" {
		RunTest(t, &Test{
			Context:        want.ctx,
			Schema:         schema,
			Query:          query,
			ExpectedResult: `null`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: want.wantError,
					Path:    []any{"users"},
				},
			},
		})
	} else {
		RunTest(t, &Test{
			Context:        want.ctx,
			Schema:         schema,
			Query:          query,
			ExpectedResult: string(marshaled),
		})
	}
}

func TestUsers_InactiveSince(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	schema := mustParseGraphQLSchema(t, db)

	now := time.Now()
	daysAgo := func(days int) time.Time {
		return now.Add(-time.Duration(days) * 24 * time.Hour)
	}

	users := []struct {
		user        database.NewUser
		lastEventAt time.Time
	}{
		{user: database.NewUser{Username: "user-1", Password: "user-1"}, lastEventAt: daysAgo(1)},
		{user: database.NewUser{Username: "user-2", Password: "user-2"}, lastEventAt: daysAgo(2)},
		{user: database.NewUser{Username: "user-3", Password: "user-3"}, lastEventAt: daysAgo(3)},
		{user: database.NewUser{Username: "user-4", Password: "user-4"}, lastEventAt: daysAgo(4)},
	}

	for _, newUser := range users {
		u, err := db.Users().Create(ctx, newUser.user)
		if err != nil {
			t.Fatal(err)
		}

		event := &database.Event{
			UserID:    uint32(u.ID),
			Timestamp: newUser.lastEventAt,
			Name:      "testevent",
			Source:    "test",
		}

		//lint:ignore SA1019 existing usage of deprecated functionality. Use EventRecorder from internal/telemetryrecorder instead.
		if err := db.EventLogs().Insert(ctx, event); err != nil {
			t.Fatal(err)
		}
	}

	ctx = actor.WithInternalActor(ctx)

	query := `
		query InactiveUsers($since: DateTime) {
			users(inactiveSince: $since) {
				nodes { username }
				totalCount
			}
		}
	`

	RunTests(t, []*Test{
		{
			Context:   ctx,
			Schema:    schema,
			Query:     query,
			Variables: map[string]any{"since": daysAgo(4).Format(time.RFC3339Nano)},
			ExpectedResult: `
			{"users": { "nodes": [], "totalCount": 0 }}
			`,
		},
		{
			Context:   ctx,
			Schema:    schema,
			Query:     query,
			Variables: map[string]any{"since": daysAgo(3).Format(time.RFC3339Nano)},
			ExpectedResult: `
			{"users": { "nodes": [{ "username": "user-4" }], "totalCount": 1 }}
			`,
		},
		{
			Context:   ctx,
			Schema:    schema,
			Query:     query,
			Variables: map[string]any{"since": daysAgo(2).Format(time.RFC3339Nano)},
			ExpectedResult: `
			{"users": { "nodes": [{ "username": "user-3" }, { "username": "user-4" }], "totalCount": 2 }}
			`,
		},
		{
			Context:   ctx,
			Schema:    schema,
			Query:     query,
			Variables: map[string]any{"since": daysAgo(1).Format(time.RFC3339Nano)},
			ExpectedResult: `
			{"users": { "nodes": [
				{ "username": "user-2" },
				{ "username": "user-3" },
				{ "username": "user-4" }
			], "totalCount": 3 }}
			`,
		},
		{
			Context:   ctx,
			Schema:    schema,
			Query:     query,
			Variables: map[string]any{"since": daysAgo(0).Format(time.RFC3339Nano)},
			ExpectedResult: `
			{"users": { "nodes": [
				{ "username": "user-1" },
				{ "username": "user-2" },
				{ "username": "user-3" },
				{ "username": "user-4" }
			], "totalCount": 4 }}
			`,
		},
	})
}

func TestUsers_CreatePassword(t *testing.T) {
	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

	db := dbmocks.NewMockDB()
	db.UsersFunc.SetDefaultReturn(users)

	actorFromSession := actor.FromMockUser(1)
	actorFromSession.FromSessionCookie = true
	actorNotFromSession := actor.FromMockUser(2)
	actorNotFromSession.FromSessionCookie = false

	RunTests(t, []*Test{
		{
			Label:   "Actor from session",
			Context: actor.WithActor(context.Background(), actorFromSession),
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				mutation {
					createPassword(newPassword:"i am gr00t1234!!") {
					  alwaysNil
					}
				  }
			`,
			ExpectedResult: `
				{
					"createPassword": {
						"alwaysNil": null
					}
				}
			`,
		},
		{
			Label:   "Actor not from session (token)",
			Context: actor.WithActor(context.Background(), actorNotFromSession),
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				mutation {
					createPassword(newPassword:"i am gr00t1234!!") {
					  alwaysNil
					}
				  }
			`,
			ExpectedResult: `{ "createPassword": null }`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: "only allowed from user session",
					Path:    []any{"createPassword"},
				},
			},
		},
	})
}
