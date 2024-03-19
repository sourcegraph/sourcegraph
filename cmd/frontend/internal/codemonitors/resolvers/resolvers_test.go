package resolvers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	batchesApitest "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/codemonitors/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/codemonitors/background"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/settings"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestCreateCodeMonitor(t *testing.T) {
	ctx := actor.WithInternalActor(context.Background())
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	r := newTestResolver(t, db)

	settings.MockCurrentUserFinal = &schema.Settings{}

	user := insertTestUser(t, db, "cm-user1", true)

	want := &database.Monitor{
		ID:          1,
		CreatedBy:   user.ID,
		CreatedAt:   r.Now(),
		ChangedBy:   user.ID,
		ChangedAt:   r.Now(),
		Description: "test monitor",
		Enabled:     true,
		UserID:      user.ID,
	}
	ctx = actor.WithActor(ctx, actor.FromUser(user.ID))

	t.Run("create monitor", func(t *testing.T) {
		got, err := r.insertTestMonitorWithOpts(ctx, t)
		require.NoError(t, err)
		castGot := got.(*monitor).Monitor
		castGot.CreatedAt, castGot.ChangedAt = want.CreatedAt, want.ChangedAt // overwrite after comparing with time equality
		require.EqualValues(t, want, castGot)

		// Toggle field enabled from true to false.
		got, err = r.ToggleCodeMonitor(ctx, &graphqlbackend.ToggleCodeMonitorArgs{
			Id:      relay.MarshalID(MonitorKind, got.(*monitor).Monitor.ID),
			Enabled: false,
		})
		require.NoError(t, err)
		require.False(t, got.(*monitor).Monitor.Enabled)

		// Delete code monitor.
		_, err = r.DeleteCodeMonitor(ctx, &graphqlbackend.DeleteCodeMonitorArgs{Id: got.ID()})
		require.NoError(t, err)
		_, err = r.db.CodeMonitors().GetMonitor(ctx, got.(*monitor).Monitor.ID)
		require.Error(t, err, "monitor should have been deleted")
	})

	t.Run("invalid slack webhook", func(t *testing.T) {
		namespace := relay.MarshalID("User", user.ID)
		_, err := r.CreateCodeMonitor(ctx, &graphqlbackend.CreateCodeMonitorArgs{
			Monitor: &graphqlbackend.CreateMonitorArgs{Namespace: namespace},
			Trigger: &graphqlbackend.CreateTriggerArgs{Query: "repo:."},
			Actions: []*graphqlbackend.CreateActionArgs{{
				SlackWebhook: &graphqlbackend.CreateActionSlackWebhookArgs{
					URL: "https://internal:3443",
				},
			}},
		})
		require.Error(t, err)
	})

	t.Run("invalid query", func(t *testing.T) {
		namespace := relay.MarshalID("User", user.ID)
		_, err := r.CreateCodeMonitor(ctx, &graphqlbackend.CreateCodeMonitorArgs{
			Monitor: &graphqlbackend.CreateMonitorArgs{Namespace: namespace},
			Trigger: &graphqlbackend.CreateTriggerArgs{Query: "type:commit (repo:a b) or (repo:c d)"}, // invalid query
			Actions: []*graphqlbackend.CreateActionArgs{{
				SlackWebhook: &graphqlbackend.CreateActionSlackWebhookArgs{
					URL: "https://internal:3443",
				},
			}},
		})
		require.Error(t, err)
		monitors, err := r.Monitors(ctx, &user.ID, &graphqlbackend.ListMonitorsArgs{First: 10})
		require.NoError(t, err)
		require.Len(t, monitors.Nodes(), 0) // the transaction should have been rolled back
	})
}

func TestListCodeMonitors(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := actor.WithInternalActor(context.Background())
	db := database.NewDB(logger, dbtest.NewDB(t))
	r := newTestResolver(t, db)

	user := insertTestUser(t, db, "cm-user1", true)
	ctx = actor.WithActor(ctx, actor.FromUser(user.ID))

	// Create a monitor.
	_, err := r.insertTestMonitorWithOpts(ctx, t)
	require.NoError(t, err)

	args := &graphqlbackend.ListMonitorsArgs{
		First: 5,
	}
	r1, err := r.Monitors(ctx, &user.ID, args)
	require.NoError(t, err)

	require.Len(t, r1.Nodes(), 1, "unexpected node count")
	require.False(t, r1.PageInfo().HasNextPage())

	// Create enough monitors to necessitate paging
	for range 10 {
		_, err := r.insertTestMonitorWithOpts(ctx, t)
		require.NoError(t, err)
	}

	r2, err := r.Monitors(ctx, &user.ID, args)
	require.NoError(t, err)

	require.Len(t, r2.Nodes(), 5, "unexpected node count")
	require.True(t, r2.PageInfo().HasNextPage())

	// The returned cursor should be usable to return the remaining monitors
	pi := r2.PageInfo()
	args = &graphqlbackend.ListMonitorsArgs{
		First: 10,
		After: pi.EndCursor(),
	}
	r3, err := r.Monitors(ctx, &user.ID, args)
	require.NoError(t, err)

	require.Len(t, r3.Nodes(), 6, "unexpected node count")
	require.False(t, r3.PageInfo().HasNextPage())
}

func TestListCodeMonitors_MissingUsers(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := actor.WithInternalActor(context.Background())
	db := database.NewDB(logger, dbtest.NewDB(t))
	r := newTestResolver(t, db)

	user := insertTestUser(t, db, "cm-user1", true)
	ctx = actor.WithActor(ctx, actor.FromUser(user.ID))

	// Create a monitor.
	_, err := r.insertTestMonitorWithOpts(ctx, t)
	require.NoError(t, err)

	args := &graphqlbackend.ListMonitorsArgs{
		First: 5,
	}

	r1, err := r.Monitors(ctx, &user.ID, args)
	require.NoError(t, err)

	require.Len(t, r1.Nodes(), 1, "unexpected node count")
	require.False(t, r1.PageInfo().HasNextPage())

	// Soft-delete user
	err = db.Users().Delete(ctx, user.ID)
	require.NoError(t, err)

	r2, err := r.Monitors(ctx, &user.ID, args)
	require.NoError(t, err)

	require.Len(t, r2.Nodes(), 0, "unexpected node count")
	require.False(t, r2.PageInfo().HasNextPage())
}

func TestIsAllowedToEdit(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))

	// Setup users and org
	owner := insertTestUser(t, db, "cm-user1", false)
	notOwner := insertTestUser(t, db, "cm-user2", false)
	siteAdmin := insertTestUser(t, db, "cm-user3", true)

	r := newTestResolver(t, db)

	// Create a monitor and set org as owner.
	ownerOpt := WithOwner(relay.MarshalID("User", owner.ID))
	admContext := actor.WithActor(context.Background(), actor.FromUser(siteAdmin.ID))
	m, err := r.insertTestMonitorWithOpts(admContext, t, ownerOpt)
	require.NoError(t, err)

	tests := []struct {
		user    int32
		allowed bool
	}{
		{
			user:    owner.ID,
			allowed: true,
		},
		{
			user:    notOwner.ID,
			allowed: false,
		},
		{
			user:    siteAdmin.ID,
			allowed: true,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("user %d", tt.user), func(t *testing.T) {
			ctx := actor.WithActor(context.Background(), actor.FromUser(tt.user))
			if err := r.isAllowedToEdit(ctx, m.ID()); (err != nil) == tt.allowed {
				t.Fatalf("unexpected permissions for user %d", tt.user)
			}
		})
	}

	t.Run("cannot change namespace to one not editable by caller", func(t *testing.T) {
		ctx := actor.WithActor(context.Background(), actor.FromUser(owner.ID))
		notMemberNamespace := relay.MarshalID("User", notOwner.ID)
		args := &graphqlbackend.UpdateCodeMonitorArgs{
			Monitor: &graphqlbackend.EditMonitorArgs{
				Id: m.ID(),
				Update: &graphqlbackend.CreateMonitorArgs{
					Namespace:   notMemberNamespace,
					Description: "updated",
				},
			},
		}

		_, err = r.UpdateCodeMonitor(ctx, args)
		require.EqualError(t, err, fmt.Sprintf("update namespace: %s", auth.ErrMustBeSiteAdminOrSameUser.Error()))
	})
}

func TestIsAllowedToCreate(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))

	// Setup users and org
	member := insertTestUser(t, db, "cm-user1", false)
	notMember := insertTestUser(t, db, "cm-user2", false)
	siteAdmin := insertTestUser(t, db, "cm-user3", true)

	admContext := actor.WithActor(context.Background(), actor.FromUser(siteAdmin.ID))
	org, err := db.Orgs().Create(admContext, "cm-test-org", nil)
	require.NoError(t, err)
	addUserToOrg(t, db, member.ID, org.ID)

	r := newTestResolver(t, db)

	tests := []struct {
		user    int32
		owner   graphql.ID
		allowed bool
	}{
		{
			user:    member.ID,
			owner:   relay.MarshalID("Org", org.ID),
			allowed: false,
		},
		{
			user:    member.ID,
			owner:   relay.MarshalID("User", notMember.ID),
			allowed: false,
		},
		{
			user:    notMember.ID,
			owner:   relay.MarshalID("Org", org.ID),
			allowed: false,
		},
		{
			user:    siteAdmin.ID,
			owner:   relay.MarshalID("Org", org.ID),
			allowed: false, // Error creating org owner
		},
		{
			user:    siteAdmin.ID,
			owner:   relay.MarshalID("User", member.ID),
			allowed: true,
		},
		{
			user:    siteAdmin.ID,
			owner:   relay.MarshalID("User", notMember.ID),
			allowed: true,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("user %d", tt.user), func(t *testing.T) {
			ctx := actor.WithActor(context.Background(), actor.FromUser(tt.user))
			if err := r.isAllowedToCreate(ctx, tt.owner); (err != nil) == tt.allowed {
				t.Fatalf("unexpected permissions for user %d", tt.user)
			}
		})
	}
}

func graphqlUserID(id int32) graphql.ID {
	return relay.MarshalID("User", id)
}

func TestQueryMonitor(t *testing.T) {
	t.Skip("Flake: https://github.com/sourcegraph/sourcegraph/issues/30477")

	logger := logtest.Scoped(t)

	ctx := actor.WithInternalActor(context.Background())
	db := database.NewDB(logger, dbtest.NewDB(t))
	r := newTestResolver(t, db)

	// Create 2 test users.
	user1 := insertTestUser(t, db, "cm-user1", true)
	user2 := insertTestUser(t, db, "cm-user2", true)

	// Create 2 code monitors, each with 1 trigger, 2 actions and two recipients per action.
	ctx = actor.WithActor(ctx, actor.FromUser(user1.ID))
	actionOpt := WithActions([]*graphqlbackend.CreateActionArgs{
		{
			Email: &graphqlbackend.CreateActionEmailArgs{
				Enabled:    false,
				Priority:   "NORMAL",
				Recipients: []graphql.ID{graphqlUserID(user1.ID), graphqlUserID(user2.ID)},
				Header:     "test header 1",
			},
		},
		{
			Email: &graphqlbackend.CreateActionEmailArgs{
				Enabled:    true,
				Priority:   "CRITICAL",
				Recipients: []graphql.ID{graphqlUserID(user1.ID), graphqlUserID(user2.ID)},
				Header:     "test header 2",
			},
		},
		{
			Webhook: &graphqlbackend.CreateActionWebhookArgs{
				Enabled:        true,
				IncludeResults: false,
				URL:            "https://generic.webhook.com",
			},
		},
		{
			SlackWebhook: &graphqlbackend.CreateActionSlackWebhookArgs{
				Enabled:        true,
				IncludeResults: false,
				URL:            "https://slack.webhook.com",
			},
		},
	})
	m, err := r.insertTestMonitorWithOpts(ctx, t, actionOpt)
	require.NoError(t, err)

	// The hooks allows us to test more complex queries by creating a realistic state
	// in the database. After we create the monitor they fill the job tables and
	// update the job status.
	postHookOpt := WithPostHooks([]hook{
		func() error { _, err := r.db.CodeMonitors().EnqueueQueryTriggerJobs(ctx); return err },
		func() error { _, err := r.db.CodeMonitors().EnqueueActionJobsForMonitor(ctx, 1, 1); return err },
		func() error {
			err := (&database.TestStore{CodeMonitorStore: r.db.CodeMonitors()}).SetJobStatus(ctx, database.ActionJobs, database.Completed, 1)
			if err != nil {
				return err
			}
			err = (&database.TestStore{CodeMonitorStore: r.db.CodeMonitors()}).SetJobStatus(ctx, database.ActionJobs, database.Completed, 2)
			if err != nil {
				return err
			}
			return (&database.TestStore{CodeMonitorStore: r.db.CodeMonitors()}).SetJobStatus(ctx, database.ActionJobs, database.Completed, 3)
		},
		func() error { _, err := r.db.CodeMonitors().EnqueueActionJobsForMonitor(ctx, 1, 1); return err },
		// Set the job status of trigger job with id = 1 to "completed". Since we already
		// created another monitor, there is still a second trigger job (id = 2) which
		// remains in status queued.
		//
		// -- cm_trigger_jobs --
		// id  query state
		// 1   1     completed
		// 2   2     queued
		func() error {
			return (&database.TestStore{CodeMonitorStore: r.db.CodeMonitors()}).SetJobStatus(ctx, database.TriggerJobs, database.Completed, 1)
		},
		// This will create a second trigger job (id = 3) for the first monitor. Since
		// the job with id = 2 is still queued, no new job will be enqueued for query 2.
		//
		// -- cm_trigger_jobs --
		// id  query state
		// 1   1     completed
		// 2   2     queued
		// 3   1	 queued
		func() error { _, err := r.db.CodeMonitors().EnqueueQueryTriggerJobs(ctx); return err },
		// To have a consistent state we have to log the number of search results for
		// each completed trigger job.
		func() error {
			return r.db.CodeMonitors().UpdateTriggerJobWithResults(ctx, 1, "", make([]*result.CommitMatch, 1))
		},
	})
	_, err = r.insertTestMonitorWithOpts(ctx, t, actionOpt, postHookOpt)
	require.NoError(t, err)

	gqlSchema, err := graphqlbackend.NewSchemaWithCodeMonitorsResolver(db, r)
	require.NoError(t, err)

	t.Run("query by user", func(t *testing.T) {
		queryByUser(ctx, t, gqlSchema, r, user1, user2)
	})
	t.Run("query by ID", func(t *testing.T) {
		queryByID(ctx, t, gqlSchema, r, m.(*monitor), user1, user2)
	})
	t.Run("monitor paging", func(t *testing.T) {
		monitorPaging(ctx, t, gqlSchema, user1)
	})
	t.Run("recipients paging", func(t *testing.T) {
		recipientPaging(ctx, t, gqlSchema, user1, user2)
	})
	t.Run("actions paging", func(t *testing.T) {
		actionPaging(ctx, t, gqlSchema, user1)
	})
	t.Run("trigger events paging", func(t *testing.T) {
		triggerEventPaging(ctx, t, gqlSchema, user1)
	})
	t.Run("action events paging", func(t *testing.T) {
		actionEventPaging(ctx, t, gqlSchema, user1)
	})
}

func queryByUser(ctx context.Context, t *testing.T, schema *graphql.Schema, r *Resolver, user1 *types.User, user2 *types.User) {
	input := map[string]any{
		"userName":     user1.Username,
		"actionCursor": relay.MarshalID(monitorActionEmailKind, 1),
	}
	response := apitest.Response{}
	batchesApitest.MustExec(ctx, t, schema, input, &response, queryByUserFmtStr)

	triggerEventEndCursor := string(relay.MarshalID(monitorTriggerEventKind, 1))
	want := apitest.Response{
		User: apitest.User{
			Monitors: apitest.MonitorConnection{
				TotalCount: 2,
				Nodes: []apitest.Monitor{{
					Id:          string(relay.MarshalID(MonitorKind, 1)),
					Description: "test monitor",
					Enabled:     true,
					Owner:       apitest.UserOrg{Name: user1.Username},
					CreatedBy:   apitest.UserOrg{Name: user1.Username},
					CreatedAt:   marshalDateTime(t, r.Now()),
					Trigger: apitest.Trigger{
						Id:    string(relay.MarshalID(monitorTriggerQueryKind, 1)),
						Query: "repo:foo",
						Events: apitest.TriggerEventConnection{
							Nodes: []apitest.TriggerEvent{
								{
									Id:        string(relay.MarshalID(monitorTriggerEventKind, 1)),
									Status:    "SUCCESS",
									Timestamp: r.Now().UTC().Format(time.RFC3339),
									Message:   nil,
								},
							},
							TotalCount: 2,
							PageInfo: apitest.PageInfo{
								HasNextPage: true,
								EndCursor:   &triggerEventEndCursor,
							},
						},
					},
					Actions: apitest.ActionConnection{
						TotalCount: 4,
						Nodes: []apitest.Action{{
							Email: &apitest.ActionEmail{
								Id:       string(relay.MarshalID(monitorActionEmailKind, 2)),
								Enabled:  true,
								Priority: "CRITICAL",
								Recipients: apitest.RecipientsConnection{
									TotalCount: 2,
									Nodes: []apitest.UserOrg{
										{Name: user1.Username},
										{Name: user2.Username},
									},
								},
								Header: "test header 2",
								Events: apitest.ActionEventConnection{
									Nodes: []apitest.ActionEvent{
										{
											Id:        string(relay.MarshalID(monitorActionEmailEventKind, 1)),
											Status:    "SUCCESS",
											Timestamp: r.Now().UTC().Format(time.RFC3339),
											Message:   nil,
										},
										{
											Id:        string(relay.MarshalID(monitorActionEmailEventKind, 4)),
											Status:    "PENDING",
											Timestamp: r.Now().UTC().Format(time.RFC3339),
											Message:   nil,
										},
									},
									TotalCount: 2,
									PageInfo: apitest.PageInfo{
										HasNextPage: true,
										EndCursor:   func() *string { s := string(relay.MarshalID(monitorActionEmailEventKind, 4)); return &s }(),
									},
								},
							},
						}, {
							Webhook: &apitest.ActionWebhook{
								Id:      string(relay.MarshalID(monitorActionWebhookKind, 1)),
								Enabled: true,
								URL:     "https://generic.webhook.com",
								Events: apitest.ActionEventConnection{
									Nodes: []apitest.ActionEvent{
										{
											Id:        string(relay.MarshalID(monitorActionEmailEventKind, 2)),
											Status:    "SUCCESS",
											Timestamp: r.Now().UTC().Format(time.RFC3339),
											Message:   nil,
										},
										{
											Id:        string(relay.MarshalID(monitorActionEmailEventKind, 5)),
											Status:    "PENDING",
											Timestamp: r.Now().UTC().Format(time.RFC3339),
											Message:   nil,
										},
									},
									TotalCount: 2,
									PageInfo: apitest.PageInfo{
										HasNextPage: true,
										EndCursor:   func() *string { s := string(relay.MarshalID(monitorActionEmailEventKind, 5)); return &s }(),
									},
								},
							},
						}, {
							SlackWebhook: &apitest.ActionSlackWebhook{
								Id:      string(relay.MarshalID(monitorActionSlackWebhookKind, 1)),
								Enabled: true,
								URL:     "https://slack.webhook.com",
								Events: apitest.ActionEventConnection{
									Nodes: []apitest.ActionEvent{
										{
											Id:        string(relay.MarshalID(monitorActionEmailEventKind, 3)),
											Status:    "SUCCESS",
											Timestamp: r.Now().UTC().Format(time.RFC3339),
											Message:   nil,
										},
										{
											Id:        string(relay.MarshalID(monitorActionEmailEventKind, 6)),
											Status:    "PENDING",
											Timestamp: r.Now().UTC().Format(time.RFC3339),
											Message:   nil,
										},
									},
									TotalCount: 2,
									PageInfo: apitest.PageInfo{
										HasNextPage: true,
										EndCursor:   func() *string { s := string(relay.MarshalID(monitorActionEmailEventKind, 6)); return &s }(),
									},
								},
							},
						}},
					},
				}},
			},
		},
	}

	if diff := cmp.Diff(want, response); diff != "" {
		t.Fatalf(diff)
	}
}

const queryByUserFmtStr = `
fragment u on User { id, username }
fragment o on Org { id, name }

query($userName: String!, $actionCursor: String!){
	user(username:$userName){
		monitors(first:1){
			totalCount
			nodes{
				id
				description
				enabled
				owner {
					... on User { ...u }
					... on Org { ...o }
				}
				createdBy { ...u }
				createdAt
				trigger {
					... on MonitorQuery {
						__typename
						id
						query
						events(first:1) {
							totalCount
							nodes {
								id
								status
								timestamp
								message
							}
							pageInfo {
								hasNextPage
								endCursor
							}
						}
					}
				}
				actions(first:3, after:$actionCursor){
					totalCount
					nodes{
						... on MonitorEmail{
							__typename
							id
							priority
							header
							enabled
							recipients {
								totalCount
								nodes {
									... on User { ...u }
									... on Org { ...o }
								}
							}
							events {
								totalCount
								nodes {
									id
									status
									timestamp
									message
								}
								pageInfo {
									hasNextPage
									endCursor
								}
							}
						}
						... on MonitorWebhook{
							__typename
							id
							enabled
							url
							events {
								totalCount
								nodes {
									id
									status
									timestamp
									message
								}
								pageInfo {
									hasNextPage
									endCursor
								}
							}
						}
						... on MonitorSlackWebhook{
							__typename
							id
							enabled
							url
							events {
								totalCount
								nodes {
									id
									status
									timestamp
									message
								}
								pageInfo {
									hasNextPage
									endCursor
								}
							}
						}
					}
				}
			}
		}
	}
}
`

func TestEditCodeMonitor(t *testing.T) {
	t.Skip("Flake: https://github.com/sourcegraph/sourcegraph/issues/30477")

	logger := logtest.Scoped(t)

	ctx := actor.WithInternalActor(context.Background())
	db := database.NewDB(logger, dbtest.NewDB(t))
	r := newTestResolver(t, db)

	// Create 2 test users.
	user1 := insertTestUser(t, db, "cm-user1", true)
	ns1 := relay.MarshalID("User", user1.ID)

	user2 := insertTestUser(t, db, "cm-user2", true)
	ns2 := relay.MarshalID("User", user2.ID)

	// Create a code monitor with 1 trigger and 2 actions.
	ctx = actor.WithActor(ctx, actor.FromUser(user1.ID))
	actionOpt := WithActions([]*graphqlbackend.CreateActionArgs{
		{
			Email: &graphqlbackend.CreateActionEmailArgs{
				Enabled:    true,
				Priority:   "NORMAL",
				Recipients: []graphql.ID{ns1},
				Header:     "header action 1",
			},
		}, {
			Email: &graphqlbackend.CreateActionEmailArgs{
				Enabled:    true,
				Priority:   "NORMAL",
				Recipients: []graphql.ID{ns1, ns2},
				Header:     "header action 2",
			},
		}, {
			Webhook: &graphqlbackend.CreateActionWebhookArgs{
				Enabled: true,
				URL:     "https://generic.webhook.com",
			},
		},
	})
	_, err := r.insertTestMonitorWithOpts(ctx, t, actionOpt)
	require.NoError(t, err)

	// Update the code monitor.
	// We update all fields, delete one action, and add a new action.
	gqlSchema, err := graphqlbackend.NewSchemaWithCodeMonitorsResolver(db, r)
	require.NoError(t, err)
	updateInput := map[string]any{
		"monitorID": string(relay.MarshalID(MonitorKind, 1)),
		"triggerID": string(relay.MarshalID(monitorTriggerQueryKind, 1)),
		"actionID":  string(relay.MarshalID(monitorActionEmailKind, 1)),
		"webhookID": string(relay.MarshalID(monitorActionWebhookKind, 1)),
		"user1ID":   ns1,
		"user2ID":   ns2,
	}
	got := apitest.UpdateCodeMonitorResponse{}
	batchesApitest.MustExec(ctx, t, gqlSchema, updateInput, &got, editMonitor)

	want := apitest.UpdateCodeMonitorResponse{
		UpdateCodeMonitor: apitest.Monitor{
			Id:          string(relay.MarshalID(MonitorKind, 1)),
			Description: "updated test monitor",
			Enabled:     false,
			Owner: apitest.UserOrg{
				Name: user1.Username,
			},
			CreatedBy: apitest.UserOrg{
				Name: user1.Username,
			},
			CreatedAt: got.UpdateCodeMonitor.CreatedAt,
			Trigger: apitest.Trigger{
				Id:    string(relay.MarshalID(monitorTriggerQueryKind, 1)),
				Query: "repo:bar",
			},
			Actions: apitest.ActionConnection{
				Nodes: []apitest.Action{{
					Email: &apitest.ActionEmail{
						Id:       string(relay.MarshalID(monitorActionEmailKind, 1)),
						Enabled:  false,
						Priority: "CRITICAL",
						Recipients: apitest.RecipientsConnection{
							Nodes: []apitest.UserOrg{
								{
									Name: user2.Username,
								},
							},
						},
						Header: "updated header action 1",
					},
				}, {
					Webhook: &apitest.ActionWebhook{
						Enabled: true,
						URL:     "https://generic.webhook.com",
					},
				}, {
					SlackWebhook: &apitest.ActionSlackWebhook{
						Enabled: true,
						URL:     "https://slack.webhook.com",
					},
				}},
			},
		},
	}

	require.Equal(t, want, got)
}

const editMonitor = `
fragment u on User {
	id
	username
}

fragment o on Org {
	id
	name
}

mutation ($monitorID: ID!, $triggerID: ID!, $actionID: ID!, $user1ID: ID!, $user2ID: ID!, $webhookID: ID!) {
	updateCodeMonitor(
		monitor: {id: $monitorID, update: {description: "updated test monitor", enabled: false, namespace: $user1ID}},
		trigger: {id: $triggerID, update: {query: "repo:bar"}},
		actions: [
		{email: {id: $actionID, update: {enabled: false, priority: CRITICAL, recipients: [$user2ID], header: "updated header action 1"}}}
		{webhook: {id: $webhookID, update: {enabled: true, url: "https://generic.webhook.com"}}}
		{slackWebhook: {update: {enabled: true, url: "https://slack.webhook.com"}}}
		]
	)
	{
		id
		description
		enabled
		owner {
			... on User {
				...u
			}
			... on Org {
				...o
			}
		}
		createdBy {
			...u
		}
		createdAt
		trigger {
			... on MonitorQuery {
				__typename
				id
				query
			}
		}
		actions {
			nodes {
				... on MonitorEmail {
					__typename
					id
					enabled
					priority
					header
					recipients {
						nodes {
							... on User {
								username
							}
							... on Org {
								name
							}
						}
					}
				}
				... on MonitorWebhook {
					__typename
					enabled
					url
				}
				... on MonitorSlackWebhook {
					__typename
					enabled
					url
				}
			}
		}
	}
}
`

func recipientPaging(ctx context.Context, t *testing.T, schema *graphql.Schema, user1 *types.User, user2 *types.User) {
	queryInput := map[string]any{
		"userName":        user1.Username,
		"recipientCursor": string(relay.MarshalID(monitorActionEmailRecipientKind, 1)),
	}
	got := apitest.Response{}
	batchesApitest.MustExec(ctx, t, schema, queryInput, &got, recipientsPagingFmtStr)

	want := apitest.Response{
		User: apitest.User{
			Monitors: apitest.MonitorConnection{
				TotalCount: 2,
				Nodes: []apitest.Monitor{{
					Actions: apitest.ActionConnection{
						Nodes: []apitest.Action{{
							Email: &apitest.ActionEmail{
								Recipients: apitest.RecipientsConnection{
									TotalCount: 2,
									Nodes: []apitest.UserOrg{{
										Name: user2.Username,
									}},
								},
							},
						}},
					},
				}},
			},
		},
	}

	require.Equal(t, want, got)
}

const recipientsPagingFmtStr = `
fragment u on User { id, username }
fragment o on Org { id, name }

query($userName: String!, $recipientCursor: String!){
	user(username:$userName){
		monitors(first:1){
			totalCount
			nodes{
				actions(first:1){
					nodes{
						... on MonitorEmail{
							__typename
							recipients(first:1, after:$recipientCursor){
								totalCount
								nodes {
									... on User { ...u }
									... on Org { ...o }
								}
							}
						}
					}
				}
			}
		}
	}
}
`

func queryByID(ctx context.Context, t *testing.T, schema *graphql.Schema, r *Resolver, m *monitor, user1 *types.User, user2 *types.User) {
	input := map[string]any{
		"id": m.ID(),
	}
	response := apitest.Node{}
	batchesApitest.MustExec(ctx, t, schema, input, &response, queryMonitorByIDFmtStr)

	want := apitest.Node{
		Node: apitest.Monitor{
			Id:          string(relay.MarshalID(MonitorKind, 1)),
			Description: "test monitor",
			Enabled:     true,
			Owner:       apitest.UserOrg{Name: user1.Username},
			CreatedBy:   apitest.UserOrg{Name: user1.Username},
			CreatedAt:   marshalDateTime(t, r.Now()),
			Trigger: apitest.Trigger{
				Id:    string(relay.MarshalID(monitorTriggerQueryKind, 1)),
				Query: "repo:foo",
			},
			Actions: apitest.ActionConnection{
				TotalCount: 4,
				Nodes: []apitest.Action{
					{
						Email: &apitest.ActionEmail{
							Id:       string(relay.MarshalID(monitorActionEmailKind, 1)),
							Enabled:  false,
							Priority: "NORMAL",
							Recipients: apitest.RecipientsConnection{
								TotalCount: 2,
								Nodes: []apitest.UserOrg{
									{
										Name: user1.Username,
									},
									{
										Name: user2.Username,
									},
								},
							},
							Header: "test header 1",
						},
					},
					{
						Email: &apitest.ActionEmail{
							Id:       string(relay.MarshalID(monitorActionEmailKind, 2)),
							Enabled:  true,
							Priority: "CRITICAL",
							Recipients: apitest.RecipientsConnection{
								TotalCount: 2,
								Nodes: []apitest.UserOrg{
									{
										Name: user1.Username,
									},
									{
										Name: user2.Username,
									},
								},
							},
							Header: "test header 2",
						},
					},
					{
						Webhook: &apitest.ActionWebhook{
							Id:      string(relay.MarshalID(monitorActionWebhookKind, 1)),
							Enabled: true,
							URL:     "https://generic.webhook.com",
						},
					},
					{
						SlackWebhook: &apitest.ActionSlackWebhook{
							Id:      string(relay.MarshalID(monitorActionSlackWebhookKind, 1)),
							Enabled: true,
							URL:     "https://slack.webhook.com",
						},
					},
				},
			},
		},
	}

	require.Equal(t, want, response)
}

const queryMonitorByIDFmtStr = `
fragment u on User { id, username }
fragment o on Org { id, name }

query ($id: ID!) {
	node(id: $id) {
		... on Monitor {
			__typename
			id
			description
			enabled
			owner {
				... on User {
					...u
				}
				... on Org {
					...o
				}
			}
			createdBy {
				...u
			}
			createdAt
			trigger {
				... on MonitorQuery {
					__typename
					id
					query
				}
			}
			actions {
				totalCount
				nodes {
					... on MonitorEmail {
						__typename
						id
						priority
						header
						enabled
						recipients {
							totalCount
							nodes {
								... on User {
									...u
								}
								... on Org {
									...o
								}
							}
						}
					}
					... on MonitorWebhook {
						__typename
						id
						enabled
						url
					}
					... on MonitorSlackWebhook {
						__typename
						id
						enabled
						url
					}
				}
			}
		}
	}
}
`

func monitorPaging(ctx context.Context, t *testing.T, schema *graphql.Schema, user1 *types.User) {
	queryInput := map[string]any{
		"userName":      user1.Username,
		"monitorCursor": string(relay.MarshalID(MonitorKind, 1)),
	}
	got := apitest.Response{}
	batchesApitest.MustExec(ctx, t, schema, queryInput, &got, monitorPagingFmtStr)

	want := apitest.Response{
		User: apitest.User{
			Monitors: apitest.MonitorConnection{
				TotalCount: 2,
				Nodes: []apitest.Monitor{{
					Id: string(relay.MarshalID(MonitorKind, 2)),
				}},
			},
		},
	}

	require.Equal(t, want, got)
}

const monitorPagingFmtStr = `
query($userName: String!, $monitorCursor: String!){
	user(username:$userName){
		monitors(first:1, after:$monitorCursor){
			totalCount
			nodes{
				id
			}
		}
	}
}
`

func actionPaging(ctx context.Context, t *testing.T, schema *graphql.Schema, user1 *types.User) {
	queryInput := map[string]any{
		"userName":     user1.Username,
		"actionCursor": string(relay.MarshalID(monitorActionEmailKind, 1)),
	}
	got := apitest.Response{}
	batchesApitest.MustExec(ctx, t, schema, queryInput, &got, actionPagingFmtStr)

	want := apitest.Response{
		User: apitest.User{
			Monitors: apitest.MonitorConnection{
				Nodes: []apitest.Monitor{{
					Actions: apitest.ActionConnection{
						TotalCount: 4,
						Nodes: []apitest.Action{
							{
								Email: &apitest.ActionEmail{
									Id: string(relay.MarshalID(monitorActionEmailKind, 2)),
								},
							},
						},
					},
				}},
			},
		},
	}

	require.Equal(t, want, got)
}

const actionPagingFmtStr = `
query($userName: String!, $actionCursor:String!){
	user(username:$userName){
		monitors(first:1){
			nodes{
				actions(first:1, after:$actionCursor) {
					totalCount
					nodes {
						... on MonitorEmail {
							__typename
							id
						}
					}
				}
			}
		}
	}
}
`

func triggerEventPaging(ctx context.Context, t *testing.T, schema *graphql.Schema, user1 *types.User) {
	queryInput := map[string]any{
		"userName":           user1.Username,
		"triggerEventCursor": relay.MarshalID(monitorTriggerEventKind, 1),
	}
	got := apitest.Response{}
	batchesApitest.MustExec(ctx, t, schema, queryInput, &got, triggerEventPagingFmtStr)

	want := apitest.Response{
		User: apitest.User{
			Monitors: apitest.MonitorConnection{
				Nodes: []apitest.Monitor{{
					Trigger: apitest.Trigger{
						Events: apitest.TriggerEventConnection{
							TotalCount: 2,
							Nodes: []apitest.TriggerEvent{
								{
									Id: string(relay.MarshalID(monitorTriggerEventKind, 3)),
								},
							},
						},
					},
				}},
			},
		},
	}

	require.Equal(t, want, got)
}

const triggerEventPagingFmtStr = `
query($userName: String!, $triggerEventCursor: String!){
	user(username:$userName){
		monitors(first:1){
			nodes{
				trigger {
					... on MonitorQuery {
						__typename
						events(first:1, after:$triggerEventCursor) {
							totalCount
							nodes {
								id
							}
						}
					}
				}
			}
		}
	}
}
`

func actionEventPaging(ctx context.Context, t *testing.T, schema *graphql.Schema, user1 *types.User) {
	queryInput := map[string]any{
		"userName":          user1.Username,
		"actionCursor":      string(relay.MarshalID(monitorActionEmailKind, 1)),
		"actionEventCursor": relay.MarshalID(monitorActionEmailEventKind, 1),
	}
	got := apitest.Response{}
	batchesApitest.MustExec(ctx, t, schema, queryInput, &got, actionEventPagingFmtStr)

	want := apitest.Response{
		User: apitest.User{
			Monitors: apitest.MonitorConnection{
				Nodes: []apitest.Monitor{{
					Actions: apitest.ActionConnection{
						TotalCount: 4,
						Nodes: []apitest.Action{
							{
								Email: &apitest.ActionEmail{
									Id: string(relay.MarshalID(monitorActionEmailKind, 2)),
									Events: apitest.ActionEventConnection{
										TotalCount: 2,
										Nodes: []apitest.ActionEvent{
											{
												Id: string(relay.MarshalID(monitorActionEmailEventKind, 4)),
											},
										},
									},
								},
							},
						},
					},
				}},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatal(diff)
	}
}

const actionEventPagingFmtStr = `
query($userName: String!, $actionCursor:String!, $actionEventCursor:String!){
	user(username:$userName){
		monitors(first:1){
			nodes{
				actions(first:1, after:$actionCursor) {
					totalCount
					nodes {
						... on MonitorEmail {
							__typename
							id
							events(first:1, after:$actionEventCursor) {
								totalCount
								nodes {
									id
								}
							}
						}
					}
				}
			}
		}
	}
}
`

func TestTriggerTestEmailAction(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)

	got := background.TemplateDataNewSearchResults{}
	background.MockSendEmailForNewSearchResult = func(ctx context.Context, db database.DB, userID int32, data *background.TemplateDataNewSearchResults) error {
		got = *data
		return nil
	}

	ctx := actor.WithInternalActor(context.Background())
	db := database.NewDB(logger, dbtest.NewDB(t))
	r := newTestResolver(t, db)

	namespaceID := relay.MarshalID("User", actor.FromContext(ctx).UID)

	_, err := r.TriggerTestEmailAction(ctx, &graphqlbackend.TriggerTestEmailActionArgs{
		Namespace:   namespaceID,
		Description: "A code monitor name",
		Email: &graphqlbackend.CreateActionEmailArgs{
			Enabled:    true,
			Priority:   "NORMAL",
			Recipients: []graphql.ID{namespaceID},
			Header:     "test header 1",
		},
	})
	require.NoError(t, err)
	require.True(t, got.IsTest, "Template data for testing email actions should have with .IsTest=true")
}

func TestMonitorKindEqualsResolvers(t *testing.T) {
	got := background.MonitorKind
	want := MonitorKind

	if got != want {
		t.Fatal("email.MonitorKind should match resolvers.MonitorKind")
	}
}

func TestValidateSlackURL(t *testing.T) {
	valid := []string{
		"https://hooks.slack.com/services/8d8d8/8dd88d/838383",
		"https://hooks.slack.com",
	}

	for _, url := range valid {
		require.NoError(t, validateSlackURL(url))
	}

	invalid := []string{
		"http://hooks.slack.com/services",
		"https://hooks.slack.com:3443/services",
		"https://internal:8989",
	}

	for _, url := range invalid {
		require.Error(t, validateSlackURL(url))
	}
}
