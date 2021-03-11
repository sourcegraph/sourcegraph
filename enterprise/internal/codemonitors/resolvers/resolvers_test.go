package resolvers

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	batchesApitest "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/resolvers/apitest"
	cm "github.com/sourcegraph/sourcegraph/enterprise/internal/codemonitors"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codemonitors/email"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codemonitors/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codemonitors/storetest"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
)

func init() {
	dbtesting.DBNameSuffix = "codemonitorsdb"
}

func TestCreateCodeMonitor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	dbtesting.SetupGlobalTestDB(t)
	r := newTestResolver(t)

	userID := insertTestUser(t, dbconn.Global, "cm-user1", true)

	want := &cm.Monitor{
		ID:              1,
		CreatedBy:       userID,
		CreatedAt:       r.Now(),
		ChangedBy:       userID,
		ChangedAt:       r.Now(),
		Description:     "test monitor",
		Enabled:         true,
		NamespaceUserID: &userID,
		NamespaceOrgID:  nil,
	}

	// Create a monitor.
	ctx = actor.WithActor(ctx, actor.FromUser(userID))
	got, err := r.insertTestMonitorWithOpts(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got.(*monitor).Monitor); diff != "" {
		t.Error(diff)
	}

	// Toggle field enabled from true to false.
	got, err = r.ToggleCodeMonitor(ctx, &graphqlbackend.ToggleCodeMonitorArgs{
		Id:      relay.MarshalID(MonitorKind, got.(*monitor).Monitor.ID),
		Enabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.(*monitor).Monitor.Enabled {
		t.Fatalf("got enabled=%T, want enabled=%T", got.(*monitor).Monitor.Enabled, false)
	}

	// Delete code monitor.
	_, err = r.DeleteCodeMonitor(ctx, &graphqlbackend.DeleteCodeMonitorArgs{Id: got.ID()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.store.MonitorByIDInt64(ctx, got.(*monitor).Monitor.ID)
	if err == nil {
		t.Fatalf("monitor should have been deleted")
	}
}

func TestListCodeMonitors(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	dbtesting.SetupGlobalTestDB(t)
	r := newTestResolver(t)

	userID := insertTestUser(t, dbconn.Global, "cm-user1", true)
	ctx = actor.WithActor(ctx, actor.FromUser(userID))

	// Create a monitor.
	_, err := r.insertTestMonitorWithOpts(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	args := &graphqlbackend.ListMonitorsArgs{
		First: 5,
	}
	r1, err := r.Monitors(ctx, userID, args)
	if err != nil {
		t.Fatal(err)
	}

	requireNodeCount(t, r1, 1)
	requireHasNextPage(t, r1, false)

	// Create enough monitors to necessitate paging
	for i := 0; i < 10; i++ {
		_, err := r.insertTestMonitorWithOpts(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
	}

	r2, err := r.Monitors(ctx, userID, args)
	if err != nil {
		t.Fatal(err)
	}

	requireNodeCount(t, r2, 5)
	requireHasNextPage(t, r2, true)

	// The returned cursor should be usable to return the remaining monitors
	pi, err := r2.PageInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	args = &graphqlbackend.ListMonitorsArgs{
		First: 10,
		After: pi.EndCursor(),
	}
	r3, err := r.Monitors(ctx, userID, args)
	if err != nil {
		t.Fatal(err)
	}

	requireNodeCount(t, r3, 6)
	requireHasNextPage(t, r3, false)
}

func requireNodeCount(t *testing.T, r graphqlbackend.MonitorConnectionResolver, c int) {
	t.Helper()

	nodes, err := r.Nodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(nodes) != c {
		t.Fatalf("got %d nodes but expected %d", len(nodes), c)
	}
}

func requireHasNextPage(t *testing.T, r graphqlbackend.MonitorConnectionResolver, hasNextPage bool) {
	t.Helper()

	pageInfo, err := r.PageInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if pageInfo.HasNextPage() != hasNextPage {
		t.Fatalf("unexpected value for HasNextPage")
	}
}

func TestIsAllowedToEdit(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	dbtesting.SetupGlobalTestDB(t)

	// Setup users and org
	member := insertTestUser(t, dbconn.Global, "cm-user1", false)
	notMember := insertTestUser(t, dbconn.Global, "cm-user2", false)
	siteAdmin := insertTestUser(t, dbconn.Global, "cm-user3", true)

	admContext := actor.WithActor(context.Background(), actor.FromUser(siteAdmin))
	org, err := database.GlobalOrgs.Create(admContext, "cm-test-org", nil)
	if err != nil {
		t.Fatal(err)
	}
	addUserToOrg(t, dbconn.Global, member, org.ID)

	r := newTestResolver(t)

	// Create a monitor and set org as owner.
	ownerOpt := WithOwner(relay.MarshalID("Org", org.ID))
	m, err := r.insertTestMonitorWithOpts(admContext, t, ownerOpt)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		user    int32
		allowed bool
	}{
		{
			user:    member,
			allowed: true,
		},
		{
			user:    notMember,
			allowed: false,
		},
		{
			user:    siteAdmin,
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
}

func TestIsAllowedToCreate(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	dbtesting.SetupGlobalTestDB(t)

	// Setup users and org
	member := insertTestUser(t, dbconn.Global, "cm-user1", false)
	notMember := insertTestUser(t, dbconn.Global, "cm-user2", false)
	siteAdmin := insertTestUser(t, dbconn.Global, "cm-user3", true)

	admContext := actor.WithActor(context.Background(), actor.FromUser(siteAdmin))
	org, err := database.GlobalOrgs.Create(admContext, "cm-test-org", nil)
	if err != nil {
		t.Fatal(err)
	}
	addUserToOrg(t, dbconn.Global, member, org.ID)

	r := newTestResolver(t)

	tests := []struct {
		user    int32
		owner   graphql.ID
		allowed bool
	}{
		{
			user:    member,
			owner:   relay.MarshalID("Org", org.ID),
			allowed: true,
		},
		{
			user:    member,
			owner:   relay.MarshalID("User", notMember),
			allowed: false,
		},

		{
			user:    notMember,
			owner:   relay.MarshalID("Org", org.ID),
			allowed: false,
		},
		{
			user:    siteAdmin,
			owner:   relay.MarshalID("Org", org.ID),
			allowed: true,
		},
		{
			user:    siteAdmin,
			owner:   relay.MarshalID("User", member),
			allowed: true,
		},
		{
			user:    siteAdmin,
			owner:   relay.MarshalID("User", notMember),
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

type testUser struct {
	name    string
	idInt32 int32
}

func (u *testUser) id() graphql.ID {
	return relay.MarshalID("User", u.idInt32)
}

func TestQueryMonitor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	dbtesting.SetupGlobalTestDB(t)
	r := newTestResolver(t)

	// Create 2 test users.
	user1 := &testUser{name: "cm-user1"}
	user1.idInt32 = insertTestUser(t, dbconn.Global, user1.name, true)
	user2 := &testUser{name: "cm-user2"}
	user2.idInt32 = insertTestUser(t, dbconn.Global, user2.name, true)

	// Create 2 code monitors, each with 1 trigger, 2 actions and two recipients per action.
	ctx = actor.WithActor(ctx, actor.FromUser(user1.idInt32))
	actionOpt := WithActions([]*graphqlbackend.CreateActionArgs{
		{
			Email: &graphqlbackend.CreateActionEmailArgs{
				Enabled:    false,
				Priority:   "NORMAL",
				Recipients: []graphql.ID{user1.id(), user2.id()},
				Header:     "test header 1",
			},
		},
		{
			Email: &graphqlbackend.CreateActionEmailArgs{
				Enabled:    true,
				Priority:   "CRITICAL",
				Recipients: []graphql.ID{user1.id(), user2.id()},
				Header:     "test header 2",
			},
		},
	})
	var err error
	var m graphqlbackend.MonitorResolver
	m, err = r.insertTestMonitorWithOpts(ctx, t, actionOpt)
	if err != nil {
		t.Fatal(err)
	}
	// The hooks allows us to test more complex queries by creating a realistic state
	// in the database. After we create the monitor they fill the job tables and
	// update the job status.
	postHookOpt := WithPostHooks([]hook{
		func() error { return r.store.EnqueueTriggerQueries(ctx) },
		func() error { return r.store.EnqueueActionEmailsForQueryIDInt64(ctx, 1, 1) },
		func() error {
			return (&storetest.TestStore{Store: r.store}).SetJobStatus(ctx, storetest.ActionJobs, storetest.Completed, 1)
		},
		func() error { return r.store.EnqueueActionEmailsForQueryIDInt64(ctx, 1, 1) },
		// Set the job status of trigger job with id = 1 to "completed". Since we already
		// created another monitor, there is still a second trigger job (id = 2) which
		// remains in status queued.
		//
		// -- cm_trigger_jobs --
		// id  query state
		// 1   1     completed
		// 2   2     queued
		func() error {
			return (&storetest.TestStore{Store: r.store}).SetJobStatus(ctx, storetest.TriggerJobs, storetest.Completed, 1)
		},
		// This will create a second trigger job (id = 3) for the first monitor. Since
		// the job with id = 2 is still queued, no new job will be enqueued for query 2.
		//
		// -- cm_trigger_jobs --
		// id  query state
		// 1   1     completed
		// 2   2     queued
		// 3   1	 queued
		func() error { return r.store.EnqueueTriggerQueries(ctx) },
		// To have a consistent state we have to log the number of search results for
		// each completed trigger job.
		func() error { return r.store.LogSearch(ctx, "", 1, 1) },
	})
	_, err = r.insertTestMonitorWithOpts(ctx, t, actionOpt, postHookOpt)
	if err != nil {
		t.Fatal(err)
	}

	schema, err := graphqlbackend.NewSchema(dbconn.Global, nil, nil, nil, nil, r, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("query by user", func(t *testing.T) {
		queryByUser(ctx, t, schema, r, user1, user2)
	})
	t.Run("query by ID", func(t *testing.T) {
		queryByID(ctx, t, schema, r, m.(*monitor), user1, user2)
	})
	t.Run("monitor paging", func(t *testing.T) {
		monitorPaging(ctx, t, schema, user1)
	})
	t.Run("recipients paging", func(t *testing.T) {
		recipientPaging(ctx, t, schema, user1, user2)
	})
	t.Run("actions paging", func(t *testing.T) {
		actionPaging(ctx, t, schema, user1)
	})
	t.Run("trigger events paging", func(t *testing.T) {
		triggerEventPaging(ctx, t, schema, user1)
	})
	t.Run("action events paging", func(t *testing.T) {
		actionEventPaging(ctx, t, schema, user1)
	})
}

func queryByUser(ctx context.Context, t *testing.T, schema *graphql.Schema, r *Resolver, user1 *testUser, user2 *testUser) {
	input := map[string]interface{}{
		"userName":     user1.name,
		"actionCursor": relay.MarshalID(monitorActionEventKind, 1),
	}
	response := apitest.Response{}
	batchesApitest.MustExec(ctx, t, schema, input, &response, queryByUserFmtStr)

	triggerEventEndCursor := string(relay.MarshalID(monitorTriggerEventKind, 1))
	actionEventEndCursor := string(relay.MarshalID(monitorActionEventKind, 2))
	want := apitest.Response{
		User: apitest.User{
			Monitors: apitest.MonitorConnection{
				TotalCount: 2,
				Nodes: []apitest.Monitor{{
					Id:          string(relay.MarshalID(MonitorKind, 1)),
					Description: "test monitor",
					Enabled:     true,
					Owner:       apitest.UserOrg{Name: user1.name},
					CreatedBy:   apitest.UserOrg{Name: user1.name},
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
						TotalCount: 2,
						Nodes: []apitest.Action{
							{
								ActionEmail: apitest.ActionEmail{
									Id:       string(relay.MarshalID(monitorActionEmailKind, 2)),
									Enabled:  true,
									Priority: "CRITICAL",
									Recipients: apitest.RecipientsConnection{
										TotalCount: 2,
										Nodes: []apitest.UserOrg{
											{Name: user1.name},
											{Name: user2.name},
										},
									},
									Header: "test header 2",
									Events: apitest.ActionEventConnection{
										Nodes: []apitest.ActionEvent{
											{
												Id:        string(relay.MarshalID(monitorActionEventKind, 1)),
												Status:    "SUCCESS",
												Timestamp: r.Now().UTC().Format(time.RFC3339),
												Message:   nil,
											},
											{
												Id:        string(relay.MarshalID(monitorActionEventKind, 2)),
												Status:    "PENDING",
												Timestamp: r.Now().UTC().Format(time.RFC3339),
												Message:   nil,
											},
										},
										TotalCount: 2,
										PageInfo: apitest.PageInfo{
											HasNextPage: true,
											EndCursor:   &actionEventEndCursor,
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
	if diff := cmp.Diff(response, want); diff != "" {
		t.Fatalf("diff: %s", diff)
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
				actions(first:1, after:$actionCursor){
					totalCount
					nodes{
						... on MonitorEmail{
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
					}
				}
			}
		}
	}
}
`

func TestEditCodeMonitor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	dbtesting.SetupGlobalTestDB(t)
	r := newTestResolver(t)

	// Create 2 test users.
	user1Name := "cm-user1"
	user1ID := insertTestUser(t, dbconn.Global, user1Name, true)
	ns1 := relay.MarshalID("User", user1ID)

	user2Name := "cm-user2"
	user2ID := insertTestUser(t, dbconn.Global, user2Name, true)
	ns2 := relay.MarshalID("User", user2ID)

	// Create a code monitor with 1 trigger and 2 actions.
	ctx = actor.WithActor(ctx, actor.FromUser(user1ID))
	actionOpt := WithActions([]*graphqlbackend.CreateActionArgs{
		{
			Email: &graphqlbackend.CreateActionEmailArgs{
				Enabled:    true,
				Priority:   "NORMAL",
				Recipients: []graphql.ID{ns1},
				Header:     "header action 1",
			}},
		{
			Email: &graphqlbackend.CreateActionEmailArgs{
				Enabled:    true,
				Priority:   "NORMAL",
				Recipients: []graphql.ID{ns1, ns2},
				Header:     "header action 2",
			},
		},
	})
	_, err := r.insertTestMonitorWithOpts(ctx, t, actionOpt)
	if err != nil {
		t.Fatal(err)
	}

	// Update the code monitor.
	// We update all fields, delete one action, and add a new action.
	schema, err := graphqlbackend.NewSchema(dbconn.Global, nil, nil, nil, nil, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	updateInput := map[string]interface{}{
		"monitorID": string(relay.MarshalID(MonitorKind, 1)),
		"triggerID": string(relay.MarshalID(monitorTriggerQueryKind, 1)),
		"actionID":  string(relay.MarshalID(monitorActionEmailKind, 1)),
		"user1ID":   ns1,
		"user2ID":   ns2,
	}
	got := apitest.UpdateCodeMonitorResponse{}
	batchesApitest.MustExec(ctx, t, schema, updateInput, &got, editMonitor)

	want := apitest.UpdateCodeMonitorResponse{
		UpdateCodeMonitor: apitest.Monitor{
			Id:          string(relay.MarshalID(MonitorKind, 1)),
			Description: "updated test monitor",
			Enabled:     false,
			Owner: apitest.UserOrg{
				Name: user1Name,
			},
			CreatedBy: apitest.UserOrg{
				Name: user1Name,
			},
			CreatedAt: marshalDateTime(t, r.store.Now()),
			Trigger: apitest.Trigger{
				Id:    string(relay.MarshalID(monitorTriggerQueryKind, 1)),
				Query: "repo:bar",
			},
			Actions: apitest.ActionConnection{
				Nodes: []apitest.Action{{
					ActionEmail: apitest.ActionEmail{
						Id:       string(relay.MarshalID(monitorActionEmailKind, 1)),
						Enabled:  false,
						Priority: "CRITICAL",
						Recipients: apitest.RecipientsConnection{
							Nodes: []apitest.UserOrg{
								{
									Name: user2Name,
								},
							},
						},
						Header: "updated header action 1",
					}}, {
					ActionEmail: apitest.ActionEmail{
						Id:       string(relay.MarshalID(monitorActionEmailKind, 3)),
						Enabled:  true,
						Priority: "NORMAL",
						Recipients: apitest.RecipientsConnection{
							Nodes: []apitest.UserOrg{
								{
									Name: user1Name,
								},
								{
									Name: user2Name,
								},
							},
						},
						Header: "header action 3",
					}},
				},
			},
		}}

	if !reflect.DeepEqual(&got, &want) {
		t.Fatalf("\ngot:\t%+v\nwant:\t%+v\n", got, want)
	}
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

mutation ($monitorID: ID!, $triggerID: ID!, $actionID: ID!, $user1ID: ID!, $user2ID: ID!) {
  updateCodeMonitor(
    monitor: {id: $monitorID, update: {description: "updated test monitor", enabled: false, namespace: $user1ID}},
	trigger: {id: $triggerID, update: {query: "repo:bar"}},
	actions: [
	  {email: {id: $actionID, update: {enabled: false, priority: CRITICAL, recipients: [$user2ID], header: "updated header action 1"}}}
	  {email: {update: {enabled: true, priority: NORMAL, recipients: [$user1ID, $user2ID], header: "header action 3"}}}
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
		id
		query
	  }
	}
	actions {
	  nodes {
		... on MonitorEmail {
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
	  }
	}
  }
}
`

func recipientPaging(ctx context.Context, t *testing.T, schema *graphql.Schema, user1 *testUser, user2 *testUser) {
	queryInput := map[string]interface{}{
		"userName":        user1.name,
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
							ActionEmail: apitest.ActionEmail{
								Recipients: apitest.RecipientsConnection{
									TotalCount: 2,
									Nodes: []apitest.UserOrg{{
										Name: user2.name,
									}},
								},
							},
						}},
					},
				}},
			},
		},
	}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatalf("diff: %s", diff)
	}
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

func queryByID(ctx context.Context, t *testing.T, schema *graphql.Schema, r *Resolver, m *monitor, user1 *testUser, user2 *testUser) {
	input := map[string]interface{}{
		"id": m.ID(),
	}
	response := apitest.Node{}
	batchesApitest.MustExec(ctx, t, schema, input, &response, queryMonitorByIDFmtStr)

	want := apitest.Node{
		Node: apitest.Monitor{
			Id:          string(relay.MarshalID(MonitorKind, 1)),
			Description: "test monitor",
			Enabled:     true,
			Owner:       apitest.UserOrg{Name: user1.name},
			CreatedBy:   apitest.UserOrg{Name: user1.name},
			CreatedAt:   marshalDateTime(t, r.Now()),
			Trigger: apitest.Trigger{
				Id:    string(relay.MarshalID(monitorTriggerQueryKind, 1)),
				Query: "repo:foo",
			},
			Actions: apitest.ActionConnection{
				TotalCount: 2,
				Nodes: []apitest.Action{
					{
						ActionEmail: apitest.ActionEmail{
							Id:       string(relay.MarshalID(monitorActionEmailKind, 1)),
							Enabled:  false,
							Priority: "NORMAL",
							Recipients: apitest.RecipientsConnection{
								TotalCount: 2,
								Nodes: []apitest.UserOrg{
									{
										Name: user1.name,
									},
									{
										Name: user2.name,
									},
								},
							},
							Header: "test header 1",
						},
					},
					{
						ActionEmail: apitest.ActionEmail{
							Id:       string(relay.MarshalID(monitorActionEmailKind, 2)),
							Enabled:  true,
							Priority: "CRITICAL",
							Recipients: apitest.RecipientsConnection{
								TotalCount: 2,
								Nodes: []apitest.UserOrg{
									{
										Name: user1.name,
									},
									{
										Name: user2.name,
									},
								},
							},
							Header: "test header 2",
						},
					},
				},
			},
		},
	}
	if diff := cmp.Diff(response, want); diff != "" {
		t.Fatalf("diff: %s", diff)
	}
}

const queryMonitorByIDFmtStr = `
fragment u on User { id, username }
fragment o on Org { id, name }

query ($id: ID!) {
  node(id: $id) {
    ... on Monitor {
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
          id
          query
        }
      }
      actions {
        totalCount
        nodes {
          ... on MonitorEmail {
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
        }
      }
    }
  }
}
`

func monitorPaging(ctx context.Context, t *testing.T, schema *graphql.Schema, user1 *testUser) {
	queryInput := map[string]interface{}{
		"userName":      user1.name,
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

	if diff := cmp.Diff(&got, &want); diff != "" {
		t.Fatalf("diff: %s", diff)
	}
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

func actionPaging(ctx context.Context, t *testing.T, schema *graphql.Schema, user1 *testUser) {
	queryInput := map[string]interface{}{
		"userName":     user1.name,
		"actionCursor": string(relay.MarshalID(monitorActionEmailKind, 1)),
	}
	got := apitest.Response{}
	batchesApitest.MustExec(ctx, t, schema, queryInput, &got, actionPagingFmtStr)

	want := apitest.Response{
		User: apitest.User{
			Monitors: apitest.MonitorConnection{
				Nodes: []apitest.Monitor{{
					Actions: apitest.ActionConnection{
						TotalCount: 2,
						Nodes: []apitest.Action{
							{
								ActionEmail: apitest.ActionEmail{
									Id: string(relay.MarshalID(monitorActionEmailKind, 2)),
								},
							},
						},
					},
				}},
			},
		},
	}

	if diff := cmp.Diff(&got, &want); diff != "" {
		t.Fatalf("diff: %s", diff)
	}
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
							id
						}
					}
				}
			}
		}
	}
}
`

func triggerEventPaging(ctx context.Context, t *testing.T, schema *graphql.Schema, user1 *testUser) {
	queryInput := map[string]interface{}{
		"userName":           user1.name,
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

	if diff := cmp.Diff(&got, &want); diff != "" {
		t.Fatalf("diff: %s", diff)
	}
}

const triggerEventPagingFmtStr = `
query($userName: String!, $triggerEventCursor: String!){
	user(username:$userName){
		monitors(first:1){
			nodes{
				trigger {
					... on MonitorQuery {
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

func actionEventPaging(ctx context.Context, t *testing.T, schema *graphql.Schema, user1 *testUser) {
	queryInput := map[string]interface{}{
		"userName":          user1.name,
		"actionCursor":      string(relay.MarshalID(monitorActionEmailKind, 1)),
		"actionEventCursor": relay.MarshalID(monitorActionEventKind, 1),
	}
	got := apitest.Response{}
	batchesApitest.MustExec(ctx, t, schema, queryInput, &got, actionEventPagingFmtStr)

	want := apitest.Response{
		User: apitest.User{
			Monitors: apitest.MonitorConnection{
				Nodes: []apitest.Monitor{{
					Actions: apitest.ActionConnection{
						TotalCount: 2,
						Nodes: []apitest.Action{
							{
								ActionEmail: apitest.ActionEmail{
									Id: string(relay.MarshalID(monitorActionEmailKind, 2)),
									Events: apitest.ActionEventConnection{
										TotalCount: 2,
										Nodes: []apitest.ActionEvent{
											{
												Id: string(relay.MarshalID(monitorActionEventKind, 2)),
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

	if diff := cmp.Diff(&got, &want); diff != "" {
		t.Fatalf("diff: %s", diff)
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

	got := email.TemplateDataNewSearchResults{}
	email.MockSendEmailForNewSearchResult = func(ctx context.Context, userID int32, data *email.TemplateDataNewSearchResults) error {
		got = *data
		return nil
	}

	ctx := backend.WithAuthzBypass(context.Background())
	r := newTestResolver(t)

	userID := 1
	namespaceID := relay.MarshalID("User", actor.FromContext(ctx).UID)

	ctx = actor.WithActor(ctx, actor.FromUser(int32(userID)))
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

	if err != nil {
		t.Fatal(err)
	}

	if !got.IsTest {
		t.Fatalf("Template data for testing email actions should have with .IsTest=true")
	}
}

func TestMonitorKindEqualsResolvers(t *testing.T) {
	got := email.MonitorKind
	want := MonitorKind

	if got != want {
		t.Fatal("email.MonitorKind should match resolvers.MonitorKind")
	}
}
